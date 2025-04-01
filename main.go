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
		// Log error but maybe continue with defaults?
		// For now, let's exit if we can't load/stat config properly.
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		// If the error was just "not exist", we could proceed with defaults.
		// However, LoadConfig now returns defaults if not found, so errors here
		// are likely more serious (permissions, decode error).
		os.Exit(1)
	}

	// Check if config file *actually* exists (LoadConfig returns defaults if not)
	configFilePath, _ := config.GetConfigPath() // We need a way to get path easily
	needsInitialSetup := false
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		needsInitialSetup = true
		// Don't save yet, let the TUI handle the initial prompt/save
	}

	// Initialize the TUI model, passing the config and setup flag
	m := tui.InitialModel(cfg, needsInitialSetup)

	// Create and run the Bubble Tea program
	p := tea.NewProgram(m, tea.WithAltScreen()) // Use AltScreen
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
