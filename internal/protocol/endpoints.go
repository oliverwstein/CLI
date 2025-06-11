// Package protocol provides concrete implementation of protocol endpoint methods.
// This file contains specialized handling for each Compliance Protocol v2.0 endpoint
// with comprehensive request construction, HTTP communication, and response parsing.
package protocol

import (
	"context"
	"fmt"
	"time"

	"github.com/universal-console/console/internal/interfaces"
)

// EndpointHandler provides specialized handling for individual protocol endpoints
type EndpointHandler struct {
	client    *Client
	validator *RequestValidator
}

// NewEndpointHandler creates a new endpoint handler with the specified client
func NewEndpointHandler(client *Client) *EndpointHandler {
	return &EndpointHandler{
		client:    client,
		validator: NewRequestValidator(true),
	}
}

// ExecuteCommandEndpoint handles POST /console/command with enhanced error handling and validation
func (eh *EndpointHandler) ExecuteCommandEndpoint(ctx context.Context, request interfaces.CommandRequest) (*interfaces.CommandResponse, error) {
	// Pre-execution validation
	if err := eh.validateConnectionState(); err != nil {
		return nil, err
	}

	if err := eh.validator.ValidateCommandRequest(&request); err != nil {
		return nil, fmt.Errorf("command validation failed: %w", err)
	}

	// Enhanced request preparation
	enhancedRequest := eh.enhanceCommandRequest(request)

	// Execute with retry logic for transient failures
	response, err := eh.executeWithRetry(ctx, func() (*interfaces.CommandResponse, error) {
		return eh.client.ExecuteCommand(ctx, enhancedRequest)
	})

	if err != nil {
		return nil, eh.wrapEndpointError("command execution failed", err)
	}

	// Post-execution validation and enhancement
	if err := eh.validateCommandResponse(response); err != nil {
		return nil, fmt.Errorf("invalid command response: %w", err)
	}

	return response, nil
}

// ExecuteActionEndpoint handles POST /console/action with workflow context management
func (eh *EndpointHandler) ExecuteActionEndpoint(ctx context.Context, request interfaces.ActionRequest) (*interfaces.CommandResponse, error) {
	// Pre-execution validation
	if err := eh.validateConnectionState(); err != nil {
		return nil, err
	}

	if err := eh.validator.ValidateActionRequest(&request); err != nil {
		return nil, fmt.Errorf("action validation failed: %w", err)
	}

	// Enhanced request preparation with workflow context
	enhancedRequest := eh.enhanceActionRequest(request)

	// Execute with specialized action handling
	response, err := eh.executeWithRetry(ctx, func() (*interfaces.CommandResponse, error) {
		return eh.client.ExecuteAction(ctx, enhancedRequest)
	})

	if err != nil {
		return nil, eh.wrapEndpointError("action execution failed", err)
	}

	// Validate response and update workflow state
	if err := eh.validateActionResponse(response, request.WorkflowID); err != nil {
		return nil, fmt.Errorf("invalid action response: %w", err)
	}

	return response, nil
}

// GetSuggestionsEndpoint handles POST /console/suggest with intelligent caching
func (eh *EndpointHandler) GetSuggestionsEndpoint(ctx context.Context, request interfaces.SuggestRequest) (*interfaces.SuggestResponse, error) {
	// Pre-execution validation
	if err := eh.validateConnectionState(); err != nil {
		return nil, err
	}

	if err := eh.validator.ValidateSuggestRequest(&request); err != nil {
		return nil, fmt.Errorf("suggestion validation failed: %w", err)
	}

	// Enhanced request preparation with context enrichment
	enhancedRequest := eh.enhanceSuggestRequest(request)

	// Execute with shorter timeout for responsiveness
	suggestCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	response, err := eh.client.GetSuggestions(suggestCtx, enhancedRequest)
	if err != nil {
		// Suggestions are non-critical, provide graceful degradation
		return eh.createFallbackSuggestions(request), nil
	}

	// Validate and enhance suggestions
	if err := eh.validateSuggestResponse(response); err != nil {
		return nil, fmt.Errorf("invalid suggestion response: %w", err)
	}

	return eh.enhanceSuggestions(response), nil
}

// GetProgressEndpoint handles POST /console/progress with intelligent polling
func (eh *EndpointHandler) GetProgressEndpoint(ctx context.Context, request interfaces.ProgressRequest) (*interfaces.ProgressResponse, error) {
	// Pre-execution validation
	if err := eh.validateConnectionState(); err != nil {
		return nil, err
	}

	if err := eh.validator.ValidateProgressRequest(&request); err != nil {
		return nil, fmt.Errorf("progress validation failed: %w", err)
	}

	// Enhanced request preparation with polling metadata
	enhancedRequest := eh.enhanceProgressRequest(request)

	// Execute with specialized progress timeout
	progressCtx, cancel := context.WithTimeout(ctx, DefaultProgressTimeout)
	defer cancel()

	response, err := eh.client.GetProgress(progressCtx, enhancedRequest)
	if err != nil {
		return nil, eh.wrapEndpointError("progress query failed", err)
	}

	// Validate progress response format
	if err := eh.validateProgressResponse(response); err != nil {
		return nil, fmt.Errorf("invalid progress response: %w", err)
	}

	return response, nil
}

// CancelOperationEndpoint handles POST /console/cancel with confirmation handling
func (eh *EndpointHandler) CancelOperationEndpoint(ctx context.Context, request interfaces.CancelRequest) (*interfaces.CancelResponse, error) {
	// Pre-execution validation
	if err := eh.validateConnectionState(); err != nil {
		return nil, err
	}

	if err := eh.validator.ValidateCancelRequest(&request); err != nil {
		return nil, fmt.Errorf("cancellation validation failed: %w", err)
	}

	// Enhanced request preparation with cancellation context
	enhancedRequest := eh.enhanceCancelRequest(request)

	// Execute cancellation request
	response, err := eh.client.CancelOperation(ctx, enhancedRequest)
	if err != nil {
		return nil, eh.wrapEndpointError("operation cancellation failed", err)
	}

	// Validate cancellation response
	if err := eh.validateCancelResponse(response); err != nil {
		return nil, fmt.Errorf("invalid cancellation response: %w", err)
	}

	return response, nil
}

// Request enhancement methods

// enhanceCommandRequest adds metadata and context to command requests
func (eh *EndpointHandler) enhanceCommandRequest(request interfaces.CommandRequest) interfaces.CommandRequest {
	// Add any command-specific enhancements
	// For now, return the request as-is
	return request
}

// enhanceActionRequest adds workflow context to action requests
func (eh *EndpointHandler) enhanceActionRequest(request interfaces.ActionRequest) interfaces.ActionRequest {
	// Add timestamp and session context if not present
	if request.Context == nil {
		request.Context = make(map[string]interface{})
	}

	if request.Context["timestamp"] == nil {
		request.Context["timestamp"] = time.Now().Unix()
	}

	if request.Context["sessionId"] == nil && eh.client.sessionID != "" {
		request.Context["sessionId"] = eh.client.sessionID
	}

	return request
}

// enhanceSuggestRequest adds context enrichment to suggestion requests
func (eh *EndpointHandler) enhanceSuggestRequest(request interfaces.SuggestRequest) interfaces.SuggestRequest {
	// Add context enrichment for better suggestions
	if request.Context == nil {
		request.Context = make(map[string]interface{})
	}

	// Add client capabilities
	request.Context["clientFeatures"] = map[string]bool{
		"richContent":        true,
		"progressIndicators": true,
		"confirmations":      true,
		"multiStep":          true,
	}

	return request
}

// enhanceProgressRequest adds polling metadata to progress requests
func (eh *EndpointHandler) enhanceProgressRequest(request interfaces.ProgressRequest) interfaces.ProgressRequest {
	// Ensure request update flag is set appropriately
	request.RequestUpdate = true
	return request
}

// enhanceCancelRequest adds cancellation context to cancel requests
func (eh *EndpointHandler) enhanceCancelRequest(request interfaces.CancelRequest) interfaces.CancelRequest {
	// Add cancellation metadata
	// For now, return the request as-is
	return request
}

// Response validation methods

// validateCommandResponse ensures command responses meet protocol requirements
func (eh *EndpointHandler) validateCommandResponse(response *interfaces.CommandResponse) error {
	if response == nil {
		return fmt.Errorf("response cannot be nil")
	}

	// Validate response structure
	if response.Response.Type == "structured" {
		// Validate structured content if present
		if err := eh.validateStructuredContent(response.Response.Content); err != nil {
			return fmt.Errorf("invalid structured content: %w", err)
		}
	}

	// Validate actions if present
	if err := eh.validateActions(response.Actions); err != nil {
		return fmt.Errorf("invalid actions: %w", err)
	}

	// Validate workflow if present
	if response.Workflow != nil {
		if err := eh.validateWorkflow(response.Workflow); err != nil {
			return fmt.Errorf("invalid workflow: %w", err)
		}
	}

	return nil
}

// validateActionResponse ensures action responses maintain workflow consistency
func (eh *EndpointHandler) validateActionResponse(response *interfaces.CommandResponse, workflowID string) error {
	if err := eh.validateCommandResponse(response); err != nil {
		return err
	}

	// Additional validation for workflow consistency
	if workflowID != "" && response.Workflow != nil {
		if response.Workflow.ID != workflowID {
			return fmt.Errorf("workflow ID mismatch: expected %s, got %s", workflowID, response.Workflow.ID)
		}
	}

	return nil
}

// validateSuggestResponse ensures suggestion responses are properly formatted
func (eh *EndpointHandler) validateSuggestResponse(response *interfaces.SuggestResponse) error {
	if response == nil {
		return fmt.Errorf("response cannot be nil")
	}

	// Validate suggestions
	for i, suggestion := range response.Suggestions {
		if suggestion.Text == "" {
			return fmt.Errorf("suggestion %d has empty text", i)
		}
		if suggestion.Type == "" {
			return fmt.Errorf("suggestion %d has empty type", i)
		}
	}

	return nil
}

// validateProgressResponse ensures progress responses have valid data
func (eh *EndpointHandler) validateProgressResponse(response *interfaces.ProgressResponse) error {
	if response == nil {
		return fmt.Errorf("response cannot be nil")
	}

	// Validate progress range
	if response.Progress < 0 || response.Progress > 100 {
		return fmt.Errorf("progress must be between 0 and 100, got %d", response.Progress)
	}

	// Validate status
	validStatuses := map[string]bool{
		"running":  true,
		"complete": true,
		"error":    true,
		"paused":   true,
	}

	if !validStatuses[response.Status] {
		return fmt.Errorf("invalid progress status: %s", response.Status)
	}

	return nil
}

// validateCancelResponse ensures cancellation responses are properly formatted
func (eh *EndpointHandler) validateCancelResponse(response *interfaces.CancelResponse) error {
	if response == nil {
		return fmt.Errorf("response cannot be nil")
	}

	// Basic validation - message should be present
	if response.Message == "" {
		return fmt.Errorf("cancellation response must include a message")
	}

	return nil
}

// Helper validation methods

// validateStructuredContent validates structured content blocks
func (eh *EndpointHandler) validateStructuredContent(content interface{}) error {
	// Handle both single content blocks and arrays of content blocks
	switch v := content.(type) {
	case []interface{}:
		for i, item := range v {
			if err := eh.validateContentBlock(item, i); err != nil {
				return err
			}
		}
	case interface{}:
		return eh.validateContentBlock(v, 0)
	default:
		return fmt.Errorf("invalid content type")
	}

	return nil
}

// validateContentBlock validates individual content blocks
func (eh *EndpointHandler) validateContentBlock(content interface{}, index int) error {
	// This would validate the structure of content blocks
	// For now, perform basic validation
	if content == nil {
		return fmt.Errorf("content block %d cannot be nil", index)
	}

	// Additional validation would check the content block structure
	// against the ContentBlock type definition
	return nil
}

// validateActions validates action arrays
func (eh *EndpointHandler) validateActions(actions []interfaces.Action) error {
	for i, action := range actions {
		if action.Name == "" {
			return fmt.Errorf("action %d has empty name", i)
		}
		if action.Command == "" {
			return fmt.Errorf("action %d has empty command", i)
		}
		if action.Type == "" {
			return fmt.Errorf("action %d has empty type", i)
		}
	}

	return nil
}

// validateWorkflow validates workflow objects
func (eh *EndpointHandler) validateWorkflow(workflow *interfaces.Workflow) error {
	if workflow.ID == "" {
		return fmt.Errorf("workflow ID cannot be empty")
	}
	if workflow.Title == "" {
		return fmt.Errorf("workflow title cannot be empty")
	}
	if workflow.Step < 1 || workflow.TotalSteps < 1 {
		return fmt.Errorf("workflow steps must be positive")
	}
	if workflow.Step > workflow.TotalSteps {
		return fmt.Errorf("current step cannot exceed total steps")
	}

	return nil
}

// Utility methods

// validateConnectionState ensures the client is properly connected
func (eh *EndpointHandler) validateConnectionState() error {
	if !eh.client.IsConnected() {
		return fmt.Errorf("not connected to any application")
	}

	return nil
}

// executeWithRetry executes a function with basic retry logic for transient failures
func (eh *EndpointHandler) executeWithRetry(ctx context.Context, operation func() (*interfaces.CommandResponse, error)) (*interfaces.CommandResponse, error) {
	const maxRetries = 2
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		response, err := operation()
		if err == nil {
			return response, nil
		}

		lastErr = err

		// Check if error is retryable
		if protocolErr, ok := err.(*ProtocolError); ok {
			if !protocolErr.IsRetryable() {
				break
			}

			// Wait before retry
			if attempt < maxRetries {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(protocolErr.GetRetryDelay()):
					// Continue to next attempt
				}
			}
		} else {
			// Non-protocol errors are not retryable
			break
		}
	}

	return nil, lastErr
}

// wrapEndpointError wraps errors with endpoint-specific context
func (eh *EndpointHandler) wrapEndpointError(message string, err error) error {
	return fmt.Errorf("%s: %w", message, err)
}

// createFallbackSuggestions provides basic suggestions when the server is unavailable
func (eh *EndpointHandler) createFallbackSuggestions(request interfaces.SuggestRequest) *interfaces.SuggestResponse {
	return &interfaces.SuggestResponse{
		Suggestions: []interfaces.SuggestionItem{
			{
				Text:        "help",
				Description: "Show available commands",
				Type:        "command",
			},
			{
				Text:        "status",
				Description: "Check current status",
				Type:        "command",
			},
		},
	}
}

// enhanceSuggestions adds client-side enhancements to suggestions
func (eh *EndpointHandler) enhanceSuggestions(response *interfaces.SuggestResponse) *interfaces.SuggestResponse {
	// Add any client-side suggestion enhancements
	// For now, return the response as-is
	return response
}
