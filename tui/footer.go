package tui

import (
	"TUI-Blender-Launcher/model"
	"fmt"
	"strings"

	lp "github.com/charmbracelet/lipgloss"
)

// renderBuildFooter renders the footer for the build list view
func (m *Model) renderBuildFooter() string {
	keyStyle := lp.NewStyle().Foreground(lp.Color(colorInfo))
	sepStyle := lp.NewStyle().Foreground(lp.Color("240"))
	separator := sepStyle.Render(" · ")

	// General commands always available
	generalCommands := []string{
		fmt.Sprintf("%s Quit", keyStyle.Render("q")),
		fmt.Sprintf("%s Settings", keyStyle.Render("s")),
		fmt.Sprintf("%s Reverse Sort", keyStyle.Render("r")),
		fmt.Sprintf("%s Fetch online builds", keyStyle.Render("f")),
	}

	// Contextual commands based on the highlighted build
	contextualCommands := []string{}
	if len(m.builds) > 0 && m.cursor < len(m.builds) {
		build := m.builds[m.cursor]
		if build.Status == model.StateLocal {
			contextualCommands = append(contextualCommands,
				fmt.Sprintf("%s Launch Build", keyStyle.Render("enter")),
				fmt.Sprintf("%s Open build Dir", keyStyle.Render("o")),
			)
			contextualCommands = append(contextualCommands,
				fmt.Sprintf("%s Delete build", keyStyle.Render("x")),
			)
		} else if build.Status == model.StateOnline || build.Status == model.StateUpdate {
			contextualCommands = append(contextualCommands,
				fmt.Sprintf("%s Download build", keyStyle.Render("d")),
			)
		}

		// Check for active download state
		buildID := build.Version
		if build.Hash != "" {
			buildID = build.Version + "-" + build.Hash[:8]
		}
		state := m.commands.downloads.GetState(buildID)
		if state != nil && (state.BuildState == model.StateDownloading || state.BuildState == model.StateExtracting) {
			// Remove any existing download command
			filtered := []string{}
			for _, cmd := range contextualCommands {
				if !strings.Contains(cmd, "Download build") {
					filtered = append(filtered, cmd)
				}
			}
			contextualCommands = filtered
			contextualCommands = append(contextualCommands,
				fmt.Sprintf("%s Cancel download", keyStyle.Render("x")),
			)
		}
	}

	line1 := strings.Join(contextualCommands, separator)
	line2 := strings.Join(generalCommands, separator)
	return footerStyle.Width(m.terminalWidth).Render(line1 + "\n" + line2)
}

// renderSettingsFooter renders the footer for the settings view
func (m *Model) renderSettingsFooter() string {
	keyStyle := lp.NewStyle().Foreground(lp.Color(colorInfo))
	sepStyle := lp.NewStyle().Foreground(lp.Color("240"))
	separator := sepStyle.Render(" · ")

	generalCommands := []string{
		fmt.Sprintf("%s Save and exit", keyStyle.Render("s")),
		fmt.Sprintf("%s Quit", keyStyle.Render("q")),
		fmt.Sprintf("%s Edit setting", keyStyle.Render("enter")),
	}
	// First line is empty (no contextual commands), second line holds general commands
	return footerStyle.Width(m.terminalWidth).Render("\n" + strings.Join(generalCommands, separator))
}
