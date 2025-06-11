// Package menu implements the Console Menu Mode interface for the Universal Application Console.
// This file defines the MenuModel structure and state management capabilities that provide
// centralized application selection and connection management as specified in section 3.2.1
// of the design specification.
package menu

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/universal-console/console/internal/interfaces"
)

// MenuModel represents the complete state and dependencies for Console Menu Mode operation
type MenuModel struct {
	// Injected dependencies for external system integration
	registryManager interfaces.RegistryManager
	configManager   interfaces.ConfigManager
	protocolClient  interfaces.ProtocolClient
	contentRenderer interfaces.ContentRenderer
	authManager     interfaces.AuthManager

	// Application registry state management
	registeredApps    []interfaces.RegisteredApp
	appHealthStatus   map[string]*interfaces.AppHealth
	selectedAppIndex  int
	lastHealthUpdate  time.Time
	healthUpdateError string

	// User interface state management
	quickConnectInput textinput.Model
	focusState        FocusState
	interfaceMode     InterfaceMode
	lastKeyPressed    string
	statusMessage     string
	errorMessage      string

	// Navigation and interaction state
	navigationHistory []NavigationEntry
	confirmationState *ConfirmationState

	// Display preferences and configuration
	showHealthDetails bool
	autoRefreshHealth bool
	refreshInterval   time.Duration
	terminalWidth     int
	terminalHeight    int

	// Background operation management
	backgroundContext context.Context
	backgroundCancel  context.CancelFunc
	healthMonitoring  bool
}

// FocusState represents the current focus location within the menu interface
type FocusState int

const (
	FocusApplicationList FocusState = iota
	FocusQuickConnect
	FocusConnectButton
	FocusCommandOptions
)

// InterfaceMode represents different operational modes of the menu interface
type InterfaceMode int

const (
	ModeNormal InterfaceMode = iota
	ModeRegistration
	ModeProfileEdit
	ModeConfirmation
	ModeError
	ModeLoading
)

// NavigationEntry tracks user navigation history for enhanced user experience
type NavigationEntry struct {
	Timestamp   time.Time  `json:"timestamp"`
	FromFocus   FocusState `json:"fromFocus"`
	ToFocus     FocusState `json:"toFocus"`
	ActionTaken string     `json:"actionTaken"`
	AppSelected string     `json:"appSelected,omitempty"`
}

// ConfirmationState manages confirmation dialogs and user decision workflows
type ConfirmationState struct {
	Title         string   `json:"title"`
	Message       string   `json:"message"`
	Options       []string `json:"options"`
	SelectedIndex int      `json:"selectedIndex"`
	OnConfirm     func() tea.Cmd
	OnCancel      func() tea.Cmd
}

// MenuPreferences contains user-configurable settings for menu behavior
type MenuPreferences struct {
	AutoRefreshHealth       bool          `json:"autoRefreshHealth"`
	HealthRefreshInterval   time.Duration `json:"healthRefreshInterval"`
	ShowHealthDetails       bool          `json:"showHealthDetails"`
	EnableKeyboardShortcuts bool          `json:"enableKeyboardShortcuts"`
	DefaultConnectTimeout   time.Duration `json:"defaultConnectTimeout"`
	RememberLastSelection   bool          `json:"rememberLastSelection"`
}

// NewMenuModel creates a new Console Menu Mode model with comprehensive dependency injection
func NewMenuModel(
	registryManager interfaces.RegistryManager,
	configManager interfaces.ConfigManager,
	protocolClient interfaces.ProtocolClient,
	contentRenderer interfaces.ContentRenderer,
	authManager interfaces.AuthManager,
) *MenuModel {
	// Initialize quick connect input component
	quickConnectInput := textinput.New()
	quickConnectInput.Placeholder = "Enter host:port (e.g., localhost:8080)"
	quickConnectInput.Width = 50
	quickConnectInput.CharLimit = 100

	// Create background context for health monitoring operations
	backgroundCtx, backgroundCancel := context.WithCancel(context.Background())

	model := &MenuModel{
		// Dependency injection
		registryManager: registryManager,
		configManager:   configManager,
		protocolClient:  protocolClient,
		contentRenderer: contentRenderer,
		authManager:     authManager,

		// Initialize state management
		appHealthStatus:   make(map[string]*interfaces.AppHealth),
		selectedAppIndex:  0,
		quickConnectInput: quickConnectInput,
		focusState:        FocusApplicationList,
		interfaceMode:     ModeNormal,

		// Configure default preferences
		showHealthDetails: true,
		autoRefreshHealth: true,
		refreshInterval:   30 * time.Second,

		// Background operation setup
		backgroundContext: backgroundCtx,
		backgroundCancel:  backgroundCancel,
		navigationHistory: make([]NavigationEntry, 0, 50),
	}

	return model
}

// Init implements the tea.Model interface for Bubble Tea initialization
func (m *MenuModel) Init() tea.Cmd {
	commands := []tea.Cmd{
		m.loadRegisteredApplications(),
		m.startHealthMonitoring(),
		textinput.Blink,
	}

	return tea.Batch(commands...)
}

// SetTerminalSize updates the model with current terminal dimensions for responsive layout
func (m *MenuModel) SetTerminalSize(width, height int) {
	m.terminalWidth = width
	m.terminalHeight = height

	// Adjust quick connect input width based on terminal size
	availableWidth := width - 20 // Account for borders and padding
	if availableWidth > 30 && availableWidth < 80 {
		m.quickConnectInput.Width = availableWidth
	}
}

// GetSelectedApplication returns the currently selected application details
func (m *MenuModel) GetSelectedApplication() (*interfaces.RegisteredApp, error) {
	if len(m.registeredApps) == 0 {
		return nil, fmt.Errorf("no applications registered")
	}

	if m.selectedAppIndex < 0 || m.selectedAppIndex >= len(m.registeredApps) {
		return nil, fmt.Errorf("invalid application selection index")
	}

	selectedApp := m.registeredApps[m.selectedAppIndex]
	return &selectedApp, nil
}

// GetApplicationHealth returns the current health status for a specific application
func (m *MenuModel) GetApplicationHealth(appName string) (*interfaces.AppHealth, bool) {
	health, exists := m.appHealthStatus[appName]
	return health, exists
}

// SetFocus changes the current focus state and records navigation history
func (m *MenuModel) SetFocus(newFocus FocusState) {
	if newFocus != m.focusState {
		// Record navigation in history
		m.recordNavigation(m.focusState, newFocus, "focus_change")
		m.focusState = newFocus
	}
}

// SetInterfaceMode changes the operational mode and manages state transitions
func (m *MenuModel) SetInterfaceMode(mode InterfaceMode) {
	m.interfaceMode = mode

	// Handle mode-specific state initialization
	switch mode {
	case ModeNormal:
		m.errorMessage = ""
		m.confirmationState = nil
	case ModeError:
		m.confirmationState = nil
	case ModeConfirmation:
		// Confirmation state should be set separately
	case ModeLoading:
		m.statusMessage = "Loading..."
	}
}

// ConnectToSelectedApplication initiates connection to the currently selected application
func (m *MenuModel) ConnectToSelectedApplication() tea.Cmd {
	selectedApp, err := m.GetSelectedApplication()
	if err != nil {
		return m.showError(fmt.Sprintf("Connection failed: %s", err.Error()))
	}

	return m.connectToApplication(selectedApp)
}

// ConnectToQuickConnectHost initiates connection using the quick connect input value
func (m *MenuModel) ConnectToQuickConnectHost() tea.Cmd {
	host := strings.TrimSpace(m.quickConnectInput.Value())
	if host == "" {
		return m.showError("Please enter a host address")
	}

	// Validate host format
	if !strings.Contains(host, ":") {
		return m.showError("Host must include port (e.g., localhost:8080)")
	}

	// Create temporary application for connection
	tempApp := &interfaces.RegisteredApp{
		Name:    fmt.Sprintf("Quick Connect - %s", host),
		Profile: "default", // Use default profile for quick connections
		Status:  "unknown",
	}

	return m.connectToApplication(tempApp)
}

// RefreshApplicationHealth triggers an immediate health status update for all applications
func (m *MenuModel) RefreshApplicationHealth() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		return healthRefreshRequestMsg{}
	})
}

// ShowConfirmation displays a confirmation dialog with customizable options
func (m *MenuModel) ShowConfirmation(title, message string, options []string, onConfirm, onCancel func() tea.Cmd) {
	m.confirmationState = &ConfirmationState{
		Title:         title,
		Message:       message,
		Options:       options,
		SelectedIndex: 0,
		OnConfirm:     onConfirm,
		OnCancel:      onCancel,
	}
	m.SetInterfaceMode(ModeConfirmation)
}

// Message types for Bubble Tea command system

// applicationListLoadedMsg carries the loaded application registry data
type applicationListLoadedMsg struct {
	apps  []interfaces.RegisteredApp
	error string
}

// healthStatusUpdatedMsg carries updated health information for applications
type healthStatusUpdatedMsg struct {
	appName string
	health  *interfaces.AppHealth
	error   string
}

// healthRefreshRequestMsg triggers a health status refresh for all applications
type healthRefreshRequestMsg struct{}

// connectionInitiatedMsg indicates that a connection attempt has begun
type connectionInitiatedMsg struct {
	appName string
	profile *interfaces.Profile
}

// connectionCompletedMsg indicates that a connection attempt has finished
type connectionCompletedMsg struct {
	appName  string
	success  bool
	error    string
	specResp *interfaces.SpecResponse
}

// errorDisplayMsg carries error information for user presentation
type errorDisplayMsg struct {
	message string
}

// statusUpdateMsg carries general status information for user feedback
type statusUpdateMsg struct {
	message string
	timeout time.Duration
}

// Command generation methods for asynchronous operations

// loadRegisteredApplications creates a command to load the application registry
func (m *MenuModel) loadRegisteredApplications() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		apps, err := m.registryManager.GetRegisteredApps()
		if err != nil {
			return applicationListLoadedMsg{
				apps:  []interfaces.RegisteredApp{},
				error: err.Error(),
			}
		}

		return applicationListLoadedMsg{
			apps:  apps,
			error: "",
		}
	})
}

// startHealthMonitoring creates a command to begin background health monitoring
func (m *MenuModel) startHealthMonitoring() tea.Cmd {
	if !m.autoRefreshHealth {
		return nil
	}

	return tea.Cmd(func() tea.Msg {
		// Start health monitoring in the background
		err := m.registryManager.StartHealthMonitoring(m.backgroundContext, m.refreshInterval)
		if err != nil {
			return statusUpdateMsg{
				message: fmt.Sprintf("Health monitoring failed to start: %s", err.Error()),
				timeout: 5 * time.Second,
			}
		}

		m.healthMonitoring = true
		return statusUpdateMsg{
			message: "Health monitoring started",
			timeout: 3 * time.Second,
		}
	})
}

// connectToApplication creates a command to establish connection to a specific application
func (m *MenuModel) connectToApplication(app *interfaces.RegisteredApp) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// Load application profile
		profile, err := m.configManager.LoadProfile(app.Profile)
		if err != nil {
			return connectionCompletedMsg{
				appName: app.Name,
				success: false,
				error:   fmt.Sprintf("Failed to load profile '%s': %s", app.Profile, err.Error()),
			}
		}

		// Signal connection initiation
		return connectionInitiatedMsg{
			appName: app.Name,
			profile: profile,
		}
	})
}

// performConnection executes the actual connection establishment process
func (m *MenuModel) performConnection(appName string, profile *interfaces.Profile) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// Create connection context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		// Attempt connection
		specResp, err := m.protocolClient.Connect(ctx, profile.Host, &profile.Auth)
		if err != nil {
			return connectionCompletedMsg{
				appName: appName,
				success: false,
				error:   err.Error(),
			}
		}

		return connectionCompletedMsg{
			appName:  appName,
			success:  true,
			specResp: specResp,
		}
	})
}

// showError creates a command to display error information to the user
func (m *MenuModel) showError(message string) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		return errorDisplayMsg{
			message: message,
		}
	})
}

// refreshHealthForApp creates a command to refresh health status for a specific application
func (m *MenuModel) refreshHealthForApp(appName string) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		health, err := m.registryManager.CheckAppHealth(ctx, appName)
		if err != nil {
			return healthStatusUpdatedMsg{
				appName: appName,
				error:   err.Error(),
			}
		}

		return healthStatusUpdatedMsg{
			appName: appName,
			health:  health,
		}
	})
}

// Utility methods for state management and navigation

// recordNavigation adds an entry to the navigation history for user experience analysis
func (m *MenuModel) recordNavigation(from, to FocusState, action string) {
	entry := NavigationEntry{
		Timestamp:   time.Now(),
		FromFocus:   from,
		ToFocus:     to,
		ActionTaken: action,
	}

	// Add selected app name if relevant
	if selectedApp, err := m.GetSelectedApplication(); err == nil {
		entry.AppSelected = selectedApp.Name
	}

	m.navigationHistory = append(m.navigationHistory, entry)

	// Limit history size to prevent memory growth
	if len(m.navigationHistory) > 50 {
		m.navigationHistory = m.navigationHistory[1:]
	}
}

// Cleanup performs necessary cleanup operations when the model is being destroyed
func (m *MenuModel) Cleanup() {
	// Cancel background operations
	if m.backgroundCancel != nil {
		m.backgroundCancel()
	}

	// Stop health monitoring if active
	if m.healthMonitoring {
		m.registryManager.StopHealthMonitoring()
	}
}
