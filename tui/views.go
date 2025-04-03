package tui

import (
	lp "github.com/charmbracelet/lipgloss"
)

func (m *Model) renderPageForView() string {

	header := renderHeader(m.terminalWidth)
	headerHeight := getHeaderHeight()

	var footer string
	var footerHeight int = 1
	var content string

	if m.currentView == viewInitialSetup || m.currentView == viewSettings {
		footer = m.renderSettingsFooter()
		content = m.renderSettingsContent(m.terminalHeight - headerHeight - footerHeight)
	} else {
		footer = m.renderBuildFooter()
		content = m.renderBuildContent(m.terminalHeight - headerHeight - footerHeight)
	}

	baseView := lp.JoinVertical(lp.Top, header, content, footer)
	// Force the base view to span the full terminal width
	baseView = lp.NewStyle().Width(m.terminalWidth).Render(baseView)
	return lp.Place(m.terminalWidth, m.terminalHeight, lp.Left, lp.Top, baseView)
}
