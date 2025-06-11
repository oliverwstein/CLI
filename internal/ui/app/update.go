// Package app implements user input processing and state management for Application Mode.
// This file contains the Bubble Tea update function that processes command input submission,
// numbered action selection, Tab navigation through focusable elements, Space key for collapsible
// section expansion, and meta command handling according to section 3.2.5 of the design specification.
package app

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/universal-console/console/internal/interfaces"
)

// Update implements the Bubble Tea Model interface for Application Mode input processing
func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var commands []tea.Cmd

	// Process the message based on its type
	switch msg := msg.(type) {
	case tea.KeyMsg:
		cmd := m.handleKeyInput(msg)
		if cmd != nil {
			commands = append(commands, cmd)
		}

	case tea.WindowSizeMsg:
		m.SetTerminalSize(msg.Width, msg.Height)

	case commandExecutedMsg:
		cmd := m.handleCommandExecuted(msg)
		if cmd != nil {
			commands = append(commands, cmd)
		}

	case actionExecutedMsg:
		cmd := m.handleActionExecuted(msg)
		if cmd != nil {
			commands = append(commands, cmd)
		}

	case sectionToggledMsg:
		m.handleSectionToggled(msg)

	case connectionStatusMsg:
		return m.handleConnectionStatus(msg)

	case applicationInfoMsg:
		m.handleApplicationInfo(msg)

	default:
		// Handle textinput updates for command input field
		if m.focusState == FocusInput {
			var cmd tea.Cmd
			m.commandInput, cmd = m.commandInput.Update(msg)
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

// handleKeyInput processes keyboard input according to focus state and navigation patterns
func (m *AppModel) handleKeyInput(msg tea.KeyMsg) tea.Cmd {
	// Handle global key commands that work regardless of focus
	switch msg.String() {
	case "ctrl+c":
		return tea.Quit
	case "esc":
		return m.handleEscapeKey()
	case "ctrl+r":
		return m.retryLastCommand()
	case "f5":
		return m.refreshConnection()
	}

	// Handle focus-specific key processing
	switch m.focusState {
	case FocusInput:
		return m.handleInputKeys(msg)
	case FocusActions:
		return m.handleActionsKeys(msg)
	case FocusContent:
		return m.handleContentKeys(msg)
	case FocusExpandable:
		return m.handleExpandableKeys(msg)
	default:
		return nil
	}
}

// handleInputKeys processes keyboard input when command input has focus
func (m *AppModel) handleInputKeys(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		command := strings.TrimSpace(m.commandInput.Value())
		if command != "" {
			m.commandInput.SetValue("")
			return m.ExecuteCommand(command)
		}
		return nil

	case "tab":
		return m.cycleFocusForward()

	case "shift+tab":
		return m.cycleFocusBackward()

	case "ctrl+up":
		return m.navigateInputHistory(-1)

	case "ctrl+down":
		return m.navigateInputHistory(1)

	case "up":
		// Navigate input history when input is focused
		return m.navigateInputHistory(-1)

	case "down":
		// Navigate input history when input is focused
		return m.navigateInputHistory(1)

	case "ctrl+l":
		return m.clearHistory()

	default:
		// Handle numbered shortcuts for quick action execution (when input is empty)
		if m.commandInput.Value() == "" {
			if num, err := strconv.Atoi(msg.String()); err == nil && num >= 1 && num <= 9 {
				return m.executeActionByNumber(num)
			}
		}

		// Let textinput handle character input
		var cmd tea.Cmd
		m.commandInput, cmd = m.commandInput.Update(msg)
		return cmd
	}
}

// handleActionsKeys processes keyboard input when actions pane has focus
func (m *AppModel) handleActionsKeys(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up", "k":
		return m.navigateActions(-1)

	case "down", "j":
		return m.navigateActions(1)

	case "enter", "space":
		return m.executeSelectedAction()

	case "tab":
		return m.cycleFocusForward()

	case "shift+tab":
		return m.cycleFocusBackward()

	default:
		// Handle numbered action selection
		if num, err := strconv.Atoi(msg.String()); err == nil && num >= 1 && num <= 9 {
			return m.executeActionByNumber(num)
		}
		return nil
	}
}

// handleContentKeys processes keyboard input when content area has focus
func (m *AppModel) handleContentKeys(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up", "k":
		return m.scrollContent(-1)

	case "down", "j":
		return m.scrollContent(1)

	case "page_up":
		return m.scrollContent(-10)

	case "page_down":
		return m.scrollContent(10)

	case "home":
		return m.scrollToTop()

	case "end":
		return m.scrollToBottom()

	case "tab":
		return m.cycleFocusForward()

	case "shift+tab":
		return m.cycleFocusBackward()

	case "space":
		return m.toggleFocusedSection()

	default:
		return nil
	}
}

// handleExpandableKeys processes keyboard input when collapsible sections have focus
func (m *AppModel) handleExpandableKeys(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up", "k":
		return m.navigateExpandableElements(-1)

	case "down", "j":
		return m.navigateExpandableElements(1)

	case "space", "enter":
		return m.toggleFocusedSection()

	case "tab":
		return m.cycleFocusForward()

	case "shift+tab":
		return m.cycleFocusBackward()

	case "left", "h":
		// Collapse focused section
		return m.collapseFocusedSection()

	case "right", "l":
		// Expand focused section
		return m.expandFocusedSection()

	default:
		return nil
	}
}

// Focus navigation methods

// cycleFocusForward moves focus to the next focusable element
func (m *AppModel) cycleFocusForward() tea.Cmd {
	m.recordNavigation(m.focusState, "tab")

	// Determine next focus state based on current state and available elements
	switch m.focusState {
	case FocusInput:
		if m.actionsVisible && len(m.currentActions) > 0 {
			m.SetFocus(FocusActions)
			m.selectedActionIndex = 0
		} else if len(m.collapsibleElements) > 0 {
			m.SetFocus(FocusExpandable)
			m.currentFocusIndex = 0
		} else {
			m.SetFocus(FocusContent)
		}

	case FocusActions:
		if len(m.collapsibleElements) > 0 {
			m.SetFocus(FocusExpandable)
			m.currentFocusIndex = 0
		} else if len(m.renderedContent) > 0 {
			m.SetFocus(FocusContent)
		} else {
			m.SetFocus(FocusInput)
		}

	case FocusContent:
		if len(m.collapsibleElements) > 0 {
			m.SetFocus(FocusExpandable)
			m.currentFocusIndex = 0
		} else {
			m.SetFocus(FocusInput)
		}

	case FocusExpandable:
		m.SetFocus(FocusInput)

	default:
		m.SetFocus(FocusInput)
	}

	return nil
}

// cycleFocusBackward moves focus to the previous focusable element
func (m *AppModel) cycleFocusBackward() tea.Cmd {
	m.recordNavigation(m.focusState, "shift+tab")

	// Cycle backward through focus states
	switch m.focusState {
	case FocusInput:
		if len(m.collapsibleElements) > 0 {
			m.SetFocus(FocusExpandable)
			m.currentFocusIndex = len(m.collapsibleElements) - 1
		} else if len(m.renderedContent) > 0 {
			m.SetFocus(FocusContent)
		} else if m.actionsVisible && len(m.currentActions) > 0 {
			m.SetFocus(FocusActions)
			m.selectedActionIndex = len(m.currentActions) - 1
		}

	case FocusActions:
		m.SetFocus(FocusInput)

	case FocusContent:
		if m.actionsVisible && len(m.currentActions) > 0 {
			m.SetFocus(FocusActions)
			m.selectedActionIndex = len(m.currentActions) - 1
		} else {
			m.SetFocus(FocusInput)
		}

	case FocusExpandable:
		if len(m.renderedContent) > 0 {
			m.SetFocus(FocusContent)
		} else if m.actionsVisible && len(m.currentActions) > 0 {
			m.SetFocus(FocusActions)
			m.selectedActionIndex = len(m.currentActions) - 1
		} else {
			m.SetFocus(FocusInput)
		}

	default:
		m.SetFocus(FocusInput)
	}

	return nil
}

// handleEscapeKey returns focus to the input component from any other focused element
func (m *AppModel) handleEscapeKey() tea.Cmd {
	if m.focusState != FocusInput {
		m.recordNavigation(m.focusState, "escape")
		m.SetFocus(FocusInput)
	}
	return nil
}

// Navigation within specific focus areas

// navigateInputHistory moves through the command input history
func (m *AppModel) navigateInputHistory(direction int) tea.Cmd {
	if len(m.inputHistory) == 0 {
		return nil
	}

	newIndex := m.inputHistoryIndex + direction

	// Handle boundary conditions
	if direction < 0 {
		// Going backward in history
		if newIndex < 0 {
			newIndex = 0
		}
	} else {
		// Going forward in history
		if newIndex >= len(m.inputHistory) {
			// Clear input when going beyond most recent
			m.inputHistoryIndex = len(m.inputHistory)
			m.commandInput.SetValue("")
			return nil
		}
	}

	m.inputHistoryIndex = newIndex
	if newIndex < len(m.inputHistory) {
		m.commandInput.SetValue(m.inputHistory[newIndex])
		m.commandInput.CursorEnd()
	}

	return nil
}

// navigateActions moves selection within the actions pane
func (m *AppModel) navigateActions(direction int) tea.Cmd {
	if len(m.currentActions) == 0 {
		return nil
	}

	newIndex := m.selectedActionIndex + direction

	// Handle wrapping
	if newIndex < 0 {
		newIndex = len(m.currentActions) - 1
	} else if newIndex >= len(m.currentActions) {
		newIndex = 0
	}

	m.selectedActionIndex = newIndex
	return nil
}

// navigateExpandableElements moves focus within collapsible sections
func (m *AppModel) navigateExpandableElements(direction int) tea.Cmd {
	if len(m.collapsibleElements) == 0 {
		return nil
	}

	newIndex := m.currentFocusIndex + direction

	// Handle wrapping
	if newIndex < 0 {
		newIndex = len(m.collapsibleElements) - 1
	} else if newIndex >= len(m.collapsibleElements) {
		newIndex = 0
	}

	m.currentFocusIndex = newIndex
	if newIndex < len(m.collapsibleElements) {
		m.focusedSectionID = m.collapsibleElements[newIndex].ID
	}

	return nil
}

// Content scrolling methods

// scrollContent scrolls the content display by the specified number of lines
func (m *AppModel) scrollContent(lines int) tea.Cmd {
	newOffset := m.scrollOffset + lines

	// Ensure scroll offset stays within bounds
	maxOffset := len(m.renderedContent) - m.maxDisplayLines
	if maxOffset < 0 {
		maxOffset = 0
	}

	if newOffset < 0 {
		newOffset = 0
	} else if newOffset > maxOffset {
		newOffset = maxOffset
	}

	m.scrollOffset = newOffset
	return nil
}

// scrollToTop scrolls to the beginning of the content
func (m *AppModel) scrollToTop() tea.Cmd {
	m.scrollOffset = 0
	return nil
}

// scrollToBottom scrolls to the end of the content
func (m *AppModel) scrollToBottom() tea.Cmd {
	maxOffset := len(m.renderedContent) - m.maxDisplayLines
	if maxOffset < 0 {
		maxOffset = 0
	}
	m.scrollOffset = maxOffset
	return nil
}

// Action execution methods

// executeActionByNumber executes an action by its numbered position
func (m *AppModel) executeActionByNumber(number int) tea.Cmd {
	index := number - 1 // Convert to zero-based index

	if index >= 0 && index < len(m.currentActions) {
		m.selectedActionIndex = index
		return m.ExecuteAction(index)
	}

	return m.showError(fmt.Sprintf("No action available for number %d", number))
}

// executeSelectedAction executes the currently selected action
func (m *AppModel) executeSelectedAction() tea.Cmd {
	if m.selectedActionIndex >= 0 && m.selectedActionIndex < len(m.currentActions) {
		return m.ExecuteAction(m.selectedActionIndex)
	}
	return nil
}

// Collapsible section management

// toggleFocusedSection toggles the expansion state of the currently focused section
func (m *AppModel) toggleFocusedSection() tea.Cmd {
	if m.focusedSectionID != "" {
		return m.ToggleSection(m.focusedSectionID)
	}
	return nil
}

// expandFocusedSection expands the currently focused section
func (m *AppModel) expandFocusedSection() tea.Cmd {
	if m.focusedSectionID != "" && !m.expandedSections[m.focusedSectionID] {
		return m.ToggleSection(m.focusedSectionID)
	}
	return nil
}

// collapseFocusedSection collapses the currently focused section
func (m *AppModel) collapseFocusedSection() tea.Cmd {
	if m.focusedSectionID != "" && m.expandedSections[m.focusedSectionID] {
		return m.ToggleSection(m.focusedSectionID)
	}
	return nil
}

// Message handling methods for asynchronous operations

// handleCommandExecuted processes the result of command execution
func (m *AppModel) handleCommandExecuted(msg commandExecutedMsg) tea.Cmd {
	// Update connection statistics
	m.connectionStats.TotalCommands++
	if msg.success {
		m.connectionStats.SuccessfulCommands++
	} else {
		m.connectionStats.FailedCommands++
	}
	m.connectionStats.LastCommandTime = time.Now()

	// Update average response time
	if msg.duration > 0 {
		if m.connectionStats.AverageResponseTime == 0 {
			m.connectionStats.AverageResponseTime = msg.duration
		} else {
			// Calculate moving average
			totalCommands := m.connectionStats.TotalCommands
			totalTime := m.connectionStats.AverageResponseTime * time.Duration(totalCommands-1)
			m.connectionStats.AverageResponseTime = (totalTime + msg.duration) / time.Duration(totalCommands)
		}
	}

	// Create history entry
	historyEntry := HistoryEntry{
		Timestamp: time.Now(),
		Command:   msg.command,
		Duration:  msg.duration,
	}

	if msg.success && msg.response != nil {
		historyEntry.Response = msg.response
		historyEntry.Actions = msg.response.Actions
		historyEntry.Workflow = msg.response.Workflow

		// Update current response state
		m.currentResponse = msg.response
		m.currentActions = msg.response.Actions
		m.currentWorkflow = msg.response.Workflow
		m.actionsVisible = len(msg.response.Actions) > 0

		// Update actions pane height based on number of actions
		if m.actionsVisible {
			m.actionsHeight = len(msg.response.Actions) + 2 // +2 for borders
		} else {
			m.actionsHeight = 0
		}

		// Process response content through content renderer
		return m.renderResponseContent(msg.response)
	} else {
		// Handle error case
		historyEntry.Error = msg.error
		m.errorMessage = msg.error
		m.currentActions = nil
		m.actionsVisible = false
		m.actionsHeight = 0
	}

	// Add to history
	m.addToHistory(historyEntry)

	// Auto-scroll to bottom if enabled
	if m.autoScroll {
		return m.scrollToBottom()
	}

	return nil
}

// handleActionExecuted processes the result of action execution
func (m *AppModel) handleActionExecuted(msg actionExecutedMsg) tea.Cmd {
	m.actionExecuting = false

	// Update connection statistics
	m.connectionStats.TotalActions++

	if msg.success && msg.response != nil {
		// Create history entry for the action
		historyEntry := HistoryEntry{
			Timestamp: time.Now(),
			Command:   fmt.Sprintf("[Action] %s", msg.action.Name),
			Response:  msg.response,
			Actions:   msg.response.Actions,
			Workflow:  msg.response.Workflow,
			Duration:  msg.duration,
		}

		// Update current response state
		m.currentResponse = msg.response
		m.currentActions = msg.response.Actions
		m.currentWorkflow = msg.response.Workflow
		m.actionsVisible = len(msg.response.Actions) > 0

		// Update actions pane height
		if m.actionsVisible {
			m.actionsHeight = len(msg.response.Actions) + 2
		} else {
			m.actionsHeight = 0
		}

		// Add to history
		m.addToHistory(historyEntry)

		// Process response content
		return m.renderResponseContent(msg.response)
	} else {
		// Handle error case
		m.errorMessage = msg.error
		m.lastActionResult = fmt.Sprintf("Action failed: %s", msg.error)
		m.currentActions = nil
		m.actionsVisible = false
		m.actionsHeight = 0
	}

	return nil
}

// handleSectionToggled processes collapsible section toggle results
func (m *AppModel) handleSectionToggled(msg sectionToggledMsg) {
	if msg.error != "" {
		m.errorMessage = msg.error
		return
	}

	if msg.sectionID == "all" {
		// Handle expand/collapse all operations
		for id := range m.expandedSections {
			m.expandedSections[id] = msg.expanded
		}
		for i := range m.collapsibleElements {
			m.collapsibleElements[i].Expanded = msg.expanded
		}
	} else {
		// Handle individual section toggle
		m.expandedSections[msg.sectionID] = msg.expanded
		for i, element := range m.collapsibleElements {
			if element.ID == msg.sectionID {
				m.collapsibleElements[i].Expanded = msg.expanded
				break
			}
		}
	}
}

// handleConnectionStatus processes connection status changes
func (m *AppModel) handleConnectionStatus(msg connectionStatusMsg) (tea.Model, tea.Cmd) {
	if !msg.connected {
		// Connection lost or intentionally disconnected
		// Return to menu mode (this would typically return a different model)
		// For now, we'll quit the application
		return m, tea.Quit
	}

	m.connected = msg.connected
	if msg.error != "" {
		m.connectionError = msg.error
	}

	return m, nil
}

// handleApplicationInfo processes application metadata updates
func (m *AppModel) handleApplicationInfo(msg applicationInfoMsg) {
	if msg.error != "" {
		m.errorMessage = msg.error
		return
	}

	m.appName = msg.appName
	m.appVersion = msg.appVersion
	m.protocolVersion = msg.protocolVersion
	m.features = msg.features
}

// Content rendering and processing

// renderResponseContent processes response content through the content renderer
func (m *AppModel) renderResponseContent(response *interfaces.CommandResponse) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// Render content using the content renderer
		renderedContent, err := m.contentRenderer.RenderContent(
			response.Response.Content,
			m.theme,
		)
		if err != nil {
			return commandExecutedMsg{
				success: false,
				error:   fmt.Sprintf("Content rendering failed: %s", err.Error()),
			}
		}

		// Update rendered content and collapsible elements
		m.renderedContent = append(m.renderedContent, renderedContent...)
		m.updateCollapsibleElements(renderedContent)

		// Limit rendered content size to prevent memory growth
		if len(m.renderedContent) > 10000 {
			m.renderedContent = m.renderedContent[1000:]
		}

		return nil
	})
}

// Helper methods

// addToHistory adds an entry to the command history
func (m *AppModel) addToHistory(entry HistoryEntry) {
	m.commandHistory = append(m.commandHistory, entry)

	// Limit history size
	if len(m.commandHistory) > m.maxHistorySize {
		m.commandHistory = m.commandHistory[1:]
	}

	m.lastUpdateTime = time.Now()
}

// updateCollapsibleElements updates the list of collapsible elements based on rendered content
func (m *AppModel) updateCollapsibleElements(content []interfaces.RenderedContent) {
	elements := make([]CollapsibleElement, 0)

	for i, item := range content {
		if item.Expanded != nil {
			element := CollapsibleElement{
				ID:       item.ID,
				Title:    fmt.Sprintf("Section %d", i+1),
				Expanded: *item.Expanded,
				Position: i,
			}
			elements = append(elements, element)

			// Initialize expansion state if not present
			if _, exists := m.expandedSections[item.ID]; !exists {
				m.expandedSections[item.ID] = *item.Expanded
			}
		}
	}

	m.collapsibleElements = elements
}

// recordNavigation records a navigation step for user experience analysis
func (m *AppModel) recordNavigation(fromFocus FocusState, method string) {
	step := NavigationStep{
		Timestamp: time.Now(),
		FromFocus: fromFocus,
		Method:    method,
	}

	m.navigationHistory = append(m.navigationHistory, step)

	// Limit navigation history size
	if len(m.navigationHistory) > 200 {
		m.navigationHistory = m.navigationHistory[1:]
	}
}

// refreshConnection attempts to refresh the connection to the application
func (m *AppModel) refreshConnection() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		if !m.protocolClient.IsConnected() {
			return connectionStatusMsg{
				connected: false,
				error:     "Connection lost",
			}
		}

		return connectionStatusMsg{
			connected: true,
		}
	})
}
