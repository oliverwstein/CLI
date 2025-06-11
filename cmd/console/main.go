// Package main implements the Universal Application Console entry point.
// This file handles command-line argument parsing, dependency injection,
// and mode selection between Console Menu Mode and direct application connection.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/universal-console/console/internal/interfaces"
)

// Version information for the Universal Application Console
const (
	Version     = "2.0.0"
	ProgramName = "Universal Application Console"
)

// CommandLineArgs represents parsed command-line arguments
type CommandLineArgs struct {
	Host        string
	Profile     string
	Theme       string
	ShowHelp    bool
	ShowVersion bool
}

// ConsoleApp represents the main application with all injected dependencies
type ConsoleApp struct {
	configManager   interfaces.ConfigManager
	protocolClient  interfaces.ProtocolClient
	contentRenderer interfaces.ContentRenderer
	registryManager interfaces.RegistryManager
	authManager     interfaces.AuthManager
	args            CommandLineArgs
}

// parseCommandLineArgs processes command-line arguments according to the specification
func parseCommandLineArgs() CommandLineArgs {
	var args CommandLineArgs

	flag.StringVar(&args.Host, "host", "", "Host and port of the Application to connect to (e.g., localhost:8080)")
	flag.StringVar(&args.Profile, "profile", "", "Profile name from configuration file to use for connection")
	flag.StringVar(&args.Theme, "theme", "", "Visual theme name for syntax highlighting and UI elements")
	flag.BoolVar(&args.ShowHelp, "help", false, "Display usage information and exit")
	flag.BoolVar(&args.ShowVersion, "version", false, "Display version information and exit")

	// Custom usage function to match the design specification
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "%s v%s\n\n", ProgramName, Version)
		fmt.Fprintf(os.Stderr, "A universal, rich terminal-based user interface for interacting with\n")
		fmt.Fprintf(os.Stderr, "any backend application that implements the Compliance Protocol v2.0.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s                           # Launch Console Menu Mode with default profile\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --host localhost:8080     # Connect directly to specified host\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --profile pokemon         # Connect using 'pokemon' profile\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --theme monokai           # Use monokai color theme\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nConfiguration file location: ~/.config/console/profiles.yaml\n")
	}

	flag.Parse()

	return args
}

// validateArguments ensures command-line arguments are valid and compatible
func validateArguments(args CommandLineArgs) error {
	// Cannot specify both host and profile simultaneously
	if args.Host != "" && args.Profile != "" {
		return fmt.Errorf("cannot specify both --host and --profile options simultaneously")
	}

	// Validate host format if provided
	if args.Host != "" {
		if !strings.Contains(args.Host, ":") {
			return fmt.Errorf("host must include port (e.g., localhost:8080)")
		}
	}

	return nil
}

// determineProfile resolves which profile to use based on command-line arguments
func (app *ConsoleApp) determineProfile() (*interfaces.Profile, error) {
	// If host is explicitly specified, create a temporary profile
	if app.args.Host != "" {
		profile := &interfaces.Profile{
			Name:          "temporary",
			Host:          app.args.Host,
			Theme:         app.args.Theme,
			Confirmations: true,
			Auth: interfaces.AuthConfig{
				Type: "none",
			},
		}

		// Apply theme override if specified
		if app.args.Theme != "" {
			profile.Theme = app.args.Theme
		}

		return profile, nil
	}

	// Use specified profile or default to "default"
	profileName := app.args.Profile
	if profileName == "" {
		profileName = "default"
	}

	profile, err := app.configManager.LoadProfile(profileName)
	if err != nil {
		return nil, fmt.Errorf("failed to load profile '%s': %w", profileName, err)
	}

	// Apply theme override if specified
	if app.args.Theme != "" {
		profile.Theme = app.args.Theme
	}

	return profile, nil
}

// shouldLaunchDirectConnection determines if the application should connect directly
// to an application instead of showing the Console Menu
func (app *ConsoleApp) shouldLaunchDirectConnection() bool {
	return app.args.Host != "" || app.args.Profile != ""
}

// createBubbleTeaProgram instantiates the appropriate Bubble Tea model based on mode
func (app *ConsoleApp) createBubbleTeaProgram() (*tea.Program, error) {
	if app.shouldLaunchDirectConnection() {
		// Direct connection mode - launch Application Mode immediately
		profile, err := app.determineProfile()
		if err != nil {
			return nil, fmt.Errorf("failed to determine connection profile: %w", err)
		}

		// TODO: Create Application Mode model with injected dependencies
		// This will be implemented in Phase 8
		model := createApplicationModel(
			profile,
			app.protocolClient,
			app.contentRenderer,
			app.configManager,
			app.authManager,
		)

		return tea.NewProgram(model, tea.WithAltScreen()), nil
	} else {
		// Console Menu Mode - show application registry and connection management
		// TODO: Create Console Menu Mode model with injected dependencies
		// This will be implemented in Phase 7
		model := createMenuModel(
			app.registryManager,
			app.configManager,
			app.protocolClient,
			app.contentRenderer,
			app.authManager,
		)

		return tea.NewProgram(model, tea.WithAltScreen()), nil
	}
}

// createConcreteImplementations instantiates all concrete implementations of interfaces
// This function will be expanded as concrete implementations are developed in subsequent phases
func createConcreteImplementations() (
	interfaces.ConfigManager,
	interfaces.ProtocolClient,
	interfaces.ContentRenderer,
	interfaces.RegistryManager,
	interfaces.AuthManager,
	error,
) {
	// TODO: Implement concrete implementations in subsequent phases
	// For now, return placeholder implementations that will be replaced

	// Phase 2: Configuration Management Implementation
	// configManager := config.NewManager()

	// Phase 3: Protocol Communication Layer
	// protocolClient := protocol.NewClient()

	// Phase 4: Authentication and Security Implementation
	// authManager := auth.NewManager()

	// Phase 5: Rich Content Rendering System
	// contentRenderer := content.NewRenderer()

	// Phase 6: Application Registration and Health Monitoring
	// registryManager := registry.NewManager()

	return nil, nil, nil, nil, nil, fmt.Errorf("concrete implementations not yet available - will be implemented in subsequent phases")
}

// Placeholder model creation functions for future implementation
// These will be properly implemented in Phases 7 and 8

func createApplicationModel(
	profile *interfaces.Profile,
	client interfaces.ProtocolClient,
	renderer interfaces.ContentRenderer,
	config interfaces.ConfigManager,
	auth interfaces.AuthManager,
) tea.Model {
	// TODO: Implement in Phase 8 - Application Mode Implementation
	panic("createApplicationModel not yet implemented")
}

func createMenuModel(
	registry interfaces.RegistryManager,
	config interfaces.ConfigManager,
	client interfaces.ProtocolClient,
	renderer interfaces.ContentRenderer,
	auth interfaces.AuthManager,
) tea.Model {
	// TODO: Implement in Phase 7 - Console Menu Mode Implementation
	panic("createMenuModel not yet implemented")
}

// main implements the application entry point with comprehensive argument handling
// and dependency injection as specified in the design document
func main() {
	// Parse and validate command-line arguments
	args := parseCommandLineArgs()

	// Handle help and version requests immediately
	if args.ShowHelp {
		flag.Usage()
		os.Exit(0)
	}

	if args.ShowVersion {
		fmt.Printf("%s v%s\n", ProgramName, Version)
		fmt.Printf("Protocol Version: 2.0\n")
		fmt.Printf("Built with Go and Charm libraries\n")
		os.Exit(0)
	}

	// Validate argument compatibility
	if err := validateArguments(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		flag.Usage()
		os.Exit(1)
	}

	// Instantiate concrete implementations of all interfaces
	configManager, protocolClient, contentRenderer, registryManager, authManager, err := createConcreteImplementations()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing application components: %v\n", err)
		fmt.Fprintf(os.Stderr, "Note: This error is expected during Phase 1 implementation.\n")
		fmt.Fprintf(os.Stderr, "Concrete implementations will be available in subsequent phases.\n")
		os.Exit(1)
	}

	// Create the main application instance with dependency injection
	app := &ConsoleApp{
		configManager:   configManager,
		protocolClient:  protocolClient,
		contentRenderer: contentRenderer,
		registryManager: registryManager,
		authManager:     authManager,
		args:            args,
	}

	// Create and start the appropriate Bubble Tea program
	program, err := app.createBubbleTeaProgram()
	if err != nil {
		log.Fatalf("Failed to create application interface: %v", err)
	}

	// Start the TUI application
	if _, err := program.Run(); err != nil {
		log.Fatalf("Application terminated with error: %v", err)
	}

	// Graceful shutdown
	fmt.Println("Universal Application Console terminated successfully.")
}
