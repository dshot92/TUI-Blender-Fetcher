package tui

import (
	lp "github.com/charmbracelet/lipgloss"
)

// renderHeader creates a styled header for the TUI
func renderHeader(width int) string {
	// Create a bold, centered title
	return lp.NewStyle().
		Bold(true).
		Foreground(lp.Color("15")). // White text
		Width(width).
		Align(lp.Center).
		Render("TUI Blender Launcher")
}

// getHeaderHeight returns the height of the header in lines
func getHeaderHeight() int {
	return 1
}
