package tui

import (
	lp "github.com/charmbracelet/lipgloss"
)

// View renders the current view of the model
func (m Model) View() string {
	// First determine if we need to show an overlay dialog
	var isDialog bool
	var dialogContent string

	switch m.currentView {
	case viewCleanupConfirm:
		isDialog = true
		dialogContent = m.renderCleanupConfirmDialog()
	case viewQuitConfirm:
		isDialog = true
		dialogContent = m.renderQuitConfirmDialog()
	default:
		isDialog = false
	}

	// ===== UNIFIED LAYOUT STRUCTURE =====
	var headerContent string
	var middleContent string
	var footerContent string

	// Calculate dynamic heights for responsive layout
	headerHeight := 2 // Title + separator
	footerHeight := 2 // Commands + 1 for separator
	middleHeight := m.terminalHeight - headerHeight - footerHeight

	// Ensure minimal height for middle section
	if middleHeight < 5 {
		middleHeight = 5
	}

	// ===== RENDER HEADER =====
	if m.currentView == viewInitialSetup || m.currentView == viewSettings {
		headerContent = m.renderSettingsHeader()
	} else {
		headerContent = m.renderBuildHeader()
	}

	// ===== RENDER MIDDLE CONTENT =====
	if m.currentView == viewInitialSetup || m.currentView == viewSettings {
		middleContent = m.renderSettingsContent(middleHeight)
	} else {
		middleContent = m.renderBuildContent(middleHeight)
	}

	// ===== RENDER FOOTER =====
	if m.currentView == viewInitialSetup || m.currentView == viewSettings {
		footerContent = m.renderSettingsFooter()
	} else {
		footerContent = m.renderBuildFooter()
	}

	// Combine all sections
	baseView := lp.JoinVertical(
		lp.Top,
		headerContent,
		middleContent,
		footerContent,
	)

	// If we're showing a dialog, place it on top of the base view
	if isDialog {
		return lp.Place(
			m.terminalWidth,
			m.terminalHeight,
			lp.Center,
			lp.Center,
			dialogContent,
		)
	}

	return baseView
}
