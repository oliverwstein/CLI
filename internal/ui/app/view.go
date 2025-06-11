// Package app implements visual presentation for Application Mode in the Universal Application Console.
// This file renders the application header with connection status, scrolling history pane with rich content,
// actions pane with appropriate styling for different action types, and input component with suggestion dropdown.
// The view creates the sophisticated interface layout demonstrated in the design specification examples.
package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/universal-console/console/internal/interfaces"
	"github.com/universal-console/console/internal/ui/components"
)

// Styling definitions for sophisticated visual presentation
var (
	// Header styling for application title and connection information
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1).
			Width(0) // Full width

	// History pane styling for conversational flow
	historyPaneStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("#6C7086")).
				Padding(1).
				Height(0) // Will be set dynamically

	// User command styling with "YOU>" prefix
	userCommandStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#89B4FA"))

	// Application response styling with "APP>" prefix
	appResponseStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#A6E3A1"))

	// Content styling for rich content rendering
	contentStyle = lipgloss.NewStyle().
			MarginLeft(6) // Indent content under APP> prefix

	// Collapsible section styling
	collapsibleHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#F38BA8"))

	collapsibleHeaderFocusedStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Color("#FFFFFF")).
					Background(lipgloss.Color("#F38BA8")).
					Padding(0, 1)

	collapsibleContentStyle = lipgloss.NewStyle().
				MarginLeft(2).
				Foreground(lipgloss.Color("#CDD6F4"))

	// Input component styling with focus indication
	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#6C7086")).
			Padding(0, 1).
			Width(0) // Will be set dynamically

	inputFocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.ThickBorder()).
				BorderForeground(lipgloss.Color("#89B4FA")).
				Padding(0, 1).
				Width(0) // Will be set dynamically

	// Status and error message styling
	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A6E3A1")).
			Italic(true)

	// Connection status indicator styling
	connectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A6E3A1")).
			Bold(true)

	disconnectedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F38BA8")).
				Bold(true)
)

// View implements the tea.Model interface to render the complete Application Mode interface
func (m *AppModel) View() string {
	// Set component widths before calculating layout
	m.actionsPane.SetWidth(m.terminalWidth)
	m.workflowManager.SetWidth(m.terminalWidth)

	var sections []string

	// Render header with connection status and application information
	sections = append(sections, m.renderHeader())

	// Render workflow breadcrumbs if present
	if m.workflowManager.IsActive() {
		sections = append(sections, m.workflowManager.View())
	}

	// Render main content history pane
	sections = append(sections, m.renderHistoryPane())

	// Render actions pane if actions are available
	if m.actionsPane.IsVisible() {
		sections = append(sections, m.actionsPane.View())
	}

	// Render input component
	sections = append(sections, m.renderInputComponent())

	// Render status messages if present
	if statusSection := m.renderStatusSection(); statusSection != "" {
		sections = append(sections, statusSection)
	}

	return strings.Join(sections, "\n")
}

// renderHeader creates the application header with connection status and metadata
func (m *AppModel) renderHeader() string {
	var headerText string

	if m.connected && m.appName != "" {
		// Connected state with application information
		headerText = fmt.Sprintf("[%s", m.appName)
		if m.appVersion != "" {
			headerText += fmt.Sprintf(" v%s", m.appVersion)
		}
		headerText += "] - "

		// Connection status indicator
		connectionStatus := connectedStyle.Render(fmt.Sprintf("Connected to %s", m.profile.Host))
		headerText += connectionStatus
	} else if m.connectionError != "" {
		// Error state
		headerText = fmt.Sprintf("[%s] - ", m.profile.Name)
		headerText += disconnectedStyle.Render(fmt.Sprintf("Connection Error: %s", m.connectionError))
	} else {
		// Default state
		headerText = fmt.Sprintf("[%s] - ", m.profile.Name)
		headerText += disconnectedStyle.Render("Disconnected")
	}

	// Add protocol version if available
	if m.protocolVersion != "" {
		headerText += fmt.Sprintf(" (Protocol %s)", m.protocolVersion)
	}

	return headerStyle.Width(m.terminalWidth).Render(headerText)
}

// renderHistoryPane creates the scrolling content area with command history and responses
func (m *AppModel) renderHistoryPane() string {
	var height int
	if m.terminalHeight > 0 {
		actionsHeight := lipgloss.Height(m.actionsPane.View())
		workflowHeight := lipgloss.Height(m.workflowManager.View())
		usedHeight := m.headerHeight + m.inputHeight + actionsHeight + workflowHeight + 2
		height = m.terminalHeight - usedHeight
		if height < 5 {
			height = 5
		}
	} else {
		height = 20 // Default height
	}

	if len(m.commandHistory) == 0 {
		emptyMessage := "Connected and ready. Type a command to get started."
		return historyPaneStyle.
			Height(height).
			Width(m.terminalWidth - 4).
			Render(statusStyle.Render(emptyMessage))
	}

	var contentLines []string

	for _, entry := range m.commandHistory {
		contentLines = append(contentLines, m.renderHistoryEntry(entry)...)
	}

	// Apply scrolling offset
	if m.scrollOffset > 0 && m.scrollOffset < len(contentLines) {
		endIndex := m.scrollOffset + height
		if endIndex > len(contentLines) {
			endIndex = len(contentLines)
		}
		contentLines = contentLines[m.scrollOffset:endIndex]
	} else if len(contentLines) > height {
		// Show most recent content
		contentLines = contentLines[len(contentLines)-height:]
	}

	content := strings.Join(contentLines, "\n")

	return historyPaneStyle.
		Height(height).
		Width(m.terminalWidth - 4).
		Render(content)
}

// renderHistoryEntry creates the visual representation of a single history entry
func (m *AppModel) renderHistoryEntry(entry HistoryEntry) []string {
	var lines []string

	// Render user command with timestamp if enabled
	commandPrefix := "YOU>"
	if m.showTimestamps {
		timestamp := entry.Timestamp.Format("15:04:05")
		commandPrefix = fmt.Sprintf("[%s] YOU>", timestamp)
	}

	commandLine := userCommandStyle.Render(commandPrefix) + " " + entry.Command
	lines = append(lines, commandLine)

	// Render application response
	if entry.Error != "" {
		// Error response
		responsePrefix := "APP>"
		if m.showTimestamps {
			responsePrefix = fmt.Sprintf("[%s] APP>", entry.Timestamp.Format("15:04:05"))
		}
		errorLine := appResponseStyle.Render(responsePrefix) + " " + components.RenderStatus("error", entry.Error)
		lines = append(lines, errorLine)
	} else if entry.Response != nil {
		// Successful response
		responseLines := m.renderResponse(entry.Response, entry.Rendered)
		lines = append(lines, responseLines...)
	}

	// Add spacing between entries
	lines = append(lines, "")

	return lines
}

// renderResponse creates the visual representation of an application response
func (m *AppModel) renderResponse(response *interfaces.CommandResponse, rendered []interfaces.RenderedContent) []string {
	var lines []string

	responsePrefix := "APP>"
	if m.showTimestamps {
		responsePrefix = fmt.Sprintf("[%s] APP>", time.Now().Format("15:04:05"))
	}

	// Handle simple text responses
	if response.Response.Type == "text" {
		if textContent, ok := response.Response.Content.(string); ok {
			responseLine := appResponseStyle.Render(responsePrefix) + " " + textContent
			lines = append(lines, responseLine)
			return lines
		}
	}

	// Handle structured content responses
	if len(rendered) > 0 {
		// Add response prefix
		lines = append(lines, appResponseStyle.Render(responsePrefix))

		// Render structured content
		for _, content := range rendered {
			contentLines := m.renderStructuredContent(content)
			lines = append(lines, contentLines...)
		}
	}

	return lines
}

// renderStructuredContent processes individual structured content elements
func (m *AppModel) renderStructuredContent(content interfaces.RenderedContent) []string {
	var lines []string

	if content.Expanded != nil {
		// Collapsible content
		lines = append(lines, m.renderCollapsibleContent(content)...)
	} else {
		// Regular content
		if content.Text != "" {
			styledContent := contentStyle.Render(content.Text)
			lines = append(lines, styledContent)
		}
	}

	return lines
}

// renderCollapsibleContent creates expandable/collapsible content sections
func (m *AppModel) renderCollapsibleContent(content interfaces.RenderedContent) []string {
	var lines []string

	// Determine if this section is focused
	isFocused := m.focusState == FocusExpandable && m.focusedSectionID == content.ID

	// Create header with expand/collapse indicator
	var indicator string
	if content.Expanded != nil && *content.Expanded {
		indicator = "▼"
	} else {
		indicator = "▶"
	}

	headerText := fmt.Sprintf("%s [%s] %s", indicator, "Toggle", content.Text)

	var headerLine string
	if isFocused {
		headerLine = contentStyle.Render(collapsibleHeaderFocusedStyle.Render(headerText))
	} else {
		headerLine = contentStyle.Render(collapsibleHeaderStyle.Render(headerText))
	}

	lines = append(lines, headerLine)

	// Render content if expanded
	if content.Expanded != nil && *content.Expanded {
		// This would contain the nested content
		// For now, we'll show a placeholder
		expandedContent := collapsibleContentStyle.Render("• Expanded content would appear here")
		lines = append(lines, contentStyle.Render(expandedContent))
	}

	return lines
}

// renderInputComponent creates the command input interface
func (m *AppModel) renderInputComponent() string {
	inputWidth := m.terminalWidth - 6
	if inputWidth < 10 {
		inputWidth = 10
	}

	var inputBox string
	if m.focusState == FocusInput {
		inputBox = inputFocusedStyle.Width(inputWidth).Render(m.commandInput.View())
	} else {
		inputBox = inputStyle.Width(inputWidth).Render(m.commandInput.View())
	}

	// Add helpful hints below the input
	var hints []string
	if m.focusState == FocusInput {
		hints = append(hints, "Ctrl+↑/↓ for history")
		if m.actionsPane.IsVisible() {
			hints = append(hints, "1-9 for quick actions")
		}
		hints = append(hints, "Tab to navigate")
	}

	result := inputBox
	if len(hints) > 0 {
		hintText := strings.Join(hints, " • ")
		result += "\n" + statusStyle.Render(hintText)
	}

	return result
}

// renderStatusSection creates status messages and connection statistics
func (m *AppModel) renderStatusSection() string {
	var statusLines []string

	// Render error messages
	if m.errorMessage != "" {
		statusLines = append(statusLines, components.RenderStatus("error", m.errorMessage))
	}

	// Render status messages
	if m.statusMessage != "" {
		statusLines = append(statusLines, components.RenderStatus("info", m.statusMessage))
	}

	// Render connection statistics if enabled
	if m.showTimestamps && m.connectionStats.TotalCommands > 0 {
		statsText := fmt.Sprintf("Commands: %d/%d successful • Avg response: %v",
			m.connectionStats.SuccessfulCommands,
			m.connectionStats.TotalCommands,
			m.connectionStats.AverageResponseTime.Truncate(time.Millisecond))
		statusLines = append(statusLines, components.RenderStatus("info", statsText))
	}

	if len(statusLines) > 0 {
		return "\n" + strings.Join(statusLines, "\n")
	}

	return ""
}
