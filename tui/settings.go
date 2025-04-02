package tui

import (
	"strings"

	lp "github.com/charmbracelet/lipgloss"
)

// renderSettingsContent renders the settings page content
func (m Model) renderSettingsContent(availableHeight int) string {
	var b strings.Builder

	if m.currentView == viewInitialSetup {
		welcome := lp.NewStyle().Bold(true).Foreground(lp.Color(colorSuccess)).Render("Welcome to TUI Blender Launcher")
		b.WriteString(welcome + "\n\n")
		b.WriteString("Please configure the following settings to get started:\n\n")
	}

	settingsCount := len(m.settingsInputs)

	// Setting labels, matching the order in initialization
	settingLabels := []string{
		"Download Directory:",
		"Version Filter:",
	}

	// Descriptions for each setting to help users understand
	settingDescriptions := []string{
		"Where Blender builds will be downloaded and installed",
		"Only show versions matching this filter (e.g., '4.0' or '3.6')",
	}

	// Render each setting with label and input field
	for i := 0; i < settingsCount; i++ {
		// Skip if we don't have a corresponding label/description
		if i >= len(settingLabels) || i >= len(settingDescriptions) {
			continue
		}

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
	return m.renderDialogBox(content.String(), 60)
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
