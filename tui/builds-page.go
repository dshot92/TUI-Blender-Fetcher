package tui

import (
	lp "github.com/charmbracelet/lipgloss"
)

// renderBuildsPage constructs the builds page with a common header, table header, build content and build footer.
func (m Model) renderBuildsPage() string {
	// Combine the common header and table header for builds page.
	header := m.renderCommonHeader()

	// Use fixed header and footer heights; adjust middle content accordingly.

	headerHeight := 5
	footerHeight := 1
	middleHeight := m.terminalHeight - headerHeight - footerHeight
	// if middleHeight < 5 {
	// 	middleHeight = 5
	// }

	body := m.renderBuildContent(middleHeight)
	footer := m.renderBuildFooter()
	baseView := lp.JoinVertical(lp.Top, header, body, footer)
	return lp.Place(m.terminalWidth, m.terminalHeight, lp.Left, lp.Top, baseView)
}
