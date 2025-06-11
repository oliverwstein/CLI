// Package actions implements the Actions Pane system for the Universal Application Console.
// This file creates numbered action lists with different visual themes for standard, confirmation,
// and error recovery options, as specified in section 3.2.1 of the design specification.
// It supports both direct number key execution and focused navigation.
package actions

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/universal-console/console/internal/interfaces"
)

// Styling definitions for the Actions Pane
var (
	actionsPaneStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("#FAB387")).
				Padding(0, 1).
				MarginTop(1)

	actionsPaneTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FAB387"))

	// Action item styles for different types and focus states
	actionStyles = map[string]lipgloss.Style{
		"primary":        lipgloss.NewStyle().Foreground(lipgloss.Color("#89B4FA")).Padding(0, 1),
		"primary_f":      lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#89B4FA")).Padding(0, 1),
		"confirmation":   lipgloss.NewStyle().Foreground(lipgloss.Color("#A6E3A1")).Padding(0, 1),
		"confirmation_f": lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#A6E3A1")).Padding(0, 1),
		"cancel":         lipgloss.NewStyle().Foreground(lipgloss.Color("#F38BA8")).Padding(0, 1),
		"cancel_f":       lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#F38BA8")).Padding(0, 1),
		"info":           lipgloss.NewStyle().Foreground(lipgloss.Color("#94E2D5")).Padding(0, 1),
		"info_f":         lipgloss.NewStyle().Foreground(lipgloss.Color("#181825")).Background(lipgloss.Color("#94E2D5")).Padding(0, 1),
		"alternative":    lipgloss.NewStyle().Foreground(lipgloss.Color("#CBA6F7")).Padding(0, 1),
		"alternative_f":  lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#CBA6F7")).Padding(0, 1),
	}
)

// Pane represents the state and logic for the interactive Actions Pane.
type Pane struct {
	actions       []interfaces.Action
	selectedIndex int
	width         int
	visible       bool
}

// NewPane creates a new Actions Pane component.
func NewPane() *Pane {
	return &Pane{
		actions:       []interfaces.Action{},
		selectedIndex: -1,
		visible:       false,
	}
}

// SetActions updates the pane with a new set of actions and makes it visible.
func (p *Pane) SetActions(actions []interfaces.Action) {
	p.actions = actions
	if len(actions) > 0 {
		p.visible = true
		p.selectedIndex = 0
	} else {
		p.visible = false
		p.selectedIndex = -1
	}
}

// Reset hides the pane and clears its actions.
func (p *Pane) Reset() {
	p.visible = false
	p.actions = []interfaces.Action{}
	p.selectedIndex = -1
}

// IsVisible returns true if the pane has actions and should be displayed.
func (p *Pane) IsVisible() bool {
	return p.visible
}

// Next moves the selection to the next action, wrapping around.
func (p *Pane) Next() {
	if !p.visible {
		return
	}
	p.selectedIndex = (p.selectedIndex + 1) % len(p.actions)
}

// Previous moves the selection to the previous action, wrapping around.
func (p *Pane) Previous() {
	if !p.visible {
		return
	}
	p.selectedIndex--
	if p.selectedIndex < 0 {
		p.selectedIndex = len(p.actions) - 1
	}
}

// Selected returns the currently selected action.
func (p *Pane) Selected() (*interfaces.Action, error) {
	if p.selectedIndex < 0 || p.selectedIndex >= len(p.actions) {
		return nil, fmt.Errorf("no action selected")
	}
	return &p.actions[p.selectedIndex], nil
}

// SetWidth sets the rendering width of the pane.
func (p *Pane) SetWidth(width int) {
	p.width = width
}

// View renders the Actions Pane as a string.
func (p *Pane) View() string {
	if !p.visible {
		return ""
	}

	paneTitle := p.getPaneTitle()
	var actionLines []string

	for i, action := range p.actions {
		isFocused := (i == p.selectedIndex)
		actionLines = append(actionLines, p.renderActionItem(i, action, isFocused))
	}

	content := strings.Join(actionLines, "\n")

	// Create bordered actions pane with a title
	titledPane := lipgloss.JoinVertical(lipgloss.Left,
		actionsPaneTitleStyle.Render(paneTitle),
		content,
	)

	return actionsPaneStyle.Width(p.width - 2).Render(titledPane)
}

// getPaneTitle determines the appropriate title based on the types of actions present.
func (p *Pane) getPaneTitle() string {
	hasConfirmation := false
	hasErrorRecovery := false

	for _, action := range p.actions {
		if action.Type == "confirmation" {
			hasConfirmation = true
		}
		if action.Type == "cancel" || action.Type == "alternative" {
			hasErrorRecovery = true
		}
	}

	if hasConfirmation {
		return "Confirmation Required"
	}
	if hasErrorRecovery {
		return "Error Recovery Options"
	}
	return "Available Actions"
}

// renderActionItem creates a single numbered action with appropriate styling.
func (p *Pane) renderActionItem(index int, action interfaces.Action, isFocused bool) string {
	number := fmt.Sprintf("[%d]", index+1)

	// Determine icon based on action type, using defaults if not provided.
	icon := p.getActionIcon(action)
	actionText := fmt.Sprintf("%-4s %s %s", number, icon, action.Name)

	// Apply styling based on action type and focus state.
	styleKey := action.Type
	if styleKey == "" {
		styleKey = "primary" // Default style
	}
	if isFocused {
		styleKey += "_f"
	}

	style, exists := actionStyles[styleKey]
	if !exists {
		// Fallback to primary style
		style = actionStyles["primary"]
		if isFocused {
			style = actionStyles["primary_f"]
		}
	}

	return style.Render(actionText)
}

// getActionIcon returns the appropriate icon for a given action.
func (p *Pane) getActionIcon(action interfaces.Action) string {
	if action.Icon != "" {
		return action.Icon
	}
	switch action.Type {
	case "confirmation":
		return "✅"
	case "cancel":
		return "❌"
	case "info":
		return "📋"
	case "alternative":
		return "🔄"
	default:
		return "▶️"
	}
}
