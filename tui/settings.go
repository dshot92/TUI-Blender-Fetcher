package tui

import (
	"strings"

	lp "github.com/charmbracelet/lipgloss"
)

// renderSettingsContent renders the settings page content with a cleaner structure.
func (m *Model) renderSettingsContent(availableHeight int) string {
	var b strings.Builder

	// Define global styles for the settings rendering
	normalTextStyle := lp.NewStyle()
	welcomeStyle := lp.NewStyle().Bold(true).Foreground(lp.Color(highlightColor))

	primaryColor := lp.Color(highlightColor) // Use highlight color (blue) from constants
	subtleColor := lp.Color(highlightColor)  // Use text color (white) from constants
	highlightBg := lp.Color(backgroundColor) // Use background color (gray) from constants

	labelStyle := lp.NewStyle().Foreground(primaryColor).Bold(true)
	labelStyleFocused := labelStyle.
		Foreground(lp.Color(highlightColor)).
		Background(lp.Color(highlightBg)).
		Bold(true)

	inputStyle := lp.NewStyle().MarginLeft(2)
	inputStyleFocused := inputStyle.Foreground(lp.Color(textColor))

	descStyle := lp.NewStyle().Foreground(subtleColor).Italic(true)
	sectionStyle := lp.NewStyle()

	optionStyle := lp.NewStyle().MarginRight(1)
	selectedOptionStyle := lp.NewStyle().
		Background(lp.Color(highlightColor)).
		Foreground(lp.Color(textColor)).
		MarginRight(1)

	// Display welcome messages if in the initial setup view
	if m.currentView == viewInitialSetup {
		b.WriteString(welcomeStyle.Render("Welcome to TUI Blender Launcher"))
		b.WriteString("\n\n")
		b.WriteString(normalTextStyle.Render("Please configure the following settings to get started:"))
		b.WriteString("\n\n")
	}

	// Helper to render a text input setting
	renderTextSetting := func(index int, label, description string) string {
		var sb strings.Builder
		isFocused := (m.focusIndex == index)
		if isFocused {
			sb.WriteString(labelStyleFocused.Render(label))
		} else {
			sb.WriteString(labelStyle.Render(label))
		}
		sb.WriteString(" ")

		inputView := m.settingsInputs[index].View()
		if isFocused {
			sb.WriteString(inputStyleFocused.Render(inputView))
		} else {
			sb.WriteString(inputStyle.Render(inputView))
		}
		sb.WriteString("\n")
		sb.WriteString(descStyle.Render(description))
		sb.WriteString("\n")
		// Add a divider line
		sb.WriteString("\n")
		return sectionStyle.Render(sb.String())
	}

	// Helper to render the build type (horizontal selector) setting
	renderBuildTypeSetting := func(label, description string) string {
		var sb strings.Builder
		// Focused when the build type setting is active (last setting)
		isFocused := (m.focusIndex == len(m.settingsInputs))
		if isFocused {
			sb.WriteString(labelStyleFocused.Render(label))
		} else {
			sb.WriteString(labelStyle.Render(label))
		}
		sb.WriteString(" ")

		var horizontalOptions strings.Builder
		selectedBuildType := m.buildType
		for _, option := range m.buildTypeOptions {
			if option == selectedBuildType {
				horizontalOptions.WriteString(selectedOptionStyle.Render(option))
			} else {
				horizontalOptions.WriteString(optionStyle.Render(option))
			}
		}
		sb.WriteString(inputStyle.Render(horizontalOptions.String()))
		sb.WriteString("\n")
		sb.WriteString(descStyle.Render(description))
		sb.WriteString("\n")
		// No divider for the last setting
		return sectionStyle.Render(sb.String())
	}

	// Render each individual setting in a clear and separate block

	// Download Directory setting (text input)
	b.WriteString(renderTextSetting(0,
		"Download Directory:",
		"Where Blender builds will be downloaded and installed"))
	b.WriteString("\n")

	// Version Filter setting (text input)
	b.WriteString(renderTextSetting(1,
		"Version Filter:",
		"Only show versions matching this filter (e.g., '4.0' or '3.6')"))
	b.WriteString("\n")

	// Build Type setting (horizontal selector)
	b.WriteString(renderBuildTypeSetting(
		"Build Type:",
		"Select which build type to fetch (daily, patch, experimental) <- to select ->"))

	return lp.Place(m.terminalWidth, availableHeight, lp.Left, lp.Top, b.String())
}
