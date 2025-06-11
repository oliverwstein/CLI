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

	// Actions pane styling with different themes for action types
	actionsPaneStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("#FAB387")).
				Padding(1).
				MarginTop(1)

	actionsPaneTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FAB387"))

	// Action item styling for different types
	actionPrimaryStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#89B4FA")).
				Padding(0, 1)

	actionPrimaryFocusedStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#FFFFFF")).
					Background(lipgloss.Color("#89B4FA")).
					Padding(0, 1)

	actionConfirmationStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#A6E3A1")).
				Padding(0, 1)

	actionConfirmationFocusedStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#FFFFFF")).
					Background(lipgloss.Color("#A6E3A1")).
					Padding(0, 1)

	actionCancelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F38BA8")).
				Padding(0, 1)

	actionCancelFocusedStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#FFFFFF")).
					Background(lipgloss.Color("#F38BA8")).
					Padding(0, 1)

	actionInfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#94E2D5")).
			Padding(0, 1)

	actionInfoFocusedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#181825")).
				Background(lipgloss.Color("#94E2D5")).
				Padding(0, 1)

	actionAlternativeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#CBA6F7")).
				Padding(0, 1)

	actionAlternativeFocusedStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#FFFFFF")).
					Background(lipgloss.Color("#CBA6F7")).
					Padding(0, 1)

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

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F38BA8")).
			Bold(true)

	// Workflow breadcrumb styling
	workflowStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#CBA6F7")).
			Foreground(lipgloss.Color("#CBA6F7")).
			Padding(0, 2).
			MarginBottom(1)

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
	// Calculate layout dimensions
	m.calculateLayoutDimensions()

	var sections []string

	// Render header with connection status and application information
	sections = append(sections, m.renderHeader())

	// Render workflow breadcrumbs if present
	if workflowSection := m.renderWorkflowBreadcrumbs(); workflowSection != "" {
		sections = append(sections, workflowSection)
	}

	// Render main content history pane
	sections = append(sections, m.renderHistoryPane())

	// Render actions pane if actions are available
	if actionSection := m.renderActionsPane(); actionSection != "" {
		sections = append(sections, actionSection)
	}

	// Render input component
	sections = append(sections, m.renderInputComponent())

	// Render status messages if present
	if statusSection := m.renderStatusSection(); statusSection != "" {
		sections = append(sections, statusSection)
	}

	return strings.Join(sections, "\n")
}

// calculateLayoutDimensions computes the available space for each interface section
func (m *AppModel) calculateLayoutDimensions() {
	if m.terminalWidth > 0 && m.terminalHeight > 0 {
		// Calculate available height for history pane
		usedHeight := m.headerHeight + m.inputHeight + m.actionsHeight + 2 // +2 for spacing
		if m.currentWorkflow != nil {
			usedHeight += 3 // Workflow breadcrumbs
		}

		availableHeight := m.terminalHeight - usedHeight
		if availableHeight < 5 {
			availableHeight = 5 // Minimum height
		}

		m.maxDisplayLines = availableHeight
	}
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

// renderWorkflowBreadcrumbs creates workflow progress indication
func (m *AppModel) renderWorkflowBreadcrumbs() string {
	if m.currentWorkflow == nil {
		return ""
	}

	breadcrumbText := fmt.Sprintf("%s (%d/%d)",
		m.currentWorkflow.Title,
		m.currentWorkflow.Step,
		m.currentWorkflow.TotalSteps)

	// Add progress bar
	progressBar := m.createProgressBar(m.currentWorkflow.Step, m.currentWorkflow.TotalSteps, 20)
	breadcrumbText += " " + progressBar

	return workflowStyle.Render(breadcrumbText)
}

// renderHistoryPane creates the scrolling content area with command history and responses
func (m *AppModel) renderHistoryPane() string {
	if len(m.commandHistory) == 0 {
		emptyMessage := "Connected and ready. Type a command to get started."
		return historyPaneStyle.
			Height(m.maxDisplayLines).
			Width(m.terminalWidth - 4).
			Render(statusStyle.Render(emptyMessage))
	}

	var contentLines []string

	// Render visible portion of command history
	startIndex := 0
	if len(m.commandHistory) > m.maxDisplayLines/3 { // Allow ~3 lines per entry
		startIndex = len(m.commandHistory) - (m.maxDisplayLines / 3)
	}

	for i := startIndex; i < len(m.commandHistory); i++ {
		entry := m.commandHistory[i]
		contentLines = append(contentLines, m.renderHistoryEntry(entry)...)
	}

	// Apply scrolling offset
	if m.scrollOffset > 0 && m.scrollOffset < len(contentLines) {
		endIndex := m.scrollOffset + m.maxDisplayLines
		if endIndex > len(contentLines) {
			endIndex = len(contentLines)
		}
		contentLines = contentLines[m.scrollOffset:endIndex]
	} else if len(contentLines) > m.maxDisplayLines {
		// Show most recent content
		contentLines = contentLines[len(contentLines)-m.maxDisplayLines:]
	}

	content := strings.Join(contentLines, "\n")

	return historyPaneStyle.
		Height(m.maxDisplayLines).
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
		errorLine := appResponseStyle.Render(responsePrefix) + " " + errorStyle.Render("Error: "+entry.Error)
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

// renderActionsPane creates the numbered actions interface
func (m *AppModel) renderActionsPane() string {
	if !m.actionsVisible || len(m.currentActions) == 0 {
		return ""
	}

	var actionLines []string

	// Determine actions pane title based on action types
	paneTitle := "Available Actions"
	if m.hasConfirmationActions() {
		paneTitle = "Confirmation Required"
	} else if m.hasErrorActions() {
		paneTitle = "Error Recovery Options"
	}

	// Render action items
	for i, action := range m.currentActions {
		actionLine := m.renderActionItem(i, action)
		actionLines = append(actionLines, actionLine)
	}

	styledTitle := actionsPaneTitleStyle.Render(paneTitle)
	separatorWidth := m.terminalWidth - lipgloss.Width(styledTitle) - 6
	if separatorWidth < 0 {
		separatorWidth = 0
	}

	// Create bordered actions pane with title
	titledPane := fmt.Sprintf("┌─ %s %s┐\n│ %s │\n└%s┘",
		styledTitle,
		strings.Repeat("─", separatorWidth),
		strings.Join(actionLines, " │\n│ "),
		strings.Repeat("─", m.terminalWidth-2))

	return actionsPaneStyle.Width(m.terminalWidth - 4).Render(titledPane)
}

// renderActionItem creates a single numbered action with appropriate styling
func (m *AppModel) renderActionItem(index int, action interfaces.Action) string {
	number := fmt.Sprintf("[%d]", index+1)
	icon := action.Icon
	if icon == "" {
		// Default icons for action types
		switch action.Type {
		case "confirmation":
			icon = "✅"
		case "cancel":
			icon = "❌"
		case "info":
			icon = "📋"
		case "alternative":
			icon = "🔄"
		default:
			icon = "▶"
		}
	}

	actionText := fmt.Sprintf("%s %s %s", number, icon, action.Name)

	// Apply styling based on action type and focus state
	isFocused := m.focusState == FocusActions && m.selectedActionIndex == index

	switch action.Type {
	case "confirmation":
		if isFocused {
			return actionConfirmationFocusedStyle.Render(actionText)
		}
		return actionConfirmationStyle.Render(actionText)

	case "cancel":
		if isFocused {
			return actionCancelFocusedStyle.Render(actionText)
		}
		return actionCancelStyle.Render(actionText)

	case "info":
		if isFocused {
			return actionInfoFocusedStyle.Render(actionText)
		}
		return actionInfoStyle.Render(actionText)

	case "alternative":
		if isFocused {
			return actionAlternativeFocusedStyle.Render(actionText)
		}
		return actionAlternativeStyle.Render(actionText)

	default:
		if isFocused {
			return actionPrimaryFocusedStyle.Render(actionText)
		}
		return actionPrimaryStyle.Render(actionText)
	}
}

// renderInputComponent creates the command input interface
func (m *AppModel) renderInputComponent() string {
	inputWidth := m.terminalWidth - 6

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
		if len(m.currentActions) > 0 {
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
		statusLines = append(statusLines, errorStyle.Render("Error: "+m.errorMessage))
	}

	// Render status messages
	if m.statusMessage != "" {
		statusLines = append(statusLines, statusStyle.Render(m.statusMessage))
	}

	// Render action execution status
	if m.actionExecuting {
		statusLines = append(statusLines, statusStyle.Render("⏳ Executing action..."))
	} else if m.lastActionResult != "" {
		statusLines = append(statusLines, statusStyle.Render(m.lastActionResult))
	}

	// Render connection statistics if enabled
	if m.showTimestamps && m.connectionStats.TotalCommands > 0 {
		statsText := fmt.Sprintf("Commands: %d/%d successful • Avg response: %v",
			m.connectionStats.SuccessfulCommands,
			m.connectionStats.TotalCommands,
			m.connectionStats.AverageResponseTime.Truncate(time.Millisecond))
		statusLines = append(statusLines, statusStyle.Render(statsText))
	}

	if len(statusLines) > 0 {
		return "\n" + strings.Join(statusLines, "\n")
	}

	return ""
}

// Helper methods for rendering logic

// hasConfirmationActions checks if any actions require confirmation
func (m *AppModel) hasConfirmationActions() bool {
	for _, action := range m.currentActions {
		if action.Type == "confirmation" {
			return true
		}
	}
	return false
}

// hasErrorActions checks if any actions are for error recovery
func (m *AppModel) hasErrorActions() bool {
	for _, action := range m.currentActions {
		if action.Type == "cancel" || action.Type == "alternative" {
			return true
		}
	}
	return false
}

// createProgressBar creates a visual progress indicator
func (m *AppModel) createProgressBar(current, total, width int) string {
	if total <= 0 {
		return ""
	}

	filled := int(float64(current) / float64(total) * float64(width))
	if filled > width {
		filled = width
	}

	progress := strings.Repeat("●", filled) + strings.Repeat("○", width-filled)
	return progress
}
