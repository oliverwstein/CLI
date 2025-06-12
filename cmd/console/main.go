// Package main implements the Universal Application Console entry point.
// This file handles command-line argument parsing, dependency injection,
// and mode selection between Console Menu Mode and direct application connection.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/universal-console/console/internal/app"
	"github.com/universal-console/console/internal/auth"
	"github.com/universal-console/console/internal/config"
	"github.com/universal-console/console/internal/content"
	"github.com/universal-console/console/internal/interfaces"
	"github.com/universal-console/console/internal/logging"
	"github.com/universal-console/console/internal/protocol"
	"github.com/universal-console/console/internal/registry"
	app_ui "github.com/universal-console/console/internal/ui/app"
)

// Application metadata
const (
	Version         = "2.0.0"
	ProgramName     = "Universal Application Console"
	ProtocolVersion = "2.0"
)

// CommandLineArgs represents parsed command-line arguments
type CommandLineArgs struct {
	Host        string
	Profile     string
	Theme       string
	ShowHelp    bool
	ShowVersion bool
}

// Dependencies holds all injected application dependencies
type Dependencies struct {
	ConfigManager   interfaces.ConfigManager
	ProtocolClient  interfaces.ProtocolClient
	ContentRenderer interfaces.ContentRenderer
	RegistryManager interfaces.RegistryManager
	AuthManager     interfaces.AuthManager
	Logger          *logging.Logger
}

// ConsoleApp represents the main application with all injected dependencies
type ConsoleApp struct {
	deps Dependencies
	args CommandLineArgs
}

func main() {
	// Parse and validate command-line arguments
	args := parseCommandLineArgs()

	// Handle immediate exit conditions
	if handleEarlyExitConditions(args) {
		return
	}

	// Initialize logging system
	logger := initializeLogging(args)

	// Validate command-line arguments
	if err := validateArguments(args); err != nil {
		logger.Error("Invalid arguments", "error", err.Error())
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		flag.Usage()
		os.Exit(1)
	}

	// Initialize all application dependencies
	deps, err := initializeDependencies(logger)
	if err != nil {
		logger.Error("Failed to initialize application components", "error", err.Error())
		fmt.Fprintf(os.Stderr, "Error initializing application: %v\n", err)
		os.Exit(1)
	}

	// Create and run the console application
	consoleApp := &ConsoleApp{
		deps: deps,
		args: args,
	}

	if err := consoleApp.Run(); err != nil {
		logger.Error("Application terminated with error", "error", err.Error())
		fmt.Fprintf(os.Stderr, "Application error: %v\n", err)
		os.Exit(1)
	}

	// Graceful shutdown
	logger.Info("Application shutdown completed successfully")
	fmt.Println("Universal Application Console terminated successfully.")
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

// handleEarlyExitConditions processes help and version flags that cause immediate exit
func handleEarlyExitConditions(args CommandLineArgs) bool {
	if args.ShowHelp {
		flag.Usage()
		return true
	}

	if args.ShowVersion {
		fmt.Printf("%s v%s\n", ProgramName, Version)
		fmt.Printf("Protocol Version: %s\n", ProtocolVersion)
		fmt.Printf("Built with Go and Charm libraries\n")
		return true
	}

	return false
}

// initializeLogging sets up the logging system based on environment and arguments
func initializeLogging(args CommandLineArgs) *logging.Logger {
	logConfig := logging.DefaultConfig()
	logConfig.Level = logging.InfoLevel

	// Enable debug logging if environment variable is set
	if os.Getenv("CONSOLE_DEBUG") == "true" {
		logConfig.Level = logging.DebugLevel
		logConfig.Format = "json"
	}

	if err := logging.InitGlobalLogger(logConfig); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logging: %v\n", err)
		os.Exit(1)
	}

	logger := logging.GetGlobalLogger()
	logger.Info("Universal Application Console starting",
		"version", Version,
		"args", fmt.Sprintf("%+v", args))

	return logger
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

// initializeDependencies creates all application dependencies with proper error handling
func initializeDependencies(logger *logging.Logger) (Dependencies, error) {
	logger.Debug("Initializing application components")

	var deps Dependencies
	deps.Logger = logger

	// Initialize configuration manager
	configManager, err := config.NewManager()
	if err != nil {
		return deps, fmt.Errorf("failed to initialize config manager: %w", err)
	}
	deps.ConfigManager = configManager

	// Initialize authentication manager
	authManager, err := auth.NewManager(configManager)
	if err != nil {
		return deps, fmt.Errorf("failed to initialize auth manager: %w", err)
	}
	deps.AuthManager = authManager

	// Initialize protocol client
	protocolClient, err := protocol.NewClient(configManager, authManager)
	if err != nil {
		return deps, fmt.Errorf("failed to initialize protocol client: %w", err)
	}
	deps.ProtocolClient = protocolClient

	// Initialize content renderer
	contentRenderer, err := content.NewRenderer()
	if err != nil {
		return deps, fmt.Errorf("failed to initialize content renderer: %w", err)
	}
	deps.ContentRenderer = contentRenderer

	// Initialize registry manager
	registryManager, err := registry.NewManager(configManager, protocolClient)
	if err != nil {
		return deps, fmt.Errorf("failed to initialize registry manager: %w", err)
	}
	deps.RegistryManager = registryManager

	logger.Info("Application components initialized successfully")
	return deps, nil
}

// Run starts the console application with the appropriate mode
func (ca *ConsoleApp) Run() error {
	ca.deps.Logger.Debug("Creating Bubble Tea program")

	program, err := ca.createBubbleTeaProgram()
	if err != nil {
		return fmt.Errorf("failed to create application interface: %w", err)
	}

	ca.deps.Logger.Info("Starting TUI application")

	_, err = program.Run()
	return err
}

// shouldLaunchDirectConnection determines if the application should connect directly
// to an application instead of showing the Console Menu
func (ca *ConsoleApp) shouldLaunchDirectConnection() bool {
	return ca.args.Host != "" || ca.args.Profile != ""
}

// createBubbleTeaProgram instantiates the appropriate Bubble Tea model based on mode
func (ca *ConsoleApp) createBubbleTeaProgram() (*tea.Program, error) {
	// Configure program options for Claude Code-like experience
	programOptions := []tea.ProgramOption{
		tea.WithAltScreen(),       // Full-screen alternate buffer like Claude Code
		tea.WithMouseCellMotion(), // Enable mouse support
	}

	if ca.shouldLaunchDirectConnection() {
		model, err := ca.createDirectConnectionModel()
		if err != nil {
			return nil, err
		}
		return tea.NewProgram(model, programOptions...), nil
	}

	model := ca.createConsoleMenuModel()
	return tea.NewProgram(model, programOptions...), nil
}

// createDirectConnectionModel creates the Application Mode model for direct connections
func (ca *ConsoleApp) createDirectConnectionModel() (tea.Model, error) {
	profile, err := ca.determineProfile()
	if err != nil {
		return nil, fmt.Errorf("failed to determine connection profile: %w", err)
	}

	// Attempt immediate connection
	_, err = ca.deps.ProtocolClient.Connect(context.Background(), profile.Host, &profile.Auth)
	if err != nil {
		// Log the error but continue, the app model will handle showing the error
		ca.deps.Logger.Warn("Direct connection failed, will show error in UI", "error", err.Error())
	}

	// Create the Application Mode model
	model := app_ui.NewAppModel(
		profile,
		ca.deps.ProtocolClient,
		ca.deps.ContentRenderer,
		ca.deps.ConfigManager,
		ca.deps.AuthManager,
	)

	return model, nil
}

// createConsoleMenuModel creates the Console Menu Mode model
func (ca *ConsoleApp) createConsoleMenuModel() tea.Model {
	return app.NewConsoleController(
		ca.deps.RegistryManager,
		ca.deps.ConfigManager,
		ca.deps.ProtocolClient,
		ca.deps.ContentRenderer,
		ca.deps.AuthManager,
	)
}

// determineProfile resolves which profile to use based on command-line arguments
func (ca *ConsoleApp) determineProfile() (*interfaces.Profile, error) {
	// If host is explicitly specified, create a temporary profile
	if ca.args.Host != "" {
		return ca.createTemporaryProfile(), nil
	}

	// Use specified profile or default to "default"
	profileName := ca.args.Profile
	if profileName == "" {
		profileName = "default"
	}

	profile, err := ca.deps.ConfigManager.LoadProfile(profileName)
	if err != nil {
		return nil, fmt.Errorf("failed to load profile '%s': %w", profileName, err)
	}

	// Apply theme override if specified
	if ca.args.Theme != "" {
		profile.Theme = ca.args.Theme
	}

	return profile, nil
}

// createTemporaryProfile creates a profile for direct host connections
func (ca *ConsoleApp) createTemporaryProfile() *interfaces.Profile {
	profile := &interfaces.Profile{
		Name:          "temporary",
		Host:          ca.args.Host,
		Theme:         "github", // Default theme
		Confirmations: true,
		Auth: interfaces.AuthConfig{
			Type: "none",
		},
	}

	// Apply theme override if specified
	if ca.args.Theme != "" {
		profile.Theme = ca.args.Theme
	}

	return profile
}
