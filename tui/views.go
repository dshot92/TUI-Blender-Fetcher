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

	var page string
	if m.currentView == viewInitialSetup || m.currentView == viewSettings {
		page = m.renderSettingsPage()
	} else {
		page = m.renderBuildsPage()
	}
	return page
}
