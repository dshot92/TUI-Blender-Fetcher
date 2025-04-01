package main

import (
	"TUI-Blender-Launcher/tui" // Import the tui package
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Initialize the TUI model
	m := tui.InitialModel()

	// Create and run the Bubble Tea program
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
