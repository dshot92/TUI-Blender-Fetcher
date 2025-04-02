package tui

import (
	lp "github.com/charmbracelet/lipgloss"
)

// View renders the current view of the model using a unified layout.
// It picks either the settings page or builds page based on the current view.
func (m Model) View() string {
	// If an overlay dialog is active, render it centered.
	if m.currentView == viewCleanupConfirm || m.currentView == viewQuitConfirm {
		var dialogContent string
		if m.currentView == viewCleanupConfirm {
			dialogContent = m.renderCleanupConfirmDialog()
		} else {
			dialogContent = m.renderQuitConfirmDialog()
		}
		return lp.Place(m.terminalWidth, m.terminalHeight, lp.Center, lp.Center, dialogContent)
	}

	return m.renderPageForView()
}

func (m Model) renderPageForView() string {
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
func (m Model) renderCommonHeader() string {
	return headerStyle.Width(m.terminalWidth).AlignHorizontal(lp.Center).Render("TUI Blender Launcher") + "\n"
}
