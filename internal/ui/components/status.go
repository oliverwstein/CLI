// Package components provides shared, reusable interface elements for the
// Universal Application Console. This file implements visual status indicators
// and progress displays as described in section 3.2.1 of the design document,
// enhancing user confidence with real-time feedback.
package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// statusStyles maps status strings to their corresponding visual style.
var statusStyles = map[string]lipgloss.Style{
	"pending":  lipgloss.NewStyle().Foreground(lipgloss.Color("#F9E2AF")),
	"success":  lipgloss.NewStyle().Foreground(lipgloss.Color("#A6E3A1")),
	"error":    lipgloss.NewStyle().Foreground(lipgloss.Color("#F38BA8")),
	"warning":  lipgloss.NewStyle().Foreground(lipgloss.Color("#FAB387")),
	"info":     lipgloss.NewStyle().Foreground(lipgloss.Color("#89B4FA")),
	"running":  lipgloss.NewStyle().Foreground(lipgloss.Color("#F9E2AF")),
	"complete": lipgloss.NewStyle().Foreground(lipgloss.Color("#A6E3A1")),
}

// statusIcons maps status strings to their corresponding icon.
var statusIcons = map[string]string{
	"pending":  "⏳",
	"success":  "✅",
	"error":    "❌",
	"warning":  "⚠️",
	"info":     "ℹ️",
	"running":  "🏃",
	"complete": "🏁",
}

// RenderStatus formats a status message with an appropriate icon and color.
// It returns a styled string ready for display.
func RenderStatus(status, message string) string {
	style, exists := statusStyles[status]
	if !exists {
		style = lipgloss.NewStyle() // Default style
	}

	icon, exists := statusIcons[status]
	if !exists {
		icon = "🔹" // Default icon
	}

	return style.Render(fmt.Sprintf("%s %s", icon, message))
}

// RenderProgressBar creates a visual textual progress bar.
// - progress: The percentage of completion (0-100).
// - width: The total width of the bar in characters.
// - fillChar: The character to use for the filled portion of the bar.
// - emptyChar: The character to use for the empty portion of the bar.
func RenderProgressBar(progress int, width int, fillChar, emptyChar string) string {
	if width <= 0 {
		return ""
	}
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}

	filledWidth := (progress * width) / 100
	emptyWidth := width - filledWidth

	filled := strings.Repeat(fillChar, filledWidth)
	empty := strings.Repeat(emptyChar, emptyWidth)

	return fmt.Sprintf("[%s%s]", filled, empty)
}

// RenderSpinner returns a spinner model from the charmbracelet/bubbles library.
// Note: This would require adding the spinner bubble as a dependency and managing its
// state within the calling model's Update function. This is a placeholder for that pattern.
func RenderSpinner() string {
	// In a real implementation, you would return a spinner.Model
	// and manage its Ticks via commands. For a static component, we return a char.
	return "⏳"
}
