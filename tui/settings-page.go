package tui

import (
	lp "github.com/charmbracelet/lipgloss"
)

// renderSettingsPage constructs the settings page with a common header, settings content and settings footer.
func (m Model) renderSettingsPage() string {
	// Use only the common header (no table header) for the settings page.
	header := m.renderCommonHeader()

	// Define header and footer heights; calculate available height for settings content.
	headerHeight := 3
	footerHeight := 2
	middleHeight := m.terminalHeight - headerHeight - footerHeight
	if middleHeight < 5 {
		middleHeight = 5
	}

	body := m.renderSettingsContent(middleHeight)
	footer := m.renderSettingsFooter()
	baseView := lp.JoinVertical(lp.Top, header, body, footer)
	return lp.Place(m.terminalWidth, m.terminalHeight, lp.Left, lp.Top, baseView)
}
