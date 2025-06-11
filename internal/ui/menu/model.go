// Package menu implements the Console Menu Mode interface for the Universal Application Console.
// This file defines the MenuModel structure containing the registered applications list,
// connection status indicators, quick connect input field, and focus management state.
// It serves as the primary interface for connecting to Compliant Applications.
package menu

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/universal-console/console/internal/interfaces"
	"github.com/universal-console/console/internal/ui/app"
)

// FocusState represents which part of the menu is currently focused.
type FocusState int

const (
	FocusList FocusState = iota
	FocusInput
)

// MenuModel represents the state of the Console Menu Mode.
type MenuModel struct {
	// Injected dependencies
	registryManager interfaces.RegistryManager
	configManager   interfaces.ConfigManager
	protocolClient  interfaces.ProtocolClient
	contentRenderer interfaces.ContentRenderer
	authManager     interfaces.AuthManager

	// UI State
	registeredApps    []interfaces.RegisteredApp
	appHealth         map[string]*interfaces.AppHealth
	selectedIndex     int
	quickConnectInput textinput.Model
	focusState        FocusState
	isConnecting      bool
	statusMessage     string
	err               error

	// Terminal dimensions
	width  int
	height int
}

// NewMenuModel creates a new instance of the Console Menu model.
func NewMenuModel(
	registry interfaces.RegistryManager,
	config interfaces.ConfigManager,
	client interfaces.ProtocolClient,
	renderer interfaces.ContentRenderer,
	auth interfaces.AuthManager,
) *MenuModel {
	ti := textinput.New()
	ti.Placeholder = "localhost:8080"
	ti.Focus()
	ti.CharLimit = 150
	ti.Width = 50

	return &MenuModel{
		registryManager:   registry,
		configManager:     config,
		protocolClient:    client,
		contentRenderer:   renderer,
		authManager:       auth,
		quickConnectInput: ti,
		focusState:        FocusList,
		appHealth:         make(map[string]*interfaces.AppHealth),
	}
}

// Init is the first command that will be executed.
func (m *MenuModel) Init() tea.Cmd {
	// Start health monitoring in the background
	m.registryManager.StartHealthMonitoring(context.Background(), 30*time.Second)
	return tea.Batch(
		m.reloadApps(), // Initial load of apps
		tick(),         // Start the timer for health updates
	)
}

// Helper commands and messages

// ConnectionResultMsg is sent after a connection attempt. It is handled by the
// parent controller to determine whether to switch to Application Mode or display an error.
// This type is EXPORTED because it is part of the package's public API.
type ConnectionResultMsg struct {
	Model tea.Model // The new AppModel on success, or nil on failure
	Err   error
}

type (
	// appsReloadedMsg is sent when the list of registered apps is reloaded.
	// This is an internal message and remains UNEXPORTED.
	appsReloadedMsg struct {
		apps []interfaces.RegisteredApp
		err  error
	}

	// healthStatusUpdatedMsg is sent when new health data is available.
	// This is an internal message and remains UNEXPORTED.
	healthStatusUpdatedMsg struct {
		health map[string]*interfaces.AppHealth
	}

	// tickMsg is used to trigger periodic health updates.
	// This is an internal message and remains UNEXPORTED.
	tickMsg struct{}
)

// tick is a command to send a tickMsg every second for health updates.
func tick() tea.Cmd {
	return tea.Every(time.Second, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

// reloadApps is a command to fetch the latest list of registered apps.
func (m *MenuModel) reloadApps() tea.Cmd {
	return func() tea.Msg {
		apps, err := m.registryManager.GetRegisteredApps()
		return appsReloadedMsg{apps: apps, err: err}
	}
}

// updateHealth is a command to fetch the latest health status for all apps.
func (m *MenuModel) updateHealth() tea.Cmd {
	return func() tea.Msg {
		apps, err := m.registryManager.GetRegisteredApps()
		if err != nil {
			return nil // Silently fail, don't interrupt user
		}
		healthMap := make(map[string]*interfaces.AppHealth)
		for _, app := range apps {
			health, err := m.registryManager.GetAppHealth(app.Name)
			if err == nil {
				healthMap[app.Name] = health
			}
		}
		return healthStatusUpdatedMsg{health: healthMap}
	}
}

// attemptConnection is a command to connect to an application using a profile.
func (m *MenuModel) attemptConnection(profileName, hostOverride string) tea.Cmd {
	return func() tea.Msg {
		var profile *interfaces.Profile
		var err error

		if hostOverride != "" {
			// Create a temporary profile for direct connection
			profile = &interfaces.Profile{
				Name:          "temporary",
				Host:          hostOverride,
				Theme:         "github", // Default theme
				Confirmations: true,
				Auth:          interfaces.AuthConfig{Type: "none"},
			}
		} else {
			// Load profile from config
			profile, err = m.configManager.LoadProfile(profileName)
			if err != nil {
				// Return the EXPORTED message type with the EXPORTED field name.
				return ConnectionResultMsg{Err: fmt.Errorf("failed to load profile '%s': %w", profileName, err)}
			}
		}

		// Perform connection
		_, err = m.protocolClient.Connect(context.Background(), profile.Host, &profile.Auth)
		if err != nil {
			// Return the EXPORTED message type with the EXPORTED field name.
			return ConnectionResultMsg{Err: fmt.Errorf("connection to %s failed: %w", profile.Host, err)}
		}

		// On success, create and return the new Application Mode model
		appModel := app.NewAppModel(
			profile,
			m.protocolClient,
			m.contentRenderer,
			m.configManager,
			m.authManager,
		)
		// Return the EXPORTED message type with the EXPORTED field name.
		return ConnectionResultMsg{Model: appModel}
	}
}
