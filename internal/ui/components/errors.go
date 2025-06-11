// Package components provides shared, reusable interface elements, including
// components for rendering structured errors. This file implements the visual
// presentation for errors, including messages, expandable details, and recovery
// action panes, as specified in the design document.
package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/universal-console/console/internal/errors"
	"github.com/universal-console/console/internal/interfaces"
)

// Styling for error components.
var (
	errorPaneStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder(), false, true, true, true).
			BorderForeground(lipgloss.Color("#F38BA8")).
			MarginTop(1).
			Padding(0, 1)

	errorHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#F38BA8"))

	errorCodeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAB387")).
			Italic(true)

	errorDetailsStyle = lipgloss.NewStyle().
				MarginTop(1).
				Border(lipgloss.NormalBorder(), true, false, false, false).
				BorderForeground(lipgloss.Color("#6C7086")).
				Foreground(lipgloss.Color("#CDD6F4"))

	recoveryTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#A6E3A1")).
				MarginTop(1)
)

// RenderErrorPane renders a complete error presentation, including the main message,
// code, details, and a title for the recovery actions that will be displayed
// separately in the Actions Pane.
func RenderErrorPane(
	currentError *errors.ProcessedError,
	contentRenderer interfaces.ContentRenderer,
	theme *interfaces.Theme,
	width int,
) string {
	if currentError == nil {
		return ""
	}

	var builder strings.Builder

	// Render Header
	header := fmt.Sprintf("❌ Error: %s", currentError.Message)
	builder.WriteString(errorHeaderStyle.Render(header))
	builder.WriteRune('\n')

	// Render Code, if available
	if currentError.Code != "" {
		code := fmt.Sprintf("   Code: %s", currentError.Code)
		builder.WriteString(errorCodeStyle.Render(code))
		builder.WriteRune('\n')
	}

	// Render Details, if available
	if currentError.Details != nil {
		// Delegate rendering of the structured details block to the content renderer.
		detailsContent, err := contentRenderer.RenderContent(
			[]interfaces.ContentBlock{*currentError.Details},
			theme,
		)
		if err == nil {
			var detailsText []string
			for _, line := range detailsContent {
				detailsText = append(detailsText, line.Text)
			}
			details := errorDetailsStyle.Render(strings.Join(detailsText, "\n"))
			builder.WriteString(details)
			builder.WriteRune('\n')
		}
	}

	// Render title for the recovery actions
	if len(currentError.RecoveryActions) > 0 {
		builder.WriteString(recoveryTitleStyle.Render("Recovery Actions:"))
	}

	return errorPaneStyle.Width(width - 4).Render(builder.String())
}
