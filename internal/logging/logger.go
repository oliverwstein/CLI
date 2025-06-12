// Package logging provides structured logging capabilities for the Universal Application Console.
// It implements a centralized logging strategy with configurable log levels and output formats.
package logging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
)

// LogLevel represents the severity of log messages
type LogLevel int

const (
	DebugLevel LogLevel = iota
	InfoLevel
	WarnLevel
	ErrorLevel
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Logger provides structured logging with context support
type Logger struct {
	logger    *slog.Logger
	level     LogLevel
	component string
}

// Config represents logging configuration
type Config struct {
	Level     LogLevel
	Format    string // "json" or "text"
	Output    string // "stdout", "stderr", or file path
	Component string
}

// DefaultConfig returns a sensible default logging configuration
func DefaultConfig() Config {
	return Config{
		Level:     InfoLevel,
		Format:    "text",
		Output:    "stdout",
		Component: "console",
	}
}

// NewLogger creates a new logger with the specified configuration
func NewLogger(config Config) (*Logger, error) {
	var handler slog.Handler
	
	// Determine output destination
	var output *os.File
	switch config.Output {
	case "stdout", "":
		output = os.Stdout
	case "stderr":
		output = os.Stderr
	default:
		// File output
		file, err := os.OpenFile(config.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file %s: %w", config.Output, err)
		}
		output = file
	}

	// Create appropriate handler based on format
	opts := &slog.HandlerOptions{
		Level: slogLevel(config.Level),
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Redact sensitive information
			if a.Key == "token" || strings.Contains(strings.ToLower(a.Key), "password") {
				return slog.String(a.Key, "[REDACTED]")
			}
			return a
		},
	}

	switch config.Format {
	case "json":
		handler = slog.NewJSONHandler(output, opts)
	default:
		handler = slog.NewTextHandler(output, opts)
	}

	logger := slog.New(handler)
	
	return &Logger{
		logger:    logger,
		level:     config.Level,
		component: config.Component,
	}, nil
}

// slogLevel converts our LogLevel to slog.Level
func slogLevel(level LogLevel) slog.Level {
	switch level {
	case DebugLevel:
		return slog.LevelDebug
	case InfoLevel:
		return slog.LevelInfo
	case WarnLevel:
		return slog.LevelWarn
	case ErrorLevel:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// WithContext creates a new logger with additional context
func (l *Logger) WithContext(ctx context.Context) *Logger {
	return &Logger{
		logger:    l.logger.With(slog.String("component", l.component)),
		level:     l.level,
		component: l.component,
	}
}

// WithComponent creates a new logger for a specific component
func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{
		logger:    l.logger.With(slog.String("component", component)),
		level:     l.level,
		component: component,
	}
}

// WithField adds a field to the logger context
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return &Logger{
		logger:    l.logger.With(slog.Any(key, value)),
		level:     l.level,
		component: l.component,
	}
}

// WithFields adds multiple fields to the logger context
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	args := make([]interface{}, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, v)
	}
	return &Logger{
		logger:    l.logger.With(args...),
		level:     l.level,
		component: l.component,
	}
}

// Debug logs a debug level message
func (l *Logger) Debug(msg string, args ...interface{}) {
	if l.level <= DebugLevel {
		l.logger.Debug(msg, args...)
	}
}

// Info logs an info level message
func (l *Logger) Info(msg string, args ...interface{}) {
	if l.level <= InfoLevel {
		l.logger.Info(msg, args...)
	}
}

// Warn logs a warning level message
func (l *Logger) Warn(msg string, args ...interface{}) {
	if l.level <= WarnLevel {
		l.logger.Warn(msg, args...)
	}
}

// Error logs an error level message
func (l *Logger) Error(msg string, args ...interface{}) {
	if l.level <= ErrorLevel {
		l.logger.Error(msg, args...)
	}
}

// LogOperation logs the start and end of an operation with duration
func (l *Logger) LogOperation(operation string, fn func() error) error {
	start := time.Now()
	opLogger := l.WithField("operation", operation)
	
	opLogger.Debug("Operation starting")
	
	err := fn()
	duration := time.Since(start)
	
	if err != nil {
		opLogger.Error("Operation failed",
			slog.Duration("duration", duration),
			slog.String("error", err.Error()))
		return err
	}
	
	opLogger.Info("Operation completed",
		slog.Duration("duration", duration))
	return nil
}

// LogConnectionAttempt logs connection attempt details
func (l *Logger) LogConnectionAttempt(host string, authType string) {
	l.Info("Attempting connection",
		slog.String("host", host),
		slog.String("auth_type", authType),
		slog.Time("timestamp", time.Now()))
}

// LogConnectionSuccess logs successful connection establishment
func (l *Logger) LogConnectionSuccess(host string, appName string, protocolVersion string, duration time.Duration) {
	l.Info("Connection established successfully",
		slog.String("host", host),
		slog.String("app_name", appName),
		slog.String("protocol_version", protocolVersion),
		slog.Duration("connection_duration", duration))
}

// LogConnectionFailure logs connection failure with detailed context
func (l *Logger) LogConnectionFailure(host string, err error, duration time.Duration) {
	l.Error("Connection failed",
		slog.String("host", host),
		slog.String("error", err.Error()),
		slog.Duration("attempt_duration", duration))
}

// LogConfigLoad logs configuration loading operations
func (l *Logger) LogConfigLoad(configPath string, profileName string) {
	l.Debug("Loading configuration",
		slog.String("config_path", configPath),
		slog.String("profile", profileName))
}

// LogConfigError logs configuration-related errors
func (l *Logger) LogConfigError(operation string, err error) {
	l.Error("Configuration error",
		slog.String("operation", operation),
		slog.String("error", err.Error()))
}

// LogAuthOperation logs authentication-related operations
func (l *Logger) LogAuthOperation(operation string, authType string) {
	l.Debug("Authentication operation",
		slog.String("operation", operation),
		slog.String("auth_type", authType))
}

// LogHTTPRequest logs HTTP request details (without sensitive data)
func (l *Logger) LogHTTPRequest(method string, url string, statusCode int, duration time.Duration) {
	l.Debug("HTTP request completed",
		slog.String("method", method),
		slog.String("url", url),
		slog.Int("status_code", statusCode),
		slog.Duration("duration", duration))
}

// LogUIStateChange logs user interface state transitions
func (l *Logger) LogUIStateChange(from string, to string, reason string) {
	l.Debug("UI state change",
		slog.String("from", from),
		slog.String("to", to),
		slog.String("reason", reason))
}

// LogHealthCheck logs application health check results
func (l *Logger) LogHealthCheck(appName string, status string, responseTime time.Duration, err error) {
	fields := []interface{}{
		slog.String("app_name", appName),
		slog.String("status", status),
		slog.Duration("response_time", responseTime),
	}
	
	if err != nil {
		fields = append(fields, slog.String("error", err.Error()))
		l.Warn("Health check failed", fields...)
	} else {
		l.Debug("Health check completed", fields...)
	}
}

// Global logger instance
var globalLogger *Logger

// InitGlobalLogger initializes the global logger with the specified configuration
func InitGlobalLogger(config Config) error {
	logger, err := NewLogger(config)
	if err != nil {
		return fmt.Errorf("failed to initialize global logger: %w", err)
	}
	globalLogger = logger
	return nil
}

// GetGlobalLogger returns the global logger instance
func GetGlobalLogger() *Logger {
	if globalLogger == nil {
		// Fallback to default configuration if not initialized
		globalLogger, _ = NewLogger(DefaultConfig())
	}
	return globalLogger
}

// Component-specific logger creators
func GetProtocolLogger() *Logger {
	return GetGlobalLogger().WithComponent("protocol")
}

func GetConfigLogger() *Logger {
	return GetGlobalLogger().WithComponent("config")
}

func GetAuthLogger() *Logger {
	return GetGlobalLogger().WithComponent("auth")
}

func GetUILogger() *Logger {
	return GetGlobalLogger().WithComponent("ui")
}

func GetRegistryLogger() *Logger {
	return GetGlobalLogger().WithComponent("registry")
}

func GetContentLogger() *Logger {
	return GetGlobalLogger().WithComponent("content")
}