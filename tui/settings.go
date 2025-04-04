package tui

import (
	"strings"

	lp "github.com/charmbracelet/lipgloss"
)

// renderSettingsContent renders the settings page content
func (m *Model) renderSettingsContent(availableHeight int) string {
	var b strings.Builder

	// Define all styles once at the beginning
	normalTextStyle := lp.NewStyle()
	welcomeStyle := lp.NewStyle().Bold(true).Foreground(lp.Color(highlightColor))

	// Enhanced unified styling - use color constants from const.go
	primaryColor := lp.Color(highlightColor) // Use the highlight color (blue) from constants
	subtleColor := lp.Color("240")           // Slightly lighter gray for better readability
	highlightBg := lp.Color("236")           // Slightly darker background for highlights

	// Label styling (consistent for all settings)
	labelStyle := lp.NewStyle().
		Foreground(primaryColor).
		Bold(true)

	labelStyleFocused := labelStyle.Copy().
		Foreground(lp.Color(highlightColor)).
		Background(highlightBg).
		Padding(0, 1).
		Bold(true)

	// Input field styling - removed border, reduced padding
	inputStyle := lp.NewStyle().
		MarginLeft(2)

	inputStyleFocused := inputStyle.Copy().
		Foreground(lp.Color(textColor))

	// Description styling
	descStyle := lp.NewStyle().
		Foreground(subtleColor).
		Italic(true).
		MarginLeft(2)

	// Section styling for each setting group - reduced padding
	sectionStyle := lp.NewStyle()

	// Horizontal option styling for build type
	optionStyle := lp.NewStyle().
		Padding(0, 1).
		MarginRight(1)

	selectedOptionStyle := lp.NewStyle().
		Background(lp.Color(highlightColor)).
		Foreground(lp.Color(textColor)).
		Padding(0, 1).
		MarginRight(1)

	if m.currentView == viewInitialSetup {
		b.WriteString(welcomeStyle.Render("Welcome to TUI Blender Launcher"))
		b.WriteString("\n\n")
		b.WriteString(normalTextStyle.Render("Please configure the following settings to get started:"))
		b.WriteString("\n\n")
	}

	// Count of text input settings
	settingsCount := len(m.settingsInputs)

	// Setting labels, matching the order in initialization
	settingLabels := []string{
		"Download Directory:",
		"Version Filter:",
		"Build Type:",
	}

	// Descriptions for each setting to help users understand
	settingDescriptions := []string{
		"Where Blender builds will be downloaded and installed",
		"Only show versions matching this filter (e.g., '4.0' or '3.6')",
		"Select which build type to fetch (daily, patch, experimental)",
	}

	// Render all settings (including text inputs and horizontal build type selector)
	for i := 0; i <= settingsCount; i++ {
		// Skip if we don't have a corresponding label/description
		if i >= len(settingLabels) || i >= len(settingDescriptions) {
			continue
		}

		sectionContent := strings.Builder{}

		// Determine if this item is focused (even without edit mode)
		isFocused := i == m.focusIndex

		// Render setting label with appropriate style
		if isFocused {
			sectionContent.WriteString(labelStyleFocused.Render(settingLabels[i]))
		} else {
			sectionContent.WriteString(labelStyle.Render(settingLabels[i]))
		}
		sectionContent.WriteString("\n")

		// Render input field or horizontal build type selector
		if i < settingsCount {
			// This is for Download Directory and Version Filter text inputs
			inputView := m.settingsInputs[i].View()

			// Add special styling for focused input
			if isFocused {
				sectionContent.WriteString(inputStyleFocused.Render(inputView))
			} else {
				sectionContent.WriteString(inputStyle.Render(inputView))
			}
		} else {
			// This is only for the Build Type horizontal selector
			var horizontalOptions strings.Builder

			// Get the currently selected build type
			selectedBuildType := m.buildType

			for _, option := range m.buildTypeOptions {
				if option == selectedBuildType {
					horizontalOptions.WriteString(selectedOptionStyle.Render(option))
				} else {
					horizontalOptions.WriteString(optionStyle.Render(option))
				}
			}

			horizontalOptions.WriteString("  (← → to select)")

			sectionContent.WriteString(inputStyle.Render(horizontalOptions.String()))
		}
		sectionContent.WriteString("\n")

		// Add description
		sectionContent.WriteString(descStyle.Render(settingDescriptions[i]))

		// Add the complete section to the builder
		b.WriteString(sectionStyle.Render(sectionContent.String()))
		b.WriteString("\n")
	}

	return lp.Place(m.terminalWidth, availableHeight, lp.Left, lp.Top, b.String())
}
