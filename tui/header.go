package tui

import (
	lp "github.com/charmbracelet/lipgloss"
)

// renderHeader creates a styled header for the TUI
func renderHeader(width int) string {
	return headerStyle.Width(width).AlignHorizontal(lp.Center).AlignVertical(lp.Center).Render("TUI Blender Launcher")
}

// getHeaderHeight returns the height of the header in lines
func getHeaderHeight() int {
	return 3
}
