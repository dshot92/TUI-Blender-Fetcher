package tui

import (
	lp "github.com/charmbracelet/lipgloss"
)

// Modified renderTitleHeader to use a default width if m.terminalWidth is not set
func (m Model) renderTitleHeader(text string) string {
	width := m.terminalWidth
	// if width <= 0 {
	// 	width = 80 // default width
	// }
	// if width < len(text) {
	// 	width = len(text) + 4 // add some padding
	// }
	return headerStyle.Width(width).AlignHorizontal(lp.Center).Render(text) + "\n"
}

// renderCommonHeader returns the common header (title) for both builds and settings pages.
func (m Model) renderCommonHeader() string {
	return m.renderTitleHeader("TUI Blender Launcher")
}
