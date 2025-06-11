// Package app provides the main application controller that orchestrates all components
// and manages the complete application lifecycle. It handles mode switching between
// Console Menu and Application modes and coordinates communication between UI models
// and backend services through dependency injection.
package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/universal-console/console/internal/interfaces"
	"github.com/universal-console/console/internal/ui/app"
	"github.com/universal-console/console/internal/ui/menu"
)

// activeView determines which model is currently visible and receiving updates.
type activeView int

const (
	menuView activeView = iota
	appView
)

// ConsoleController is the main application model that manages state transitions
// between the Menu and Application modes.
type ConsoleController struct {
	// Child UI Models
	menuModel tea.Model
	appModel  tea.Model

	// Active View State
	currentView activeView

	// Terminal dimensions
	width  int
	height int

	// Error state
	err error
}

// NewConsoleController creates the main controller with all dependencies injected.
func NewConsoleController(
	registryManager interfaces.RegistryManager,
	configManager interfaces.ConfigManager,
	protocolClient interfaces.ProtocolClient,
	contentRenderer interfaces.ContentRenderer,
	authManager interfaces.AuthManager,
) *ConsoleController {
	// The menu model is created immediately.
	menuModel := menu.NewMenuModel(
		registryManager,
		configManager,
		protocolClient,
		contentRenderer,
		authManager,
	)

	return &ConsoleController{
		menuModel:   menuModel,
		currentView: menuView,
	}
}

// Init initializes the main controller and its initial child model.
func (c *ConsoleController) Init() tea.Cmd {
	return c.menuModel.Init()
}

// Update handles all messages and delegates them to the active child model.
// It also manages the transition between the menu and app views.
func (c *ConsoleController) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return c, tea.Quit
		}

	case tea.WindowSizeMsg:
		c.width = msg.Width
		c.height = msg.Height
		// Propagate size to child models
		if c.currentView == menuView && c.menuModel != nil {
			c.menuModel, _ = c.menuModel.Update(msg)
		}
		if c.currentView == appView && c.appModel != nil {
			c.appModel, _ = c.appModel.Update(msg)
		}

	case menu.ConnectionResultMsg:
		// This is the message that signals a switch from menu to app.
		if msg.Err != nil {
			c.err = msg.Err
			// We can pass this error to the menu model to display.
			c.menuModel, cmd = c.menuModel.Update(msg)
			return c, cmd
		}
		c.appModel = msg.Model
		c.currentView = appView
		// Send window size to the new model and initialize it.
		c.appModel, cmd = c.appModel.Update(tea.WindowSizeMsg{Width: c.width, Height: c.height})
		cmds = append(cmds, cmd, c.appModel.Init())
		return c, tea.Batch(cmds...)

	case app.ConnectionStatusMsg:
		// This message signals a switch from app back to menu.
		if !msg.Connected {
			c.appModel = nil
			c.currentView = menuView
			// Optionally, tell the menu to reload its state
			cmds = append(cmds, c.menuModel.Init())
		}
		return c, tea.Batch(cmds...)

	case error:
		c.err = msg
		return c, nil
	}

	// Delegate messages to the active model.
	switch c.currentView {
	case menuView:
		c.menuModel, cmd = c.menuModel.Update(msg)
		cmds = append(cmds, cmd)

	case appView:
		// Type assertion to get the concrete model for switching
		var newAppModel tea.Model
		newAppModel, cmd = c.appModel.Update(msg)

		// Check if the update returned a different model (signaling a state change)
		if newAppModel != c.appModel {
			c.appModel = newAppModel
		}

		cmds = append(cmds, cmd)
	}

	return c, tea.Batch(cmds...)
}

// View renders the view of the currently active child model.
func (c *ConsoleController) View() string {
	switch c.currentView {
	case menuView:
		return c.menuModel.View()
	case appView:
		return c.appModel.View()
	default:
		return "Error: Unknown view state."
	}
}
