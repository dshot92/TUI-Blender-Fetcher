package main

import (
	"TUI-Blender-Launcher/config" // Import config package
	"TUI-Blender-Launcher/tui"    // Import the tui package
	"fmt"
	"os"
	"syscall"
	"time"

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
	p := tea.NewProgram(&m,
		tea.WithAltScreen(),       // Use AltScreen
		tea.WithMouseCellMotion(), // Enable mouse support
	)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}

	// After TUI exits, check if we should launch Blender
	launchPath := os.Getenv("TUI_BLENDER_LAUNCH")
	if launchPath != "" {
		// Read the executable path from the file
		execBytes, err := os.ReadFile(launchPath)
		if err == nil && len(execBytes) > 0 {
			blenderExe := string(execBytes)

			// Clean up the temporary file
			os.Remove(launchPath)

			// Launch Blender, replacing the current process
			fmt.Printf("Launching Blender...\n")

			// Sleep briefly to allow terminal to reset after TUI exit
			time.Sleep(100 * time.Millisecond)

			// Use syscall.Exec to replace current process with Blender
			err = syscall.Exec(blenderExe, []string{blenderExe}, os.Environ())
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error launching Blender: %v\n", err)
				os.Exit(1)
			}
		}
	}
}
