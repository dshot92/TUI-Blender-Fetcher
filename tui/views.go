package tui

import (
	lp "github.com/charmbracelet/lipgloss"
)

func (m *Model) renderPageForView() string {
	header := m.renderCommonHeader()
	headerHeight := 5
	var footer string
	var footerHeight int
	var content string
	if m.currentView == viewInitialSetup || m.currentView == viewSettings {
		footer = m.renderSettingsFooter()
		footerHeight = 2
		content = m.renderSettingsContent(m.terminalHeight - headerHeight - footerHeight)
	} else {
		footer = m.renderBuildFooter()
		footerHeight = 1
		content = m.renderBuildContent(m.terminalHeight - headerHeight - footerHeight)
	}
	baseView := lp.JoinVertical(lp.Top, header, content, footer)
	return lp.Place(m.terminalWidth, m.terminalHeight, lp.Left, lp.Top, baseView)
}

// renderCommonHeader returns the common header (title) for both builds and settings pages.
func (m *Model) renderCommonHeader() string {
	return headerStyle.Width(m.terminalWidth).AlignHorizontal(lp.Center).Render("TUI Blender Launcher") + "\n"
}
