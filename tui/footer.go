package tui

import (
	"TUI-Blender-Launcher/types"
	"fmt"
	"strings"

	lp "github.com/charmbracelet/lipgloss"
)

// renderBuildFooter renders the footer for the build list view
func (m Model) renderBuildFooter() string {
	var b strings.Builder

	// Define key command display styles
	keyStyle := lp.NewStyle().Foreground(lp.Color(colorInfo))
	sepStyle := lp.NewStyle().Foreground(lp.Color("240"))
	separator := sepStyle.Render(" · ")

	// Add command keys info
	var commands []string

	// Basic commands always available
	commands = append(commands, fmt.Sprintf("%s Quit", keyStyle.Render("q")))
	commands = append(commands, fmt.Sprintf("%s Run", keyStyle.Render("r")))
	commands = append(commands, fmt.Sprintf("%s Settings", keyStyle.Render("s")))

	// Add version-specific commands based on state of selected build
	if len(m.builds) > 0 && m.cursor < len(m.builds) {
		build := m.builds[m.cursor]

		// Only show download if not already downloaded
		if build.Status != types.StateLocal {
			commands = append(commands, fmt.Sprintf("%s Download", keyStyle.Render("d")))
		} else {
			// If build is local, show delete option
			commands = append(commands, fmt.Sprintf("%s Delete", keyStyle.Render("x")))
		}

		// If we have a download in progress, show cancel option
		buildID := build.Version + "-" + build.Hash
		if state, exists := m.downloadStates[buildID]; exists &&
			(state.BuildState == types.StateDownloading || state.BuildState == types.StateExtracting) {
			commands = append(commands, fmt.Sprintf("%s Cancel", keyStyle.Render("c")))
		}
	}

	// Fetch command if manual fetch is enabled
	if m.config.ManualFetch {
		commands = append(commands, fmt.Sprintf("%s Fetch", keyStyle.Render("f")))
	}

	// Show cleanup command if we have local builds
	hasLocalBuilds := false
	for _, build := range m.builds {
		if build.Status == types.StateLocal {
			hasLocalBuilds = true
			break
		}
	}
	if hasLocalBuilds {
		commands = append(commands, fmt.Sprintf("%s Cleanup", keyStyle.Render("C")))
	}

	// Join commands with separators
	b.WriteString(strings.Join(commands, separator))

	// Set bold style for entire footer
	footerText := b.String()

	// Render footer with full width, adding a separator line above
	return fmt.Sprintf("%s\n%s",
		separator,
		footerStyle.Width(m.terminalWidth).Render(footerText))
}

// renderSettingsFooter renders the footer for the settings view
func (m Model) renderSettingsFooter() string {
	var b strings.Builder

	// Define key command display styles
	keyStyle := lp.NewStyle().Foreground(lp.Color(colorInfo))
	sepStyle := lp.NewStyle().Foreground(lp.Color("240"))
	separator := sepStyle.Render(" · ")

	// Different commands based on edit mode
	var commands []string

	if m.editMode {
		commands = append(commands, fmt.Sprintf("%s/%s Navigate", keyStyle.Render("↑"), keyStyle.Render("↓")))
		commands = append(commands, fmt.Sprintf("%s Save", keyStyle.Render("Enter")))
		commands = append(commands, fmt.Sprintf("%s Exit Edit", keyStyle.Render("Esc")))
	} else {
		commands = append(commands, fmt.Sprintf("%s Edit", keyStyle.Render("e")))
		commands = append(commands, fmt.Sprintf("%s Back", keyStyle.Render("Esc")))
	}

	// Join commands with separators
	b.WriteString(strings.Join(commands, separator))

	// Set bold style for entire footer
	footerText := b.String()

	// Render footer with full width, adding a separator line above
	return fmt.Sprintf("%s\n%s",
		separator,
		footerStyle.Width(m.terminalWidth).Render(footerText))
}
