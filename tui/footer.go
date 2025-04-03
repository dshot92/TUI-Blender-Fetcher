package tui

import (
	"TUI-Blender-Launcher/model"
	"fmt"
	"strings"

	lp "github.com/charmbracelet/lipgloss"
)

// renderBuildFooter renders the footer for the build list view
func (m *Model) renderBuildFooter() string {
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
	commands = append(commands, fmt.Sprintf("%s Fetch online builds", keyStyle.Render("f")))

	// Add build-specific commands
	if len(m.builds) > 0 && m.cursor < len(m.builds) {
		build := m.builds[m.cursor]

		// Launch and Open build directory
		commands = append(commands, fmt.Sprintf("%s Launch Build", keyStyle.Render("enter")))
		commands = append(commands, fmt.Sprintf("%s Open build Dir", keyStyle.Render("o")))

		// Status-dependent commands
		if build.Status == model.StateLocal {
			// For local builds, show delete option
			commands = append(commands, fmt.Sprintf("%s Delete build", keyStyle.Render("x")))
		} else if build.Status == model.StateOnline || build.Status == model.StateUpdate {
			// For online builds, show download option
			commands = append(commands, fmt.Sprintf("%s Download build", keyStyle.Render("d")))
		}

		// If downloading or extracting, show cancel option
		buildID := build.Version
		if build.Hash != "" {
			buildID = build.Version + "-" + build.Hash[:8]
		}

		state := m.commands.downloads.GetState(buildID)
		if state != nil && (state.BuildState == model.StateDownloading || state.BuildState == model.StateExtracting) {
			// Replace any previous command with cancel
			commands = append(commands, fmt.Sprintf("%s Cancel download", keyStyle.Render("x")))
		}
	}

	return footerStyle.Width(m.terminalWidth).Render(strings.Join(commands, separator))
}

// renderSettingsFooter renders the footer for the settings view
func (m *Model) renderSettingsFooter() string {
	// Define key command display styles
	keyStyle := lp.NewStyle().Foreground(lp.Color(colorInfo))
	sepStyle := lp.NewStyle().Foreground(lp.Color("240"))
	separator := sepStyle.Render(" · ")

	var commands []string

	// Always show these commands
	commands = append(commands, fmt.Sprintf("%s Save and exit", keyStyle.Render("s")))
	commands = append(commands, fmt.Sprintf("%s Quit", keyStyle.Render("q")))
	commands = append(commands, fmt.Sprintf("%s Edit setting", keyStyle.Render("enter")))

	return footerStyle.Width(m.terminalWidth).Render(strings.Join(commands, separator))
}
