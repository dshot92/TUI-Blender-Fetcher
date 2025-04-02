package tui

import (
	"fmt"
	"strings"

	lp "github.com/charmbracelet/lipgloss"
)

// renderSettingsContent renders the settings page
func (m Model) renderSettingsContent(availableHeight int) string {
	var b strings.Builder

	settingsCount := len(m.settingsInputs)

	// Setting labels, matching the order in initialization
	settingLabels := []string{
		"Download Directory:",
		"Version Filter:",
		"Manual Fetch:",
	}

	// Descriptions for each setting to help users understand
	settingDescriptions := []string{
		"Where Blender builds will be downloaded and installed",
		"Only show versions matching this filter (e.g., '4.0' or '3.6')",
		"If true, online builds are only checked when requested",
	}

	// Render each setting with label and input field
	for i := 0; i < settingsCount; i++ {
		// Determine if this input is focused
		isFocused := m.editMode && i == m.focusIndex

		// Style for the label - make it bold if this input is focused
		labelStyle := lp.NewStyle().Bold(isFocused)

		// Render setting label
		b.WriteString(labelStyle.Render(settingLabels[i]) + "\n")

		// Render input field (will show as active if focused)
		b.WriteString(m.settingsInputs[i].View() + "\n")

		// Add description in smaller, dimmed text
		descStyle := lp.NewStyle().Faint(true).Italic(true)
		b.WriteString(descStyle.Render(settingDescriptions[i]) + "\n\n")
	}

	return lp.Place(m.terminalWidth, availableHeight, lp.Left, lp.Top, b.String())
}

// renderInitialSetupView renders the initial setup view when app is first run
func (m Model) renderInitialSetupView() string {
	var b strings.Builder

	// Welcome message
	welcome := lp.NewStyle().Bold(true).Foreground(lp.Color(colorSuccess)).Render("Welcome to TUI Blender Launcher")
	b.WriteString(welcome + "\n\n")

	// Instructions for the setup
	instructions := "Please configure the following settings to get started:\n\n"
	b.WriteString(instructions)

	// Add the settings form
	b.WriteString(m.renderSettingsContent(m.terminalHeight - 10))

	return b.String()
}

// renderSettingsView renders the settings view
func (m Model) renderSettingsView() string {
	var b strings.Builder

	b.WriteString(m.renderSettingsContent(m.terminalHeight - 5))

	return b.String()
}

// renderCleanupConfirmDialog renders the dialog for confirming cleanup of older builds
func (m Model) renderCleanupConfirmDialog() string {
	var content strings.Builder

	title := "Clean up old Blender builds?"
	content.WriteString(lp.NewStyle().Bold(true).Render(title) + "\n\n")

	// Show information about what will be removed
	content.WriteString(fmt.Sprintf("This will remove %d %s, freeing up %s of disk space.\n\n",
		m.oldBuildsCount,
		pluralize("build", m.oldBuildsCount),
		formatByteSize(m.oldBuildsSize)))

	// Instructions
	content.WriteString("Press Enter to confirm, Esc to cancel")

	// Create a styled dialog box
	return m.renderDialogBox(content.String(), cleanupDialogWidth)
}

// renderQuitConfirmDialog renders the dialog confirming a quit during an active download
func (m Model) renderQuitConfirmDialog() string {
	var content strings.Builder

	title := "Quit during active download?"
	content.WriteString(lp.NewStyle().Bold(true).Render(title) + "\n\n")

	// Warning message
	content.WriteString("Warning: A download is currently in progress.\n")
	content.WriteString("Quitting now may result in incomplete files.\n\n")

	// Instructions
	content.WriteString("Press Enter to quit anyway, Esc to cancel")

	// Create a styled dialog box
	return m.renderDialogBox(content.String(), quitDialogWidth)
}

// renderDialogBox creates a styled dialog box with the given content
func (m Model) renderDialogBox(content string, width int) string {
	// Create a box with a border
	boxStyle := lp.NewStyle().
		Border(lp.NormalBorder()).
		BorderForeground(lp.Color(colorInfo)).
		Padding(1, 2).
		Width(width).
		Align(lp.Center)

	return boxStyle.Render(content)
}
