package main

import (
	"TUI-Blender-Launcher/config" // Import config package
	"TUI-Blender-Launcher/tui"    // Import the tui package
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Check if config file *actually* exists (LoadConfig returns defaults if not)
	configFilePath, _ := config.GetConfigPath()
	needsInitialSetup := false
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		needsInitialSetup = true
	}

	// Initialize the TUI model, passing the config and setup flag
	m := tui.InitialModel(cfg, needsInitialSetup)

	// Create and run the Bubble Tea program
	p := tea.NewProgram(m,
		tea.WithAltScreen(),       // Use AltScreen
		tea.WithMouseCellMotion(), // Enable mouse support
	)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
