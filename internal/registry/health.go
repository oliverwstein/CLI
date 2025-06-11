// Package registry provides real-time health monitoring for registered applications
// in the Universal Application Console. This implementation performs connectivity checks,
// protocol validation, and comprehensive health status assessment to ensure reliable
// application availability reporting for the Console Menu interface.
package registry

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/universal-console/console/internal/interfaces"
)

// HealthMonitor provides comprehensive health monitoring capabilities for registered applications
type HealthMonitor struct {
	httpClient      *http.Client
	checkTimeouts   map[string]time.Duration
	retryPolicies   map[string]RetryPolicy
	healthHistory   map[string][]HealthSnapshot
	alertThresholds map[string]AlertThreshold
	mutex           sync.RWMutex
	maxHistorySize  int
}

// HealthCheckType represents different types of health checks that can be performed
type HealthCheckType string

const (
	HealthCheckConnectivity HealthCheckType = "connectivity"
	HealthCheckProtocol     HealthCheckType = "protocol"
	HealthCheckHandshake    HealthCheckType = "handshake"
	HealthCheckFunctional   HealthCheckType = "functional"
)

// HealthSnapshot captures a point-in-time health assessment
type HealthSnapshot struct {
	Timestamp    time.Time              `json:"timestamp"`
	Status       string                 `json:"status"`
	ResponseTime time.Duration          `json:"responseTime"`
	CheckType    HealthCheckType        `json:"checkType"`
	Error        string                 `json:"error,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
	ServerInfo   *ServerInfo            `json:"serverInfo,omitempty"`
}

// ServerInfo contains detailed information about the application server
type ServerInfo struct {
	AppName         string            `json:"appName"`
	AppVersion      string            `json:"appVersion"`
	ProtocolVersion string            `json:"protocolVersion"`
	Features        map[string]bool   `json:"features"`
	ServerHeaders   map[string]string `json:"serverHeaders"`
	Uptime          time.Duration     `json:"uptime,omitempty"`
}

// RetryPolicy defines retry behavior for health checks
type RetryPolicy struct {
	MaxAttempts   int           `json:"maxAttempts"`
	InitialDelay  time.Duration `json:"initialDelay"`
	MaxDelay      time.Duration `json:"maxDelay"`
	BackoffFactor float64       `json:"backoffFactor"`
	RetryOn       []string      `json:"retryOn"` // Error types to retry on
}

// AlertThreshold defines conditions that trigger health alerts
type AlertThreshold struct {
	MaxResponseTime     time.Duration `json:"maxResponseTime"`
	MinUptimePercent    float64       `json:"minUptimePercent"`
	MaxConsecutiveFails int           `json:"maxConsecutiveFails"`
	AlertCooldown       time.Duration `json:"alertCooldown"`
	LastAlert           time.Time     `json:"lastAlert"`
}

// HealthCheckResult represents the comprehensive result of a health assessment
type HealthCheckResult struct {
	Overall         interfaces.AppHealth            `json:"overall"`
	CheckResults    map[HealthCheckType]CheckResult `json:"checkResults"`
	ServerInfo      *ServerInfo                     `json:"serverInfo,omitempty"`
	Recommendations []string                        `json:"recommendations,omitempty"`
	AlertTriggered  bool                            `json:"alertTriggered"`
}

// CheckResult represents the result of an individual health check
type CheckResult struct {
	Status       string                 `json:"status"`
	ResponseTime time.Duration          `json:"responseTime"`
	Error        string                 `json:"error,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
	Severity     string                 `json:"severity"` // "low", "medium", "high", "critical"
}

// NewHealthMonitor creates a new health monitor with optimized settings
func NewHealthMonitor() *HealthMonitor {
	// Configure HTTP client with appropriate timeouts and connection pooling
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:        10,
			IdleConnTimeout:     30 * time.Second,
			DisableCompression:  false,
			DisableKeepAlives:   false,
			MaxIdleConnsPerHost: 2,
		},
	}

	return &HealthMonitor{
		httpClient:      httpClient,
		checkTimeouts:   make(map[string]time.Duration),
		retryPolicies:   make(map[string]RetryPolicy),
		healthHistory:   make(map[string][]HealthSnapshot),
		alertThresholds: make(map[string]AlertThreshold),
		maxHistorySize:  100,
	}
}

// CheckApplicationHealth performs comprehensive health assessment for an application
func (hm *HealthMonitor) CheckApplicationHealth(
	ctx context.Context,
	app *interfaces.RegisteredApp,
	configManager interfaces.ConfigManager,
	protocolClient interfaces.ProtocolClient,
) (*interfaces.AppHealth, error) {
	startTime := time.Now()

	// Load application profile for connection details
	profile, err := configManager.LoadProfile(app.Profile)
	if err != nil {
		return nil, fmt.Errorf("failed to load profile '%s': %w", app.Profile, err)
	}

	// Perform comprehensive health check
	result, err := hm.performComprehensiveHealthCheck(ctx, app, profile, protocolClient)
	if err != nil {
		// Create error health status
		health := &interfaces.AppHealth{
			Name:         app.Name,
			Status:       "error",
			LastChecked:  time.Now(),
			ResponseTime: time.Since(startTime),
			Error:        err.Error(),
		}

		hm.recordHealthSnapshot(app.Name, HealthSnapshot{
			Timestamp:    time.Now(),
			Status:       "error",
			ResponseTime: time.Since(startTime),
			CheckType:    HealthCheckConnectivity,
			Error:        err.Error(),
		})

		return health, nil
	}

	// Convert comprehensive result to AppHealth
	health := &interfaces.AppHealth{
		Name:         app.Name,
		Status:       result.Overall.Status,
		LastChecked:  time.Now(),
		ResponseTime: result.Overall.ResponseTime,
		Error:        result.Overall.Error,
	}

	// Record successful health snapshot
	hm.recordHealthSnapshot(app.Name, HealthSnapshot{
		Timestamp:    time.Now(),
		Status:       health.Status,
		ResponseTime: health.ResponseTime,
		CheckType:    HealthCheckProtocol,
		ServerInfo:   result.ServerInfo,
	})

	return health, nil
}

// performComprehensiveHealthCheck executes all health check types
func (hm *HealthMonitor) performComprehensiveHealthCheck(
	ctx context.Context,
	app *interfaces.RegisteredApp,
	profile *interfaces.Profile,
	protocolClient interfaces.ProtocolClient,
) (*HealthCheckResult, error) {
	result := &HealthCheckResult{
		CheckResults: make(map[HealthCheckType]CheckResult),
	}

	// 1. Connectivity Check
	connectivityResult := hm.performConnectivityCheck(ctx, profile.Host)
	result.CheckResults[HealthCheckConnectivity] = connectivityResult

	if connectivityResult.Status != "ready" {
		result.Overall = interfaces.AppHealth{
			Status:       "offline",
			ResponseTime: connectivityResult.ResponseTime,
			Error:        connectivityResult.Error,
		}
		return result, nil
	}

	// 2. Protocol Handshake Check
	handshakeResult := hm.performHandshakeCheck(ctx, profile, protocolClient)
	result.CheckResults[HealthCheckHandshake] = handshakeResult

	if handshakeResult.Status != "ready" {
		result.Overall = interfaces.AppHealth{
			Status:       "error",
			ResponseTime: handshakeResult.ResponseTime,
			Error:        handshakeResult.Error,
		}
		return result, nil
	}

	// Extract server information from handshake
	if serverInfo, ok := handshakeResult.Details["serverInfo"].(*ServerInfo); ok {
		result.ServerInfo = serverInfo
	}

	// 3. Functional Check (optional, lightweight)
	functionalResult := hm.performFunctionalCheck(ctx, profile, protocolClient)
	result.CheckResults[HealthCheckFunctional] = functionalResult

	// Determine overall status
	overallStatus := "ready"
	maxResponseTime := time.Duration(0)
	var lastError string

	for _, checkResult := range result.CheckResults {
		if checkResult.ResponseTime > maxResponseTime {
			maxResponseTime = checkResult.ResponseTime
		}

		if checkResult.Status != "ready" {
			overallStatus = "degraded"
			if checkResult.Error != "" {
				lastError = checkResult.Error
			}
		}
	}

	result.Overall = interfaces.AppHealth{
		Status:       overallStatus,
		ResponseTime: maxResponseTime,
		Error:        lastError,
	}

	// Check for alert conditions
	hm.evaluateAlertConditions(app.Name, result)

	return result, nil
}

// performConnectivityCheck verifies basic network connectivity to the application
func (hm *HealthMonitor) performConnectivityCheck(ctx context.Context, host string) CheckResult {
	startTime := time.Now()

	// Parse host and port
	if !strings.Contains(host, ":") {
		return CheckResult{
			Status:       "error",
			ResponseTime: time.Since(startTime),
			Error:        "invalid host format: port required",
			Severity:     "high",
		}
	}

	// Attempt TCP connection
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", host)
	responseTime := time.Since(startTime)

	if err != nil {
		return CheckResult{
			Status:       "offline",
			ResponseTime: responseTime,
			Error:        fmt.Sprintf("connection failed: %v", err),
			Severity:     "critical",
			Details: map[string]interface{}{
				"host":        host,
				"errorType":   classifyNetworkError(err),
				"connectable": false,
			},
		}
	}

	conn.Close()

	return CheckResult{
		Status:       "ready",
		ResponseTime: responseTime,
		Severity:     "low",
		Details: map[string]interface{}{
			"host":        host,
			"connectable": true,
		},
	}
}

// performHandshakeCheck verifies protocol compliance and application availability
func (hm *HealthMonitor) performHandshakeCheck(
	ctx context.Context,
	profile *interfaces.Profile,
	protocolClient interfaces.ProtocolClient,
) CheckResult {
	startTime := time.Now()

	// Attempt protocol handshake
	specResponse, err := protocolClient.Connect(ctx, profile.Host, &profile.Auth)
	responseTime := time.Since(startTime)

	if err != nil {
		return CheckResult{
			Status:       "error",
			ResponseTime: responseTime,
			Error:        fmt.Sprintf("handshake failed: %v", err),
			Severity:     "high",
			Details: map[string]interface{}{
				"protocolCompliant": false,
				"errorType":         classifyProtocolError(err),
			},
		}
	}

	// Validate protocol version compatibility
	if specResponse.ProtocolVersion != "2.0" {
		return CheckResult{
			Status:       "degraded",
			ResponseTime: responseTime,
			Error:        fmt.Sprintf("protocol version mismatch: expected 2.0, got %s", specResponse.ProtocolVersion),
			Severity:     "medium",
			Details: map[string]interface{}{
				"protocolCompliant": false,
				"protocolVersion":   specResponse.ProtocolVersion,
				"expectedVersion":   "2.0",
			},
		}
	}

	// Create server info from spec response
	serverInfo := &ServerInfo{
		AppName:         specResponse.AppName,
		AppVersion:      specResponse.AppVersion,
		ProtocolVersion: specResponse.ProtocolVersion,
		Features:        specResponse.Features,
		ServerHeaders:   make(map[string]string),
	}

	return CheckResult{
		Status:       "ready",
		ResponseTime: responseTime,
		Severity:     "low",
		Details: map[string]interface{}{
			"protocolCompliant": true,
			"serverInfo":        serverInfo,
			"features":          specResponse.Features,
		},
	}
}

// performFunctionalCheck performs lightweight functional verification
func (hm *HealthMonitor) performFunctionalCheck(
	ctx context.Context,
	profile *interfaces.Profile,
	protocolClient interfaces.ProtocolClient,
) CheckResult {
	startTime := time.Now()

	// Perform a simple suggest request as a lightweight functional test
	suggestRequest := interfaces.SuggestRequest{
		CurrentInput: "help",
		Context:      map[string]interface{}{"healthCheck": true},
	}

	_, err := protocolClient.GetSuggestions(ctx, suggestRequest)
	responseTime := time.Since(startTime)

	if err != nil {
		return CheckResult{
			Status:       "degraded",
			ResponseTime: responseTime,
			Error:        fmt.Sprintf("functional check failed: %v", err),
			Severity:     "medium",
			Details: map[string]interface{}{
				"functionallyResponsive": false,
				"testType":               "suggestions",
			},
		}
	}

	return CheckResult{
		Status:       "ready",
		ResponseTime: responseTime,
		Severity:     "low",
		Details: map[string]interface{}{
			"functionallyResponsive": true,
			"testType":               "suggestions",
		},
	}
}

// GetHealthHistory returns the health check history for an application
func (hm *HealthMonitor) GetHealthHistory(appName string, limit int) ([]HealthSnapshot, error) {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()

	history, exists := hm.healthHistory[appName]
	if !exists {
		return []HealthSnapshot{}, nil
	}

	// Return most recent entries up to limit
	start := 0
	if len(history) > limit && limit > 0 {
		start = len(history) - limit
	}

	result := make([]HealthSnapshot, len(history[start:]))
	copy(result, history[start:])
	return result, nil
}

// GetHealthTrends analyzes health trends for an application
func (hm *HealthMonitor) GetHealthTrends(appName string, duration time.Duration) (*HealthTrends, error) {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()

	history, exists := hm.healthHistory[appName]
	if !exists {
		return nil, fmt.Errorf("no health history available for application '%s'", appName)
	}

	cutoffTime := time.Now().Add(-duration)
	var recentHistory []HealthSnapshot

	// Filter history to requested duration
	for _, snapshot := range history {
		if snapshot.Timestamp.After(cutoffTime) {
			recentHistory = append(recentHistory, snapshot)
		}
	}

	if len(recentHistory) == 0 {
		return &HealthTrends{
			AppName:        appName,
			AnalysisPeriod: duration,
			SampleCount:    0,
		}, nil
	}

	// Calculate trends
	trends := &HealthTrends{
		AppName:        appName,
		AnalysisPeriod: duration,
		SampleCount:    len(recentHistory),
	}

	// Calculate uptime percentage
	healthyCount := 0
	totalResponseTime := time.Duration(0)

	for _, snapshot := range recentHistory {
		if snapshot.Status == "ready" {
			healthyCount++
		}
		totalResponseTime += snapshot.ResponseTime
	}

	trends.UptimePercentage = float64(healthyCount) / float64(len(recentHistory)) * 100
	trends.AverageResponseTime = totalResponseTime / time.Duration(len(recentHistory))

	// Calculate availability trend
	if len(recentHistory) >= 2 {
		firstHalf := recentHistory[:len(recentHistory)/2]
		secondHalf := recentHistory[len(recentHistory)/2:]

		firstHalfHealthy := 0
		secondHalfHealthy := 0

		for _, snapshot := range firstHalf {
			if snapshot.Status == "ready" {
				firstHalfHealthy++
			}
		}

		for _, snapshot := range secondHalf {
			if snapshot.Status == "ready" {
				secondHalfHealthy++
			}
		}

		firstHalfPercent := float64(firstHalfHealthy) / float64(len(firstHalf)) * 100
		secondHalfPercent := float64(secondHalfHealthy) / float64(len(secondHalf)) * 100

		if secondHalfPercent > firstHalfPercent {
			trends.AvailabilityTrend = "improving"
		} else if secondHalfPercent < firstHalfPercent {
			trends.AvailabilityTrend = "degrading"
		} else {
			trends.AvailabilityTrend = "stable"
		}
	}

	return trends, nil
}

// HealthTrends represents health trend analysis for an application
type HealthTrends struct {
	AppName             string        `json:"appName"`
	AnalysisPeriod      time.Duration `json:"analysisPeriod"`
	SampleCount         int           `json:"sampleCount"`
	UptimePercentage    float64       `json:"uptimePercentage"`
	AverageResponseTime time.Duration `json:"averageResponseTime"`
	AvailabilityTrend   string        `json:"availabilityTrend"` // "improving", "degrading", "stable"
	RecommendedActions  []string      `json:"recommendedActions,omitempty"`
}

// Helper methods for internal operations

// recordHealthSnapshot adds a health snapshot to the history
func (hm *HealthMonitor) recordHealthSnapshot(appName string, snapshot HealthSnapshot) {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()

	if hm.healthHistory[appName] == nil {
		hm.healthHistory[appName] = make([]HealthSnapshot, 0)
	}

	hm.healthHistory[appName] = append(hm.healthHistory[appName], snapshot)

	// Trim history if it exceeds maximum size
	if len(hm.healthHistory[appName]) > hm.maxHistorySize {
		hm.healthHistory[appName] = hm.healthHistory[appName][1:]
	}
}

// evaluateAlertConditions checks if any alert thresholds are exceeded
func (hm *HealthMonitor) evaluateAlertConditions(appName string, result *HealthCheckResult) {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()

	threshold, exists := hm.alertThresholds[appName]
	if !exists {
		// Set default threshold
		threshold = AlertThreshold{
			MaxResponseTime:     5 * time.Second,
			MinUptimePercent:    95.0,
			MaxConsecutiveFails: 3,
			AlertCooldown:       10 * time.Minute,
		}
		hm.alertThresholds[appName] = threshold
	}

	alertTriggered := false

	// Check response time threshold
	if result.Overall.ResponseTime > threshold.MaxResponseTime {
		alertTriggered = true
		result.Recommendations = append(result.Recommendations,
			fmt.Sprintf("Response time (%v) exceeds threshold (%v)",
				result.Overall.ResponseTime, threshold.MaxResponseTime))
	}

	// Check if enough time has passed since last alert
	if alertTriggered && time.Since(threshold.LastAlert) < threshold.AlertCooldown {
		alertTriggered = false
	}

	if alertTriggered {
		threshold.LastAlert = time.Now()
		hm.alertThresholds[appName] = threshold
	}

	result.AlertTriggered = alertTriggered
}

// classifyNetworkError categorizes network errors for better diagnostics
func classifyNetworkError(err error) string {
	errStr := err.Error()

	if strings.Contains(errStr, "connection refused") {
		return "connection_refused"
	}
	if strings.Contains(errStr, "timeout") {
		return "timeout"
	}
	if strings.Contains(errStr, "no such host") {
		return "dns_failure"
	}
	if strings.Contains(errStr, "network unreachable") {
		return "network_unreachable"
	}

	return "unknown_network_error"
}

// classifyProtocolError categorizes protocol errors for better diagnostics
func classifyProtocolError(err error) string {
	errStr := err.Error()

	if strings.Contains(errStr, "authentication") {
		return "authentication_error"
	}
	if strings.Contains(errStr, "protocol version") {
		return "protocol_version_mismatch"
	}
	if strings.Contains(errStr, "handshake") {
		return "handshake_failure"
	}
	if strings.Contains(errStr, "timeout") {
		return "protocol_timeout"
	}

	return "unknown_protocol_error"
}

// SetRetryPolicy configures retry behavior for a specific application
func (hm *HealthMonitor) SetRetryPolicy(appName string, policy RetryPolicy) {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()

	hm.retryPolicies[appName] = policy
}

// SetAlertThreshold configures alert thresholds for a specific application
func (hm *HealthMonitor) SetAlertThreshold(appName string, threshold AlertThreshold) {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()

	hm.alertThresholds[appName] = threshold
}

// ClearHealthHistory removes all health history for an application
func (hm *HealthMonitor) ClearHealthHistory(appName string) {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()

	delete(hm.healthHistory, appName)
}
