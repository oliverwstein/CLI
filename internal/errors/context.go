// Package errors provides enhanced error context and propagation mechanisms
// for the Universal Application Console. It implements structured error handling
// with diagnostic context preservation and actionable recovery suggestions.
package errors

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/universal-console/console/internal/interfaces"
	"github.com/universal-console/console/internal/logging"
)

// ErrorType categorizes different types of errors for appropriate handling
type ErrorType string

const (
	ErrorTypeConnection     ErrorType = "connection"
	ErrorTypeAuthentication ErrorType = "authentication"
	ErrorTypeConfiguration  ErrorType = "configuration"
	ErrorTypeProtocol       ErrorType = "protocol"
	ErrorTypeNetwork        ErrorType = "network"
	ErrorTypeValidation     ErrorType = "validation"
	ErrorTypeRuntime        ErrorType = "runtime"
	ErrorTypeUserInterface  ErrorType = "ui"
)

// ErrorSeverity indicates the impact level of an error
type ErrorSeverity string

const (
	SeverityLow      ErrorSeverity = "low"
	SeverityMedium   ErrorSeverity = "medium"
	SeverityHigh     ErrorSeverity = "high"
	SeverityCritical ErrorSeverity = "critical"
)

// ContextualError provides enhanced error information with diagnostic context
type ContextualError struct {
	Type         ErrorType              `json:"type"`
	Severity     ErrorSeverity          `json:"severity"`
	Message      string                 `json:"message"`
	UserMessage  string                 `json:"userMessage,omitempty"`
	Code         string                 `json:"code,omitempty"`
	Component    string                 `json:"component"`
	Operation    string                 `json:"operation,omitempty"`
	Context      map[string]interface{} `json:"context,omitempty"`
	Timestamp    time.Time              `json:"timestamp"`
	StackTrace   []string               `json:"stackTrace,omitempty"`
	Cause        error                  `json:"-"`
	Recoverable  bool                   `json:"recoverable"`
	RetryAfter   *time.Duration         `json:"retryAfter,omitempty"`
	Actions      []interfaces.Action    `json:"actions,omitempty"`
}

// Error implements the error interface
func (e *ContextualError) Error() string {
	return fmt.Sprintf("[%s:%s] %s", e.Component, e.Type, e.Message)
}

// Unwrap provides access to the underlying error
func (e *ContextualError) Unwrap() error {
	return e.Cause
}

// GetUserMessage returns a user-friendly error message
func (e *ContextualError) GetUserMessage() string {
	if e.UserMessage != "" {
		return e.UserMessage
	}
	return e.Message
}

// IsRecoverable indicates if the error can potentially be resolved
func (e *ContextualError) IsRecoverable() bool {
	return e.Recoverable
}

// GetRecoveryActions returns suggested actions for error recovery
func (e *ContextualError) GetRecoveryActions() []interfaces.Action {
	if len(e.Actions) > 0 {
		return e.Actions
	}
	
	// Generate default recovery actions based on error type
	switch e.Type {
	case ErrorTypeConnection:
		return []interfaces.Action{
			{Name: "Retry Connection", Command: "retry_connection", Type: "primary", Icon: "🔄"},
			{Name: "Check Network", Command: "check_network", Type: "info", Icon: "🌐"},
			{Name: "Use Different Host", Command: "change_host", Type: "alternative", Icon: "🔀"},
		}
	case ErrorTypeAuthentication:
		return []interfaces.Action{
			{Name: "Update Credentials", Command: "update_auth", Type: "primary", Icon: "🔑"},
			{Name: "Refresh Token", Command: "refresh_token", Type: "info", Icon: "🔄"},
			{Name: "Login Again", Command: "relogin", Type: "alternative", Icon: "👤"},
		}
	case ErrorTypeConfiguration:
		return []interfaces.Action{
			{Name: "Edit Configuration", Command: "edit_config", Type: "primary", Icon: "⚙️"},
			{Name: "Reset to Defaults", Command: "reset_config", Type: "alternative", Icon: "🔄"},
			{Name: "Show Config Help", Command: "config_help", Type: "info", Icon: "❓"},
		}
	default:
		return []interfaces.Action{
			{Name: "Retry Operation", Command: "retry", Type: "primary", Icon: "🔄"},
			{Name: "Report Issue", Command: "report_issue", Type: "info", Icon: "🐛"},
		}
	}
}

// ErrorBuilder provides a fluent interface for creating contextual errors
type ErrorBuilder struct {
	err       *ContextualError
	logger    *logging.Logger
	captureStack bool
}

// NewErrorBuilder creates a new error builder with default settings
func NewErrorBuilder(errorType ErrorType, component string) *ErrorBuilder {
	return &ErrorBuilder{
		err: &ContextualError{
			Type:        errorType,
			Severity:    SeverityMedium,
			Component:   component,
			Context:     make(map[string]interface{}),
			Timestamp:   time.Now(),
			Recoverable: true,
		},
		logger:       logging.GetGlobalLogger().WithComponent(component),
		captureStack: true,
	}
}

// WithSeverity sets the error severity level
func (eb *ErrorBuilder) WithSeverity(severity ErrorSeverity) *ErrorBuilder {
	eb.err.Severity = severity
	return eb
}

// WithMessage sets the technical error message
func (eb *ErrorBuilder) WithMessage(message string) *ErrorBuilder {
	eb.err.Message = message
	return eb
}

// WithUserMessage sets a user-friendly error message
func (eb *ErrorBuilder) WithUserMessage(userMessage string) *ErrorBuilder {
	eb.err.UserMessage = userMessage
	return eb
}

// WithCode sets an error code for categorization
func (eb *ErrorBuilder) WithCode(code string) *ErrorBuilder {
	eb.err.Code = code
	return eb
}

// WithOperation sets the operation that failed
func (eb *ErrorBuilder) WithOperation(operation string) *ErrorBuilder {
	eb.err.Operation = operation
	return eb
}

// WithCause sets the underlying error that caused this error
func (eb *ErrorBuilder) WithCause(cause error) *ErrorBuilder {
	eb.err.Cause = cause
	return eb
}

// WithContext adds contextual information to the error
func (eb *ErrorBuilder) WithContext(key string, value interface{}) *ErrorBuilder {
	eb.err.Context[key] = value
	return eb
}

// WithContextMap adds multiple context values
func (eb *ErrorBuilder) WithContextMap(context map[string]interface{}) *ErrorBuilder {
	for k, v := range context {
		eb.err.Context[k] = v
	}
	return eb
}

// WithRecoverable sets whether the error is recoverable
func (eb *ErrorBuilder) WithRecoverable(recoverable bool) *ErrorBuilder {
	eb.err.Recoverable = recoverable
	return eb
}

// WithRetryAfter sets a suggested retry delay
func (eb *ErrorBuilder) WithRetryAfter(delay time.Duration) *ErrorBuilder {
	eb.err.RetryAfter = &delay
	return eb
}

// WithActions sets custom recovery actions
func (eb *ErrorBuilder) WithActions(actions []interfaces.Action) *ErrorBuilder {
	eb.err.Actions = actions
	return eb
}

// WithoutStackTrace disables stack trace capture
func (eb *ErrorBuilder) WithoutStackTrace() *ErrorBuilder {
	eb.captureStack = false
	return eb
}

// Build creates the contextual error and logs it appropriately
func (eb *ErrorBuilder) Build() *ContextualError {
	if eb.captureStack {
		eb.err.StackTrace = captureStackTrace(3) // Skip Build, caller, and runtime frames
	}
	
	// Log the error with appropriate level based on severity
	logFields := map[string]interface{}{
		"error_type":   eb.err.Type,
		"severity":     eb.err.Severity,
		"operation":    eb.err.Operation,
		"recoverable":  eb.err.Recoverable,
		"error_code":   eb.err.Code,
	}
	
	// Add context fields to log
	for k, v := range eb.err.Context {
		logFields["ctx_"+k] = v
	}
	
	logMessage := eb.err.Message
	if eb.err.Cause != nil {
		logMessage = fmt.Sprintf("%s: %v", eb.err.Message, eb.err.Cause)
	}
	
	loggerWithFields := eb.logger.WithFields(logFields)
	
	switch eb.err.Severity {
	case SeverityCritical:
		loggerWithFields.Error(logMessage)
	case SeverityHigh:
		loggerWithFields.Error(logMessage)
	case SeverityMedium:
		loggerWithFields.Warn(logMessage)
	case SeverityLow:
		loggerWithFields.Info(logMessage)
	}
	
	return eb.err
}

// captureStackTrace captures the current stack trace
func captureStackTrace(skip int) []string {
	var traces []string
	for i := skip; i < skip+10; i++ { // Capture up to 10 frames
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		
		fn := runtime.FuncForPC(pc)
		funcName := "unknown"
		if fn != nil {
			funcName = fn.Name()
		}
		
		// Simplify file path to just filename
		if idx := strings.LastIndex(file, "/"); idx >= 0 {
			file = file[idx+1:]
		}
		
		traces = append(traces, fmt.Sprintf("%s:%d %s", file, line, funcName))
	}
	return traces
}

// Component-specific error builders
func NewConnectionError(component string) *ErrorBuilder {
	return NewErrorBuilder(ErrorTypeConnection, component).WithSeverity(SeverityHigh)
}

func NewAuthenticationError(component string) *ErrorBuilder {
	return NewErrorBuilder(ErrorTypeAuthentication, component).WithSeverity(SeverityHigh)
}

func NewConfigurationError(component string) *ErrorBuilder {
	return NewErrorBuilder(ErrorTypeConfiguration, component).WithSeverity(SeverityMedium)
}

func NewProtocolError(component string) *ErrorBuilder {
	return NewErrorBuilder(ErrorTypeProtocol, component).WithSeverity(SeverityHigh)
}

func NewNetworkError(component string) *ErrorBuilder {
	return NewErrorBuilder(ErrorTypeNetwork, component).WithSeverity(SeverityMedium)
}

func NewValidationError(component string) *ErrorBuilder {
	return NewErrorBuilder(ErrorTypeValidation, component).WithSeverity(SeverityMedium)
}

func NewRuntimeError(component string) *ErrorBuilder {
	return NewErrorBuilder(ErrorTypeRuntime, component).WithSeverity(SeverityHigh)
}

func NewUIError(component string) *ErrorBuilder {
	return NewErrorBuilder(ErrorTypeUserInterface, component).WithSeverity(SeverityLow)
}

// ErrorChain represents a sequence of related errors
type ErrorChain struct {
	errors []error
	logger *logging.Logger
}

// NewErrorChain creates a new error chain
func NewErrorChain(logger *logging.Logger) *ErrorChain {
	return &ErrorChain{
		errors: make([]error, 0),
		logger: logger,
	}
}

// Add appends an error to the chain
func (ec *ErrorChain) Add(err error) *ErrorChain {
	if err != nil {
		ec.errors = append(ec.errors, err)
		if ec.logger != nil {
			ec.logger.Debug("Error added to chain", "error", err.Error(), "chain_length", len(ec.errors))
		}
	}
	return ec
}

// HasErrors returns true if the chain contains any errors
func (ec *ErrorChain) HasErrors() bool {
	return len(ec.errors) > 0
}

// GetErrors returns all errors in the chain
func (ec *ErrorChain) GetErrors() []error {
	return ec.errors
}

// GetFirst returns the first error in the chain
func (ec *ErrorChain) GetFirst() error {
	if len(ec.errors) > 0 {
		return ec.errors[0]
	}
	return nil
}

// GetLast returns the last error in the chain
func (ec *ErrorChain) GetLast() error {
	if len(ec.errors) > 0 {
		return ec.errors[len(ec.errors)-1]
	}
	return nil
}

// ToCombinedError creates a single error that represents the entire chain
func (ec *ErrorChain) ToCombinedError(component string) *ContextualError {
	if !ec.HasErrors() {
		return nil
	}
	
	messages := make([]string, len(ec.errors))
	for i, err := range ec.errors {
		messages[i] = err.Error()
	}
	
	return NewErrorBuilder(ErrorTypeRuntime, component).
		WithMessage(fmt.Sprintf("Multiple errors occurred: %s", strings.Join(messages, "; "))).
		WithUserMessage(fmt.Sprintf("%d errors occurred during operation", len(ec.errors))).
		WithCause(ec.GetFirst()).
		WithContext("error_count", len(ec.errors)).
		WithContext("all_errors", messages).
		Build()
}

// ErrorRecoveryContext provides context for error recovery operations
type ErrorRecoveryContext struct {
	OriginalError *ContextualError
	AttemptCount  int
	MaxAttempts   int
	RetryDelay    time.Duration
	Context       context.Context
	Logger        *logging.Logger
}

// CanRetry determines if another retry attempt is allowed
func (erc *ErrorRecoveryContext) CanRetry() bool {
	if erc.Context.Err() != nil {
		return false // Context cancelled
	}
	return erc.AttemptCount < erc.MaxAttempts && erc.OriginalError.IsRecoverable()
}

// WaitForRetry waits for the appropriate retry delay
func (erc *ErrorRecoveryContext) WaitForRetry() error {
	select {
	case <-erc.Context.Done():
		return erc.Context.Err()
	case <-time.After(erc.RetryDelay):
		return nil
	}
}

// IncrementAttempt increases the attempt count and adjusts retry delay
func (erc *ErrorRecoveryContext) IncrementAttempt() {
	erc.AttemptCount++
	// Exponential backoff
	erc.RetryDelay = time.Duration(float64(erc.RetryDelay) * 1.5)
	
	if erc.Logger != nil {
		erc.Logger.Debug("Retry attempt incremented",
			"attempt", erc.AttemptCount,
			"max_attempts", erc.MaxAttempts,
			"next_delay", erc.RetryDelay)
	}
}