// Package config implements comprehensive configuration management for the Universal Application Console.
// This implementation handles profile loading, theme management, application registry, and secure credential storage
// according to the specifications in section 3.5 of the design document.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/universal-console/console/internal/errors"
	"github.com/universal-console/console/internal/interfaces"
	"github.com/universal-console/console/internal/logging"
	"gopkg.in/yaml.v3"
)

// Config represents the complete configuration file structure
type Config struct {
	Profiles       map[string]interfaces.Profile `yaml:"profiles"`
	Themes         map[string]interfaces.Theme   `yaml:"themes"`
	RegisteredApps []interfaces.RegisteredApp    `yaml:"registered_apps"`
}

// Manager implements the ConfigManager interface with comprehensive configuration handling
type Manager struct {
	configPath   string
	securityMgr  SecurityManager
	cachedConfig *Config
	logger       *logging.Logger
}

// NewManager creates a new configuration manager with OS-appropriate paths and security setup
func NewManager() (*Manager, error) {
	logger := logging.GetConfigLogger()
	
	logger.Debug("Initializing configuration manager")
	
	configPath, err := getConfigPath()
	if err != nil {
		logger.Error("Failed to determine configuration path", "error", err.Error())
		return nil, errors.NewConfigurationError("config").
			WithMessage("Failed to determine configuration path").
			WithUserMessage("Unable to locate configuration directory. Please check system permissions.").
			WithOperation("get_config_path").
			WithCause(err).
			Build()
	}
	
	logger.Debug("Configuration path determined", "path", configPath)

	securityMgr, err := NewSecurityManager()
	if err != nil {
		logger.Error("Failed to initialize security manager", "error", err.Error())
		return nil, errors.NewConfigurationError("config").
			WithMessage("Failed to initialize security manager").
			WithUserMessage("Unable to set up secure credential storage. Please check system permissions.").
			WithOperation("init_security_manager").
			WithCause(err).
			Build()
	}
	
	logger.Debug("Security manager initialized successfully")

	manager := &Manager{
		configPath:  configPath,
		securityMgr: securityMgr,
		logger:      logger,
	}

	// Ensure configuration directory exists with appropriate permissions
	if err := manager.ensureConfigDirectory(); err != nil {
		logger.Error("Failed to create configuration directory", "error", err.Error())
		return nil, errors.NewConfigurationError("config").
			WithMessage("Failed to create configuration directory").
			WithUserMessage("Cannot create configuration directory. Please check permissions.").
			WithOperation("ensure_config_directory").
			WithCause(err).
			WithContext("path", configPath).
			Build()
	}
	
	logger.Info("Configuration manager initialized successfully", "config_path", configPath)

	return manager, nil
}

// getConfigPath determines the OS-appropriate configuration file path
func getConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine home directory: %w", err)
	}

	// Use OS-appropriate configuration directory
	var configDir string
	if xdgConfigHome := os.Getenv("XDG_CONFIG_HOME"); xdgConfigHome != "" {
		configDir = filepath.Join(xdgConfigHome, "console")
	} else {
		configDir = filepath.Join(homeDir, ".config", "console")
	}

	return filepath.Join(configDir, "profiles.yaml"), nil
}

// ensureConfigDirectory creates the configuration directory with secure permissions
func (m *Manager) ensureConfigDirectory() error {
	configDir := filepath.Dir(m.configPath)

	// Create directory with restrictive permissions (readable/writable by owner only)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory %s: %w", configDir, err)
	}

	return nil
}

// loadConfig reads and parses the configuration file, creating defaults if necessary
func (m *Manager) loadConfig() (*Config, error) {
	m.logger.Debug("Loading configuration")
	
	// Return cached configuration if available
	if m.cachedConfig != nil {
		m.logger.Debug("Using cached configuration")
		return m.cachedConfig, nil
	}

	// Check if configuration file exists
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		m.logger.Info("Configuration file not found, creating default configuration", "path", m.configPath)
		// Create default configuration if file doesn't exist
		config := m.createDefaultConfig()
		if err := m.saveConfig(config); err != nil {
			m.logger.Error("Failed to create default configuration", "error", err.Error())
			return nil, errors.NewConfigurationError("config").
				WithMessage("Failed to create default configuration").
				WithUserMessage("Cannot create default configuration file. Please check directory permissions.").
				WithOperation("create_default_config").
				WithCause(err).
				WithContext("config_path", m.configPath).
				Build()
		}
		m.cachedConfig = config
		m.logger.Info("Default configuration created successfully")
		return config, nil
	}

	// Read existing configuration file
	m.logger.Debug("Reading configuration file", "path", m.configPath)
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		m.logger.Error("Failed to read configuration file", "error", err.Error(), "path", m.configPath)
		return nil, errors.NewConfigurationError("config").
			WithMessage("Failed to read configuration file").
			WithUserMessage("Cannot read configuration file. Please check file permissions.").
			WithOperation("read_config_file").
			WithCause(err).
			WithContext("config_path", m.configPath).
			Build()
	}

	m.logger.Debug("Parsing configuration file", "size_bytes", len(data))
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		m.logger.Error("Failed to parse configuration file", "error", err.Error())
		return nil, errors.NewConfigurationError("config").
			WithMessage("Failed to parse configuration file").
			WithUserMessage("Configuration file format is invalid. Please check YAML syntax.").
			WithOperation("parse_config_yaml").
			WithCause(err).
			WithContext("config_path", m.configPath).
			Build()
	}

	// Validate and decrypt sensitive fields in profiles
	m.logger.Debug("Processing profiles", "profile_count", len(config.Profiles))
	for name, profile := range config.Profiles {
		if profile.Auth.Type == "bearer" && profile.Auth.Token != "" {
			m.logger.Debug("Decrypting credentials for profile", "profile", name)
			decryptedToken, err := m.securityMgr.DecryptCredential(profile.Auth.Token)
			if err != nil {
				m.logger.Error("Failed to decrypt credentials", "profile", name, "error", err.Error())
				return nil, errors.NewConfigurationError("config").
					WithMessage(fmt.Sprintf("Failed to decrypt token for profile %s", name)).
					WithUserMessage("Unable to decrypt saved credentials. They may be corrupted.").
					WithOperation("decrypt_credentials").
					WithCause(err).
					WithContext("profile", name).
					Build()
			}
			profile.Auth.Token = decryptedToken
			config.Profiles[name] = profile
		}
	}

	// Validate configuration
	if err := m.validateConfig(&config); err != nil {
		m.logger.Error("Configuration validation failed", "error", err.Error())
		return nil, err
	}

	m.cachedConfig = &config
	m.logger.Info("Configuration loaded successfully", 
		"profiles", len(config.Profiles),
		"themes", len(config.Themes),
		"apps", len(config.RegisteredApps))
	return &config, nil
}

// saveConfig writes the configuration to disk with encrypted sensitive data
func (m *Manager) saveConfig(config *Config) error {
	// Create a copy for encryption to avoid modifying the original
	configCopy := *config
	configCopy.Profiles = make(map[string]interfaces.Profile)

	// Encrypt sensitive fields before saving
	for name, profile := range config.Profiles {
		profileCopy := profile
		if profile.Auth.Type == "bearer" && profile.Auth.Token != "" {
			encryptedToken, err := m.securityMgr.EncryptCredential(profile.Auth.Token)
			if err != nil {
				return fmt.Errorf("failed to encrypt token for profile %s: %w", name, err)
			}
			profileCopy.Auth.Token = encryptedToken
		}
		configCopy.Profiles[name] = profileCopy
	}

	// Marshal configuration to YAML
	data, err := yaml.Marshal(&configCopy)
	if err != nil {
		return fmt.Errorf("failed to marshal configuration: %w", err)
	}

	// Write with secure file permissions (readable/writable by owner only)
	if err := os.WriteFile(m.configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write configuration file: %w", err)
	}

	return nil
}

// createDefaultConfig generates a sensible default configuration
func (m *Manager) createDefaultConfig() *Config {
	return &Config{
		Profiles: map[string]interfaces.Profile{
			"default": {
				Name:          "default",
				Host:          "localhost:8080",
				Theme:         "github",
				Confirmations: true,
				Auth: interfaces.AuthConfig{
					Type: "none",
				},
			},
		},
		Themes: map[string]interfaces.Theme{
			"github": {
				Name:    "github",
				Success: "#28a745",
				Error:   "#dc3545",
				Warning: "#ffc107",
				Info:    "#17a2b8",
			},
			"monokai": {
				Name:    "monokai",
				Success: "#a6e22e",
				Error:   "#f92672",
				Warning: "#fd971f",
				Info:    "#66d9ef",
			},
		},
		RegisteredApps: []interfaces.RegisteredApp{},
	}
}

// LoadProfile retrieves a profile by name from the configuration file
func (m *Manager) LoadProfile(name string) (*interfaces.Profile, error) {
	config, err := m.loadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	profile, exists := config.Profiles[name]
	if !exists {
		return nil, fmt.Errorf("profile '%s' not found", name)
	}

	// Set the name field to ensure consistency
	profile.Name = name

	// Validate the profile before returning
	if err := m.ValidateProfile(&profile); err != nil {
		return nil, fmt.Errorf("profile '%s' is invalid: %w", name, err)
	}

	return &profile, nil
}

// SaveProfile persists a profile to the configuration file
func (m *Manager) SaveProfile(profile *interfaces.Profile) error {
	if err := m.ValidateProfile(profile); err != nil {
		return fmt.Errorf("cannot save invalid profile: %w", err)
	}

	config, err := m.loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Initialize profiles map if it doesn't exist
	if config.Profiles == nil {
		config.Profiles = make(map[string]interfaces.Profile)
	}

	// Add or update the profile
	config.Profiles[profile.Name] = *profile

	// Save the updated configuration
	if err := m.saveConfig(config); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	// Update cached configuration
	m.cachedConfig = config

	return nil
}

// ListProfiles returns all available profile names
func (m *Manager) ListProfiles() ([]string, error) {
	config, err := m.loadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	var profileNames []string
	for name := range config.Profiles {
		profileNames = append(profileNames, name)
	}

	return profileNames, nil
}

// LoadTheme retrieves theme configuration by name
func (m *Manager) LoadTheme(name string) (*interfaces.Theme, error) {
	config, err := m.loadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	theme, exists := config.Themes[name]
	if !exists {
		return nil, fmt.Errorf("theme '%s' not found", name)
	}

	// Ensure the name field is set correctly
	theme.Name = name

	return &theme, nil
}

// GetRegisteredApps returns all registered applications
func (m *Manager) GetRegisteredApps() ([]interfaces.RegisteredApp, error) {
	config, err := m.loadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	return config.RegisteredApps, nil
}

// RegisterApp adds a new application to the registry
func (m *Manager) RegisterApp(app interfaces.RegisteredApp) error {
	config, err := m.loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Check if application is already registered
	for i, existingApp := range config.RegisteredApps {
		if existingApp.Name == app.Name {
			// Update existing application
			config.RegisteredApps[i] = app
			if err := m.saveConfig(config); err != nil {
				return fmt.Errorf("failed to save configuration: %w", err)
			}
			m.cachedConfig = config
			return nil
		}
	}

	// Add new application
	config.RegisteredApps = append(config.RegisteredApps, app)

	if err := m.saveConfig(config); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	m.cachedConfig = config
	return nil
}

// ValidateProfile ensures profile has all required fields
func (m *Manager) ValidateProfile(profile *interfaces.Profile) error {
	if profile == nil {
		return fmt.Errorf("profile cannot be nil")
	}

	if strings.TrimSpace(profile.Name) == "" {
		return fmt.Errorf("profile name cannot be empty")
	}

	if strings.TrimSpace(profile.Host) == "" {
		return fmt.Errorf("profile host cannot be empty")
	}

	// Validate host format (should contain port)
	if !strings.Contains(profile.Host, ":") {
		return fmt.Errorf("host must include port (e.g., localhost:8080)")
	}

	// Validate authentication configuration
	switch profile.Auth.Type {
	case "none":
		// No additional validation needed
	case "bearer":
		if strings.TrimSpace(profile.Auth.Token) == "" {
			return fmt.Errorf("bearer token cannot be empty when auth type is 'bearer'")
		}
		// Validate token format
		if err := m.validateBearerToken(profile.Auth.Token); err != nil {
			return fmt.Errorf("invalid bearer token: %w", err)
		}
	default:
		return fmt.Errorf("unsupported authentication type: %s", profile.Auth.Type)
	}

	return nil
}

// validateBearerToken performs basic validation on bearer token format
func (m *Manager) validateBearerToken(token string) error {
	// Basic validation - token should not be empty and should not contain whitespace
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("token cannot be empty")
	}

	if strings.ContainsAny(token, " \t\n\r") {
		return fmt.Errorf("token cannot contain whitespace characters")
	}

	// Additional validation could include JWT format checking, but we keep it simple
	// for compatibility with various token formats
	return nil
}

// validateConfig performs comprehensive validation of the entire configuration
func (m *Manager) validateConfig(config *Config) error {
	m.logger.Debug("Validating configuration structure")
	
	if config == nil {
		return errors.NewValidationError("config").
			WithMessage("Configuration cannot be nil").
			WithUserMessage("Configuration is empty or corrupted").
			WithOperation("validate_config").
			Build()
	}

	// Validate all profiles
	for name, profile := range config.Profiles {
		profileCopy := profile // Create a copy to pass by reference
		if err := m.ValidateProfile(&profileCopy); err != nil {
			m.logger.Warn("Invalid profile found", "profile", name, "error", err.Error())
			return errors.NewValidationError("config").
				WithMessage(fmt.Sprintf("Invalid profile '%s'", name)).
				WithUserMessage(fmt.Sprintf("Profile '%s' has invalid configuration", name)).
				WithOperation("validate_profile").
				WithCause(err).
				WithContext("profile_name", name).
				Build()
		}
	}

	// Validate themes
	for name, theme := range config.Themes {
		if err := m.validateTheme(name, &theme); err != nil {
			m.logger.Warn("Invalid theme found", "theme", name, "error", err.Error())
			return errors.NewValidationError("config").
				WithMessage(fmt.Sprintf("Invalid theme '%s'", name)).
				WithUserMessage(fmt.Sprintf("Theme '%s' has invalid configuration", name)).
				WithOperation("validate_theme").
				WithCause(err).
				WithContext("theme_name", name).
				Build()
		}
	}

	// Validate registered applications
	for i, app := range config.RegisteredApps {
		if err := m.validateRegisteredApp(&app); err != nil {
			m.logger.Warn("Invalid registered app found", "app", app.Name, "error", err.Error())
			return errors.NewValidationError("config").
				WithMessage(fmt.Sprintf("Invalid registered application at index %d", i)).
				WithUserMessage(fmt.Sprintf("Registered application '%s' has invalid configuration", app.Name)).
				WithOperation("validate_registered_app").
				WithCause(err).
				WithContext("app_index", i).
				WithContext("app_name", app.Name).
				Build()
		}
	}

	m.logger.Debug("Configuration validation completed successfully")
	return nil
}

// validateTheme validates a theme configuration
func (m *Manager) validateTheme(name string, theme *interfaces.Theme) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("theme name cannot be empty")
	}
	
	if theme.Name == "" {
		theme.Name = name // Set name if not provided
	}
	
	// Basic color validation - ensure they're not empty
	if strings.TrimSpace(theme.Success) == "" ||
		strings.TrimSpace(theme.Error) == "" ||
		strings.TrimSpace(theme.Warning) == "" ||
		strings.TrimSpace(theme.Info) == "" {
		return fmt.Errorf("theme colors cannot be empty")
	}
	
	return nil
}

// validateRegisteredApp validates a registered application configuration
func (m *Manager) validateRegisteredApp(app *interfaces.RegisteredApp) error {
	if strings.TrimSpace(app.Name) == "" {
		return fmt.Errorf("application name cannot be empty")
	}
	
	if strings.TrimSpace(app.Profile) == "" {
		return fmt.Errorf("application profile cannot be empty")
	}
	
	return nil
}

// GetConfigPath returns the path to the configuration file
func (m *Manager) GetConfigPath() string {
	return m.configPath
}

// InvalidateCache clears the cached configuration, forcing a reload on next access
func (m *Manager) InvalidateCache() {
	m.cachedConfig = nil
}

// DeleteProfile removes a profile from the configuration
func (m *Manager) DeleteProfile(name string) error {
	config, err := m.loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	if _, exists := config.Profiles[name]; !exists {
		return fmt.Errorf("profile '%s' does not exist", name)
	}

	// Prevent deletion of the default profile
	if name == "default" {
		return fmt.Errorf("cannot delete the default profile")
	}

	delete(config.Profiles, name)

	if err := m.saveConfig(config); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	m.cachedConfig = config
	return nil
}

// UnregisterApp removes an application from the registry
func (m *Manager) UnregisterApp(name string) error {
	config, err := m.loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Find and remove the application
	for i, app := range config.RegisteredApps {
		if app.Name == name {
			config.RegisteredApps = append(config.RegisteredApps[:i], config.RegisteredApps[i+1:]...)
			if err := m.saveConfig(config); err != nil {
				return fmt.Errorf("failed to save configuration: %w", err)
			}
			m.cachedConfig = config
			return nil
		}
	}

	return fmt.Errorf("application '%s' not found in registry", name)
}
