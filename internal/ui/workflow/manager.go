// Package workflow implements multi-step operation context management for the
// Universal Application Console. This file handles workflow state, renders
// breadcrumb navigation, and provides cancellation mechanisms for long-running
// operations, as specified in section 3.4.1 of the design specification.
package workflow

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/universal-console/console/internal/interfaces"
	"github.com/universal-console/console/internal/ui/components"
)

// Styling definitions for the Workflow breadcrumb display.
var (
	workflowStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#CBA6F7")).
		Foreground(lipgloss.Color("#CBA6F7")).
		Padding(0, 1).
		MarginBottom(1)
)

// Manager handles the state and presentation of a multi-step workflow.
type Manager struct {
	currentWorkflow *interfaces.Workflow
	active          bool
	width           int
}

// NewManager creates a new Workflow Manager.
func NewManager() *Manager {
	return &Manager{
		active: false,
	}
}

// UpdateState processes a new workflow object from a server response.
// It starts a new workflow or updates an existing one.
func (m *Manager) UpdateState(workflow *interfaces.Workflow) {
	if workflow == nil || workflow.ID == "" {
		m.EndWorkflow()
		return
	}

	m.currentWorkflow = workflow
	m.active = true
}

// EndWorkflow clears the current workflow state.
func (m *Manager) EndWorkflow() {
	m.currentWorkflow = nil
	m.active = false
}

// IsActive returns true if a workflow is currently in progress.
func (m *Manager) IsActive() bool {
	return m.active && m.currentWorkflow != nil
}

// GetCurrentWorkflow returns the current workflow state.
func (m *Manager) GetCurrentWorkflow() *interfaces.Workflow {
	return m.currentWorkflow
}

// SetWidth sets the rendering width of the workflow component.
func (m *Manager) SetWidth(width int) {
	m.width = width
}

// View renders the workflow breadcrumb navigation as a string.
func (m *Manager) View() string {
	if !m.IsActive() {
		return ""
	}

	wf := m.currentWorkflow
	breadcrumbText := fmt.Sprintf("Workflow: %s (%d/%d)", wf.Title, wf.Step, wf.TotalSteps)

	// Calculate available width for the progress bar
	// Subtracting padding, borders, and text length
	availableWidth := m.width - lipgloss.Width(breadcrumbText) - 6
	if availableWidth < 10 {
		availableWidth = 10
	}

	// Create a progress bar using the shared component
	progressBar := components.RenderProgressBar(
		(wf.Step*100)/wf.TotalSteps,
		availableWidth,
		"●",
		"○",
	)

	// Combine text and progress bar
	fullView := lipgloss.JoinHorizontal(lipgloss.Left, breadcrumbText, " ", progressBar)

	return workflowStyle.Width(m.width - 2).Render(fullView)
}
