// Package protocol implements the Compliance Protocol v2.0 communication layer.
// This file defines all request and response structures that correspond exactly
// to the JSON specifications outlined in section 4 of the design document.
package protocol

import (
	"fmt"
	"strings"
	"time"

	"github.com/universal-console/console/internal/interfaces"
)

// Protocol version constant for handshake validation
const ProtocolVersion = "2.0"

// HTTP endpoint paths as defined in the Compliance Protocol specification
const (
	EndpointSpec     = "/console/spec"
	EndpointCommand  = "/console/command"
	EndpointAction   = "/console/action"
	EndpointSuggest  = "/console/suggest"
	EndpointProgress = "/console/progress"
	EndpointCancel   = "/console/cancel"
)

// HTTP timeout configurations for reliable communication
const (
	DefaultConnectTimeout  = 10 * time.Second
	DefaultRequestTimeout  = 30 * time.Second
	DefaultProgressTimeout = 5 * time.Second
	HandshakeTimeout       = 15 * time.Second
)

// SpecRequest represents the handshake request to retrieve application metadata
// This is sent as a GET request with no body, but the struct maintains consistency
type SpecRequest struct {
	// No fields required for GET request
}

// SpecResponseInternal extends the interface SpecResponse with internal tracking
type SpecResponseInternal struct {
	interfaces.SpecResponse
	ReceivedAt    time.Time         `json:"-"`
	ServerHeaders map[string]string `json:"-"`
}

// CommandRequestInternal extends the interface CommandRequest with request metadata
type CommandRequestInternal struct {
	interfaces.CommandRequest
	RequestID string    `json:"requestId,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

// ActionRequestInternal extends the interface ActionRequest with tracking information
type ActionRequestInternal struct {
	interfaces.ActionRequest
	RequestID string    `json:"requestId,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

// SuggestRequestInternal extends the interface SuggestRequest with context tracking
type SuggestRequestInternal struct {
	interfaces.SuggestRequest
	RequestID string    `json:"requestId,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

// ProgressRequestInternal extends the interface ProgressRequest with polling metadata
type ProgressRequestInternal struct {
	interfaces.ProgressRequest
	RequestID   string    `json:"requestId,omitempty"`
	Timestamp   time.Time `json:"timestamp,omitempty"`
	PollAttempt int       `json:"pollAttempt,omitempty"`
}

// CancelRequestInternal extends the interface CancelRequest with cancellation tracking
type CancelRequestInternal struct {
	interfaces.CancelRequest
	RequestID string    `json:"requestId,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty"`
	Reason    string    `json:"reason,omitempty"`
}

// ResponseMetadata contains common metadata for all response types
type ResponseMetadata struct {
	RequestID     string        `json:"requestId,omitempty"`
	ResponseTime  time.Duration `json:"responseTime,omitempty"`
	ServerVersion string        `json:"serverVersion,omitempty"`
	ContentLength int64         `json:"contentLength,omitempty"`
}

// CommandResponseInternal extends the interface CommandResponse with response metadata
type CommandResponseInternal struct {
	interfaces.CommandResponse
	Metadata ResponseMetadata `json:"-"`
}

// SuggestResponseInternal extends the interface SuggestResponse with response metadata
type SuggestResponseInternal struct {
	interfaces.SuggestResponse
	Metadata ResponseMetadata `json:"-"`
}

// ProgressResponseInternal extends the interface ProgressResponse with polling metadata
type ProgressResponseInternal struct {
	interfaces.ProgressResponse
	Metadata   ResponseMetadata `json:"-"`
	LastUpdate time.Time        `json:"lastUpdate,omitempty"`
}

// CancelResponseInternal extends the interface CancelResponse with cancellation metadata
type CancelResponseInternal struct {
	interfaces.CancelResponse
	Metadata ResponseMetadata `json:"-"`
}

// ErrorResponseInternal extends the interface ErrorResponse with error tracking
type ErrorResponseInternal struct {
	interfaces.ErrorResponse
	Metadata    ResponseMetadata `json:"-"`
	HTTPStatus  int              `json:"-"`
	RetryAfter  *time.Duration   `json:"-"`
	Recoverable bool             `json:"-"`
}

// ConnectionState represents the current state of the protocol client connection
type ConnectionState struct {
	Connected     bool                   `json:"connected"`
	Host          string                 `json:"host"`
	AppName       string                 `json:"appName,omitempty"`
	AppVersion    string                 `json:"appVersion,omitempty"`
	LastHandshake time.Time              `json:"lastHandshake,omitempty"`
	Features      map[string]bool        `json:"features,omitempty"`
	Auth          *interfaces.AuthConfig `json:"-"` // Add this field to store current auth config
	LastError     error                  `json:"lastError,omitempty"`
	Statistics    ConnectionStatistics   `json:"statistics"`
}

// ConnectionStatistics tracks communication metrics for monitoring and debugging
type ConnectionStatistics struct {
	TotalRequests       int           `json:"totalRequests"`
	SuccessfulRequests  int           `json:"successfulRequests"`
	FailedRequests      int           `json:"failedRequests"`
	AverageResponseTime time.Duration `json:"averageResponseTime"`
	LastRequestTime     time.Time     `json:"lastRequestTime"`
	BytesSent           int64         `json:"bytesSent"`
	BytesReceived       int64         `json:"bytesReceived"`
}

// RequestContext provides context information for protocol requests
type RequestContext struct {
	UserAgent       string            `json:"userAgent"`
	ClientVersion   string            `json:"clientVersion"`
	SessionID       string            `json:"sessionId,omitempty"`
	RequestMetadata map[string]string `json:"requestMetadata,omitempty"`
}

// AuthenticationContext contains authentication-related request information
type AuthenticationContext struct {
	Type            string    `json:"type"`
	TokenExpiry     time.Time `json:"tokenExpiry,omitempty"`
	RefreshRequired bool      `json:"refreshRequired,omitempty"`
	Permissions     []string  `json:"permissions,omitempty"`
}

// HTTPErrorDetails provides detailed information about HTTP-level errors
type HTTPErrorDetails struct {
	StatusCode    int               `json:"statusCode"`
	StatusText    string            `json:"statusText"`
	Headers       map[string]string `json:"headers"`
	Body          string            `json:"body,omitempty"`
	ContentType   string            `json:"contentType,omitempty"`
	ContentLength int64             `json:"contentLength"`
}

// NetworkErrorDetails provides information about network-level connection errors
type NetworkErrorDetails struct {
	ErrorType   string        `json:"errorType"` // "timeout", "connection_refused", "dns_failure", etc.
	Timeout     time.Duration `json:"timeout,omitempty"`
	RetryCount  int           `json:"retryCount"`
	LastAttempt time.Time     `json:"lastAttempt"`
}

// ProtocolError represents errors that occur during protocol communication
type ProtocolError struct {
	Type            string               `json:"type"` // "network", "http", "protocol", "authentication"
	Message         string               `json:"message"`
	HTTPDetails     *HTTPErrorDetails    `json:"httpDetails,omitempty"`
	NetworkDetails  *NetworkErrorDetails `json:"networkDetails,omitempty"`
	OriginalError   error                `json:"-"`
	Timestamp       time.Time            `json:"timestamp"`
	Recoverable     bool                 `json:"recoverable"`
	SuggestedAction string               `json:"suggestedAction,omitempty"`
}

// Error implements the error interface for ProtocolError
func (pe *ProtocolError) Error() string {
	return pe.Message
}

// Unwrap provides access to the original underlying error
func (pe *ProtocolError) Unwrap() error {
	return pe.OriginalError
}

// IsRetryable determines if the error condition might be resolved by retrying
func (pe *ProtocolError) IsRetryable() bool {
	switch pe.Type {
	case "network":
		return pe.NetworkDetails != nil && pe.NetworkDetails.ErrorType == "timeout"
	case "http":
		return pe.HTTPDetails != nil && (pe.HTTPDetails.StatusCode == 429 || pe.HTTPDetails.StatusCode >= 500)
	case "authentication":
		return false // Authentication errors typically require user intervention
	case "protocol":
		return false // Protocol errors indicate implementation issues
	default:
		return pe.Recoverable
	}
}

// GetRetryDelay calculates the appropriate delay before retrying the request
func (pe *ProtocolError) GetRetryDelay() time.Duration {
	if !pe.IsRetryable() {
		return 0
	}

	switch pe.Type {
	case "network":
		if pe.NetworkDetails != nil {
			// Exponential backoff for network errors
			baseDelay := time.Second
			return baseDelay * time.Duration(1<<pe.NetworkDetails.RetryCount)
		}
	case "http":
		if pe.HTTPDetails != nil && pe.HTTPDetails.StatusCode == 429 {
			// Honor Retry-After header if present, otherwise use default
			return 5 * time.Second
		}
	}

	return time.Second
}

// ValidationError represents errors in request validation before sending
type ValidationError struct {
	Field   string      `json:"field"`
	Message string      `json:"message"`
	Value   interface{} `json:"value,omitempty"`
}

// Error implements the error interface for ValidationError
func (ve *ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s': %s", ve.Field, ve.Message)
}

// RequestValidator provides validation for protocol requests before transmission
type RequestValidator struct {
	strictMode bool
}

// NewRequestValidator creates a new request validator with specified validation mode
func NewRequestValidator(strictMode bool) *RequestValidator {
	return &RequestValidator{
		strictMode: strictMode,
	}
}

// ValidateCommandRequest ensures command requests meet protocol requirements
func (rv *RequestValidator) ValidateCommandRequest(req *interfaces.CommandRequest) error {
	if req == nil {
		return &ValidationError{Field: "request", Message: "request cannot be nil"}
	}

	if strings.TrimSpace(req.Command) == "" {
		return &ValidationError{Field: "command", Message: "command cannot be empty"}
	}

	if rv.strictMode {
		// Additional strict mode validations
		if len(req.Command) > 1000 {
			return &ValidationError{Field: "command", Message: "command exceeds maximum length of 1000 characters"}
		}
	}

	return nil
}

// ValidateActionRequest ensures action requests meet protocol requirements
func (rv *RequestValidator) ValidateActionRequest(req *interfaces.ActionRequest) error {
	if req == nil {
		return &ValidationError{Field: "request", Message: "request cannot be nil"}
	}

	if strings.TrimSpace(req.Command) == "" {
		return &ValidationError{Field: "command", Message: "action command cannot be empty"}
	}

	if rv.strictMode {
		// Validate workflow ID format if present
		if req.WorkflowID != "" && !isValidWorkflowID(req.WorkflowID) {
			return &ValidationError{Field: "workflowId", Message: "invalid workflow ID format"}
		}
	}

	return nil
}

// ValidateSuggestRequest ensures suggestion requests meet protocol requirements
func (rv *RequestValidator) ValidateSuggestRequest(req *interfaces.SuggestRequest) error {
	if req == nil {
		return &ValidationError{Field: "request", Message: "request cannot be nil"}
	}

	// Current input can be empty for suggestions
	if rv.strictMode {
		if len(req.CurrentInput) > 500 {
			return &ValidationError{Field: "current_input", Message: "input exceeds maximum length of 500 characters"}
		}
	}

	return nil
}

// ValidateProgressRequest ensures progress requests meet protocol requirements
func (rv *RequestValidator) ValidateProgressRequest(req *interfaces.ProgressRequest) error {
	if req == nil {
		return &ValidationError{Field: "request", Message: "request cannot be nil"}
	}

	if strings.TrimSpace(req.OperationID) == "" {
		return &ValidationError{Field: "operationId", Message: "operation ID cannot be empty"}
	}

	return nil
}

// ValidateCancelRequest ensures cancellation requests meet protocol requirements
func (rv *RequestValidator) ValidateCancelRequest(req *interfaces.CancelRequest) error {
	if req == nil {
		return &ValidationError{Field: "request", Message: "request cannot be nil"}
	}

	if strings.TrimSpace(req.OperationID) == "" && strings.TrimSpace(req.WorkflowID) == "" {
		return &ValidationError{Field: "identifiers", Message: "either operationId or workflowId must be provided"}
	}

	return nil
}

// isValidWorkflowID performs basic validation on workflow ID format
func isValidWorkflowID(workflowID string) bool {
	// Basic validation: alphanumeric characters, hyphens, and underscores
	for _, char := range workflowID {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '-' || char == '_') {
			return false
		}
	}
	return len(workflowID) > 0 && len(workflowID) <= 100
}
