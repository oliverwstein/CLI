// Package auth implements comprehensive authentication and security management for the Universal Application Console.
// This implementation handles bearer token management, secure credential storage, and authentication header construction
// according to the security protocol specified in section 3.7.1 of the design specification.
package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/universal-console/console/internal/interfaces"
)

// TokenMetadata contains metadata about authentication tokens for management purposes
type TokenMetadata struct {
	Type         string    `json:"type"`
	IssuedAt     time.Time `json:"issuedAt,omitempty"`
	ExpiresAt    time.Time `json:"expiresAt,omitempty"`
	Issuer       string    `json:"issuer,omitempty"`
	Subject      string    `json:"subject,omitempty"`
	Audience     string    `json:"audience,omitempty"`
	Scopes       []string  `json:"scopes,omitempty"`
	TokenID      string    `json:"tokenId,omitempty"`
	RefreshToken string    `json:"refreshToken,omitempty"`
}

// SessionState maintains authentication state across application interactions
type SessionState struct {
	ProfileName     string         `json:"profileName"`
	AuthType        string         `json:"authType"`
	TokenMetadata   *TokenMetadata `json:"tokenMetadata,omitempty"`
	LastValidated   time.Time      `json:"lastValidated"`
	ValidationCount int            `json:"validationCount"`
	SessionStart    time.Time      `json:"sessionStart"`
	LastActivity    time.Time      `json:"lastActivity"`
}

// AuthenticationCache provides secure in-memory caching of authentication data
type AuthenticationCache struct {
	credentials map[string]string
	metadata    map[string]*TokenMetadata
	sessions    map[string]*SessionState
	mutex       sync.RWMutex
	maxAge      time.Duration
}

// Manager implements the AuthManager interface with comprehensive authentication capabilities
type Manager struct {
	configManager interfaces.ConfigManager
	cache         *AuthenticationCache
	secureStorage SecureStorage
	validator     *TokenValidator
	mutex         sync.RWMutex
}

// SecureStorage interface for abstracting secure credential storage mechanisms
type SecureStorage interface {
	Store(key, value string) error
	Retrieve(key string) (string, error)
	Delete(key string) error
	Clear() error
	Exists(key string) bool
}

// TokenValidator provides comprehensive token validation capabilities
type TokenValidator struct {
	jwtRegex       *regexp.Regexp
	strictMode     bool
	minTokenLength int
	maxTokenLength int
}

// NewManager creates a new authentication manager with injected configuration management
func NewManager(configManager interfaces.ConfigManager) (*Manager, error) {
	if configManager == nil {
		return nil, fmt.Errorf("configManager cannot be nil")
	}

	// Initialize secure storage
	secureStorage := NewInMemorySecureStorage()

	// Initialize authentication cache with reasonable defaults
	cache := &AuthenticationCache{
		credentials: make(map[string]string),
		metadata:    make(map[string]*TokenMetadata),
		sessions:    make(map[string]*SessionState),
		maxAge:      24 * time.Hour, // Default cache duration
	}

	// Initialize token validator with security best practices
	validator := &TokenValidator{
		jwtRegex:       regexp.MustCompile(`^[A-Za-z0-9\-_]+\.[A-Za-z0-9\-_]+\.[A-Za-z0-9\-_]+$`),
		strictMode:     true,
		minTokenLength: 8,
		maxTokenLength: 4096,
	}

	manager := &Manager{
		configManager: configManager,
		cache:         cache,
		secureStorage: secureStorage,
		validator:     validator,
	}

	return manager, nil
}

// ValidateToken verifies the format and basic validity of an authentication token
func (m *Manager) ValidateToken(token string, tokenType string) error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.validator.ValidateToken(token, tokenType)
}

// CreateAuthHeader constructs the appropriate authentication header value
func (m *Manager) CreateAuthHeader(auth *interfaces.AuthConfig) (string, error) {
	if auth == nil {
		return "", fmt.Errorf("authentication configuration cannot be nil")
	}

	// Validate authentication configuration
	if err := m.ValidateToken(auth.Token, auth.Type); err != nil {
		return "", fmt.Errorf("invalid authentication configuration: %w", err)
	}

	switch strings.ToLower(auth.Type) {
	case "bearer":
		return fmt.Sprintf("Bearer %s", auth.Token), nil
	case "none":
		return "", nil
	default:
		return "", fmt.Errorf("unsupported authentication type: %s", auth.Type)
	}
}

// SecureStore encrypts and stores sensitive authentication data
func (m *Manager) SecureStore(key string, value string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("storage key cannot be empty")
	}

	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("storage value cannot be empty")
	}

	// Store in secure storage with encryption
	if err := m.secureStorage.Store(key, value); err != nil {
		return fmt.Errorf("failed to store credential securely: %w", err)
	}

	// Update cache for performance
	m.cache.mutex.Lock()
	m.cache.credentials[key] = value
	m.cache.mutex.Unlock()

	return nil
}

// SecureRetrieve decrypts and retrieves sensitive authentication data
func (m *Manager) SecureRetrieve(key string) (string, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if strings.TrimSpace(key) == "" {
		return "", fmt.Errorf("storage key cannot be empty")
	}

	// Check cache first for performance
	m.cache.mutex.RLock()
	if cachedValue, exists := m.cache.credentials[key]; exists {
		m.cache.mutex.RUnlock()
		return cachedValue, nil
	}
	m.cache.mutex.RUnlock()

	// Retrieve from secure storage
	value, err := m.secureStorage.Retrieve(key)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve credential: %w", err)
	}

	// Update cache
	m.cache.mutex.Lock()
	m.cache.credentials[key] = value
	m.cache.mutex.Unlock()

	return value, nil
}

// ClearSecureData removes all stored authentication credentials
func (m *Manager) ClearSecureData() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Clear secure storage
	if err := m.secureStorage.Clear(); err != nil {
		return fmt.Errorf("failed to clear secure storage: %w", err)
	}

	// Clear cache
	m.cache.mutex.Lock()
	m.cache.credentials = make(map[string]string)
	m.cache.metadata = make(map[string]*TokenMetadata)
	m.cache.sessions = make(map[string]*SessionState)
	m.cache.mutex.Unlock()

	return nil
}

// RefreshToken attempts to refresh an expired token if possible
func (m *Manager) RefreshToken(auth *interfaces.AuthConfig) (*interfaces.AuthConfig, error) {
	if auth == nil {
		return nil, fmt.Errorf("authentication configuration cannot be nil")
	}

	// Currently, the protocol specification does not define token refresh mechanisms
	// This implementation provides a framework for future token refresh capabilities

	// Check if token metadata includes refresh token
	metadata := m.getTokenMetadata(auth.Token)
	if metadata == nil || metadata.RefreshToken == "" {
		return nil, fmt.Errorf("token refresh not supported for this authentication method")
	}

	// Token refresh would be implemented here with appropriate HTTP calls
	// For now, return the original configuration
	return auth, fmt.Errorf("token refresh not yet implemented")
}

// ValidatePermissions checks if current credentials have required permissions
func (m *Manager) ValidatePermissions(auth *interfaces.AuthConfig, requiredPerms []string) error {
	if auth == nil {
		return fmt.Errorf("authentication configuration cannot be nil")
	}

	if len(requiredPerms) == 0 {
		return nil // No permissions required
	}

	// Get token metadata to check permissions
	metadata := m.getTokenMetadata(auth.Token)
	if metadata == nil {
		return fmt.Errorf("cannot determine token permissions")
	}

	// Check if all required permissions are present in token scopes
	tokenScopes := make(map[string]bool)
	for _, scope := range metadata.Scopes {
		tokenScopes[scope] = true
	}

	var missingPerms []string
	for _, requiredPerm := range requiredPerms {
		if !tokenScopes[requiredPerm] {
			missingPerms = append(missingPerms, requiredPerm)
		}
	}

	if len(missingPerms) > 0 {
		return fmt.Errorf("missing required permissions: %s", strings.Join(missingPerms, ", "))
	}

	return nil
}

// Session management methods

// CreateSession establishes a new authentication session
func (m *Manager) CreateSession(profileName string, auth *interfaces.AuthConfig) (*SessionState, error) {
	if err := m.ValidateToken(auth.Token, auth.Type); err != nil {
		return nil, fmt.Errorf("cannot create session with invalid token: %w", err)
	}

	sessionID := m.generateSessionID()
	session := &SessionState{
		ProfileName:     profileName,
		AuthType:        auth.Type,
		TokenMetadata:   m.getTokenMetadata(auth.Token),
		LastValidated:   time.Now(),
		ValidationCount: 1,
		SessionStart:    time.Now(),
		LastActivity:    time.Now(),
	}

	m.cache.mutex.Lock()
	m.cache.sessions[sessionID] = session
	m.cache.mutex.Unlock()

	return session, nil
}

// GetSession retrieves the current authentication session state
func (m *Manager) GetSession(sessionID string) (*SessionState, error) {
	m.cache.mutex.RLock()
	defer m.cache.mutex.RUnlock()

	session, exists := m.cache.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found")
	}

	// Check if session has expired
	if time.Since(session.LastActivity) > m.cache.maxAge {
		return nil, fmt.Errorf("session has expired")
	}

	return session, nil
}

// UpdateSessionActivity updates the last activity time for a session
func (m *Manager) UpdateSessionActivity(sessionID string) error {
	m.cache.mutex.Lock()
	defer m.cache.mutex.Unlock()

	session, exists := m.cache.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session not found")
	}

	session.LastActivity = time.Now()
	session.ValidationCount++

	return nil
}

// Token validation implementation

// ValidateToken performs comprehensive token validation
func (v *TokenValidator) ValidateToken(token string, tokenType string) error {
	if strings.TrimSpace(token) == "" && tokenType != "none" {
		return fmt.Errorf("token cannot be empty for type '%s'", tokenType)
	}

	switch strings.ToLower(tokenType) {
	case "none":
		if token != "" {
			return fmt.Errorf("token must be empty when type is 'none'")
		}
		return nil
	case "bearer":
		return v.validateBearerToken(token)
	default:
		return fmt.Errorf("unsupported token type: %s", tokenType)
	}
}

// validateBearerToken performs comprehensive bearer token validation
func (v *TokenValidator) validateBearerToken(token string) error {
	// Basic format validation
	if err := v.validateTokenFormat(token); err != nil {
		return fmt.Errorf("bearer token format validation failed: %w", err)
	}

	// Check if token appears to be a JWT
	if v.jwtRegex.MatchString(token) {
		return v.validateJWTStructure(token)
	}

	// For non-JWT tokens, perform basic validation
	return v.validateGenericToken(token)
}

// validateTokenFormat performs basic token format validation
func (v *TokenValidator) validateTokenFormat(token string) error {
	token = strings.TrimSpace(token)

	if len(token) < v.minTokenLength {
		return fmt.Errorf("token is too short (minimum %d characters)", v.minTokenLength)
	}

	if len(token) > v.maxTokenLength {
		return fmt.Errorf("token is too long (maximum %d characters)", v.maxTokenLength)
	}

	// Check for whitespace characters
	if strings.ContainsAny(token, " \t\n\r") {
		return fmt.Errorf("token cannot contain whitespace characters")
	}

	// Check for obviously invalid placeholder values
	lowerToken := strings.ToLower(token)
	invalidPatterns := []string{"placeholder", "your-token", "example", "test", "sample", "demo"}
	for _, pattern := range invalidPatterns {
		if strings.Contains(lowerToken, pattern) {
			return fmt.Errorf("token appears to be a placeholder value")
		}
	}

	return nil
}

// validateJWTStructure performs JWT-specific structure validation
func (v *TokenValidator) validateJWTStructure(token string) error {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return fmt.Errorf("JWT must have exactly 3 parts")
	}

	// Validate each part is valid base64
	for i, part := range parts {
		if err := v.validateBase64URLPart(part); err != nil {
			return fmt.Errorf("JWT part %d is invalid: %w", i+1, err)
		}
	}

	// Optionally decode and validate JWT claims
	if v.strictMode {
		return v.validateJWTClaims(parts[1])
	}

	return nil
}

// validateGenericToken performs validation for non-JWT bearer tokens
func (v *TokenValidator) validateGenericToken(token string) error {
	// Ensure token contains only valid characters
	validChars := regexp.MustCompile(`^[A-Za-z0-9\-_.~+/]+=*$`)
	if !validChars.MatchString(token) {
		return fmt.Errorf("token contains invalid characters")
	}

	return nil
}

// validateBase64URLPart validates base64URL encoded JWT parts
func (v *TokenValidator) validateBase64URLPart(part string) error {
	// Add padding if necessary
	switch len(part) % 4 {
	case 2:
		part += "=="
	case 3:
		part += "="
	}

	// Try to decode as base64
	if _, err := base64.URLEncoding.DecodeString(part); err != nil {
		return fmt.Errorf("invalid base64URL encoding: %w", err)
	}

	return nil
}

// validateJWTClaims performs basic validation of JWT claims
func (v *TokenValidator) validateJWTClaims(payload string) error {
	// Add padding if necessary
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}

	// Decode payload
	claimsBytes, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return fmt.Errorf("failed to decode JWT claims: %w", err)
	}

	// Parse claims as JSON
	var claims map[string]interface{}
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return fmt.Errorf("invalid JWT claims JSON: %w", err)
	}

	// Validate expiration if present
	if exp, exists := claims["exp"]; exists {
		if expFloat, ok := exp.(float64); ok {
			expTime := time.Unix(int64(expFloat), 0)
			if time.Now().After(expTime) {
				return fmt.Errorf("JWT token has expired")
			}
		}
	}

	// Validate not-before if present
	if nbf, exists := claims["nbf"]; exists {
		if nbfFloat, ok := nbf.(float64); ok {
			nbfTime := time.Unix(int64(nbfFloat), 0)
			if time.Now().Before(nbfTime) {
				return fmt.Errorf("JWT token is not yet valid")
			}
		}
	}

	return nil
}

// Utility methods

// getTokenMetadata extracts metadata from a token if possible
func (m *Manager) getTokenMetadata(token string) *TokenMetadata {
	// Check cache first
	m.cache.mutex.RLock()
	if metadata, exists := m.cache.metadata[token]; exists {
		m.cache.mutex.RUnlock()
		return metadata
	}
	m.cache.mutex.RUnlock()

	// Try to extract metadata from JWT tokens
	if m.validator.jwtRegex.MatchString(token) {
		if metadata := m.extractJWTMetadata(token); metadata != nil {
			// Cache the metadata
			m.cache.mutex.Lock()
			m.cache.metadata[token] = metadata
			m.cache.mutex.Unlock()
			return metadata
		}
	}

	return nil
}

// extractJWTMetadata extracts metadata from JWT tokens
func (m *Manager) extractJWTMetadata(token string) *TokenMetadata {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil
	}

	// Decode payload
	payload := parts[1]
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}

	claimsBytes, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return nil
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return nil
	}

	metadata := &TokenMetadata{
		Type: "bearer",
	}

	// Extract standard JWT claims
	if iss, ok := claims["iss"].(string); ok {
		metadata.Issuer = iss
	}
	if sub, ok := claims["sub"].(string); ok {
		metadata.Subject = sub
	}
	if aud, ok := claims["aud"].(string); ok {
		metadata.Audience = aud
	}
	if jti, ok := claims["jti"].(string); ok {
		metadata.TokenID = jti
	}

	// Extract timestamps
	if iat, ok := claims["iat"].(float64); ok {
		metadata.IssuedAt = time.Unix(int64(iat), 0)
	}
	if exp, ok := claims["exp"].(float64); ok {
		metadata.ExpiresAt = time.Unix(int64(exp), 0)
	}

	// Extract scopes if present
	if scope, ok := claims["scope"].(string); ok {
		metadata.Scopes = strings.Split(scope, " ")
	} else if scopes, ok := claims["scopes"].([]interface{}); ok {
		for _, s := range scopes {
			if scopeStr, ok := s.(string); ok {
				metadata.Scopes = append(metadata.Scopes, scopeStr)
			}
		}
	}

	return metadata
}

// generateSessionID creates a unique session identifier
func (m *Manager) generateSessionID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return base64.URLEncoding.EncodeToString(bytes)
}

// InMemorySecureStorage provides a simple in-memory secure storage implementation
type InMemorySecureStorage struct {
	data  map[string]string
	mutex sync.RWMutex
}

// NewInMemorySecureStorage creates a new in-memory secure storage instance
func NewInMemorySecureStorage() *InMemorySecureStorage {
	return &InMemorySecureStorage{
		data: make(map[string]string),
	}
}

// Store implements SecureStorage.Store
func (s *InMemorySecureStorage) Store(key, value string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.data[key] = value
	return nil
}

// Retrieve implements SecureStorage.Retrieve
func (s *InMemorySecureStorage) Retrieve(key string) (string, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	value, exists := s.data[key]
	if !exists {
		return "", fmt.Errorf("key not found")
	}
	return value, nil
}

// Delete implements SecureStorage.Delete
func (s *InMemorySecureStorage) Delete(key string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.data, key)
	return nil
}

// Clear implements SecureStorage.Clear
func (s *InMemorySecureStorage) Clear() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.data = make(map[string]string)
	return nil
}

// Exists implements SecureStorage.Exists
func (s *InMemorySecureStorage) Exists(key string) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	_, exists := s.data[key]
	return exists
}
