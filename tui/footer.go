package tui

import (
	"TUI-Blender-Launcher/types"
	"fmt"
	"strings"

	lp "github.com/charmbracelet/lipgloss"
)

// renderBuildFooter renders the footer for the build list view
func (m Model) renderBuildFooter() string {
	// Define key command display styles
	keyStyle := lp.NewStyle().Foreground(lp.Color(colorInfo))
	sepStyle := lp.NewStyle().Foreground(lp.Color("240"))
	separator := sepStyle.Render(" · ")

	// Add command keys info
	var commands []string

	// Basic commands always available
	commands = append(commands, fmt.Sprintf("%s Quit", keyStyle.Render("q")))
	commands = append(commands, fmt.Sprintf("%s Settings", keyStyle.Render("s")))
	commands = append(commands, fmt.Sprintf("%s Reverse Sort", keyStyle.Render("r")))

	// Add commands for the selected build
	if len(m.builds) > 0 && m.cursor < len(m.builds) {
		commands = append(commands, fmt.Sprintf("%s Launch Build", keyStyle.Render("enter")))
		commands = append(commands, fmt.Sprintf("%s Open build Dir", keyStyle.Render("o")))

		build := m.builds[m.cursor]
		if build.Status == types.StateLocal {
			// For local builds, key 'x' deletes the build
			commands = append(commands, fmt.Sprintf("%s Delete", keyStyle.Render("x")))
		} else {
			// For non-local builds, use key 'x' for cancel if a download is active, otherwise 'd' to download
			buildID := build.Version + "-" + build.Hash
			if state, exists := m.downloadStates[buildID]; exists &&
				(state.BuildState == types.StateDownloading || state.BuildState == types.StateExtracting) {
				commands = append(commands, fmt.Sprintf("%s Cancel", keyStyle.Render("x")))
			} else {
				commands = append(commands, fmt.Sprintf("%s Download", keyStyle.Render("d")))
			}
		}
	}

	// Fetch command if manual fetch is enabled
	if m.config.ManualFetch {
		commands = append(commands, fmt.Sprintf("%s Fetch builds", keyStyle.Render("f")))
	}

	return strings.Join(commands, separator)
}

// renderSettingsFooter renders the footer for the settings view
func (m Model) renderSettingsFooter() string {
	// Define key command display styles
	keyStyle := lp.NewStyle().Foreground(lp.Color(colorInfo))
	sepStyle := lp.NewStyle().Foreground(lp.Color("240"))
	separator := sepStyle.Render(" · ")

	var commands []string
	commands = append(commands, fmt.Sprintf("%s Save", keyStyle.Render("s")))
	commands = append(commands, fmt.Sprintf("%s Quit", keyStyle.Render("q")))
	commands = append(commands, fmt.Sprintf("%s Edit", keyStyle.Render("enter")))
	commands = append(commands, fmt.Sprintf("%s Cleanup", keyStyle.Render("c")))

	footerText := strings.Join(commands, separator)

	return fmt.Sprintf("%s\n%s",
		separator,
		footerStyle.Width(m.terminalWidth).Render(footerText))
}
