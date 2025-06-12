// Package protocol implements HTTP communication with Compliant Applications.
// This file provides the concrete implementation of the ProtocolClient interface.
// It manages the complete request lifecycle, including request validation,
// authentication, execution with retries, and response parsing/validation.
package protocol

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/universal-console/console/internal/errors"
	"github.com/universal-console/console/internal/interfaces"
	"github.com/universal-console/console/internal/logging"
)

// Client implements the ProtocolClient interface with comprehensive HTTP communication capabilities
type Client struct {
	httpClient      *http.Client
	configManager   interfaces.ConfigManager
	authManager     interfaces.AuthManager
	validator       *RequestValidator
	connectionState *ConnectionState
	mutex           sync.RWMutex
	userAgent       string
	sessionID       string
	logger          *logging.Logger
}

// NewClient creates a new protocol client with injected dependencies and secure defaults
func NewClient(configManager interfaces.ConfigManager, authManager interfaces.AuthManager) (*Client, error) {
	if configManager == nil {
		return nil, fmt.Errorf("configManager cannot be nil")
	}

	if authManager == nil {
		return nil, fmt.Errorf("authManager cannot be nil")
	}

	httpClient := &http.Client{
		Timeout: DefaultRequestTimeout,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			IdleConnTimeout:     30 * time.Second,
			DisableCompression:  false,
			DisableKeepAlives:   false,
			MaxIdleConnsPerHost: 2,
		},
	}

	logger := logging.GetProtocolLogger().WithField("session_id", generateSessionID())
	
	client := &Client{
		httpClient:    httpClient,
		configManager: configManager,
		authManager:   authManager,
		validator:     NewRequestValidator(true), // Enable strict validation
		connectionState: &ConnectionState{
			Connected:  false,
			Statistics: ConnectionStatistics{},
		},
		userAgent: fmt.Sprintf("Universal-Console/%s (Protocol/%s)", "2.0.0", ProtocolVersion),
		sessionID: generateSessionID(),
		logger:    logger,
	}
	
	logger.Info("Protocol client initialized",
		logging.GetGlobalLogger().WithField("user_agent", client.userAgent))

	return client, nil
}

// Connect establishes connection and performs handshake with the application
func (c *Client) Connect(ctx context.Context, host string, auth *interfaces.AuthConfig) (*interfaces.SpecResponse, error) {
	startTime := time.Now()
	authType := "none"
	if auth != nil {
		authType = auth.Type
	}
	
	c.logger.LogConnectionAttempt(host, authType)
	c.logger.Debug("Starting connection process",
		"host", host,
		"timeout", HandshakeTimeout,
		"has_auth", auth != nil)

	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.logger.Debug("Validating connection parameters")
	if err := c.validateConnectionParams(host, auth); err != nil {
		c.logger.Error("Connection parameter validation failed", "error", err.Error())
		contextualErr := errors.NewConnectionError("protocol").
			WithMessage("Connection parameter validation failed").
			WithUserMessage("Invalid connection configuration. Please check host and authentication settings.").
			WithOperation("validate_connection_params").
			WithCause(err).
			WithContext("host", host).
			WithContext("auth_type", authType).
			Build()
		return nil, contextualErr
	}

	c.connectionState.Host = host
	c.connectionState.Connected = false
	c.connectionState.LastError = nil
	c.connectionState.Auth = auth // Store auth config for subsequent requests

	handshakeURL := c.buildURL(host, EndpointSpec)
	c.logger.Debug("Built handshake URL", "url", handshakeURL)
	
	handshakeCtx, cancel := context.WithTimeout(ctx, HandshakeTimeout)
	defer cancel()

	c.logger.Debug("Creating handshake request")
	req, err := c.createHandshakeRequest(handshakeCtx, handshakeURL, auth)
	if err != nil {
		c.logger.Error("Failed to create handshake request", "error", err.Error())
		contextualErr := errors.NewConnectionError("protocol").
			WithMessage("Failed to create handshake request").
			WithUserMessage("Unable to prepare connection request. Please check your connection settings.").
			WithOperation("create_handshake_request").
			WithCause(err).
			WithContext("url", handshakeURL).
			WithContext("host", host).
			Build()
		c.connectionState.LastError = contextualErr
		return nil, contextualErr
	}

	c.logger.Debug("Executing handshake request", "method", req.Method, "url", req.URL.String())
	requestStartTime := time.Now()
	resp, err := c.httpClient.Do(req)
	requestDuration := time.Since(requestStartTime)
	c.updateRequestStatisticsUnsafe(requestDuration, err == nil)

	if err != nil {
		c.logger.LogConnectionFailure(host, err, requestDuration)
		c.logger.Error("Handshake HTTP request failed", 
			"error", err.Error(),
			"duration", requestDuration,
			"url", handshakeURL)
		
		contextualErr := errors.NewNetworkError("protocol").
			WithMessage("Handshake request failed").
			WithUserMessage("Unable to connect to the application. Please check if the server is running and accessible.").
			WithOperation("handshake_request").
			WithCause(err).
			WithContext("host", host).
			WithContext("url", handshakeURL).
			WithContext("duration", requestDuration).
			WithRetryAfter(5 * time.Second).
			Build()
		c.connectionState.LastError = contextualErr
		return nil, contextualErr
	}
	defer resp.Body.Close()

	c.logger.Debug("Received handshake response", 
		"status_code", resp.StatusCode,
		"status", resp.Status,
		"duration", requestDuration)

	specResponse, err := c.processHandshakeResponse(resp)
	if err != nil {
		c.logger.Error("Handshake response processing failed", "error", err.Error())
		contextualErr := errors.NewProtocolError("protocol").
			WithMessage("Invalid handshake response").
			WithUserMessage("The server response is not compatible with this client. Please check if you're connecting to a supported application.").
			WithOperation("process_handshake_response").
			WithCause(err).
			WithContext("host", host).
			WithContext("status_code", resp.StatusCode).
			WithContext("status", resp.Status).
			Build()
		c.connectionState.LastError = contextualErr
		return nil, contextualErr
	}

	totalDuration := time.Since(startTime)
	c.connectionState.Connected = true
	c.connectionState.AppName = specResponse.AppName
	c.connectionState.AppVersion = specResponse.AppVersion
	c.connectionState.LastHandshake = time.Now()
	c.connectionState.Features = specResponse.Features

	c.logger.LogConnectionSuccess(host, specResponse.AppName, specResponse.ProtocolVersion, totalDuration)
	c.logger.Info("Connection established successfully",
		"app_name", specResponse.AppName,
		"app_version", specResponse.AppVersion,
		"protocol_version", specResponse.ProtocolVersion,
		"total_duration", totalDuration,
		"features", len(specResponse.Features))

	return &specResponse.SpecResponse, nil
}

// ExecuteCommand sends a command to the application, handling the full request lifecycle.
func (c *Client) ExecuteCommand(ctx context.Context, request interfaces.CommandRequest) (*interfaces.CommandResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected to any application")
	}

	if err := c.validator.ValidateCommandRequest(&request); err != nil {
		return nil, fmt.Errorf("invalid command request: %w", err)
	}

	internalReq := &CommandRequestInternal{
		CommandRequest: request,
		RequestID:      generateRequestID(),
		Timestamp:      time.Now(),
	}

	operation := func() (*interfaces.CommandResponse, error) {
		respBody, err := c.executeJSONRequest(ctx, EndpointCommand, internalReq)
		if err != nil {
			return nil, err
		}
		var cmdResponse CommandResponseInternal
		if err := json.Unmarshal(respBody, &cmdResponse); err != nil {
			return nil, c.wrapProtocolError("failed to parse command response", err)
		}
		return &cmdResponse.CommandResponse, nil
	}

	response, err := executeWithRetry(ctx, operation)
	if err != nil {
		return nil, err
	}

	if err := c.validateCommandResponse(response); err != nil {
		return nil, fmt.Errorf("invalid command response received: %w", err)
	}

	return response, nil
}

// ExecuteAction sends an action request, handling the full request lifecycle.
func (c *Client) ExecuteAction(ctx context.Context, request interfaces.ActionRequest) (*interfaces.CommandResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected to any application")
	}

	if err := c.validator.ValidateActionRequest(&request); err != nil {
		return nil, fmt.Errorf("invalid action request: %w", err)
	}

	internalReq := &ActionRequestInternal{
		ActionRequest: request,
		RequestID:     generateRequestID(),
		Timestamp:     time.Now(),
	}

	operation := func() (*interfaces.CommandResponse, error) {
		respBody, err := c.executeJSONRequest(ctx, EndpointAction, internalReq)
		if err != nil {
			return nil, err
		}
		var cmdResponse CommandResponseInternal
		if err := json.Unmarshal(respBody, &cmdResponse); err != nil {
			return nil, c.wrapProtocolError("failed to parse action response", err)
		}
		return &cmdResponse.CommandResponse, nil
	}

	response, err := executeWithRetry(ctx, operation)
	if err != nil {
		return nil, err
	}

	if err := c.validateCommandResponse(response); err != nil {
		return nil, fmt.Errorf("invalid action response received: %w", err)
	}

	return response, nil
}

// GetSuggestions requests command suggestions, handling the full request lifecycle.
func (c *Client) GetSuggestions(ctx context.Context, request interfaces.SuggestRequest) (*interfaces.SuggestResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected to any application")
	}

	if err := c.validator.ValidateSuggestRequest(&request); err != nil {
		return nil, fmt.Errorf("invalid suggest request: %w", err)
	}

	internalReq := &SuggestRequestInternal{
		SuggestRequest: request,
		RequestID:      generateRequestID(),
		Timestamp:      time.Now(),
	}

	// Suggestions should be fast, so use a shorter timeout and no retry.
	suggestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	respBody, err := c.executeJSONRequest(suggestCtx, EndpointSuggest, internalReq)
	if err != nil {
		return nil, err
	}

	var suggestResponse SuggestResponseInternal
	if err := json.Unmarshal(respBody, &suggestResponse); err != nil {
		return nil, c.wrapProtocolError("failed to parse suggest response", err)
	}

	if err := c.validateSuggestResponse(&suggestResponse.SuggestResponse); err != nil {
		return nil, fmt.Errorf("invalid suggestion response received: %w", err)
	}

	return &suggestResponse.SuggestResponse, nil
}

// GetProgress requests operation progress, handling the full request lifecycle.
func (c *Client) GetProgress(ctx context.Context, request interfaces.ProgressRequest) (*interfaces.ProgressResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected to any application")
	}

	if err := c.validator.ValidateProgressRequest(&request); err != nil {
		return nil, fmt.Errorf("invalid progress request: %w", err)
	}

	internalReq := &ProgressRequestInternal{
		ProgressRequest: request,
		RequestID:       generateRequestID(),
		Timestamp:       time.Now(),
	}

	progressCtx, cancel := context.WithTimeout(ctx, DefaultProgressTimeout)
	defer cancel()

	respBody, err := c.executeJSONRequest(progressCtx, EndpointProgress, internalReq)
	if err != nil {
		return nil, err
	}

	var progressResponse ProgressResponseInternal
	if err := json.Unmarshal(respBody, &progressResponse); err != nil {
		return nil, c.wrapProtocolError("failed to parse progress response", err)
	}

	if err := c.validateProgressResponse(&progressResponse.ProgressResponse); err != nil {
		return nil, fmt.Errorf("invalid progress response received: %w", err)
	}

	return &progressResponse.ProgressResponse, nil
}

// CancelOperation requests operation cancellation, handling the full request lifecycle.
func (c *Client) CancelOperation(ctx context.Context, request interfaces.CancelRequest) (*interfaces.CancelResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected to any application")
	}

	if err := c.validator.ValidateCancelRequest(&request); err != nil {
		return nil, fmt.Errorf("invalid cancel request: %w", err)
	}

	internalReq := &CancelRequestInternal{
		CancelRequest: request,
		RequestID:     generateRequestID(),
		Timestamp:     time.Now(),
		Reason:        "user_requested",
	}

	respBody, err := c.executeJSONRequest(ctx, EndpointCancel, internalReq)
	if err != nil {
		return nil, err
	}

	var cancelResponse CancelResponseInternal
	if err := json.Unmarshal(respBody, &cancelResponse); err != nil {
		return nil, c.wrapProtocolError("failed to parse cancel response", err)
	}

	if err := c.validateCancelResponse(&cancelResponse.CancelResponse); err != nil {
		return nil, fmt.Errorf("invalid cancel response received: %w", err)
	}

	return &cancelResponse.CancelResponse, nil
}

// IsConnected returns whether the client is currently connected.
func (c *Client) IsConnected() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.connectionState.Connected
}

// Disconnect closes the connection to the application.
func (c *Client) Disconnect() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.connectionState.Connected = false
	c.connectionState.AppName = ""
	c.connectionState.AppVersion = ""
	c.connectionState.Features = nil
	c.connectionState.Auth = nil
	c.connectionState.LastError = nil

	c.httpClient.CloseIdleConnections()

	return nil
}

// GetLastError returns the last communication error.
func (c *Client) GetLastError() error {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.connectionState.LastError
}

// GetConnectionState returns the current connection state for UI access
func (c *Client) GetConnectionState() *ConnectionState {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	// Return a copy to prevent external modification
	stateCopy := *c.connectionState
	return &stateCopy
}

// --- Internal Helper Methods ---

// executeJSONRequest handles the core logic of making a POST request with a JSON body.
func (c *Client) executeJSONRequest(ctx context.Context, endpoint string, payload interface{}) ([]byte, error) {
	c.logger.Debug("Creating JSON request", "endpoint", endpoint)
	req, err := c.createJSONRequest(ctx, endpoint, payload)
	if err != nil {
		c.logger.Error("Failed to create JSON request", "endpoint", endpoint, "error", err.Error())
		return nil, c.wrapProtocolError("failed to create request", err)
	}

	c.logger.Debug("Executing JSON request", 
		"method", req.Method, 
		"url", req.URL.String(),
		"content_type", req.Header.Get("Content-Type"))
	
	startTime := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(startTime)
	c.updateRequestStatistics(duration, err == nil)

	if err != nil {
		c.logger.Error("JSON request execution failed", 
			"endpoint", endpoint,
			"error", err.Error(),
			"duration", duration)
		return nil, c.wrapNetworkError("request execution failed", err)
	}
	defer resp.Body.Close()

	c.logger.LogHTTPRequest(req.Method, req.URL.String(), resp.StatusCode, duration)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logger.Error("Failed to read response body", 
			"endpoint", endpoint,
			"status_code", resp.StatusCode,
			"error", err.Error())
		return nil, c.wrapNetworkError("failed to read response body", err)
	}

	c.logger.Debug("Received response", 
		"endpoint", endpoint,
		"status_code", resp.StatusCode,
		"content_length", len(body),
		"duration", duration)

	if resp.StatusCode >= 400 {
		c.logger.Warn("HTTP error response", 
			"endpoint", endpoint,
			"status_code", resp.StatusCode,
			"status", resp.Status)
		return nil, c.handleHTTPError(resp, body)
	}

	return body, nil
}

// createJSONRequest creates an HTTP request for JSON payload endpoints.
func (c *Client) createJSONRequest(ctx context.Context, endpoint string, payload interface{}) (*http.Request, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request payload: %w", err)
	}

	url := c.buildURL(c.connectionState.Host, endpoint)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	c.setStandardHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	if c.connectionState.Auth != nil && c.connectionState.Auth.Type != "none" {
		if err := c.setAuthenticationHeaders(req, c.connectionState.Auth); err != nil {
			return nil, fmt.Errorf("failed to set authentication headers: %w", err)
		}
	}

	return req, nil
}

// createHandshakeRequest creates the initial handshake HTTP request.
func (c *Client) createHandshakeRequest(ctx context.Context, url string, auth *interfaces.AuthConfig) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	c.setStandardHeaders(req)
	if auth != nil && auth.Type != "none" {
		if err := c.setAuthenticationHeaders(req, auth); err != nil {
			return nil, err
		}
	}
	return req, nil
}

// --- Validation and Processing Helpers ---

func (c *Client) validateConnectionParams(host string, auth *interfaces.AuthConfig) error {
	if strings.TrimSpace(host) == "" {
		return fmt.Errorf("host cannot be empty")
	}
	if !strings.Contains(host, ":") {
		return fmt.Errorf("host must include port (e.g., localhost:8080)")
	}
	if auth != nil {
		if err := c.authManager.ValidateToken(auth.Token, auth.Type); err != nil {
			return fmt.Errorf("invalid authentication: %w", err)
		}
	}
	return nil
}

func (c *Client) processHandshakeResponse(resp *http.Response) (*SpecResponseInternal, error) {
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("handshake failed with status %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read handshake response: %w", err)
	}
	var specResp SpecResponseInternal
	if err := json.Unmarshal(body, &specResp); err != nil {
		return nil, fmt.Errorf("failed to parse handshake response JSON: %w", err)
	}
	if specResp.ProtocolVersion != ProtocolVersion {
		return nil, fmt.Errorf("incompatible protocol version: server=%s, client=%s", specResp.ProtocolVersion, ProtocolVersion)
	}
	return &specResp, nil
}

func (c *Client) validateCommandResponse(response *interfaces.CommandResponse) error {
	if response == nil {
		return fmt.Errorf("response cannot be nil")
	}
	if response.Response.Type == "structured" {
		if response.Response.Content == nil {
			return fmt.Errorf("structured response content cannot be nil")
		}
	}
	return nil
}

func (c *Client) validateSuggestResponse(response *interfaces.SuggestResponse) error {
	if response == nil {
		return fmt.Errorf("response cannot be nil")
	}
	for i, suggestion := range response.Suggestions {
		if suggestion.Text == "" {
			return fmt.Errorf("suggestion %d has empty text", i)
		}
	}
	return nil
}

func (c *Client) validateProgressResponse(response *interfaces.ProgressResponse) error {
	if response == nil {
		return fmt.Errorf("response cannot be nil")
	}
	if response.Progress < 0 || response.Progress > 100 {
		return fmt.Errorf("progress must be between 0 and 100, got %d", response.Progress)
	}
	validStatuses := map[string]bool{"running": true, "complete": true, "error": true, "paused": true}
	if !validStatuses[response.Status] {
		return fmt.Errorf("invalid progress status: %s", response.Status)
	}
	return nil
}

func (c *Client) validateCancelResponse(response *interfaces.CancelResponse) error {
	if response == nil {
		return fmt.Errorf("response cannot be nil")
	}
	if response.Message == "" {
		return fmt.Errorf("cancellation response must include a message")
	}
	return nil
}

// --- Header and URL Helpers ---

func (c *Client) buildURL(host, endpoint string) string {
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		host = "http://" + host
	}
	baseURL, _ := url.Parse(host)
	// Use JoinPath for safer URL joining
	return baseURL.JoinPath(strings.TrimPrefix(endpoint, "/")).String()
}

func (c *Client) setStandardHeaders(req *http.Request) {
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Console-Version", "2.0.0")
	req.Header.Set("X-Protocol-Version", ProtocolVersion)
	req.Header.Set("X-Session-ID", c.sessionID)
}

func (c *Client) setAuthenticationHeaders(req *http.Request, auth *interfaces.AuthConfig) error {
	authHeader, err := c.authManager.CreateAuthHeader(auth)
	if err != nil {
		return err
	}
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	return nil
}

// --- Error Handling ---

func (c *Client) handleHTTPError(resp *http.Response, body []byte) error {
	protocolErr := &ProtocolError{
		Type:          "http",
		Message:       fmt.Sprintf("HTTP error %s", resp.Status),
		OriginalError: fmt.Errorf("server returned status %s", resp.Status),
		Timestamp:     time.Now(),
		Recoverable:   resp.StatusCode >= 500,
		HTTPDetails:   &HTTPErrorDetails{StatusCode: resp.StatusCode, StatusText: resp.Status, Body: string(body)},
	}

	// Try to parse a more specific structured error.
	var errorResp ErrorResponseInternal
	if err := json.Unmarshal(body, &errorResp); err == nil {
		protocolErr.Type = "http_structured"
		protocolErr.Message = errorResp.Error.Message
		protocolErr.OriginalError = fmt.Errorf("server returned status %s with code %s", resp.Status, errorResp.Error.Code)
	}

	c.mutex.Lock()
	c.connectionState.LastError = protocolErr
	c.mutex.Unlock()

	return protocolErr
}

func (c *Client) wrapConnectionError(message string, err error) error {
	protocolErr := &ProtocolError{Type: "connection", Message: fmt.Sprintf("%s: %v", message, err), OriginalError: err, Timestamp: time.Now(), Recoverable: true}
	c.mutex.Lock()
	c.connectionState.LastError = protocolErr
	c.mutex.Unlock()
	return protocolErr
}

func (c *Client) wrapConnectionErrorUnsafe(message string, err error) error {
	protocolErr := &ProtocolError{Type: "connection", Message: fmt.Sprintf("%s: %v", message, err), OriginalError: err, Timestamp: time.Now(), Recoverable: true}
	c.connectionState.LastError = protocolErr
	return protocolErr
}

func (c *Client) wrapNetworkError(message string, err error) error {
	protocolErr := &ProtocolError{Type: "network", Message: fmt.Sprintf("%s: %v", message, err), OriginalError: err, Timestamp: time.Now(), Recoverable: true}
	c.mutex.Lock()
	c.connectionState.LastError = protocolErr
	c.mutex.Unlock()
	return protocolErr
}

func (c *Client) wrapProtocolError(message string, err error) error {
	protocolErr := &ProtocolError{Type: "protocol", Message: fmt.Sprintf("%s: %v", message, err), OriginalError: err, Timestamp: time.Now(), Recoverable: false}
	c.mutex.Lock()
	c.connectionState.LastError = protocolErr
	c.mutex.Unlock()
	return protocolErr
}

// updateRequestStatistics is a method on the Client to update connection statistics.
func (c *Client) updateRequestStatistics(responseTime time.Duration, success bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.updateRequestStatisticsUnsafe(responseTime, success)
}

// updateRequestStatisticsUnsafe updates statistics without acquiring the mutex (caller must hold lock)
func (c *Client) updateRequestStatisticsUnsafe(responseTime time.Duration, success bool) {
	stats := &c.connectionState.Statistics
	stats.TotalRequests++
	stats.LastRequestTime = time.Now()
	if success {
		stats.SuccessfulRequests++
	} else {
		stats.FailedRequests++
	}

	if stats.TotalRequests > 0 {
		total := stats.AverageResponseTime * time.Duration(stats.TotalRequests-1)
		stats.AverageResponseTime = (total + responseTime) / time.Duration(stats.TotalRequests)
	} else {
		stats.AverageResponseTime = responseTime
	}
}

// --- Utility and Helper Functions ---

// executeWithRetry executes a function with basic retry logic for transient failures.
// This is a package-private FUNCTION, not a method.
func executeWithRetry[T any](ctx context.Context, operation func() (*T, error)) (*T, error) {
	const maxRetries = 2
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		response, err := operation()
		if err == nil {
			return response, nil
		}
		lastErr = err

		if protocolErr, ok := err.(*ProtocolError); ok && protocolErr.IsRetryable() {
			if attempt < maxRetries {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(protocolErr.GetRetryDelay()):
					// continue to next attempt
				}
			}
		} else {
			break // Non-protocol or non-retryable errors break the loop
		}
	}
	return nil, lastErr
}

func generateSessionID() string {
	return fmt.Sprintf("console_%d", time.Now().UnixNano())
}
func generateRequestID() string {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}
