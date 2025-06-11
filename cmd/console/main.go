// Package main implements the Universal Application Console entry point.
// This file handles command-line argument parsing, dependency injection,
// and mode selection between Console Menu Mode and direct application connection.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/universal-console/console/internal/app"
	"github.com/universal-console/console/internal/auth"
	"github.com/universal-console/console/internal/config"
	"github.com/universal-console/console/internal/content"
	"github.com/universal-console/console/internal/interfaces"
	"github.com/universal-console/console/internal/protocol"
	"github.com/universal-console/console/internal/registry"
	app_ui "github.com/universal-console/console/internal/ui/app"
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
func (ca *ConsoleApp) determineProfile() (*interfaces.Profile, error) {
	// If host is explicitly specified, create a temporary profile
	if ca.args.Host != "" {
		profile := &interfaces.Profile{
			Name:          "temporary",
			Host:          ca.args.Host,
			Theme:         ca.args.Theme,
			Confirmations: true,
			Auth: interfaces.AuthConfig{
				Type: "none",
			},
		}

		// Apply theme override if specified
		if ca.args.Theme != "" {
			profile.Theme = ca.args.Theme
		}

		return profile, nil
	}

	// Use specified profile or default to "default"
	profileName := ca.args.Profile
	if profileName == "" {
		profileName = "default"
	}

	profile, err := ca.configManager.LoadProfile(profileName)
	if err != nil {
		return nil, fmt.Errorf("failed to load profile '%s': %w", profileName, err)
	}

	// Apply theme override if specified
	if ca.args.Theme != "" {
		profile.Theme = ca.args.Theme
	}

	return profile, nil
}

// shouldLaunchDirectConnection determines if the application should connect directly
// to an application instead of showing the Console Menu
func (ca *ConsoleApp) shouldLaunchDirectConnection() bool {
	return ca.args.Host != "" || ca.args.Profile != ""
}

// createBubbleTeaProgram instantiates the appropriate Bubble Tea model based on mode
func (ca *ConsoleApp) createBubbleTeaProgram() (*tea.Program, error) {
	if ca.shouldLaunchDirectConnection() {
		// Direct connection mode - launch Application Mode immediately
		profile, err := ca.determineProfile()
		if err != nil {
			return nil, fmt.Errorf("failed to determine connection profile: %w", err)
		}

		// Connect immediately
		_, err = ca.protocolClient.Connect(context.Background(), profile.Host, &profile.Auth)
		if err != nil {
			// Log the error but continue, the app model will handle showing the error
			log.Printf("Direct connection failed: %v", err)
		}

		// Create the Application Mode model directly
		model := app_ui.NewAppModel(
			profile,
			ca.protocolClient,
			ca.contentRenderer,
			ca.configManager,
			ca.authManager,
		)

		return tea.NewProgram(model, tea.WithAltScreen()), nil
	} else {
		// Console Menu Mode - use the main controller
		model := app.NewConsoleController(
			ca.registryManager,
			ca.configManager,
			ca.protocolClient,
			ca.contentRenderer,
			ca.authManager,
		)

		return tea.NewProgram(model, tea.WithAltScreen()), nil
	}
}

// createConcreteImplementations instantiates all concrete implementations of interfaces
func createConcreteImplementations() (
	interfaces.ConfigManager,
	interfaces.ProtocolClient,
	interfaces.ContentRenderer,
	interfaces.RegistryManager,
	interfaces.AuthManager,
	error,
) {
	// Phase 2: Configuration Management Implementation
	configManager, err := config.NewManager()
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to init config manager: %w", err)
	}

	// Phase 4: Authentication and Security Implementation
	authManager, err := auth.NewManager(configManager)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to init auth manager: %w", err)
	}

	// Phase 3: Protocol Communication Layer
	protocolClient, err := protocol.NewClient(configManager, authManager)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to init protocol client: %w", err)
	}

	// Phase 5: Rich Content Rendering System
	contentRenderer, err := content.NewRenderer()
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to init content renderer: %w", err)
	}

	// Phase 6: Application Registration and Health Monitoring
	registryManager, err := registry.NewManager(configManager, protocolClient)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to init registry manager: %w", err)
	}

	return configManager, protocolClient, contentRenderer, registryManager, authManager, nil
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
		os.Exit(1)
	}

	// Create the main application instance with dependency injection
	consoleApp := &ConsoleApp{
		configManager:   configManager,
		protocolClient:  protocolClient,
		contentRenderer: contentRenderer,
		registryManager: registryManager,
		authManager:     authManager,
		args:            args,
	}

	// Create and start the appropriate Bubble Tea program
	program, err := consoleApp.createBubbleTeaProgram()
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
