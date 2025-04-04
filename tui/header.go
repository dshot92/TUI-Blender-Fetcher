package tui

import (
	lp "github.com/charmbracelet/lipgloss"
)

// renderHeader creates a styled header for the TUI
func renderHeader(width int) string {
	// Create a bold, centered title
	return lp.NewStyle().
		Bold(true).
		Foreground(lp.Color(textColor)). // Use our textColor constant
		Width(width).
		Align(lp.Center).
		Render("TUI Blender Launcher")
}
