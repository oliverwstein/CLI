// Package app implements the Application Mode interface for the Universal Application Console.
// This file defines the AppModel structure containing command history, current response content,
// actions pane state, workflow context, and focus management for all interactive elements.
// The model maintains conversational flow and rich content presentation as specified in
// section 3.2.2 of the design specification.
package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/universal-console/console/internal/interfaces"
)

// AppModel represents the complete state and dependencies for Application Mode operation
type AppModel struct {
	// Injected dependencies for external system integration
	profile         *interfaces.Profile
	protocolClient  interfaces.ProtocolClient
	contentRenderer interfaces.ContentRenderer
	configManager   interfaces.ConfigManager
	authManager     interfaces.AuthManager

	// Connection state and application information
	connected       bool
	appName         string
	appVersion      string
	protocolVersion string
	features        map[string]bool
	connectionError string

	// Command history and interaction state
	commandHistory    []HistoryEntry
	historyIndex      int
	commandInput      textinput.Model
	inputHistory      []string
	inputHistoryIndex int

	// Current response content and display state
	currentResponse *interfaces.CommandResponse
	renderedContent []interfaces.RenderedContent
	currentActions  []interfaces.Action
	currentWorkflow *interfaces.Workflow
	scrollOffset    int
	maxDisplayLines int

	// Actions pane state management
	actionsVisible      bool
	selectedActionIndex int
	actionExecuting     bool
	lastActionResult    string

	// Focus management and keyboard navigation
	focusState        FocusState
	focusableElements []FocusableElement
	currentFocusIndex int
	navigationHistory []NavigationStep

	// Collapsible content management
	expandedSections    map[string]bool
	focusedSectionID    string
	collapsibleElements []CollapsibleElement

	// Workflow and operation context
	operationHistory  []OperationRecord
	pendingOperations map[string]*PendingOperation

	// User interface preferences and configuration
	showTimestamps     bool
	showLineNumbers    bool
	autoScroll         bool
	confirmDestructive bool
	maxHistorySize     int
	theme              *interfaces.Theme

	// Terminal dimensions for responsive layout
	terminalWidth  int
	terminalHeight int
	headerHeight   int
	inputHeight    int
	actionsHeight  int

	// Status and error management
	statusMessage   string
	errorMessage    string
	lastUpdateTime  time.Time
	connectionStats ConnectionStatistics
}

// FocusState represents the current focus location within the application interface
type FocusState int

const (
	FocusInput FocusState = iota
	FocusActions
	FocusContent
	FocusExpandable
)

// FocusableElement represents an interactive element that can receive keyboard focus
type FocusableElement struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"` // "input", "action", "collapsible", "content"
	Position int                    `json:"position"`
	Data     map[string]interface{} `json:"data"`
}

// CollapsibleElement represents expandable content sections within the interface
type CollapsibleElement struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Expanded bool   `json:"expanded"`
	Level    int    `json:"level"`
	Position int    `json:"position"`
}

// HistoryEntry represents a single interaction in the command history
type HistoryEntry struct {
	Timestamp time.Time                    `json:"timestamp"`
	Command   string                       `json:"command"`
	Response  *interfaces.CommandResponse  `json:"response"`
	Rendered  []interfaces.RenderedContent `json:"rendered"`
	Actions   []interfaces.Action          `json:"actions"`
	Workflow  *interfaces.Workflow         `json:"workflow"`
	Error     string                       `json:"error,omitempty"`
	Duration  time.Duration                `json:"duration"`
}

// NavigationStep tracks focus navigation for user experience analysis
type NavigationStep struct {
	Timestamp time.Time  `json:"timestamp"`
	FromFocus FocusState `json:"fromFocus"`
	ToFocus   FocusState `json:"toFocus"`
	Method    string     `json:"method"` // "tab", "click", "shortcut", etc.
	ElementID string     `json:"elementId,omitempty"`
}

// WorkflowContext maintains state for multi-step operations
type WorkflowContext struct {
	ID           string                 `json:"id"`
	Title        string                 `json:"title"`
	CurrentStep  int                    `json:"currentStep"`
	TotalSteps   int                    `json:"totalSteps"`
	StartTime    time.Time              `json:"startTime"`
	LastActivity time.Time              `json:"lastActivity"`
	Context      map[string]interface{} `json:"context"`
	Breadcrumbs  []string               `json:"breadcrumbs"`
}

// OperationRecord tracks executed operations for audit and recovery
type OperationRecord struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"` // "command", "action", "meta"
	Content   string                 `json:"content"`
	Timestamp time.Time              `json:"timestamp"`
	Duration  time.Duration          `json:"duration"`
	Success   bool                   `json:"success"`
	Error     string                 `json:"error,omitempty"`
	Context   map[string]interface{} `json:"context,omitempty"`
}

// PendingOperation represents operations awaiting completion
type PendingOperation struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	StartTime   time.Time              `json:"startTime"`
	ExpectedEnd time.Time              `json:"expectedEnd"`
	Context     map[string]interface{} `json:"context"`
	Cancelable  bool                   `json:"cancelable"`
}

// ConnectionStatistics tracks communication metrics with the connected application
type ConnectionStatistics struct {
	TotalCommands       int           `json:"totalCommands"`
	SuccessfulCommands  int           `json:"successfulCommands"`
	FailedCommands      int           `json:"failedCommands"`
	TotalActions        int           `json:"totalActions"`
	AverageResponseTime time.Duration `json:"averageResponseTime"`
	LastCommandTime     time.Time     `json:"lastCommandTime"`
	SessionDuration     time.Duration `json:"sessionDuration"`
	SessionStartTime    time.Time     `json:"sessionStartTime"`
}

// NewAppModel creates a new Application Mode model with comprehensive dependency injection
func NewAppModel(
	profile *interfaces.Profile,
	protocolClient interfaces.ProtocolClient,
	contentRenderer interfaces.ContentRenderer,
	configManager interfaces.ConfigManager,
	authManager interfaces.AuthManager,
) *AppModel {
	// Initialize command input component
	commandInput := textinput.New()
	commandInput.Placeholder = "Enter a command..."
	commandInput.Width = 50
	commandInput.Focus()

	// Load theme from configuration
	var theme *interfaces.Theme
	if profile.Theme != "" {
		if loadedTheme, err := configManager.LoadTheme(profile.Theme); err == nil {
			theme = loadedTheme
		}
	}

	model := &AppModel{
		// Dependency injection
		profile:         profile,
		protocolClient:  protocolClient,
		contentRenderer: contentRenderer,
		configManager:   configManager,
		authManager:     authManager,

		// Initialize command handling
		commandHistory:    make([]HistoryEntry, 0),
		historyIndex:      -1,
		commandInput:      commandInput,
		inputHistory:      make([]string, 0),
		inputHistoryIndex: -1,

		// Initialize UI state
		focusState:          FocusInput,
		focusableElements:   make([]FocusableElement, 0),
		currentFocusIndex:   0,
		navigationHistory:   make([]NavigationStep, 0),
		expandedSections:    make(map[string]bool),
		collapsibleElements: make([]CollapsibleElement, 0),

		// Initialize operation tracking
		operationHistory:  make([]OperationRecord, 0),
		pendingOperations: make(map[string]*PendingOperation),

		// Configure default preferences
		showTimestamps:     false,
		showLineNumbers:    false,
		autoScroll:         true,
		confirmDestructive: true,
		maxHistorySize:     1000,
		theme:              theme,

		// Initialize connection state
		connected: protocolClient.IsConnected(),
		connectionStats: ConnectionStatistics{
			SessionStartTime: time.Now(),
		},

		// Set default UI dimensions
		headerHeight:  3,
		inputHeight:   3,
		actionsHeight: 0,
	}

	// Initialize focusable elements
	model.updateFocusableElements()

	return model
}

// Init implements the tea.Model interface for Bubble Tea initialization
func (m *AppModel) Init() tea.Cmd {
	commands := []tea.Cmd{
		textinput.Blink,
		m.loadApplicationInfo(),
	}

	return tea.Batch(commands...)
}

// SetTerminalSize updates the model with current terminal dimensions for responsive layout
func (m *AppModel) SetTerminalSize(width, height int) {
	m.terminalWidth = width
	m.terminalHeight = height

	// Calculate available space for content display
	availableHeight := height - m.headerHeight - m.inputHeight - m.actionsHeight
	m.maxDisplayLines = availableHeight - 2 // Account for borders

	// Adjust command input width based on terminal size
	availableWidth := width - 10 // Account for borders and padding
	if availableWidth > 20 && availableWidth < 100 {
		m.commandInput.Width = availableWidth
	}
}

// ExecuteCommand processes a user command and sends it to the connected application
func (m *AppModel) ExecuteCommand(command string) tea.Cmd {
	if !m.connected {
		return m.showError("Not connected to any application")
	}

	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}

	// Check for meta commands
	if strings.HasPrefix(command, "/") {
		return m.handleMetaCommand(command)
	}

	// Add to input history
	m.addToInputHistory(command)

	// Create command request
	request := interfaces.CommandRequest{
		Command: command,
	}

	return tea.Cmd(func() tea.Msg {
		startTime := time.Now()

		// Execute command
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		response, err := m.protocolClient.ExecuteCommand(ctx, request)
		duration := time.Since(startTime)

		if err != nil {
			return commandExecutedMsg{
				command:  command,
				success:  false,
				error:    err.Error(),
				duration: duration,
			}
		}

		return commandExecutedMsg{
			command:  command,
			response: response,
			success:  true,
			duration: duration,
		}
	})
}

// ExecuteAction processes a user action selection from the Actions Pane
func (m *AppModel) ExecuteAction(actionIndex int) tea.Cmd {
	if !m.connected {
		return m.showError("Not connected to any application")
	}

	if actionIndex < 0 || actionIndex >= len(m.currentActions) {
		return m.showError(fmt.Sprintf("Invalid action index: %d", actionIndex+1))
	}

	action := m.currentActions[actionIndex]
	m.selectedActionIndex = actionIndex
	m.actionExecuting = true

	// Create action request
	request := interfaces.ActionRequest{
		Command: action.Command,
	}

	// Include workflow context if present
	if m.currentWorkflow != nil {
		request.WorkflowID = m.currentWorkflow.ID
		request.Context = make(map[string]interface{})
		request.Context["workflowStep"] = m.currentWorkflow.Step
	}

	return tea.Cmd(func() tea.Msg {
		startTime := time.Now()

		// Execute action
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		response, err := m.protocolClient.ExecuteAction(ctx, request)
		duration := time.Since(startTime)

		if err != nil {
			return actionExecutedMsg{
				actionIndex: actionIndex,
				action:      action,
				success:     false,
				error:       err.Error(),
				duration:    duration,
			}
		}

		return actionExecutedMsg{
			actionIndex: actionIndex,
			action:      action,
			response:    response,
			success:     true,
			duration:    duration,
		}
	})
}

// SetFocus changes the current focus state and updates navigation tracking
func (m *AppModel) SetFocus(newFocus FocusState) {
	if newFocus != m.focusState {
		// Record navigation step
		step := NavigationStep{
			Timestamp: time.Now(),
			FromFocus: m.focusState,
			ToFocus:   newFocus,
			Method:    "programmatic",
		}
		m.navigationHistory = append(m.navigationHistory, step)

		// Limit navigation history size
		if len(m.navigationHistory) > 100 {
			m.navigationHistory = m.navigationHistory[1:]
		}

		m.focusState = newFocus
		m.updateFocusableElements()
	}
}

// ToggleSection expands or collapses a collapsible content section
func (m *AppModel) ToggleSection(sectionID string) tea.Cmd {
	if sectionID == "" {
		return nil
	}

	// Toggle expansion state
	m.expandedSections[sectionID] = !m.expandedSections[sectionID]

	// Update collapsible elements
	for i, element := range m.collapsibleElements {
		if element.ID == sectionID {
			m.collapsibleElements[i].Expanded = m.expandedSections[sectionID]
			break
		}
	}

	// Use content renderer to toggle the section
	return tea.Cmd(func() tea.Msg {
		err := m.contentRenderer.ToggleCollapsible(sectionID)
		if err != nil {
			return sectionToggledMsg{
				sectionID: sectionID,
				expanded:  m.expandedSections[sectionID],
				error:     err.Error(),
			}
		}

		return sectionToggledMsg{
			sectionID: sectionID,
			expanded:  m.expandedSections[sectionID],
		}
	})
}

// Message types for Bubble Tea command system

// commandExecutedMsg carries the result of command execution
type commandExecutedMsg struct {
	command  string
	response *interfaces.CommandResponse
	success  bool
	error    string
	duration time.Duration
}

// actionExecutedMsg carries the result of action execution
type actionExecutedMsg struct {
	actionIndex int
	action      interfaces.Action
	response    *interfaces.CommandResponse
	success     bool
	error       string
	duration    time.Duration
}

// sectionToggledMsg indicates that a collapsible section was toggled
type sectionToggledMsg struct {
	sectionID string
	expanded  bool
	error     string
}

// connectionStatusMsg carries connection status updates
type connectionStatusMsg struct {
	connected bool
	error     string
}

// applicationInfoMsg carries application metadata from the connected service
type applicationInfoMsg struct {
	appName         string
	appVersion      string
	protocolVersion string
	features        map[string]bool
	error           string
}

// Helper methods for internal state management

// loadApplicationInfo retrieves application metadata from the connected service
func (m *AppModel) loadApplicationInfo() tea.Cmd {
	if !m.connected {
		return nil
	}

	return tea.Cmd(func() tea.Msg {
		// Application info should be available from the protocol client's connection state
		// In a real implementation, this might query the client for current connection details
		return applicationInfoMsg{
			appName:         "Connected Application",
			appVersion:      "Unknown",
			protocolVersion: "2.0",
			features:        make(map[string]bool),
		}
	})
}

// handleMetaCommand processes console meta commands
func (m *AppModel) handleMetaCommand(command string) tea.Cmd {
	switch strings.ToLower(command) {
	case "/quit", "/exit":
		return m.disconnectAndReturn()
	case "/clear":
		return m.clearHistory()
	case "/help":
		return m.showHelp()
	case "/expand-all":
		return m.expandAllSections()
	case "/collapse-all":
		return m.collapseAllSections()
	case "/retry":
		return m.retryLastCommand()
	case "/history":
		return m.showCommandHistory()
	default:
		return m.showError(fmt.Sprintf("Unknown meta command: %s", command))
	}
}

// Command generation methods for meta commands

func (m *AppModel) disconnectAndReturn() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		// Disconnect from the protocol client
		if m.protocolClient.IsConnected() {
			m.protocolClient.Disconnect()
		}

		// Signal return to menu mode
		return connectionStatusMsg{
			connected: false,
		}
	})
}

func (m *AppModel) clearHistory() tea.Cmd {
	m.commandHistory = make([]HistoryEntry, 0)
	m.renderedContent = make([]interfaces.RenderedContent, 0)
	m.scrollOffset = 0
	return nil
}

func (m *AppModel) showHelp() tea.Cmd {
	helpText := `Available Meta Commands:
/quit, /exit    - Disconnect and return to Console Menu
/clear          - Clear command history
/help           - Show this help message
/expand-all     - Expand all collapsible sections
/collapse-all   - Collapse all collapsible sections
/retry          - Retry the last command
/history        - Show command history

Keyboard Navigation:
Tab             - Cycle through focusable elements
Shift+Tab       - Cycle backward through elements
Space           - Toggle expansion of focused collapsible sections
Enter           - Execute focused action or submit command
Escape          - Return focus to command input
Ctrl+↑/↓        - Navigate command history
Numbers 1-9     - Quick execute numbered actions`

	// Create a mock help response
	return tea.Cmd(func() tea.Msg {
		return commandExecutedMsg{
			command: "/help",
			response: &interfaces.CommandResponse{
				Response: struct {
					Type    string      `json:"type"`
					Content interface{} `json:"content"`
				}{
					Type:    "text",
					Content: helpText,
				},
			},
			success:  true,
			duration: 0,
		}
	})
}

func (m *AppModel) expandAllSections() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		err := m.contentRenderer.ExpandAll()
		if err != nil {
			return sectionToggledMsg{error: err.Error()}
		}

		// Update local state
		for id := range m.expandedSections {
			m.expandedSections[id] = true
		}

		return sectionToggledMsg{sectionID: "all", expanded: true}
	})
}

func (m *AppModel) collapseAllSections() tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		err := m.contentRenderer.CollapseAll()
		if err != nil {
			return sectionToggledMsg{error: err.Error()}
		}

		// Update local state
		for id := range m.expandedSections {
			m.expandedSections[id] = false
		}

		return sectionToggledMsg{sectionID: "all", expanded: false}
	})
}

func (m *AppModel) retryLastCommand() tea.Cmd {
	if len(m.commandHistory) == 0 {
		return m.showError("No previous command to retry")
	}

	lastEntry := m.commandHistory[len(m.commandHistory)-1]
	return m.ExecuteCommand(lastEntry.Command)
}

func (m *AppModel) showCommandHistory() tea.Cmd {
	if len(m.commandHistory) == 0 {
		return m.showError("No command history available")
	}

	historyText := "Command History:\n"
	for i, entry := range m.commandHistory {
		historyText += fmt.Sprintf("%d. %s (%s)\n",
			i+1, entry.Command, entry.Timestamp.Format("15:04:05"))
	}

	return tea.Cmd(func() tea.Msg {
		return commandExecutedMsg{
			command: "/history",
			response: &interfaces.CommandResponse{
				Response: struct {
					Type    string      `json:"type"`
					Content interface{} `json:"content"`
				}{
					Type:    "text",
					Content: historyText,
				},
			},
			success:  true,
			duration: 0,
		}
	})
}

// showError creates a command to display error messages
func (m *AppModel) showError(message string) tea.Cmd {
	return tea.Cmd(func() tea.Msg {
		return commandExecutedMsg{
			command:  "",
			success:  false,
			error:    message,
			duration: 0,
		}
	})
}

// Utility methods

// addToInputHistory adds a command to the input history
func (m *AppModel) addToInputHistory(command string) {
	m.inputHistory = append(m.inputHistory, command)

	// Limit history size
	if len(m.inputHistory) > 100 {
		m.inputHistory = m.inputHistory[1:]
	}

	m.inputHistoryIndex = len(m.inputHistory)
}

// updateFocusableElements rebuilds the list of focusable elements
func (m *AppModel) updateFocusableElements() {
	elements := make([]FocusableElement, 0)

	// Add input element
	elements = append(elements, FocusableElement{
		ID:       "command_input",
		Type:     "input",
		Position: 0,
	})

	// Add action elements if actions are visible
	if m.actionsVisible && len(m.currentActions) > 0 {
		for i, action := range m.currentActions {
			elements = append(elements, FocusableElement{
				ID:       fmt.Sprintf("action_%d", i),
				Type:     "action",
				Position: i + 1,
				Data: map[string]interface{}{
					"actionIndex": i,
					"actionName":  action.Name,
				},
			})
		}
	}

	// Add collapsible elements
	for i, element := range m.collapsibleElements {
		elements = append(elements, FocusableElement{
			ID:       element.ID,
			Type:     "collapsible",
			Position: len(elements),
			Data: map[string]interface{}{
				"title":    element.Title,
				"expanded": element.Expanded,
				"level":    element.Level,
				"index":    i,
			},
		})
	}

	m.focusableElements = elements
}
