// In internal/ui/menu/update.go

package menu

import (
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
)

// Update handles messages and updates the model state.
func (m *MenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Clear error on any key press, but not while connecting
		if m.err != nil && !m.isConnecting {
			m.err = nil
		}
		// Don't process key presses while a connection is in progress
		if m.isConnecting {
			return m, nil
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
		if !m.isConnecting {
			cmds = append(cmds, m.updateHealth())
		}
		cmds = append(cmds, tick())
	// This case is necessary if we are not handling character input inside the KeyMsg case
	// for the text input. We let the default bubble tea update handle non-key messages.
	default:
		if m.focusState == FocusInput && !m.isConnecting {
			m.quickConnectInput, cmd = m.quickConnectInput.Update(msg)
			cmds = append(cmds, cmd)
		}
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
		return m.quickConnectInput.Focus()
	default:
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
	// Check for keys we want to handle specially
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
		return nil // Do nothing if input is empty
	case "tab", "shift+tab":
		m.focusState = FocusList
		m.quickConnectInput.Blur()
		return nil
	default:
		// If it's not a special key, pass it to the textinput component for character entry.
		var cmd tea.Cmd
		m.quickConnectInput, cmd = m.quickConnectInput.Update(msg)
		return cmd
	}
}
