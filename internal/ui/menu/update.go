// Package menu implements user input processing and state management for Console Menu Mode.
// This file contains the Bubble Tea update function that processes keyboard input for
// numbered application selection, Tab navigation, Enter key for connection initiation,
// and meta commands for application registration and profile management.
package menu

import (
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
)

// Update handles messages and updates the model state.
func (m *MenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	// If we are connecting, ignore all other input.
	if m.isConnecting {
		switch msg := msg.(type) {
		case connectionResultMsg:
			m.isConnecting = false
			if msg.err != nil {
				m.err = msg.err
				return m, nil
			}
			// On successful connection, switch to the Application Mode model.
			return msg.model, msg.model.Init()
		default:
			return m, nil
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Clear error on any key press
		if m.err != nil {
			m.err = nil
		}

		switch m.focusState {
		case FocusList:
			cmd = m.handleListKeys(msg)
		case FocusInput:
			cmd = m.handleInputKeys(msg)
		}
		cmds = append(cmds, cmd)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	// Handle our custom messages
	case appsReloadedMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.registeredApps = msg.apps
		}

	case healthStatusUpdatedMsg:
		for name, health := range msg.health {
			m.appHealth[name] = health
		}

	case tickMsg:
		cmds = append(cmds, m.updateHealth(), tick()) // Re-queue the tick

	case connectionResultMsg:
		m.isConnecting = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		// Successful connection, switch to AppModel.
		return msg.model.Update(nil) // Pass control to the new model
	}

	// Update the text input if it's focused
	if m.focusState == FocusInput {
		m.quickConnectInput, cmd = m.quickConnectInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// handleListKeys processes key presses when the application list is focused.
func (m *MenuModel) handleListKeys(msg tea.KeyMsg) tea.Cmd {
	switch key := msg.String(); key {
	case "ctrl+c", "q":
		return tea.Quit

	case "up", "k":
		if m.selectedIndex > 0 {
			m.selectedIndex--
		}

	case "down", "j":
		if m.selectedIndex < len(m.registeredApps)-1 {
			m.selectedIndex++
		}

	case "enter":
		if len(m.registeredApps) > 0 && m.selectedIndex < len(m.registeredApps) {
			m.isConnecting = true
			m.statusMessage = "Connecting to " + m.registeredApps[m.selectedIndex].Name + "..."
			m.err = nil
			return m.attemptConnection(m.registeredApps[m.selectedIndex].Profile, "")
		}

	case "tab":
		m.focusState = FocusInput
		m.quickConnectInput.Focus()

	default:
		// Allow connecting via number keys
		if i, err := strconv.Atoi(key); err == nil {
			if i >= 1 && i <= len(m.registeredApps) {
				m.selectedIndex = i - 1
				m.isConnecting = true
				m.statusMessage = "Connecting to " + m.registeredApps[m.selectedIndex].Name + "..."
				m.err = nil
				return m.attemptConnection(m.registeredApps[m.selectedIndex].Profile, "")
			}
		}
	}
	return nil
}

// handleInputKeys processes key presses when the quick connect input is focused.
func (m *MenuModel) handleInputKeys(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "ctrl+c", "q":
		return tea.Quit

	case "enter":
		host := m.quickConnectInput.Value()
		if host != "" {
			m.isConnecting = true
			m.statusMessage = "Connecting to " + host + "..."
			m.err = nil
			return m.attemptConnection("", host)
		}

	case "tab", "shift+tab":
		m.focusState = FocusList
		m.quickConnectInput.Blur()
	}
	return nil
}
