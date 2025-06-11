// Package menu implements the visual presentation for Console Menu Mode.
// This file renders the registered applications list with health status indicators,
// the quick connect interface, and command options using Lipgloss styling,
// matching the design specified in cli_design_v2.md.
package menu

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/universal-console/console/internal/ui/components"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#CBA6F7")).
			Padding(1, 2)

	focusedBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color("#89B4FA")).
			Padding(1, 2)

	listItemStyle    = lipgloss.NewStyle().PaddingLeft(1)
	focusedItemStyle = lipgloss.NewStyle().
				PaddingLeft(1).
				Foreground(lipgloss.Color("#1e1e2e")).
				Background(lipgloss.Color("#FAB387"))

	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086")).Padding(1, 0)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F38BA8")).
			Bold(true)
)

// View renders the UI for the menu model.
func (m *MenuModel) View() string {
	var s strings.Builder

	// Title
	s.WriteString(titleStyle.Width(m.width).Render("Universal Application Console v2.0"))
	s.WriteString("\n\n")

	// If connecting, show a simple status message.
	if m.isConnecting {
		msg := components.RenderStatus("running", m.statusMessage)
		s.WriteString(boxStyle.Render(msg))
		s.WriteString("\n")
		return s.String()
	}

	// Registered Apps List
	s.WriteString(m.viewAppList())
	s.WriteString("\n\n")

	// Quick Connect
	s.WriteString(m.viewQuickConnect())
	s.WriteString("\n\n")

	// Footer / Help
	s.WriteString(helpStyle.Render("Commands: [Enter] Connect | [Tab] Navigate | [Q]uit"))

	// Error message
	if m.err != nil {
		s.WriteString("\n\n")
		s.WriteString(errorStyle.Render("Error: " + m.err.Error()))
	}

	return s.String()
}

// viewAppList renders the list of registered applications.
func (m *MenuModel) viewAppList() string {
	var listItems []string
	listTitle := "Registered Applications"

	if len(m.registeredApps) == 0 {
		listItems = append(listItems, helpStyle.Render("No applications registered. Use the CLI to register one."))
	} else {
		for i, app := range m.registeredApps {
			health, ok := m.appHealth[app.Name]
			status := "unknown"
			if ok {
				status = health.Status
			}

			// Choose the right status component
			var statusRendered string
			switch status {
			case "ready":
				statusRendered = components.RenderStatus("success", "Ready")
			case "offline":
				statusRendered = components.RenderStatus("error", "Offline")
			case "error":
				statusRendered = components.RenderStatus("error", "Error")
			default:
				statusRendered = components.RenderStatus("pending", "Checking...")
			}

			itemStr := fmt.Sprintf("[%d] %s (%s) - %s", i+1, app.Name, app.Profile, statusRendered)

			if m.focusState == FocusList && i == m.selectedIndex {
				listItems = append(listItems, focusedItemStyle.Render(itemStr))
			} else {
				listItems = append(listItems, listItemStyle.Render(itemStr))
			}
		}
	}

	listContent := lipgloss.JoinVertical(lipgloss.Left, listItems...)

	style := boxStyle
	if m.focusState == FocusList {
		style = focusedBoxStyle
	}

	return style.Render(lipgloss.JoinVertical(lipgloss.Left, lipgloss.NewStyle().Bold(true).Render(listTitle), listContent))
}

// viewQuickConnect renders the quick connect input box.
func (m *MenuModel) viewQuickConnect() string {
	boxTitle := "Quick Connect"
	inputView := m.quickConnectInput.View()

	style := boxStyle
	if m.focusState == FocusInput {
		style = focusedBoxStyle
	}

	return style.Render(lipgloss.JoinVertical(lipgloss.Left, lipgloss.NewStyle().Bold(true).Render(boxTitle), "Host: "+inputView))
}
