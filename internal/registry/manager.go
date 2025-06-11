// Package registry implements comprehensive application registration and health monitoring
// for the Universal Application Console. This implementation provides centralized
// application management capabilities with persistent storage and real-time status
// monitoring as specified in section 3.2.4 of the design specification.
package registry

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/universal-console/console/internal/interfaces"
)

// Manager implements the RegistryManager interface with comprehensive application management capabilities
type Manager struct {
	configManager    interfaces.ConfigManager
	protocolClient   interfaces.ProtocolClient
	healthMonitor    *HealthMonitor
	registeredApps   map[string]*interfaces.RegisteredApp
	appHealth        map[string]*interfaces.AppHealth
	mutex            sync.RWMutex
	monitoringActive bool
	monitoringCancel context.CancelFunc
	preferences      RegistryPreferences
	statistics       RegistryStatistics
}

// RegistryPreferences defines configuration options for application registry behavior
type RegistryPreferences struct {
	AutoHealthCheck     bool          `json:"autoHealthCheck"`
	HealthCheckInterval time.Duration `json:"healthCheckInterval"`
	HealthCheckTimeout  time.Duration `json:"healthCheckTimeout"`
	RetryAttempts       int           `json:"retryAttempts"`
	RetryDelay          time.Duration `json:"retryDelay"`
	PersistHealth       bool          `json:"persistHealth"`
	ConcurrentChecks    int           `json:"concurrentChecks"`
	AlertThreshold      time.Duration `json:"alertThreshold"`
}

// RegistryStatistics tracks metrics about application registration and health monitoring
type RegistryStatistics struct {
	TotalApplications   int                   `json:"totalApplications"`
	HealthyApplications int                   `json:"healthyApplications"`
	OfflineApplications int                   `json:"offlineApplications"`
	ErrorApplications   int                   `json:"errorApplications"`
	TotalHealthChecks   int64                 `json:"totalHealthChecks"`
	SuccessfulChecks    int64                 `json:"successfulChecks"`
	FailedChecks        int64                 `json:"failedChecks"`
	AverageResponseTime time.Duration         `json:"averageResponseTime"`
	LastUpdateTime      time.Time             `json:"lastUpdateTime"`
	ApplicationMetrics  map[string]AppMetrics `json:"applicationMetrics"`
}

// AppMetrics provides detailed metrics for individual applications
type AppMetrics struct {
	TotalChecks         int64         `json:"totalChecks"`
	SuccessfulChecks    int64         `json:"successfulChecks"`
	FailedChecks        int64         `json:"failedChecks"`
	AverageResponseTime time.Duration `json:"averageResponseTime"`
	UptimePercentage    float64       `json:"uptimePercentage"`
	LastOnlineTime      time.Time     `json:"lastOnlineTime"`
	LastOfflineTime     time.Time     `json:"lastOfflineTime"`
	ConsecutiveFailures int           `json:"consecutiveFailures"`
}

// RegistryEventType represents different types of registry events for monitoring
type RegistryEventType string

const (
	EventAppRegistered   RegistryEventType = "app_registered"
	EventAppUnregistered RegistryEventType = "app_unregistered"
	EventAppStatusChange RegistryEventType = "app_status_change"
	EventHealthCheckFail RegistryEventType = "health_check_fail"
	EventHealthCheckPass RegistryEventType = "health_check_pass"
	EventMonitoringStart RegistryEventType = "monitoring_start"
	EventMonitoringStop  RegistryEventType = "monitoring_stop"
)

// RegistryEvent represents an event in the application registry
type RegistryEvent struct {
	Type       RegistryEventType `json:"type"`
	AppName    string            `json:"appName"`
	Timestamp  time.Time         `json:"timestamp"`
	Details    string            `json:"details"`
	PrevStatus string            `json:"prevStatus,omitempty"`
	NewStatus  string            `json:"newStatus,omitempty"`
	Error      string            `json:"error,omitempty"`
	Duration   time.Duration     `json:"duration,omitempty"`
}

// NewManager creates a new application registry manager with injected dependencies
func NewManager(configManager interfaces.ConfigManager, protocolClient interfaces.ProtocolClient) (*Manager, error) {
	if configManager == nil {
		return nil, fmt.Errorf("configManager cannot be nil")
	}

	if protocolClient == nil {
		return nil, fmt.Errorf("protocolClient cannot be nil")
	}

	// Initialize health monitor
	healthMonitor := NewHealthMonitor()

	// Set default preferences
	preferences := RegistryPreferences{
		AutoHealthCheck:     true,
		HealthCheckInterval: 30 * time.Second,
		HealthCheckTimeout:  5 * time.Second,
		RetryAttempts:       3,
		RetryDelay:          2 * time.Second,
		PersistHealth:       true,
		ConcurrentChecks:    5,
		AlertThreshold:      5 * time.Minute,
	}

	manager := &Manager{
		configManager:  configManager,
		protocolClient: protocolClient,
		healthMonitor:  healthMonitor,
		registeredApps: make(map[string]*interfaces.RegisteredApp),
		appHealth:      make(map[string]*interfaces.AppHealth),
		preferences:    preferences,
		statistics: RegistryStatistics{
			ApplicationMetrics: make(map[string]AppMetrics),
			LastUpdateTime:     time.Now(),
		},
	}

	// Load existing applications from configuration
	if err := manager.loadRegisteredApps(); err != nil {
		return nil, fmt.Errorf("failed to load registered applications: %w", err)
	}

	return manager, nil
}

// GetRegisteredApps returns all registered applications with current status
func (m *Manager) GetRegisteredApps() ([]interfaces.RegisteredApp, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var apps []interfaces.RegisteredApp
	for _, app := range m.registeredApps {
		// Create copy with current health status
		appCopy := *app
		if health, exists := m.appHealth[app.Name]; exists {
			appCopy.Status = health.Status
		} else {
			appCopy.Status = "unknown"
		}
		apps = append(apps, appCopy)
	}

	return apps, nil
}

// RegisterApp adds a new application to the registry
func (m *Manager) RegisterApp(app interfaces.RegisteredApp) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Validate application details
	if err := m.validateApp(&app); err != nil {
		return fmt.Errorf("invalid application: %w", err)
	}

	// Check if application already exists
	if existingApp, exists := m.registeredApps[app.Name]; exists {
		// Update existing application
		existingApp.Profile = app.Profile
		existingApp.AutoStart = app.AutoStart
		m.logEvent(EventAppRegistered, app.Name, "Application updated", "")
	} else {
		// Add new application
		m.registeredApps[app.Name] = &app
		m.statistics.TotalApplications++

		// Initialize health status
		m.appHealth[app.Name] = &interfaces.AppHealth{
			Name:        app.Name,
			Status:      "unknown",
			LastChecked: time.Now(),
		}

		// Initialize metrics
		m.statistics.ApplicationMetrics[app.Name] = AppMetrics{
			UptimePercentage: 0.0,
		}

		m.logEvent(EventAppRegistered, app.Name, "Application registered", "")
	}

	// Persist to configuration
	if err := m.persistRegisteredApps(); err != nil {
		return fmt.Errorf("failed to persist application registry: %w", err)
	}

	// Trigger immediate health check if monitoring is active
	if m.monitoringActive {
		go m.performImmediateHealthCheck(app.Name)
	}

	return nil
}

// UnregisterApp removes an application from the registry
func (m *Manager) UnregisterApp(name string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.registeredApps[name]; !exists {
		return fmt.Errorf("application '%s' not found in registry", name)
	}

	// Remove application and health data
	delete(m.registeredApps, name)
	delete(m.appHealth, name)
	delete(m.statistics.ApplicationMetrics, name)
	m.statistics.TotalApplications--

	m.logEvent(EventAppUnregistered, name, "Application unregistered", "")

	// Persist changes to configuration
	if err := m.persistRegisteredApps(); err != nil {
		return fmt.Errorf("failed to persist application registry after removal: %w", err)
	}

	return nil
}

// UpdateAppStatus updates the health status of an application
func (m *Manager) UpdateAppStatus(name string, status interfaces.AppHealth) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.registeredApps[name]; !exists {
		return fmt.Errorf("application '%s' not found in registry", name)
	}

	// Update health status
	previousStatus := "unknown"
	if existingHealth, exists := m.appHealth[name]; exists {
		previousStatus = existingHealth.Status
	}

	m.appHealth[name] = &status
	m.updateStatistics(name, &status)

	// Log status change if different
	if previousStatus != status.Status {
		m.logEvent(EventAppStatusChange, name,
			fmt.Sprintf("Status changed from %s to %s", previousStatus, status.Status),
			"")
	}

	return nil
}

// GetAppHealth returns current health information for an application
func (m *Manager) GetAppHealth(name string) (*interfaces.AppHealth, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if _, exists := m.registeredApps[name]; !exists {
		return nil, fmt.Errorf("application '%s' not found in registry", name)
	}

	if health, exists := m.appHealth[name]; exists {
		// Return a copy to prevent external modification
		healthCopy := *health
		return &healthCopy, nil
	}

	return nil, fmt.Errorf("no health information available for application '%s'", name)
}

// StartHealthMonitoring begins periodic health checks for all registered applications
func (m *Manager) StartHealthMonitoring(ctx context.Context, interval time.Duration) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.monitoringActive {
		return fmt.Errorf("health monitoring is already active")
	}

	// Override interval if provided, otherwise use preference
	monitoringInterval := interval
	if interval == 0 {
		monitoringInterval = m.preferences.HealthCheckInterval
	}

	// Create monitoring context
	monitoringCtx, cancel := context.WithCancel(ctx)
	m.monitoringCancel = cancel
	m.monitoringActive = true

	m.logEvent(EventMonitoringStart, "",
		fmt.Sprintf("Health monitoring started with interval %v", monitoringInterval), "")

	// Start monitoring goroutine
	go m.runHealthMonitoring(monitoringCtx, monitoringInterval)

	return nil
}

// StopHealthMonitoring stops all health monitoring
func (m *Manager) StopHealthMonitoring() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.monitoringActive {
		return fmt.Errorf("health monitoring is not currently active")
	}

	// Cancel monitoring context
	if m.monitoringCancel != nil {
		m.monitoringCancel()
		m.monitoringCancel = nil
	}

	m.monitoringActive = false
	m.logEvent(EventMonitoringStop, "", "Health monitoring stopped", "")

	return nil
}

// CheckAppHealth performs an immediate health check for a specific application
func (m *Manager) CheckAppHealth(ctx context.Context, appName string) (*interfaces.AppHealth, error) {
	m.mutex.RLock()
	app, exists := m.registeredApps[appName]
	m.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("application '%s' not found in registry", appName)
	}

	// Perform health check using health monitor
	healthResult, err := m.healthMonitor.CheckApplicationHealth(ctx, app, m.configManager, m.protocolClient)
	if err != nil {
		m.logEvent(EventHealthCheckFail, appName, "Health check failed", err.Error())
		return nil, fmt.Errorf("health check failed for application '%s': %w", appName, err)
	}

	// Update stored health information
	m.mutex.Lock()
	m.appHealth[appName] = healthResult
	m.updateStatistics(appName, healthResult)
	m.mutex.Unlock()

	if healthResult.Status == "ready" {
		m.logEvent(EventHealthCheckPass, appName, "Health check passed", "")
	} else {
		m.logEvent(EventHealthCheckFail, appName, "Health check failed", healthResult.Error)
	}

	return healthResult, nil
}

// GetAppByName retrieves application details by name
func (m *Manager) GetAppByName(name string) (*interfaces.RegisteredApp, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if app, exists := m.registeredApps[name]; exists {
		// Return a copy to prevent external modification
		appCopy := *app
		return &appCopy, nil
	}

	return nil, fmt.Errorf("application '%s' not found in registry", name)
}

// GetRegistryStatistics returns comprehensive statistics about the application registry
func (m *Manager) GetRegistryStatistics() RegistryStatistics {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Create a copy of statistics to prevent external modification
	statsCopy := m.statistics
	statsCopy.ApplicationMetrics = make(map[string]AppMetrics)
	for name, metrics := range m.statistics.ApplicationMetrics {
		statsCopy.ApplicationMetrics[name] = metrics
	}

	return statsCopy
}

// UpdatePreferences updates the registry manager preferences
func (m *Manager) UpdatePreferences(preferences RegistryPreferences) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.preferences = preferences

	// If monitoring is active and interval changed, restart monitoring
	if m.monitoringActive && m.monitoringCancel != nil {
		// Note: This would require restarting monitoring with new interval
		// For simplicity, we'll just update the preferences
		// In a production implementation, we'd restart the monitoring goroutine
	}

	return nil
}

// Helper methods for internal operations

// validateApp ensures application details are valid before registration
func (m *Manager) validateApp(app *interfaces.RegisteredApp) error {
	if app.Name == "" {
		return fmt.Errorf("application name cannot be empty")
	}

	if app.Profile == "" {
		return fmt.Errorf("application profile cannot be empty")
	}

	// Validate that the profile exists in configuration
	_, err := m.configManager.LoadProfile(app.Profile)
	if err != nil {
		return fmt.Errorf("profile '%s' does not exist: %w", app.Profile, err)
	}

	return nil
}

// loadRegisteredApps loads applications from the configuration manager
func (m *Manager) loadRegisteredApps() error {
	apps, err := m.configManager.GetRegisteredApps()
	if err != nil {
		return fmt.Errorf("failed to load registered applications: %w", err)
	}

	for _, app := range apps {
		m.registeredApps[app.Name] = &app
		m.appHealth[app.Name] = &interfaces.AppHealth{
			Name:        app.Name,
			Status:      "unknown",
			LastChecked: time.Now(),
		}
		m.statistics.ApplicationMetrics[app.Name] = AppMetrics{
			UptimePercentage: 0.0,
		}
	}

	m.statistics.TotalApplications = len(apps)
	return nil
}

// persistRegisteredApps saves the current application registry to configuration
func (m *Manager) persistRegisteredApps() error {
	var apps []interfaces.RegisteredApp
	for _, app := range m.registeredApps {
		apps = append(apps, *app)
	}

	// Note: This assumes the ConfigManager interface has a method to save registered apps
	// In the actual implementation, this might be handled differently
	for _, app := range apps {
		if err := m.configManager.RegisterApp(app); err != nil {
			return fmt.Errorf("failed to persist application '%s': %w", app.Name, err)
		}
	}

	return nil
}

// runHealthMonitoring executes the health monitoring loop
func (m *Manager) runHealthMonitoring(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.performHealthCheckCycle(ctx)
		}
	}
}

// performHealthCheckCycle checks the health of all registered applications
func (m *Manager) performHealthCheckCycle(ctx context.Context) {
	m.mutex.RLock()
	apps := make([]*interfaces.RegisteredApp, 0, len(m.registeredApps))
	for _, app := range m.registeredApps {
		apps = append(apps, app)
	}
	m.mutex.RUnlock()

	// Use semaphore to limit concurrent checks
	semaphore := make(chan struct{}, m.preferences.ConcurrentChecks)

	for _, app := range apps {
		select {
		case <-ctx.Done():
			return
		case semaphore <- struct{}{}:
			go func(appToCheck *interfaces.RegisteredApp) {
				defer func() { <-semaphore }()
				m.performSingleHealthCheck(ctx, appToCheck)
			}(app)
		}
	}
}

// performSingleHealthCheck checks the health of a single application
func (m *Manager) performSingleHealthCheck(ctx context.Context, app *interfaces.RegisteredApp) {
	healthCtx, cancel := context.WithTimeout(ctx, m.preferences.HealthCheckTimeout)
	defer cancel()

	healthResult, err := m.healthMonitor.CheckApplicationHealth(healthCtx, app, m.configManager, m.protocolClient)

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if err != nil {
		// Create error health status
		healthResult = &interfaces.AppHealth{
			Name:        app.Name,
			Status:      "error",
			LastChecked: time.Now(),
			Error:       err.Error(),
		}
	}

	// Update health status
	previousStatus := "unknown"
	if existingHealth, exists := m.appHealth[app.Name]; exists {
		previousStatus = existingHealth.Status
	}

	m.appHealth[app.Name] = healthResult
	m.updateStatistics(app.Name, healthResult)

	// Log significant status changes
	if previousStatus != healthResult.Status {
		if healthResult.Status == "ready" {
			m.logEvent(EventHealthCheckPass, app.Name, "Application is healthy", "")
		} else {
			m.logEvent(EventHealthCheckFail, app.Name, "Application health check failed", healthResult.Error)
		}
	}
}

// performImmediateHealthCheck performs an immediate health check for a specific application
func (m *Manager) performImmediateHealthCheck(appName string) {
	ctx, cancel := context.WithTimeout(context.Background(), m.preferences.HealthCheckTimeout)
	defer cancel()

	m.CheckAppHealth(ctx, appName)
}

// updateStatistics updates registry statistics based on health check results
func (m *Manager) updateStatistics(appName string, health *interfaces.AppHealth) {
	// Update overall statistics
	m.statistics.TotalHealthChecks++
	m.statistics.LastUpdateTime = time.Now()

	if health.Status == "ready" {
		m.statistics.SuccessfulChecks++
	} else {
		m.statistics.FailedChecks++
	}

	// Update application-specific metrics
	metrics := m.statistics.ApplicationMetrics[appName]
	metrics.TotalChecks++

	if health.Status == "ready" {
		metrics.SuccessfulChecks++
		metrics.LastOnlineTime = health.LastChecked
		metrics.ConsecutiveFailures = 0
	} else {
		metrics.FailedChecks++
		metrics.LastOfflineTime = health.LastChecked
		metrics.ConsecutiveFailures++
	}

	// Calculate uptime percentage
	if metrics.TotalChecks > 0 {
		metrics.UptimePercentage = float64(metrics.SuccessfulChecks) / float64(metrics.TotalChecks) * 100
	}

	// Update average response time if available
	if health.ResponseTime > 0 {
		if metrics.AverageResponseTime == 0 {
			metrics.AverageResponseTime = health.ResponseTime
		} else {
			// Calculate moving average
			metrics.AverageResponseTime = (metrics.AverageResponseTime + health.ResponseTime) / 2
		}
	}

	m.statistics.ApplicationMetrics[appName] = metrics

	// Update overall status counts
	m.recalculateStatusCounts()
}

// recalculateStatusCounts recalculates the overall status counts in statistics
func (m *Manager) recalculateStatusCounts() {
	m.statistics.HealthyApplications = 0
	m.statistics.OfflineApplications = 0
	m.statistics.ErrorApplications = 0

	for _, health := range m.appHealth {
		switch health.Status {
		case "ready":
			m.statistics.HealthyApplications++
		case "offline":
			m.statistics.OfflineApplications++
		case "error":
			m.statistics.ErrorApplications++
		}
	}
}

// logEvent logs registry events for monitoring and debugging
func (m *Manager) logEvent(eventType RegistryEventType, appName, details, errorMsg string) {
	// In a production implementation, this would log to a structured logging system
	// For now, we'll keep it simple
	event := RegistryEvent{
		Type:      eventType,
		AppName:   appName,
		Timestamp: time.Now(),
		Details:   details,
		Error:     errorMsg,
	}

	// This could be enhanced to support event listeners or persistent event storage
	_ = event
}
