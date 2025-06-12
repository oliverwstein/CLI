# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The Universal Application Console is a sophisticated TUI (Terminal User Interface) application built in Go that provides a standardized interface for interacting with any backend application implementing the Compliance Protocol v2.0. It uses the Bubble Tea framework for TUI functionality with rich content rendering and workflow management.

## Build and Development Commands

```bash
# Build the main console application
go build -o console ./cmd/console

# Build the mock server for testing
go build -o mock_server ./mock_server.go

# Run tests (currently no test files exist)
go test ./...

# Check code formatting
gofmt -l .

# Apply code formatting
gofmt -w .

# Run static analysis
go vet ./...

# Update dependencies
go mod tidy
```

## Debugging and Observability

The application includes comprehensive observability features for debugging and monitoring:

### Debug Logging
Enable detailed debug logging by setting the CONSOLE_DEBUG environment variable:
```bash
# Enable debug logging with JSON format
CONSOLE_DEBUG=true ./console --host localhost:8080

# Normal operation (INFO level, text format)
./console --host localhost:8080
```

### Logging Features
- **Structured Logging**: JSON formatted logs with contextual fields for easy parsing
- **Component-based Logging**: Each major component (protocol, config, auth) has dedicated loggers
- **Operation Tracking**: Connection attempts, configuration loading, and HTTP requests are fully logged
- **Error Context**: Enhanced error messages with recovery suggestions and diagnostic context
- **Performance Metrics**: Request timing, response sizes, and operation durations

### Log Components
- `protocol`: HTTP communication, connection establishment, request/response cycles
- `config`: Configuration loading, validation, credential decryption
- `auth`: Authentication operations and credential management
- `ui`: User interface state changes and interactions
- `registry`: Application registration and health monitoring
- `content`: Content rendering and processing

### Connection Debugging
The logging system provides detailed visibility into connection issues:
- DNS resolution and TCP connection establishment
- HTTP request construction and execution timing
- Protocol handshake validation and feature negotiation
- Authentication header construction and validation
- Detailed error context with specific recovery suggestions

### Mock Server Testing
The mock server includes comprehensive logging for development:
```bash
# Start mock server with logging
./mock_server > mock_server.log 2>&1 &

# Test connection with detailed logging
CONSOLE_DEBUG=true ./console --host localhost:8080
```

## Architecture Overview

The application follows a clean architecture pattern with dependency injection:

### Core Components
- **Main Controller** (`internal/app/console.go`): Orchestrates mode switching between Console Menu and Application modes
- **Protocol Client** (`internal/protocol/`): Handles HTTP communication with Compliant Applications via REST API
- **Content Renderer** (`internal/content/`): Transforms structured responses into rich TUI presentations
- **Registry Manager** (`internal/registry/`): Manages application registration and health monitoring
- **Configuration Manager** (`internal/config/`): Handles profile and theme management with YAML configuration

### UI Architecture
Two distinct operating modes:
1. **Console Menu Mode** (`internal/ui/menu/`): Application selection and registry management
2. **Application Mode** (`internal/ui/app/`): Interactive command execution with rich content display

### Interface-Driven Design
All major components implement interfaces defined in `internal/interfaces/interfaces.go` enabling:
- Comprehensive dependency injection
- Easy mocking for testing
- Modular component replacement

## Protocol Implementation

The application implements the Compliance Protocol v2.0 with these endpoints:
- `GET /console/spec` - Handshake and feature negotiation
- `POST /console/command` - Command execution with rich responses
- `POST /console/action` - Action execution from Actions Pane
- `POST /console/suggest` - Command suggestions and completion
- `POST /console/progress` - Long-running operation progress
- `POST /console/cancel` - Operation cancellation

## Configuration

- **Config location**: `~/.config/console/profiles.yaml`
- **Format**: YAML with profiles, themes, and registered applications
- **Authentication**: Bearer token support with secure credential storage
- **Themes**: Customizable color schemes for syntax highlighting and UI elements

## Key Design Patterns

### Dependency Injection
All major components receive their dependencies through constructor injection rather than global state, enabling testability and modularity.

### Progressive Disclosure
Content rendering supports collapsible sections, structured tables, and expandable error details to manage information complexity.

### Workflow Management
Multi-step operations maintain context across command cycles with breadcrumb navigation and state preservation.

### Error Recovery
Structured error responses include recovery actions and detailed diagnostic information with specialized visual presentation.

## Development Notes

- Use `gofmt` to maintain consistent code formatting
- Follow Go naming conventions and add proper documentation comments
- The mock server (`mock_server.go`) provides a test endpoint for development
- All interfaces should be implemented by concrete types in their respective packages
- Rich content support includes syntax highlighting via Chroma, tables, progress indicators, and collapsible sections