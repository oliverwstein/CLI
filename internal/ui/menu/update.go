// Package menu implements user input processing and state management for Console Menu Mode.
// This file contains the Bubble Tea update function that handles keyboard navigation,
// application selection, connection initiation, and meta command processing according
// to the interaction patterns specified in section 3.2.5 of the design specification.
package menu

import (
	"fmt"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/universal-console/console/internal/interfaces"
)

// Update implements the Bubble Tea Model interface for Console Menu Mode input processing
func (m *MenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var commands []tea.Cmd

	// Handle interface mode-specific processing first
	if cmd := m.handleModeSpecificInput(msg); cmd != nil {
		commands = append(commands, cmd)
	}

	// Process the message based on its type
	switch msg := msg.(type) {
	case tea.KeyMsg:
		cmd := m.handleKeyInput(msg)
		if cmd != nil {
			commands = append(commands, cmd)
		}

	case tea.WindowSizeMsg:
		m.SetTerminalSize(msg.Width, msg.Height)

	case applicationListLoadedMsg:
		cmd := m.handleApplicationListLoaded(msg)
		if cmd != nil {
			commands = append(commands, cmd)
		}

	case healthStatusUpdatedMsg:
		m.handleHealthStatusUpdate(msg)

	case healthRefreshRequestMsg:
		cmd := m.handleHealthRefreshRequest()
		if cmd != nil {
			commands = append(commands, cmd)
		}

	case connectionInitiatedMsg:
		cmd := m.handleConnectionInitiated(msg)
		if cmd != nil {
			commands = append(commands, cmd)
		}

	case connectionCompletedMsg:
		return m.handleConnectionCompleted(msg)

	case errorDisplayMsg:
		m.handleErrorDisplay(msg)

	case statusUpdateMsg:
		m.handleStatusUpdate(msg)

	default:
		// Handle textinput updates for quick connect field
		if m.focusState == FocusQuickConnect {
			var cmd tea.Cmd
			m.quickConnectInput, cmd = m.quickConnectInput.Update(msg)
			if cmd != nil {
				commands = append(commands, cmd)
			}
		}
	}

	// Return updated model with batched commands
	if len(commands) > 0 {
		return m, tea.Batch(commands...)
	}
	return m, nil
}

// handleModeSpecificInput processes input based on the current interface mode
func (m *MenuModel) handleModeSpecificInput(msg tea.Msg) tea.Cmd {
	switch m.interfaceMode {
	case ModeConfirmation:
		return m.handleConfirmationModeInput(msg)
	case ModeError:
		return m.handleErrorModeInput(msg)
	case ModeLoading:
		return m.handleLoadingModeInput(msg)
	default:
		return nil
	}
}

// handleKeyInput processes keyboard input for normal menu operation
func (m *MenuModel) handleKeyInput(msg tea.KeyMsg) tea.Cmd {
	// Store the last key pressed for debugging and user experience analysis
	m.lastKeyPressed = msg.String()

	// Handle global key commands that work regardless of focus
	switch msg.String() {
	case "ctrl+c", "esc":
		return m.handleExitRequest()
	case "ctrl+r":
		return m.RefreshApplicationHealth()
	case "f5":
		return m.loadRegisteredApplications()
	}

	// Handle focus-specific key processing
	switch m.focusState {
	case FocusApplicationList:
		return m.handleApplicationListKeys(msg)
	case FocusQuickConnect:
		return m.handleQuickConnectKeys(msg)
	case FocusConnectButton:
		return m.handleConnectButtonKeys(msg)
	case FocusCommandOptions:
		return m.handleCommandOptionsKeys(msg)
	default:
		return nil
	}
}

// handleApplicationListKeys processes keyboard input when application list has focus
func (m *MenuModel) handleApplicationListKeys(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up", "k":
		return m.navigateApplicationList(-1)
	case "down", "j":
		return m.navigateApplicationList(1)
	case "enter", "space":
		return m.ConnectToSelectedApplication()
	case "tab":
		m.SetFocus(FocusQuickConnect)
		return nil
	case "shift+tab":
		m.SetFocus(FocusCommandOptions)
		return nil
	default:
		// Handle numbered selection (1-9)
		if num, err := strconv.Atoi(msg.String()); err == nil && num >= 1 && num <= 9 {
			return m.selectApplicationByNumber(num)
		}
		return nil
	}
}

// handleQuickConnectKeys processes keyboard input when quick connect field has focus
func (m *MenuModel) handleQuickConnectKeys(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		return m.ConnectToQuickConnectHost()
	case "tab":
		m.SetFocus(FocusConnectButton)
		return nil
	case "shift+tab":
		m.SetFocus(FocusApplicationList)
		return nil
	case "up":
		m.SetFocus(FocusApplicationList)
		return nil
	case "down":
		m.SetFocus(FocusConnectButton)
		return nil
	default:
		// Let textinput handle character input
		var cmd tea.Cmd
		m.quickConnectInput, cmd = m.quickConnectInput.Update(msg)
		return cmd
	}
}

// handleConnectButtonKeys processes keyboard input when connect button has focus
func (m *MenuModel) handleConnectButtonKeys(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter", "space":
		return m.ConnectToQuickConnectHost()
	case "tab":
		m.SetFocus(FocusCommandOptions)
		return nil
	case "shift+tab":
		m.SetFocus(FocusQuickConnect)
		return nil
	case "up":
		m.SetFocus(FocusQuickConnect)
		return nil
	case "down":
		m.SetFocus(FocusCommandOptions)
		return nil
	default:
		return nil
	}
}

// handleCommandOptionsKeys processes keyboard input when command options have focus
func (m *MenuModel) handleCommandOptionsKeys(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "r", "R":
		return m.initiateApplicationRegistration()
	case "e", "E":
		return m.initiateProfileEdit()
	case "q", "Q":
		return m.handleExitRequest()
	case "tab":
		m.SetFocus(FocusApplicationList)
		return nil
	case "shift+tab":
		m.SetFocus(FocusConnectButton)
		return nil
	case "up":
		m.SetFocus(FocusConnectButton)
		return nil
	case "down":
		m.SetFocus(FocusApplicationList)
		return nil
	default:
		return nil
	}
}

// handleConfirmationModeInput processes input during confirmation dialogs
func (m *MenuModel) handleConfirmationModeInput(msg tea.Msg) tea.Cmd {
	if m.confirmationState == nil {
		m.SetInterfaceMode(ModeNormal)
		return nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}

	switch keyMsg.String() {
	case "up", "k":
		if m.confirmationState.SelectedIndex > 0 {
			m.confirmationState.SelectedIndex--
		}
		return nil
	case "down", "j":
		if m.confirmationState.SelectedIndex < len(m.confirmationState.Options)-1 {
			m.confirmationState.SelectedIndex++
		}
		return nil
	case "enter", "space":
		return m.executeConfirmationChoice()
	case "esc", "ctrl+c":
		return m.cancelConfirmation()
	default:
		// Handle numbered selection for confirmation options
		if num, err := strconv.Atoi(keyMsg.String()); err == nil && num >= 1 && num <= len(m.confirmationState.Options) {
			m.confirmationState.SelectedIndex = num - 1
			return m.executeConfirmationChoice()
		}
		return nil
	}
}

// handleErrorModeInput processes input during error display
func (m *MenuModel) handleErrorModeInput(msg tea.Msg) tea.Cmd {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}

	switch keyMsg.String() {
	case "enter", "space", "esc":
		m.SetInterfaceMode(ModeNormal)
		m.errorMessage = ""
		return nil
	default:
		return nil
	}
}

// handleLoadingModeInput processes input during loading operations
func (m *MenuModel) handleLoadingModeInput(msg tea.Msg) tea.Cmd {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}

	// Allow cancellation during loading
	switch keyMsg.String() {
	case "ctrl+c", "esc":
		m.SetInterfaceMode(ModeNormal)
		m.statusMessage = "Operation cancelled"
		return nil
	default:
		return nil
	}
}

// Message handling methods for asynchronous operations

// handleApplicationListLoaded processes the result of loading the application registry
func (m *MenuModel) handleApplicationListLoaded(msg applicationListLoadedMsg) tea.Cmd {
	if msg.error != "" {
		m.errorMessage = msg.error
		m.SetInterfaceMode(ModeError)
		return nil
	}

	m.registeredApps = msg.apps

	// Ensure selected index is valid
	if len(m.registeredApps) == 0 {
		m.selectedAppIndex = 0
	} else if m.selectedAppIndex >= len(m.registeredApps) {
		m.selectedAppIndex = len(m.registeredApps) - 1
	}

	m.lastHealthUpdate = time.Now()

	// Trigger health status refresh for all applications
	return m.refreshAllApplicationHealth()
}

// handleHealthStatusUpdate processes health status updates from background monitoring
func (m *MenuModel) handleHealthStatusUpdate(msg healthStatusUpdatedMsg) {
	if msg.error != "" {
		m.healthUpdateError = msg.error
		// Create error health status for display
		m.appHealthStatus[msg.appName] = &interfaces.AppHealth{
			Name:        msg.appName,
			Status:      "error",
			LastChecked: time.Now(),
			Error:       msg.error,
		}
	} else {
		m.appHealthStatus[msg.appName] = msg.health
		m.healthUpdateError = ""
	}

	m.lastHealthUpdate = time.Now()
}

// handleHealthRefreshRequest initiates a comprehensive health refresh for all applications
func (m *MenuModel) handleHealthRefreshRequest() tea.Cmd {
	m.statusMessage = "Refreshing application health status..."
	return m.refreshAllApplicationHealth()
}

// handleConnectionInitiated processes the start of a connection attempt
func (m *MenuModel) handleConnectionInitiated(msg connectionInitiatedMsg) tea.Cmd {
	m.SetInterfaceMode(ModeLoading)
	m.statusMessage = fmt.Sprintf("Connecting to %s...", msg.appName)

	// Proceed with the actual connection attempt
	return m.performConnection(msg.appName, msg.profile)
}

// handleConnectionCompleted processes the completion of a connection attempt
func (m *MenuModel) handleConnectionCompleted(msg connectionCompletedMsg) (tea.Model, tea.Cmd) {
	if !msg.success {
		m.SetInterfaceMode(ModeError)
		m.errorMessage = fmt.Sprintf("Connection to %s failed: %s", msg.appName, msg.error)
		return m, nil
	}

	// Connection successful - transition to Application Mode
	// This would typically return a different model or trigger application mode
	// For now, we'll show a success message and remain in menu mode
	m.SetInterfaceMode(ModeNormal)
	m.statusMessage = fmt.Sprintf("Successfully connected to %s (%s v%s)",
		msg.appName, msg.specResp.AppName, msg.specResp.AppVersion)

	// In a real implementation, this would transition to Application Mode
	// return applicationModeModel, tea.ClearScreen

	return m, tea.Cmd(func() tea.Msg {
		return statusUpdateMsg{
			message: "Connection established - transitioning to Application Mode",
			timeout: 2 * time.Second,
		}
	})
}

// handleErrorDisplay processes error display requests
func (m *MenuModel) handleErrorDisplay(msg errorDisplayMsg) {
	m.errorMessage = msg.message
	m.SetInterfaceMode(ModeError)
}

// handleStatusUpdate processes status message updates
func (m *MenuModel) handleStatusUpdate(msg statusUpdateMsg) {
	m.statusMessage = msg.message

	// Clear status message after timeout if specified
	if msg.timeout > 0 {
		// This would typically use a timer command in a real implementation
		time.AfterFunc(msg.timeout, func() {
			m.statusMessage = ""
		})
	}
}

// Navigation and selection methods

// navigateApplicationList moves the selection within the application list
func (m *MenuModel) navigateApplicationList(direction int) tea.Cmd {
	if len(m.registeredApps) == 0 {
		return nil
	}

	newIndex := m.selectedAppIndex + direction

	// Handle wrapping
	if newIndex < 0 {
		newIndex = len(m.registeredApps) - 1
	} else if newIndex >= len(m.registeredApps) {
		newIndex = 0
	}

	m.selectedAppIndex = newIndex
	m.recordNavigation(FocusApplicationList, FocusApplicationList, "list_navigation")

	return nil
}

// selectApplicationByNumber selects an application by its numbered position
func (m *MenuModel) selectApplicationByNumber(number int) tea.Cmd {
	index := number - 1 // Convert to zero-based index

	if index >= 0 && index < len(m.registeredApps) {
		m.selectedAppIndex = index
		m.recordNavigation(FocusApplicationList, FocusApplicationList,
			fmt.Sprintf("numbered_selection_%d", number))

		// Auto-connect if the number key is pressed
		return m.ConnectToSelectedApplication()
	}

	return nil
}

// Command execution methods

// initiateApplicationRegistration starts the application registration workflow
func (m *MenuModel) initiateApplicationRegistration() tea.Cmd {
	m.ShowConfirmation(
		"Register New Application",
		"This will start the application registration wizard.",
		[]string{"Continue", "Cancel"},
		func() tea.Cmd {
			m.SetInterfaceMode(ModeRegistration)
			// In a real implementation, this would launch registration workflow
			return m.showError("Application registration not yet implemented")
		},
		func() tea.Cmd {
			m.SetInterfaceMode(ModeNormal)
			return nil
		},
	)
	return nil
}

// initiateProfileEdit starts the profile editing workflow
func (m *MenuModel) initiateProfileEdit() tea.Cmd {
	if len(m.registeredApps) == 0 {
		return m.showError("No applications registered to edit")
	}

	selectedApp, err := m.GetSelectedApplication()
	if err != nil {
		return m.showError(fmt.Sprintf("Cannot edit profile: %s", err.Error()))
	}

	m.ShowConfirmation(
		"Edit Application Profile",
		fmt.Sprintf("Edit profile for %s (%s)?", selectedApp.Name, selectedApp.Profile),
		[]string{"Edit", "Cancel"},
		func() tea.Cmd {
			m.SetInterfaceMode(ModeProfileEdit)
			// In a real implementation, this would launch profile editor
			return m.showError("Profile editing not yet implemented")
		},
		func() tea.Cmd {
			m.SetInterfaceMode(ModeNormal)
			return nil
		},
	)
	return nil
}

// handleExitRequest processes application exit requests with confirmation
func (m *MenuModel) handleExitRequest() tea.Cmd {
	m.ShowConfirmation(
		"Exit Console",
		"Are you sure you want to exit the Universal Application Console?",
		[]string{"Exit", "Cancel"},
		func() tea.Cmd {
			return tea.Quit
		},
		func() tea.Cmd {
			m.SetInterfaceMode(ModeNormal)
			return nil
		},
	)
	return nil
}

// executeConfirmationChoice executes the selected confirmation option
func (m *MenuModel) executeConfirmationChoice() tea.Cmd {
	if m.confirmationState == nil {
		m.SetInterfaceMode(ModeNormal)
		return nil
	}

	selectedIndex := m.confirmationState.SelectedIndex

	// Execute the appropriate callback based on selection
	if selectedIndex == 0 && m.confirmationState.OnConfirm != nil {
		return m.confirmationState.OnConfirm()
	} else if selectedIndex > 0 && m.confirmationState.OnCancel != nil {
		return m.confirmationState.OnCancel()
	}

	m.SetInterfaceMode(ModeNormal)
	return nil
}

// cancelConfirmation cancels the current confirmation dialog
func (m *MenuModel) cancelConfirmation() tea.Cmd {
	if m.confirmationState != nil && m.confirmationState.OnCancel != nil {
		return m.confirmationState.OnCancel()
	}

	m.SetInterfaceMode(ModeNormal)
	return nil
}

// Health monitoring methods

// refreshAllApplicationHealth initiates health checks for all registered applications
func (m *MenuModel) refreshAllApplicationHealth() tea.Cmd {
	if len(m.registeredApps) == 0 {
		return nil
	}

	commands := make([]tea.Cmd, 0, len(m.registeredApps))

	for _, app := range m.registeredApps {
		commands = append(commands, m.refreshHealthForApp(app.Name))
	}

	return tea.Batch(commands...)
}
