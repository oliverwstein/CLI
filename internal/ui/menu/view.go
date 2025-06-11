// Package menu implements visual presentation for Console Menu Mode in the Universal Application Console.
// This file renders the complete interface layout with health status indicators, application lists,
// and interactive components using Lipgloss styling to deliver the professional interface
// specified in section 3.2.1 of the design specification.
package menu

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/universal-console/console/internal/interfaces"
)

// Styling definitions for consistent visual presentation
var (
	// Header styling for application title and version information
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1).
			MarginBottom(1)

	// Section border styling for main interface components
	sectionStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#874BFD")).
			Padding(1).
			MarginBottom(1)

	// Application list item styling with focus indication
	appItemStyle = lipgloss.NewStyle().
			Padding(0, 1)

	appItemFocusedStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Background(lipgloss.Color("#874BFD")).
				Foreground(lipgloss.Color("#FFFFFF"))

	// Health status indicator styling
	healthReadyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#28a745")).
				Bold(true)

	healthOfflineStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#dc3545")).
				Bold(true)

	healthErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#fd7e14")).
				Bold(true)

	healthUnknownStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#6c757d")).
				Bold(true)

	// Input field styling for quick connect functionality
	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#874BFD")).
			Padding(0, 1)

	inputFocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("#FF7CCB")).
				Padding(0, 1)

	// Button styling for interactive elements
	buttonStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#874BFD")).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 2).
			MarginLeft(1)

	buttonFocusedStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#FF7CCB")).
				Foreground(lipgloss.Color("#FFFFFF")).
				Padding(0, 2).
				MarginLeft(1)

	// Command options styling
	commandStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#874BFD"))

	commandFocusedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF7CCB")).
				Bold(true)

	// Error and status message styling
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#dc3545")).
			Bold(true).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#dc3545")).
			Padding(1).
			MarginBottom(1)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#28a745")).
			Italic(true).
			MarginBottom(1)

	// Confirmation dialog styling
	confirmationStyle = lipgloss.NewStyle().
				Border(lipgloss.ThickBorder()).
				BorderForeground(lipgloss.Color("#FF7CCB")).
				Background(lipgloss.Color("#1a1a1a")).
				Padding(2).
				MarginTop(2).
				MarginBottom(2)

	confirmationTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				MarginBottom(1)

	confirmationOptionStyle = lipgloss.NewStyle().
				Padding(0, 1)

	confirmationOptionFocusedStyle = lipgloss.NewStyle().
					Padding(0, 1).
					Background(lipgloss.Color("#FF7CCB")).
					Foreground(lipgloss.Color("#FFFFFF"))
)

// View implements the tea.Model interface to render the complete Console Menu Mode interface
func (m *MenuModel) View() string {
	// Handle different interface modes with appropriate rendering
	switch m.interfaceMode {
	case ModeConfirmation:
		return m.renderWithConfirmation()
	case ModeError:
		return m.renderWithError()
	case ModeLoading:
		return m.renderWithLoading()
	default:
		return m.renderNormalMode()
	}
}

// renderNormalMode renders the standard Console Menu Mode interface
func (m *MenuModel) renderNormalMode() string {
	var sections []string

	// Render header section with application title and version
	sections = append(sections, m.renderHeader())

	// Render registered applications section
	sections = append(sections, m.renderApplicationsSection())

	// Render quick connect section
	sections = append(sections, m.renderQuickConnectSection())

	// Render command options section
	sections = append(sections, m.renderCommandsSection())

	// Render status messages if present
	if statusSection := m.renderStatusSection(); statusSection != "" {
		sections = append(sections, statusSection)
	}

	return strings.Join(sections, "\n")
}

// renderWithConfirmation renders the interface with an overlay confirmation dialog
func (m *MenuModel) renderWithConfirmation() string {
	baseInterface := m.renderNormalMode()

	if m.confirmationState == nil {
		return baseInterface
	}

	confirmationDialog := m.renderConfirmationDialog()

	// Overlay the confirmation dialog on the base interface
	return baseInterface + "\n" + confirmationDialog
}

// renderWithError renders the interface with error information prominently displayed
func (m *MenuModel) renderWithError() string {
	var sections []string

	sections = append(sections, m.renderHeader())

	// Render error message prominently
	if m.errorMessage != "" {
		errorSection := errorStyle.Render("Error: " + m.errorMessage)
		sections = append(sections, errorSection)
	}

	sections = append(sections, m.renderApplicationsSection())
	sections = append(sections, m.renderQuickConnectSection())
	sections = append(sections, m.renderCommandsSection())
	sections = append(sections, "\nPress Enter to continue...")

	return strings.Join(sections, "\n")
}

// renderWithLoading renders the interface with loading indicators
func (m *MenuModel) renderWithLoading() string {
	var sections []string

	sections = append(sections, m.renderHeader())

	// Render loading status
	if m.statusMessage != "" {
		loadingSection := statusStyle.Render("⏳ " + m.statusMessage)
		sections = append(sections, loadingSection)
	}

	sections = append(sections, m.renderApplicationsSection())
	sections = append(sections, m.renderQuickConnectSection())
	sections = append(sections, m.renderCommandsSection())
	sections = append(sections, "\nPress Esc to cancel...")

	return strings.Join(sections, "\n")
}

// renderHeader creates the application header with title and version information
func (m *MenuModel) renderHeader() string {
	title := "Universal Application Console v2.0"
	return headerStyle.Render(title)
}

// renderApplicationsSection creates the registered applications list with health indicators
func (m *MenuModel) renderApplicationsSection() string {
	sectionTitle := "Registered Applications"

	if len(m.registeredApps) == 0 {
		emptyMessage := "No applications registered. Use [R] to register your first application."
		content := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6c757d")).
			Italic(true).
			Render(emptyMessage)

		return sectionStyle.
			BorderTop(true).
			BorderTopForeground(lipgloss.Color("#874BFD")).
			Render(fmt.Sprintf("┌─ %s ──────────────────────────────────────────────────────┐\n│ %s │\n└─────────────────────────────────────────────────────────────────┘",
				sectionTitle, content))
	}

	var appLines []string

	for i, app := range m.registeredApps {
		appLine := m.renderApplicationItem(i, app)
		appLines = append(appLines, appLine)
	}

	content := strings.Join(appLines, "\n")

	return sectionStyle.
		BorderTop(true).
		BorderTopForeground(lipgloss.Color("#874BFD")).
		Render(fmt.Sprintf("┌─ %s ──────────────────────────────────────────────────────┐\n│ %s │\n└─────────────────────────────────────────────────────────────────┘",
			sectionTitle, content))
}

// renderApplicationItem creates a single application list item with health status
func (m *MenuModel) renderApplicationItem(index int, app interfaces.RegisteredApp) string {
	number := fmt.Sprintf("[%d]", index+1)

	// Get profile information for display
	profile, err := m.configManager.LoadProfile(app.Profile)
	hostInfo := app.Profile
	if err == nil {
		hostInfo = profile.Host
	}

	// Format basic application information
	appInfo := fmt.Sprintf("%s (%s)", app.Name, hostInfo)

	// Get health status with appropriate styling
	healthText := m.renderHealthStatus(app.Name)

	// Construct the complete application line
	fullLine := fmt.Sprintf("%-4s %-50s - %s", number, appInfo, healthText)

	// Apply focus styling if this item is selected
	if index == m.selectedAppIndex && m.focusState == FocusApplicationList {
		return appItemFocusedStyle.Render(fullLine)
	}

	return appItemStyle.Render(fullLine)
}

// renderHealthStatus creates styled health status text for an application
func (m *MenuModel) renderHealthStatus(appName string) string {
	health, exists := m.appHealthStatus[appName]
	if !exists {
		return healthUnknownStyle.Render("Unknown")
	}

	var statusText string
	var style lipgloss.Style

	switch health.Status {
	case "ready":
		statusText = "Ready"
		style = healthReadyStyle
	case "offline":
		statusText = "Offline"
		style = healthOfflineStyle
	case "error":
		statusText = "Error"
		style = healthErrorStyle
	default:
		statusText = "Unknown"
		style = healthUnknownStyle
	}

	// Add response time information if available and details are enabled
	if m.showHealthDetails && health.ResponseTime > 0 {
		responseTime := health.ResponseTime.Truncate(time.Millisecond)
		statusText += fmt.Sprintf(" (%v)", responseTime)
	}

	return style.Render(statusText)
}

// renderQuickConnectSection creates the quick connect input interface
func (m *MenuModel) renderQuickConnectSection() string {
	sectionTitle := "Quick Connect"

	// Render the input field with appropriate focus styling
	var inputField string
	if m.focusState == FocusQuickConnect {
		inputField = inputFocusedStyle.Render(m.quickConnectInput.View())
	} else {
		inputField = inputStyle.Render(m.quickConnectInput.View())
	}

	// Render the connect button with appropriate focus styling
	var connectButton string
	if m.focusState == FocusConnectButton {
		connectButton = buttonFocusedStyle.Render("Connect")
	} else {
		connectButton = buttonStyle.Render("Connect")
	}

	// Construct the quick connect line
	connectLine := fmt.Sprintf("Host: %s %s", inputField, connectButton)

	return sectionStyle.
		BorderTop(true).
		BorderTopForeground(lipgloss.Color("#874BFD")).
		Render(fmt.Sprintf("┌─ %s ────────────────────────────────────────────────────┐\n│ %s │\n└─────────────────────────────────────────────────────────────────┘",
			sectionTitle, connectLine))
}

// renderCommandsSection creates the command options interface
func (m *MenuModel) renderCommandsSection() string {
	var commands []string

	// Register application command
	registerCmd := "[R]egister App"
	if m.focusState == FocusCommandOptions {
		registerCmd = commandFocusedStyle.Render(registerCmd)
	} else {
		registerCmd = commandStyle.Render(registerCmd)
	}
	commands = append(commands, registerCmd)

	// Edit profile command
	editCmd := "[E]dit Profile"
	if m.focusState == FocusCommandOptions {
		editCmd = commandFocusedStyle.Render(editCmd)
	} else {
		editCmd = commandStyle.Render(editCmd)
	}
	commands = append(commands, editCmd)

	// Quit command
	quitCmd := "[Q]uit"
	if m.focusState == FocusCommandOptions {
		quitCmd = commandFocusedStyle.Render(quitCmd)
	} else {
		quitCmd = commandStyle.Render(quitCmd)
	}
	commands = append(commands, quitCmd)

	commandsLine := "Commands: " + strings.Join(commands, " | ")

	return commandsLine
}

// renderStatusSection creates status and error message display
func (m *MenuModel) renderStatusSection() string {
	var statusLines []string

	// Render status messages
	if m.statusMessage != "" {
		statusLines = append(statusLines, statusStyle.Render(m.statusMessage))
	}

	// Render health update information
	if !m.lastHealthUpdate.IsZero() {
		healthInfo := fmt.Sprintf("Last health update: %s",
			m.lastHealthUpdate.Format("15:04:05"))
		if m.healthUpdateError != "" {
			healthInfo += fmt.Sprintf(" (Error: %s)", m.healthUpdateError)
		}
		statusLines = append(statusLines,
			lipgloss.NewStyle().Foreground(lipgloss.Color("#6c757d")).Render(healthInfo))
	}

	// Render navigation help for new users
	if len(statusLines) == 0 {
		helpText := "Use ↑↓ to navigate, Tab to switch sections, Enter to connect, Ctrl+R to refresh"
		statusLines = append(statusLines,
			lipgloss.NewStyle().Foreground(lipgloss.Color("#6c757d")).Render(helpText))
	}

	if len(statusLines) > 0 {
		return "\n" + strings.Join(statusLines, "\n")
	}

	return ""
}

// renderConfirmationDialog creates an overlay confirmation dialog
func (m *MenuModel) renderConfirmationDialog() string {
	if m.confirmationState == nil {
		return ""
	}

	// Render dialog title
	title := confirmationTitleStyle.Render(m.confirmationState.Title)

	// Render dialog message
	message := m.confirmationState.Message

	// Render options with focus indication
	var optionLines []string
	for i, option := range m.confirmationState.Options {
		optionText := fmt.Sprintf("[%d] %s", i+1, option)

		if i == m.confirmationState.SelectedIndex {
			optionLines = append(optionLines, confirmationOptionFocusedStyle.Render(optionText))
		} else {
			optionLines = append(optionLines, confirmationOptionStyle.Render(optionText))
		}
	}

	// Construct complete dialog content
	dialogContent := fmt.Sprintf("%s\n\n%s\n\n%s\n\nUse ↑↓ to select, Enter to confirm, Esc to cancel",
		title, message, strings.Join(optionLines, "\n"))

	return confirmationStyle.Render(dialogContent)
}
