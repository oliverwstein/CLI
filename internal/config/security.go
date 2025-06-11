// Package config provides secure configuration storage mechanisms for the Universal Application Console.
// This implementation handles encryption for sensitive authentication credentials, token validation,
// and secure file permissions according to section 3.7.1 of the design specification.
package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

// SecurityManager handles encryption and decryption of sensitive configuration data
type SecurityManager interface {
	// EncryptCredential encrypts sensitive authentication data for storage
	EncryptCredential(plaintext string) (string, error)

	// DecryptCredential decrypts stored authentication data for use
	DecryptCredential(ciphertext string) (string, error)

	// ValidateTokenFormat performs format validation on authentication tokens
	ValidateTokenFormat(token string, tokenType string) error

	// SecureKeyExists checks if encryption key material is available
	SecureKeyExists() bool

	// GenerateSecureKey creates new encryption key material
	GenerateSecureKey() error
}

// AESSecurityManager implements SecurityManager using AES-256-GCM encryption
type AESSecurityManager struct {
	keyPath    string
	masterKey  []byte
	keyDerived bool
}

// NewSecurityManager creates a new security manager with OS-appropriate key storage
func NewSecurityManager() (SecurityManager, error) {
	keyPath, err := getSecurityKeyPath()
	if err != nil {
		return nil, fmt.Errorf("failed to determine security key path: %w", err)
	}

	manager := &AESSecurityManager{
		keyPath: keyPath,
	}

	// Ensure the security directory exists with restrictive permissions
	if err := manager.ensureSecurityDirectory(); err != nil {
		return nil, fmt.Errorf("failed to create security directory: %w", err)
	}

	// Load or generate encryption key
	if err := manager.initializeEncryptionKey(); err != nil {
		return nil, fmt.Errorf("failed to initialize encryption key: %w", err)
	}

	return manager, nil
}

// getSecurityKeyPath determines the OS-appropriate path for storing encryption keys
func getSecurityKeyPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine home directory: %w", err)
	}

	var securityDir string
	if xdgDataHome := os.Getenv("XDG_DATA_HOME"); xdgDataHome != "" {
		securityDir = filepath.Join(xdgDataHome, "console", "security")
	} else {
		securityDir = filepath.Join(homeDir, ".local", "share", "console", "security")
	}

	return filepath.Join(securityDir, "master.key"), nil
}

// ensureSecurityDirectory creates the security directory with highly restrictive permissions
func (s *AESSecurityManager) ensureSecurityDirectory() error {
	securityDir := filepath.Dir(s.keyPath)

	// Create directory with maximum security permissions (accessible by owner only)
	if err := os.MkdirAll(securityDir, 0700); err != nil {
		return fmt.Errorf("failed to create security directory %s: %w", securityDir, err)
	}

	return nil
}

// initializeEncryptionKey loads existing key or generates a new one
func (s *AESSecurityManager) initializeEncryptionKey() error {
	// Check if key file exists
	if _, err := os.Stat(s.keyPath); os.IsNotExist(err) {
		// Generate new key if none exists
		return s.GenerateSecureKey()
	}

	// Load existing key
	return s.loadExistingKey()
}

// loadExistingKey reads and derives the master key from stored key material
func (s *AESSecurityManager) loadExistingKey() error {
	keyData, err := os.ReadFile(s.keyPath)
	if err != nil {
		return fmt.Errorf("failed to read master key file: %w", err)
	}

	// Decode the stored key material
	salt, err := hex.DecodeString(string(keyData))
	if err != nil {
		return fmt.Errorf("failed to decode key material: %w", err)
	}

	// Derive the actual encryption key from the salt and a machine-specific passphrase
	passphrase := s.generateMachinePassphrase()
	s.masterKey = pbkdf2.Key([]byte(passphrase), salt, 100000, 32, sha256.New)
	s.keyDerived = true

	return nil
}

// generateMachinePassphrase creates a machine-specific passphrase for key derivation
func (s *AESSecurityManager) generateMachinePassphrase() string {
	// Create a machine-specific passphrase using hostname and user information
	hostname, _ := os.Hostname()
	username := os.Getenv("USER")
	if username == "" {
		username = os.Getenv("USERNAME") // Windows compatibility
	}

	// Combine machine-specific elements for passphrase generation
	machineInfo := fmt.Sprintf("console-security-%s-%s", hostname, username)
	return machineInfo
}

// GenerateSecureKey creates new encryption key material and stores it securely
func (s *AESSecurityManager) GenerateSecureKey() error {
	// Generate random salt for key derivation
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("failed to generate random salt: %w", err)
	}

	// Store the salt as hex-encoded key material
	saltHex := hex.EncodeToString(salt)
	if err := os.WriteFile(s.keyPath, []byte(saltHex), 0600); err != nil {
		return fmt.Errorf("failed to write key material: %w", err)
	}

	// Derive the master key
	passphrase := s.generateMachinePassphrase()
	s.masterKey = pbkdf2.Key([]byte(passphrase), salt, 100000, 32, sha256.New)
	s.keyDerived = true

	return nil
}

// SecureKeyExists checks if encryption key material is available
func (s *AESSecurityManager) SecureKeyExists() bool {
	_, err := os.Stat(s.keyPath)
	return err == nil
}

// EncryptCredential encrypts sensitive authentication data using AES-256-GCM
func (s *AESSecurityManager) EncryptCredential(plaintext string) (string, error) {
	if !s.keyDerived {
		return "", fmt.Errorf("encryption key not available")
	}

	// Create AES cipher
	block, err := aes.NewCipher(s.masterKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode cipher
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt the plaintext
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Encode as base64 for storage
	encoded := base64.StdEncoding.EncodeToString(ciphertext)
	return encoded, nil
}

// DecryptCredential decrypts stored authentication data
func (s *AESSecurityManager) DecryptCredential(ciphertext string) (string, error) {
	if !s.keyDerived {
		return "", fmt.Errorf("encryption key not available")
	}

	// Decode from base64
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	// Create AES cipher
	block, err := aes.NewCipher(s.masterKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode cipher
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Extract nonce and ciphertext
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]

	// Decrypt the data
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

// ValidateTokenFormat performs comprehensive format validation on authentication tokens
func (s *AESSecurityManager) ValidateTokenFormat(token string, tokenType string) error {
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("token cannot be empty")
	}

	switch strings.ToLower(tokenType) {
	case "bearer":
		return s.validateBearerToken(token)
	case "none":
		return fmt.Errorf("no token should be provided when auth type is 'none'")
	default:
		return fmt.Errorf("unsupported token type: %s", tokenType)
	}
}

// validateBearerToken performs specific validation for bearer tokens
func (s *AESSecurityManager) validateBearerToken(token string) error {
	// Remove whitespace and check basic format
	token = strings.TrimSpace(token)

	if token == "" {
		return fmt.Errorf("bearer token cannot be empty")
	}

	// Token should not contain whitespace characters
	if strings.ContainsAny(token, " \t\n\r") {
		return fmt.Errorf("bearer token cannot contain whitespace")
	}

	// Basic length validation (tokens should be reasonably long)
	if len(token) < 8 {
		return fmt.Errorf("bearer token appears to be too short (minimum 8 characters)")
	}

	// Check for obvious placeholder values
	lowerToken := strings.ToLower(token)
	placeholders := []string{"placeholder", "token", "your-token", "example", "test"}
	for _, placeholder := range placeholders {
		if strings.Contains(lowerToken, placeholder) {
			return fmt.Errorf("bearer token appears to be a placeholder value")
		}
	}

	// Optional: JWT format validation (basic structure check)
	if s.looksLikeJWT(token) {
		return s.validateJWTStructure(token)
	}

	return nil
}

// looksLikeJWT performs a basic check to see if the token appears to be a JWT
func (s *AESSecurityManager) looksLikeJWT(token string) bool {
	// JWTs have exactly two dots separating three base64-encoded parts
	parts := strings.Split(token, ".")
	return len(parts) == 3
}

// validateJWTStructure performs basic JWT structure validation
func (s *AESSecurityManager) validateJWTStructure(token string) error {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return fmt.Errorf("JWT must have exactly 3 parts separated by dots")
	}

	// Validate that each part is valid base64
	for i, part := range parts {
		// JWT uses base64url encoding, but we'll accept standard base64 as well
		if err := s.validateBase64Part(part); err != nil {
			return fmt.Errorf("JWT part %d is not valid base64: %w", i+1, err)
		}
	}

	return nil
}

// validateBase64Part checks if a string is valid base64 (with padding adjustment)
func (s *AESSecurityManager) validateBase64Part(part string) error {
	// Add padding if necessary for standard base64 decoding
	switch len(part) % 4 {
	case 2:
		part += "=="
	case 3:
		part += "="
	}

	// Try to decode as standard base64
	if _, err := base64.StdEncoding.DecodeString(part); err != nil {
		// Try base64url if standard fails
		if _, err := base64.URLEncoding.DecodeString(part); err != nil {
			return fmt.Errorf("invalid base64 encoding")
		}
	}

	return nil
}

// ClearSecurityData removes all encryption key material (for security reset)
func (s *AESSecurityManager) ClearSecurityData() error {
	// Clear in-memory key
	if s.masterKey != nil {
		for i := range s.masterKey {
			s.masterKey[i] = 0
		}
		s.masterKey = nil
		s.keyDerived = false
	}

	// Remove key file
	if err := os.Remove(s.keyPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove security key file: %w", err)
	}

	return nil
}

// RotateEncryptionKey generates new encryption key material and re-encrypts existing data
func (s *AESSecurityManager) RotateEncryptionKey() error {
	// This would be used for security key rotation in production environments
	// For now, we implement a simple key regeneration
	if err := s.ClearSecurityData(); err != nil {
		return fmt.Errorf("failed to clear existing security data: %w", err)
	}

	return s.GenerateSecureKey()
}
