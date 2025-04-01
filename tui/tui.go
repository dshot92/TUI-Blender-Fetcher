package tui

import (
	"TUI-Blender-Launcher/config"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

// CreateApp creates a new bubbletea application with the initial model
func CreateApp(cfg config.Config, needsSetup bool) *tea.Program {
	initialModel := InitialModel(cfg, needsSetup)
	return tea.NewProgram(initialModel, tea.WithAltScreen())
}

// Run runs the TUI application
func Run(cfg config.Config, needsSetup bool) (string, error) {
	app := CreateApp(cfg, needsSetup)
	_, err := app.Run()

	// Check if we need to launch Blender after quit
	blenderToLaunch := ""
	if err == nil {
		// Get the path to launch Blender
		blenderToLaunch = getLaunchPath()
	}

	return blenderToLaunch, err
}

// Helper function to get the Blender launch path
func getLaunchPath() string {
	// Read from environment variable
	launchPath := os.Getenv(envLaunchVariable)
	if launchPath != "" {
		// Read the launch command from the file
		if data, err := os.ReadFile(launchPath); err == nil {
			return string(data)
		}
	}
	return ""
}
