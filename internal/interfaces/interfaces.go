// Package interfaces defines all core interfaces required for dependency injection
// and comprehensive testability throughout the Universal Application Console.
package interfaces

import (
	"context"
	"time"
)

// Profile represents a complete configuration profile for connecting to an application
type Profile struct {
	Name          string            `yaml:"name"`
	Host          string            `yaml:"host"`
	Theme         string            `yaml:"theme"`
	Confirmations bool              `yaml:"confirmations"`
	Auth          AuthConfig        `yaml:"auth"`
	Metadata      map[string]string `yaml:"metadata,omitempty"`
}

// AuthConfig represents authentication configuration for a profile
type AuthConfig struct {
	Type  string `yaml:"type"`  // "bearer", "none"
	Token string `yaml:"token,omitempty"`
}

// Theme represents visual styling configuration
type Theme struct {
	Name    string `yaml:"name"`
	Success string `yaml:"success"`
	Error   string `yaml:"error"`
	Warning string `yaml:"warning"`
	Info    string `yaml:"info"`
}

// RegisteredApp represents an application registered in the Console Menu
type RegisteredApp struct {
	Name      string `yaml:"name"`
	Profile   string `yaml:"profile"`
	AutoStart bool   `yaml:"autoStart"`
	Status    string `json:"status"` // "ready", "offline", "error"
}

// ConfigManager handles profile and authentication management
type ConfigManager interface {
	// LoadProfile retrieves a profile by name from the configuration file
	LoadProfile(name string) (*Profile, error)
	
	// SaveProfile persists a profile to the configuration file
	SaveProfile(profile *Profile) error
	
	// ListProfiles returns all available profile names
	ListProfiles() ([]string, error)
	
	// LoadTheme retrieves theme configuration by name
	LoadTheme(name string) (*Theme, error)
	
	// GetRegisteredApps returns all registered applications
	GetRegisteredApps() ([]RegisteredApp, error)
	
	// RegisterApp adds a new application to the registry
	RegisterApp(app RegisteredApp) error
	
	// ValidateProfile ensures profile has all required fields
	ValidateProfile(profile *Profile) error
	
	// GetConfigPath returns the path to the configuration file
	GetConfigPath() string
}

// SpecResponse represents the handshake response from a Compliant Application
type SpecResponse struct {
	AppName         string            `json:"appName"`
	AppVersion      string            `json:"appVersion"`
	ProtocolVersion string            `json:"protocolVersion"`
	Features        map[string]bool   `json:"features"`
}

// CommandRequest represents a command execution request
type CommandRequest struct {
	Command string `json:"command"`
}

// ActionRequest represents an action execution request
type ActionRequest struct {
	Command    string                 `json:"command"`
	WorkflowID string                 `json:"workflowId,omitempty"`
	Context    map[string]interface{} `json:"context,omitempty"`
}

// SuggestRequest represents a request for command suggestions
type SuggestRequest struct {
	CurrentInput     string            `json:"current_input"`
	Context          map[string]interface{} `json:"context,omitempty"`
}

// ProgressRequest represents a request for operation progress
type ProgressRequest struct {
	OperationID   string `json:"operationId"`
	RequestUpdate bool   `json:"requestUpdate"`
}

// CancelRequest represents a request to cancel an operation
type CancelRequest struct {
	OperationID string `json:"operationId,omitempty"`
	WorkflowID  string `json:"workflowId,omitempty"`
}

// ContentBlock represents structured content in responses
type ContentBlock struct {
	Type      string                 `json:"type"`
	Content   interface{}            `json:"content,omitempty"`
	Status    string                 `json:"status,omitempty"`
	Title     string                 `json:"title,omitempty"`
	Collapsed *bool                  `json:"collapsed,omitempty"`
	Headers   []string               `json:"headers,omitempty"`
	Rows      [][]string             `json:"rows,omitempty"`
	Items     []string               `json:"items,omitempty"`
	Language  string                 `json:"language,omitempty"`
	Progress  *int                   `json:"progress,omitempty"`
	Label     string                 `json:"label,omitempty"`
}

// Action represents an executable action from the Actions Pane
type Action struct {
	Name string `json:"name"`
	Command string `json:"command"`
	Type string `json:"type"` // "primary", "confirmation", "cancel", "info", "alternative"
	Icon string `json:"icon,omitempty"`
}

// Workflow represents multi-step operation context
type Workflow struct {
	ID         string `json:"id"`
	Step       int    `json:"step"`
	TotalSteps int    `json:"totalSteps"`
	Title      string `json:"title"`
}

// CommandResponse represents a structured response from command execution
type CommandResponse struct {
	Response struct {
		Type    string         `json:"type"` // "text" or "structured"
		Content interface{}    `json:"content"` // string for "text", []ContentBlock for "structured"
	} `json:"response"`
	Actions             []Action  `json:"actions,omitempty"`
	Workflow            *Workflow `json:"workflow,omitempty"`
	RequiresConfirmation bool     `json:"requiresConfirmation,omitempty"`
}

// SuggestionItem represents a single command suggestion
type SuggestionItem struct {
	Text                string `json:"text"`
	Description         string `json:"description"`
	Type                string `json:"type"`
	RequiresConfirmation bool   `json:"requiresConfirmation,omitempty"`
}

// SuggestResponse represents suggestions for command completion
type SuggestResponse struct {
	Suggestions []SuggestionItem `json:"suggestions"`
}

// ProgressResponse represents the status of a long-running operation
type ProgressResponse struct {
	Progress int    `json:"progress"` // 0-100
	Status   string `json:"status"`   // "running", "complete", "error"
	Message  string `json:"message"`
	Details  struct {
		Completed int    `json:"completed"`
		Total     int    `json:"total"`
		Current   string `json:"current"`
	} `json:"details,omitempty"`
}

// CancelResponse represents the result of canceling an operation
type CancelResponse struct {
	Cancelled        bool   `json:"cancelled"`
	Message          string `json:"message"`
	RollbackRequired bool   `json:"rollbackRequired"`
}

// ErrorResponse represents structured error information
type ErrorResponse struct {
	Error struct {
		Message         string          `json:"message"`
		Code            string          `json:"code"`
		Details         *ContentBlock   `json:"details,omitempty"`
		RecoveryActions []Action        `json:"recoveryActions,omitempty"`
	} `json:"error"`
}

// ProtocolClient handles HTTP communication with Compliant Applications
type ProtocolClient interface {
	// Connect establishes connection and performs handshake with the application
	Connect(ctx context.Context, host string, auth *AuthConfig) (*SpecResponse, error)
	
	// ExecuteCommand sends a command to the application
	ExecuteCommand(ctx context.Context, request CommandRequest) (*CommandResponse, error)
	
	// ExecuteAction sends an action execution request
	ExecuteAction(ctx context.Context, request ActionRequest) (*CommandResponse, error)
	
	// GetSuggestions requests command suggestions
	GetSuggestions(ctx context.Context, request SuggestRequest) (*SuggestResponse, error)
	
	// GetProgress requests operation progress
	GetProgress(ctx context.Context, request ProgressRequest) (*ProgressResponse, error)
	
	// CancelOperation requests operation cancellation
	CancelOperation(ctx context.Context, request CancelRequest) (*CancelResponse, error)
	
	// IsConnected returns whether the client is currently connected
	IsConnected() bool
	
	// Disconnect closes the connection to the application
	Disconnect() error
	
	// GetLastError returns the last communication error
	GetLastError() error
}

// RenderedContent represents content after processing for display
type RenderedContent struct {
	Text      string
	Focusable bool
	Expanded  *bool
	ID        string
}

// ContentRenderer processes structured content for display
type ContentRenderer interface {
	// RenderContent transforms structured content into display-ready format
	RenderContent(content interface{}, theme *Theme) ([]RenderedContent, error)
	
	// RenderActions formats actions for the Actions Pane
	RenderActions(actions []Action, theme *Theme) (string, error)
	
	// RenderError formats error responses with recovery options
	RenderError(errorResp *ErrorResponse, theme *Theme) (string, error)
	
	// RenderProgress formats progress indicators
	RenderProgress(progress *ProgressResponse, theme *Theme) (string, error)
	
	// RenderWorkflow formats workflow breadcrumbs
	RenderWorkflow(workflow *Workflow, theme *Theme) (string, error)
	
	// ToggleCollapsible expands or collapses a collapsible section
	ToggleCollapsible(contentID string) error
	
	// ExpandAll expands all collapsible sections
	ExpandAll() error
	
	// CollapseAll collapses all collapsible sections
	CollapseAll() error
}

// AppHealth represents the health status of a registered application
type AppHealth struct {
	Name         string    `json:"name"`
	Status       string    `json:"status"` // "ready", "offline", "error", "checking"
	LastChecked  time.Time `json:"lastChecked"`
	ResponseTime time.Duration `json:"responseTime,omitempty"`
	Error        string    `json:"error,omitempty"`
}

// RegistryManager handles application registration and health monitoring
type RegistryManager interface {
	// GetRegisteredApps returns all registered applications with current status
	GetRegisteredApps() ([]RegisteredApp, error)
	
	// RegisterApp adds a new application to the registry
	RegisterApp(app RegisteredApp) error
	
	// UnregisterApp removes an application from the registry
	UnregisterApp(name string) error
	
	// UpdateAppStatus updates the health status of an application
	UpdateAppStatus(name string, status AppHealth) error
	
	// GetAppHealth returns current health information for an application
	GetAppHealth(name string) (*AppHealth, error)
	
	// StartHealthMonitoring begins periodic health checks for all registered apps
	StartHealthMonitoring(ctx context.Context, interval time.Duration) error
	
	// StopHealthMonitoring stops all health monitoring
	StopHealthMonitoring() error
	
	// CheckAppHealth performs an immediate health check for a specific application
	CheckAppHealth(ctx context.Context, appName string) (*AppHealth, error)
	
	// GetAppByName retrieves application details by name
	GetAppByName(name string) (*RegisteredApp, error)
}

// AuthManager handles security credentials and authentication
type AuthManager interface {
	// ValidateToken verifies the format and basic validity of an authentication token
	ValidateToken(token string, tokenType string) error
	
	// CreateAuthHeader constructs the appropriate authentication header value
	CreateAuthHeader(auth *AuthConfig) (string, error)
	
	// SecureStore encrypts and stores sensitive authentication data
	SecureStore(key string, value string) error
	
	// SecureRetrieve decrypts and retrieves sensitive authentication data
	SecureRetrieve(key string) (string, error)
	
	// ClearSecureData removes all stored authentication credentials
	ClearSecureData() error
	
	// RefreshToken attempts to refresh an expired token if possible
	RefreshToken(auth *AuthConfig) (*AuthConfig, error)
	
	// ValidatePermissions checks if current credentials have required permissions
	ValidatePermissions(auth *AuthConfig, requiredPerms []string) error
}