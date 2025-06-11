## Revised Step-by-Step Implementation Plan for Universal Application Console

### Phase 1: Project Foundation and Interface Definitions

Initialize the project directory structure and establish the Go module. Create `go.mod` with module declaration and dependency specifications for github.com/charmbracelet/bubbletea, github.com/charmbracelet/lipgloss, github.com/charmbracelet/bubbles, and github.com/alecthomas/chroma. This establishes the foundation for the entire project and ensures proper dependency management as specified in the design document.

Create `internal/interfaces/interfaces.go` defining all core interfaces required for dependency injection and testability. This file establishes the ConfigManager interface for profile and authentication management, the ProtocolClient interface for HTTP communication with Compliant Applications, the ContentRenderer interface for structured content processing, the RegistryManager interface for application registration and health monitoring, and the AuthManager interface for security credential handling. These interfaces enable comprehensive testing and modular component replacement throughout the application architecture.

Develop `cmd/console/main.go` as the application entry point implementing command-line argument parsing for host specification, profile selection, help display, and version information as outlined in section 3.5 of the design specification. The main function determines whether to launch Console Menu Mode or directly connect to a specified application, implementing the dual-mode operation described in section 3.2.1. This file instantiates concrete implementations of all interfaces and passes them to the main console controller through dependency injection.

### Phase 2: Configuration Management Implementation

Implement `internal/config/config.go` to handle all configuration management requirements from section 3.5. This file defines the Profile, Theme, and Config structures that match the YAML specification exactly, implements secure loading and saving of configuration files to the specified location, and provides validation functions for profile completeness and authentication credential verification. The implementation conforms to the ConfigManager interface to ensure testability and modular design.

Create `internal/config/security.go` providing secure configuration storage mechanisms that implement encryption for sensitive authentication credentials, validate token formats, and establish secure file permissions for configuration storage. The security implementation protects authentication credentials while maintaining usability for profile management operations, supporting the authentication protocol specified in section 3.7.1.

### Phase 3: Protocol Communication Layer

Develop `internal/protocol/client.go` to manage all HTTP communication with Compliant Applications through the ProtocolClient interface. This file creates the HTTP client with proper timeout configurations, implements authentication header management using bearer tokens as specified in section 3.7.1, and provides connection establishment flow including the GET /console/spec handshake described in section 4.1. The implementation ensures proper dependency injection for configuration and authentication managers.

Create `internal/protocol/types.go` containing all request and response structures that match the JSON specifications in section 4. Define the SpecResponse, CommandRequest, CommandResponse, ActionRequest, SuggestRequest, SuggestResponse, ProgressRequest, ProgressResponse, CancelRequest, and ErrorResponse structures with proper JSON tags that correspond exactly to the protocol specification.

Implement `internal/protocol/endpoints.go` containing the concrete implementation of protocol endpoint methods. Create ExecuteCommand for POST /console/command, ExecuteAction for POST /console/action, GetSuggestions for POST /console/suggest, GetProgress for POST /console/progress, and CancelOperation for POST /console/cancel. Each function handles request construction, HTTP communication, and response parsing according to the specifications in sections 4.2 through 4.6 while maintaining interface compliance for testability.

### Phase 4: Authentication and Security Implementation

Create `internal/auth/manager.go` implementing the AuthManager interface for authentication protocol specified in section 3.7.1. This file handles bearer token management, secure credential storage, and authentication header construction for all HTTP requests. The authentication manager ensures proper security protocol implementation while maintaining session state across application interactions through dependency injection of the configuration manager.

### Phase 5: Rich Content Rendering System

Develop `internal/content/renderer.go` as the concrete implementation of the ContentRenderer interface specified in section 3.3. This file transforms structured content responses into Lipgloss-styled components, supporting all content types including text with status indicators, collapsible sections, tables, code blocks with syntax highlighting, progress indicators, and lists. The renderer creates the sophisticated visual presentations described in the enhanced protocol examples while maintaining interface compliance for testing.

Implement `internal/content/types.go` defining content type structures that match the structured response format in section 4.2.1. Create ContentBlock, TextContent, CollapsibleContent, TableContent, CodeContent, ProgressContent, and ListContent structures that correspond to the JSON content specifications, enabling proper parsing and rendering of rich application responses.

Create `internal/content/collapsible.go` implementing the progressive disclosure mechanisms described in section 3.3.1. This file creates collapsible section components with expand and collapse functionality, keyboard navigation support through Space key activation, and visual indicators showing section state. The implementation enables the hierarchical content organization demonstrated in the Pokemon battle examples.

### Phase 6: Application Registration and Health Monitoring

Implement `internal/registry/manager.go` providing the concrete implementation of the RegistryManager interface described in section 3.2.4. This file maintains the persistent registry of applications, performs health status monitoring through periodic connectivity checks, and provides application metadata management. The registry manager enables the centralized application launch capabilities that define Console Menu Mode operation through dependency injection of configuration and protocol managers.

Create `internal/registry/health.go` providing real-time health monitoring for registered applications. This file performs periodic health checks, updates application status indicators, and provides diagnostic information for connection troubleshooting. The health monitoring system ensures accurate status representation in the Console Menu interface while working through the established interfaces.

### Phase 7: Console Menu Mode Implementation

Create `internal/ui/menu/model.go` implementing the Console Menu Mode interface specified in section 3.2.1. This file defines the MenuModel structure containing registered applications list, connection status indicators, quick connect input field, and focus management state. The model receives all necessary dependencies through constructor injection, including registry manager, configuration manager, and protocol client interfaces.

Implement `internal/ui/menu/update.go` containing the Bubble Tea update function for Menu Mode. This file processes keyboard input for numbered application selection, Tab navigation between interface sections, Enter key for connection initiation, and meta commands for application registration and profile management. The update logic implements the focus navigation model described in section 3.2.5 while utilizing injected dependencies for all external operations.

Develop `internal/ui/menu/view.go` creating the visual presentation for Console Menu Mode. This file renders the registered applications list with health status indicators, quick connect interface, and command options using Lipgloss styling. The view implementation creates the bordered, organized interface layout shown in the Console Menu Mode example.

### Phase 8: Application Mode Implementation

Create `internal/ui/app/model.go` implementing the Application Mode interface specified in section 3.2.2. This file defines the AppModel structure containing command history, current response content, actions pane state, workflow context, and focus management for all interactive elements. The model maintains the conversational flow and rich content presentation described in the design specification through dependency injection of content renderer, protocol client, and workflow manager interfaces.

Implement `internal/ui/app/update.go` containing the Bubble Tea update function for Application Mode. This file processes command input submission, numbered action selection, Tab navigation through focusable elements, Space key for collapsible section expansion, and meta command handling for application disconnection. The update logic implements the sophisticated keyboard navigation patterns specified in section 3.2.5 while utilizing injected dependencies for protocol communication and content processing.

Develop `internal/ui/app/view.go` creating the visual presentation for Application Mode. This file renders the application header with connection status, scrolling history pane with rich content, actions pane with appropriate styling for different action types, and input component with suggestion dropdown. The view implementation creates the sophisticated interface layout demonstrated in the Pokemon battle examples.

### Phase 9: Actions and Workflow Management

Create `internal/ui/actions/pane.go` implementing the Actions Pane system described in section 3.2.1. This file creates numbered action lists with different visual themes for standard actions, confirmations, and error recovery options. The implementation supports both direct number key execution and focused navigation with Enter key activation, providing the dual interaction methods specified in the design.

Implement `internal/ui/workflow/manager.go` handling multi-step operation context as specified in section 3.4.1. This file maintains workflow state across command cycles, displays breadcrumb navigation showing current step progress, and provides cancellation mechanisms for long-running operations. The workflow manager preserves operation context during connection interruptions and interface state changes through proper interface design.

Develop `internal/ui/components/status.go` creating status indicators and progress displays described in section 3.2.1. This file implements visual markers for pending, success, error, and warning states using appropriate icons and colors. The status system provides the real-time feedback mechanisms that enhance user confidence during complex operations.

### Phase 10: Error Handling and Recovery Systems

Create `internal/errors/handler.go` implementing comprehensive error management specified in section 4.7. This file processes structured error responses from applications, creates visual error presentations with distinct styling, and generates recovery action suggestions. The error handler transforms protocol error responses into user-friendly interface presentations with actionable recovery options while maintaining interface compliance for testing.

Implement `internal/errors/recovery.go` providing error recovery mechanisms described in the enhanced error response specification. This file creates recovery action menus, maintains error context for diagnostic purposes, and guides users through error resolution workflows. The recovery system ensures graceful degradation and clear resolution paths for all error scenarios.

Develop `internal/ui/components/errors.go` creating error-specific interface components. This file renders error messages with appropriate visual styling, expandable error detail sections, and recovery action panes with specialized formatting. The error interface components provide the sophisticated error presentation demonstrated in the error handling sequence diagrams.

### Phase 11: Main Application Controller and Integration

Create `internal/app/console.go` as the main application controller that orchestrates all components and manages the complete application lifecycle. This file handles mode switching between Console Menu and Application modes, coordinates communication between UI components and protocol handlers, maintains overall application state, and manages the interaction between different subsystems including configuration management, protocol communication, content rendering, and user interface components. The console controller receives all dependencies through constructor injection, ensuring proper separation of concerns and comprehensive testability.

The console controller implements the coordination responsibilities for data flow and state synchronization across all application components, integration of all implemented components into the cohesive user experience described in the design specification, and management of the complete user workflow from application startup through connection establishment, command execution, and graceful shutdown. This consolidated approach clarifies the top-level control flow while maintaining modular design principles.

Develop comprehensive integration testing in `internal/test/integration_test.go` validating complete user workflows through interface mocking and dependency injection. This file creates test scenarios for connection establishment, command execution, error handling, and mode switching operations. The integration tests verify that all components work together to deliver the sophisticated user experience specified in the design document while ensuring comprehensive test coverage through the established interface architecture.

This revised implementation plan provides precise file creation sequence and responsibility allocation with explicit interface requirements for dependency injection, ensuring comprehensive testability while maintaining clear architectural organization. The consolidation of coordination responsibilities into the main console controller clarifies the top-level control flow while preserving the modular design principles necessary for professional software development.