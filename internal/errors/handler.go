// Package errors implements comprehensive error management for the Universal Application Console.
// This file provides the Handler component, which processes structured error responses from applications,
// creates user-friendly error presentations, and prepares recovery action suggestions,
// as specified in section 4.7 of the design specification.
package errors

import (
	"fmt"
	"time"

	"github.com/universal-console/console/internal/interfaces"
)

// ProcessedError represents a structured error that has been prepared for display.
// It decouples the raw protocol error from the UI's representation.
type ProcessedError struct {
	Timestamp       time.Time
	Message         string
	Code            string
	Details         *interfaces.ContentBlock
	RecoveryActions []interfaces.Action
}

// Handler processes raw protocol errors into a format suitable for the UI.
type Handler struct {
	// In the future, this could hold dependencies, like a ContentRenderer
	// for pre-rendering details, but for now, it's stateless.
}

// NewHandler creates a new error handler.
func NewHandler() *Handler {
	return &Handler{}
}

// ProcessErrorResponse transforms a raw ErrorResponse from the protocol into a
// structured ProcessedError for the UI model to use.
func (h *Handler) ProcessErrorResponse(errResp *interfaces.ErrorResponse) (*ProcessedError, error) {
	if errResp == nil {
		return nil, fmt.Errorf("cannot process a nil error response")
	}

	processed := &ProcessedError{
		Timestamp:       time.Now(),
		Message:         errResp.Error.Message,
		Code:            errResp.Error.Code,
		Details:         errResp.Error.Details,
		RecoveryActions: errResp.Error.RecoveryActions,
	}

	// If no recovery actions are provided, add a default "Dismiss" action.
	if len(processed.RecoveryActions) == 0 {
		processed.RecoveryActions = append(processed.RecoveryActions, interfaces.Action{
			Name:    "Dismiss",
			Command: "internal_dismiss_error",
			Type:    "cancel",
			Icon:    "👌",
		})
	}

	return processed, nil
}
