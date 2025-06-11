// Package protocol implements HTTP communication with Compliant Applications.
// This file provides the concrete implementation of the ProtocolClient interface
// with comprehensive timeout management, authentication handling, and connection lifecycle management.
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

	"github.com/universal-console/console/internal/interfaces"
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
}

// NewClient creates a new protocol client with injected dependencies and secure defaults
func NewClient(configManager interfaces.ConfigManager, authManager interfaces.AuthManager) (*Client, error) {
	if configManager == nil {
		return nil, fmt.Errorf("configManager cannot be nil")
	}

	if authManager == nil {
		return nil, fmt.Errorf("authManager cannot be nil")
	}

	// Configure HTTP client with appropriate timeouts and security settings
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

	// Generate session ID for request tracking
	sessionID := generateSessionID()

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
		sessionID: sessionID,
	}

	return client, nil
}

// Connect establishes connection and performs handshake with the application
func (c *Client) Connect(ctx context.Context, host string, auth *interfaces.AuthConfig) (*interfaces.SpecResponse, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Validate connection parameters
	if err := c.validateConnectionParams(host, auth); err != nil {
		return nil, fmt.Errorf("invalid connection parameters: %w", err)
	}

	// Update connection state
	c.connectionState.Host = host
	c.connectionState.Connected = false
	c.connectionState.LastError = nil

	// Perform handshake request
	handshakeURL := c.buildURL(host, EndpointSpec)

	// Create context with handshake timeout
	handshakeCtx, cancel := context.WithTimeout(ctx, HandshakeTimeout)
	defer cancel()

	req, err := c.createHandshakeRequest(handshakeCtx, handshakeURL, auth)
	if err != nil {
		return nil, c.wrapConnectionError("failed to create handshake request", err)
	}

	// Execute handshake request
	startTime := time.Now()
	resp, err := c.httpClient.Do(req)
	responseTime := time.Since(startTime)

	// Update statistics
	c.updateRequestStatistics(responseTime, err == nil)

	if err != nil {
		return nil, c.wrapConnectionError("handshake request failed", err)
	}
	defer resp.Body.Close()

	// Process handshake response
	specResponse, err := c.processHandshakeResponse(resp)
	if err != nil {
		return nil, c.wrapConnectionError("invalid handshake response", err)
	}

	// Update connection state on successful handshake
	c.connectionState.Connected = true
	c.connectionState.AppName = specResponse.AppName
	c.connectionState.AppVersion = specResponse.AppVersion
	c.connectionState.LastHandshake = time.Now()
	c.connectionState.Features = specResponse.Features
	if auth != nil {
		c.connectionState.AuthType = auth.Type
	}

	return &specResponse.SpecResponse, nil
}

// ExecuteCommand sends a command to the application
func (c *Client) ExecuteCommand(ctx context.Context, request interfaces.CommandRequest) (*interfaces.CommandResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected to any application")
	}

	// Validate request
	if err := c.validator.ValidateCommandRequest(&request); err != nil {
		return nil, fmt.Errorf("invalid command request: %w", err)
	}

	// Create internal request with metadata
	internalReq := &CommandRequestInternal{
		CommandRequest: request,
		RequestID:      generateRequestID(),
		Timestamp:      time.Now(),
	}

	// Execute HTTP request
	response, err := c.executeJSONRequest(ctx, EndpointCommand, internalReq)
	if err != nil {
		return nil, err
	}

	// Parse response
	var cmdResponse CommandResponseInternal
	if err := json.Unmarshal(response.Body, &cmdResponse); err != nil {
		return nil, c.wrapProtocolError("failed to parse command response", err)
	}

	cmdResponse.Metadata = response.Metadata
	return &cmdResponse.CommandResponse, nil
}

// ExecuteAction sends an action execution request
func (c *Client) ExecuteAction(ctx context.Context, request interfaces.ActionRequest) (*interfaces.CommandResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected to any application")
	}

	// Validate request
	if err := c.validator.ValidateActionRequest(&request); err != nil {
		return nil, fmt.Errorf("invalid action request: %w", err)
	}

	// Create internal request with metadata
	internalReq := &ActionRequestInternal{
		ActionRequest: request,
		RequestID:     generateRequestID(),
		Timestamp:     time.Now(),
	}

	// Execute HTTP request
	response, err := c.executeJSONRequest(ctx, EndpointAction, internalReq)
	if err != nil {
		return nil, err
	}

	// Parse response (actions return CommandResponse format)
	var cmdResponse CommandResponseInternal
	if err := json.Unmarshal(response.Body, &cmdResponse); err != nil {
		return nil, c.wrapProtocolError("failed to parse action response", err)
	}

	cmdResponse.Metadata = response.Metadata
	return &cmdResponse.CommandResponse, nil
}

// GetSuggestions requests command suggestions
func (c *Client) GetSuggestions(ctx context.Context, request interfaces.SuggestRequest) (*interfaces.SuggestResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected to any application")
	}

	// Validate request
	if err := c.validator.ValidateSuggestRequest(&request); err != nil {
		return nil, fmt.Errorf("invalid suggest request: %w", err)
	}

	// Create internal request with metadata
	internalReq := &SuggestRequestInternal{
		SuggestRequest: request,
		RequestID:      generateRequestID(),
		Timestamp:      time.Now(),
	}

	// Execute HTTP request
	response, err := c.executeJSONRequest(ctx, EndpointSuggest, internalReq)
	if err != nil {
		return nil, err
	}

	// Parse response
	var suggestResponse SuggestResponseInternal
	if err := json.Unmarshal(response.Body, &suggestResponse); err != nil {
		return nil, c.wrapProtocolError("failed to parse suggest response", err)
	}

	suggestResponse.Metadata = response.Metadata
	return &suggestResponse.SuggestResponse, nil
}

// GetProgress requests operation progress
func (c *Client) GetProgress(ctx context.Context, request interfaces.ProgressRequest) (*interfaces.ProgressResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected to any application")
	}

	// Validate request
	if err := c.validator.ValidateProgressRequest(&request); err != nil {
		return nil, fmt.Errorf("invalid progress request: %w", err)
	}

	// Create internal request with metadata
	internalReq := &ProgressRequestInternal{
		ProgressRequest: request,
		RequestID:       generateRequestID(),
		Timestamp:       time.Now(),
	}

	// Use shorter timeout for progress requests
	progressCtx, cancel := context.WithTimeout(ctx, DefaultProgressTimeout)
	defer cancel()

	// Execute HTTP request
	response, err := c.executeJSONRequestWithContext(progressCtx, EndpointProgress, internalReq)
	if err != nil {
		return nil, err
	}

	// Parse response
	var progressResponse ProgressResponseInternal
	if err := json.Unmarshal(response.Body, &progressResponse); err != nil {
		return nil, c.wrapProtocolError("failed to parse progress response", err)
	}

	progressResponse.Metadata = response.Metadata
	progressResponse.LastUpdate = time.Now()
	return &progressResponse.ProgressResponse, nil
}

// CancelOperation requests operation cancellation
func (c *Client) CancelOperation(ctx context.Context, request interfaces.CancelRequest) (*interfaces.CancelResponse, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected to any application")
	}

	// Validate request
	if err := c.validator.ValidateCancelRequest(&request); err != nil {
		return nil, fmt.Errorf("invalid cancel request: %w", err)
	}

	// Create internal request with metadata
	internalReq := &CancelRequestInternal{
		CancelRequest: request,
		RequestID:     generateRequestID(),
		Timestamp:     time.Now(),
		Reason:        "user_requested",
	}

	// Execute HTTP request
	response, err := c.executeJSONRequest(ctx, EndpointCancel, internalReq)
	if err != nil {
		return nil, err
	}

	// Parse response
	var cancelResponse CancelResponseInternal
	if err := json.Unmarshal(response.Body, &cancelResponse); err != nil {
		return nil, c.wrapProtocolError("failed to parse cancel response", err)
	}

	cancelResponse.Metadata = response.Metadata
	return &cancelResponse.CancelResponse, nil
}

// IsConnected returns whether the client is currently connected
func (c *Client) IsConnected() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.connectionState.Connected
}

// Disconnect closes the connection to the application
func (c *Client) Disconnect() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.connectionState.Connected = false
	c.connectionState.AppName = ""
	c.connectionState.AppVersion = ""
	c.connectionState.Features = nil
	c.connectionState.AuthType = ""
	c.connectionState.LastError = nil

	// Close idle connections
	c.httpClient.CloseIdleConnections()

	return nil
}

// GetLastError returns the last communication error
func (c *Client) GetLastError() error {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if c.connectionState.LastError != nil {
		return c.connectionState.LastError
	}
	return nil
}

// GetConnectionState returns the current connection state for debugging
func (c *Client) GetConnectionState() *ConnectionState {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// Return a copy to prevent external modification
	stateCopy := *c.connectionState
	return &stateCopy
}

// Helper methods for internal operation

// validateConnectionParams ensures connection parameters are valid
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

// buildURL constructs the full URL for an endpoint
func (c *Client) buildURL(host, endpoint string) string {
	// Ensure proper URL scheme
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		host = "http://" + host
	}

	baseURL, _ := url.Parse(host)
	endpointURL, _ := url.Parse(endpoint)
	return baseURL.ResolveReference(endpointURL).String()
}

// createHandshakeRequest creates the initial handshake HTTP request
func (c *Client) createHandshakeRequest(ctx context.Context, url string, auth *interfaces.AuthConfig) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Set standard headers
	c.setStandardHeaders(req)

	// Set authentication headers if provided
	if auth != nil && auth.Type != "none" {
		if err := c.setAuthenticationHeaders(req, auth); err != nil {
			return nil, fmt.Errorf("failed to set authentication headers: %w", err)
		}
	}

	return req, nil
}

// createJSONRequest creates an HTTP request for JSON payload endpoints
func (c *Client) createJSONRequest(ctx context.Context, endpoint string, payload interface{}) (*http.Request, error) {
	// Marshal payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request payload: %w", err)
	}

	// Build full URL
	url := c.buildURL(c.connectionState.Host, endpoint)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	// Set standard headers
	c.setStandardHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	// Set authentication headers if connected
	if c.connectionState.AuthType != "" && c.connectionState.AuthType != "none" {
		// Retrieve current auth config (this is a simplified approach)
		// In production, auth context should be maintained separately
		req.Header.Set("Authorization", "Bearer "+c.sessionID) // Placeholder
	}

	return req, nil
}

// setStandardHeaders sets common headers for all requests
func (c *Client) setStandardHeaders(req *http.Request) {
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Console-Version", "2.0.0")
	req.Header.Set("X-Protocol-Version", ProtocolVersion)
	req.Header.Set("X-Session-ID", c.sessionID)
}

// setAuthenticationHeaders adds authentication headers to the request
func (c *Client) setAuthenticationHeaders(req *http.Request, auth *interfaces.AuthConfig) error {
	authHeader, err := c.authManager.CreateAuthHeader(auth)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", authHeader)
	return nil
}

// executeJSONRequest executes a JSON request and returns the parsed response
func (c *Client) executeJSONRequest(ctx context.Context, endpoint string, payload interface{}) (*InternalResponse, error) {
	return c.executeJSONRequestWithContext(ctx, endpoint, payload)
}

// executeJSONRequestWithContext executes a JSON request with explicit context
func (c *Client) executeJSONRequestWithContext(ctx context.Context, endpoint string, payload interface{}) (*InternalResponse, error) {
	// Create HTTP request
	req, err := c.createJSONRequest(ctx, endpoint, payload)
	if err != nil {
		return nil, c.wrapProtocolError("failed to create request", err)
	}

	// Execute request
	startTime := time.Now()
	resp, err := c.httpClient.Do(req)
	responseTime := time.Since(startTime)

	// Update statistics
	c.updateRequestStatistics(responseTime, err == nil)

	if err != nil {
		return nil, c.wrapNetworkError("request execution failed", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, c.wrapNetworkError("failed to read response body", err)
	}

	// Handle HTTP error status codes
	if resp.StatusCode >= 400 {
		return nil, c.handleHTTPError(resp, body)
	}

	// Create internal response
	internalResp := &InternalResponse{
		StatusCode: resp.StatusCode,
		Headers:    make(map[string]string),
		Body:       body,
		Metadata: ResponseMetadata{
			ResponseTime:  responseTime,
			ContentLength: resp.ContentLength,
		},
	}

	// Copy relevant headers
	for key, values := range resp.Header {
		if len(values) > 0 {
			internalResp.Headers[key] = values[0]
		}
	}

	return internalResp, nil
}

// InternalResponse represents the processed HTTP response
type InternalResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
	Metadata   ResponseMetadata
}

// processHandshakeResponse processes the /console/spec response
func (c *Client) processHandshakeResponse(resp *http.Response) (*SpecResponseInternal, error) {
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("handshake failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read handshake response: %w", err)
	}

	var specResp SpecResponseInternal
	if err := json.Unmarshal(body, &specResp); err != nil {
		return nil, fmt.Errorf("failed to parse handshake response: %w", err)
	}

	// Validate protocol version compatibility
	if specResp.ProtocolVersion != ProtocolVersion {
		return nil, fmt.Errorf("incompatible protocol version: server=%s, client=%s",
			specResp.ProtocolVersion, ProtocolVersion)
	}

	specResp.ReceivedAt = time.Now()
	specResp.ServerHeaders = make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			specResp.ServerHeaders[key] = values[0]
		}
	}

	return &specResp, nil
}

// Error handling and wrapping methods

// wrapConnectionError wraps connection-related errors
func (c *Client) wrapConnectionError(message string, err error) error {
	protocolErr := &ProtocolError{
		Type:            "network",
		Message:         fmt.Sprintf("%s: %v", message, err),
		OriginalError:   err,
		Timestamp:       time.Now(),
		Recoverable:     true,
		SuggestedAction: "Check network connectivity and application status",
	}

	c.setLastError(protocolErr)
	return protocolErr
}

// wrapNetworkError wraps network-related errors
func (c *Client) wrapNetworkError(message string, err error) error {
	protocolErr := &ProtocolError{
		Type:          "network",
		Message:       fmt.Sprintf("%s: %v", message, err),
		OriginalError: err,
		Timestamp:     time.Now(),
		Recoverable:   true,
		NetworkDetails: &NetworkErrorDetails{
			ErrorType:   "network_failure",
			LastAttempt: time.Now(),
		},
	}

	c.setLastError(protocolErr)
	return protocolErr
}

// wrapProtocolError wraps protocol-related errors
func (c *Client) wrapProtocolError(message string, err error) error {
	protocolErr := &ProtocolError{
		Type:            "protocol",
		Message:         fmt.Sprintf("%s: %v", message, err),
		OriginalError:   err,
		Timestamp:       time.Now(),
		Recoverable:     false,
		SuggestedAction: "Check application protocol implementation",
	}

	c.setLastError(protocolErr)
	return protocolErr
}

// handleHTTPError processes HTTP error responses
func (c *Client) handleHTTPError(resp *http.Response, body []byte) error {
	// Try to parse as structured error response
	var errorResp ErrorResponseInternal
	if err := json.Unmarshal(body, &errorResp); err == nil {
		// Convert structured error to protocol error
		protocolErr := &ProtocolError{
			Type:        "http",
			Message:     errorResp.Error.Message,
			Timestamp:   time.Now(),
			Recoverable: resp.StatusCode >= 500,
			HTTPDetails: &HTTPErrorDetails{
				StatusCode:    resp.StatusCode,
				StatusText:    resp.Status,
				Headers:       make(map[string]string),
				Body:          string(body),
				ContentType:   resp.Header.Get("Content-Type"),
				ContentLength: resp.ContentLength,
			},
		}

		// Copy headers
		for key, values := range resp.Header {
			if len(values) > 0 {
				protocolErr.HTTPDetails.Headers[key] = values[0]
			}
		}

		c.setLastError(protocolErr)
		return protocolErr
	}

	// Generic HTTP error
	protocolErr := &ProtocolError{
		Type:        "http",
		Message:     fmt.Sprintf("HTTP error %d: %s", resp.StatusCode, resp.Status),
		Timestamp:   time.Now(),
		Recoverable: resp.StatusCode >= 500,
		HTTPDetails: &HTTPErrorDetails{
			StatusCode:    resp.StatusCode,
			StatusText:    resp.Status,
			Body:          string(body),
			ContentType:   resp.Header.Get("Content-Type"),
			ContentLength: resp.ContentLength,
		},
	}

	c.setLastError(protocolErr)
	return protocolErr
}

// setLastError updates the connection state with the latest error
func (c *Client) setLastError(err *ProtocolError) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.connectionState.LastError = err
}

// updateRequestStatistics updates connection statistics
func (c *Client) updateRequestStatistics(responseTime time.Duration, success bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	stats := &c.connectionState.Statistics
	stats.TotalRequests++
	stats.LastRequestTime = time.Now()

	if success {
		stats.SuccessfulRequests++
	} else {
		stats.FailedRequests++
	}

	// Update average response time
	if stats.TotalRequests == 1 {
		stats.AverageResponseTime = responseTime
	} else {
		// Calculate moving average
		total := stats.AverageResponseTime * time.Duration(stats.TotalRequests-1)
		stats.AverageResponseTime = (total + responseTime) / time.Duration(stats.TotalRequests)
	}
}

// Utility functions

// generateSessionID creates a unique session identifier
func generateSessionID() string {
	return fmt.Sprintf("console_%d", time.Now().UnixNano())
}

// generateRequestID creates a unique request identifier
func generateRequestID() string {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}
