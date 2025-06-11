## Universal Application Console Project Structure

The completed Universal Application Console implementation will follow standard Go project conventions with clear separation of concerns across functional domains. The project structure supports both development efficiency and long-term maintainability through logical organization of components.

### Root Directory Structure

The project root contains essential configuration and documentation files that establish the development environment and project metadata. The `go.mod` file defines the module declaration and all external dependencies including the Bubble Tea framework, Lipgloss styling library, Bubbles components, and Chroma syntax highlighting. The `go.sum` file maintains cryptographic checksums for dependency verification and reproducible builds.

Documentation files include `README.md` providing project overview, installation instructions, and usage examples, along with `LICENSE` containing the project licensing terms. The `.gitignore` file excludes generated binaries, temporary files, and system-specific artifacts from version control.

### Command Entry Point

The `cmd/console/` directory contains the application entry point following Go convention for executable commands. The `main.go` file within this directory handles command-line argument parsing, dependency injection setup, and application bootstrapping. This structure enables easy extension with additional command-line utilities while maintaining clear separation between application logic and executable creation.

### Internal Package Organization

The `internal/` directory contains all application-specific code that should not be imported by external packages. This organization enforces proper encapsulation and prevents inadvertent dependencies on internal implementation details.

### Interface Definitions and Contracts

The `internal/interfaces/` directory establishes the dependency injection architecture through comprehensive interface definitions. The `interfaces.go` file contains all core interfaces including ConfigManager, ProtocolClient, ContentRenderer, RegistryManager, and AuthManager. These interfaces enable comprehensive testing and modular component replacement throughout the application.

### Configuration Management

The `internal/config/` directory manages all configuration-related functionality including profile management, theme configuration, and authentication credential handling. The `config.go` file implements the ConfigManager interface with YAML-based configuration loading and validation. The `security.go` file provides secure storage mechanisms for sensitive authentication credentials with appropriate encryption and file permission management.

### Protocol Communication

The `internal/protocol/` directory handles all HTTP communication with Compliant Applications following the specified protocol requirements. The `client.go` file implements the ProtocolClient interface with proper timeout configuration and authentication header management. The `types.go` file defines all request and response structures that correspond exactly to the JSON specifications. The `endpoints.go` file implements each protocol endpoint as discrete functions with comprehensive error handling and response parsing.

### Authentication and Security

The `internal/auth/` directory manages authentication protocols and security credential handling. The `manager.go` file implements the AuthManager interface for bearer token management, secure credential storage, and authentication header construction for all HTTP requests.

### Content Rendering System

The `internal/content/` directory provides sophisticated content rendering capabilities for transforming structured protocol responses into visual presentations. The `renderer.go` file implements the ContentRenderer interface supporting all specified content types including text with status indicators, collapsible sections, tables, code blocks with syntax highlighting, and progress indicators. The `types.go` file defines content structures that match the protocol specification. The `collapsible.go` file implements progressive disclosure mechanisms with keyboard navigation support.

### Application Registry and Health Monitoring

The `internal/registry/` directory manages application registration and connectivity monitoring. The `manager.go` file implements the RegistryManager interface for maintaining the persistent registry of applications and performing application metadata management. The `health.go` file provides real-time health monitoring through periodic connectivity checks and status indicator updates.

### User Interface Components

The `internal/ui/` directory organizes all user interface components following the dual-mode architecture specified in the design. The `menu/` subdirectory contains Console Menu Mode implementation with `model.go` defining the menu state structure, `update.go` implementing Bubble Tea update logic for keyboard input processing, and `view.go` creating the visual presentation with Lipgloss styling.

The `app/` subdirectory contains Application Mode implementation with parallel file organization. The `model.go` file defines the application interaction state including command history and workflow context. The `update.go` file processes command input submission and navigation. The `view.go` file renders the sophisticated interface layout demonstrated in the protocol examples.

The `actions/` subdirectory implements the Actions Pane system through `pane.go`, providing numbered action lists with appropriate visual themes for different action types. The `workflow/` subdirectory contains `manager.go` for multi-step operation context management and workflow state preservation.

The `components/` subdirectory provides shared interface elements including `status.go` for status indicators and progress displays, and `errors.go` for error-specific interface components with expandable detail sections and recovery action panes.

### Error Handling and Recovery

The `internal/errors/` directory implements comprehensive error management and recovery mechanisms. The `handler.go` file processes structured error responses and creates visual error presentations with actionable recovery options. The `recovery.go` file provides error recovery workflows and maintains error context for diagnostic purposes.

### Main Application Controller

The `internal/app/` directory contains the primary application orchestration logic. The `console.go` file serves as the main application controller that manages mode switching, coordinates communication between components, and maintains overall application state through dependency injection of all required interfaces.

### Testing Infrastructure

The `internal/test/` directory provides comprehensive testing capabilities including `integration_test.go` for validating complete user workflows through interface mocking and dependency injection. Additional test files can be co-located with their respective implementation files following Go testing conventions.

### Complete Project Structure

```
universal-application-console/
├── go.mod
├── go.sum
├── README.md
├── LICENSE
├── .gitignore
├── cmd/
│   └── console/
│       └── main.go
└── internal/
    ├── interfaces/
    │   └── interfaces.go
    ├── config/
    │   ├── config.go
    │   └── security.go
    ├── protocol/
    │   ├── client.go
    │   ├── types.go
    │   └── endpoints.go
    ├── auth/
    │   └── manager.go
    ├── content/
    │   ├── renderer.go
    │   ├── types.go
    │   └── collapsible.go
    ├── registry/
    │   ├── manager.go
    │   └── health.go
    ├── ui/
    │   ├── menu/
    │   │   ├── model.go
    │   │   ├── update.go
    │   │   └── view.go
    │   ├── app/
    │   │   ├── model.go
    │   │   ├── update.go
    │   │   └── view.go
    │   ├── actions/
    │   │   └── pane.go
    │   ├── workflow/
    │   │   └── manager.go
    │   └── components/
    │       ├── status.go
    │       └── errors.go
    ├── errors/
    │   ├── handler.go
    │   └── recovery.go
    ├── app/
    │   └── console.go
    └── test/
        └── integration_test.go
```