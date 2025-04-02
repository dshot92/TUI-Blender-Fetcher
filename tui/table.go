package tui

import (
	"TUI-Blender-Launcher/types"
	"bytes"
	"fmt"
	"strings"
	"time"

	lp "github.com/charmbracelet/lipgloss"
)

// renderBuildContent renders the table content
func (m Model) renderBuildContent(availableHeight int) string {
	if m.isLoading {
		// Show loading message in the middle of the screen
		return lp.Place(
			m.terminalWidth,
			availableHeight,
			lp.Center,
			lp.Center,
			lp.NewStyle().Foreground(lp.Color(colorInfo)).Render("Loading Blender builds..."),
		)
	}

	if len(m.builds) == 0 {
		// No builds to display
		var msg string
		if m.config.ManualFetch {
			msg = "No Blender builds found locally.\nPress 'f' to fetch available builds."
		} else {
			msg = "No Blender builds found locally or online."
		}
		return lp.Place(
			m.terminalWidth,
			availableHeight,
			lp.Center,
			lp.Center,
			lp.NewStyle().Foreground(lp.Color(colorInfo)).Render(msg),
		)
	}

	var output bytes.Buffer

	// Calculate how many builds we can show in the available space
	maxShownBuilds := availableHeight - 2 // Subtract 2 for the top/bottom padding

	// Ensure we have at least 1 row
	if maxShownBuilds < 1 {
		maxShownBuilds = 1
	}

	// Update the model's visible rows count
	m.visibleRows = maxShownBuilds

	// Adjust scroll offset if needed to ensure the selected build is visible
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	} else if m.cursor >= m.scrollOffset+maxShownBuilds {
		m.scrollOffset = m.cursor - maxShownBuilds + 1
	}

	// Ensure scroll offset doesn't go out of bounds
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	if m.scrollOffset > len(m.builds)-1 {
		m.scrollOffset = len(m.builds) - 1
	}

	// Calculate the end index for iteration
	endIdx := m.scrollOffset + maxShownBuilds
	if endIdx > len(m.builds) {
		endIdx = len(m.builds)
	}

	// Define column visibility and style maps
	columns := []struct {
		name    string
		key     string
		visible bool
		width   int
		style   func(string) string
	}{
		{
			name:    "Blender",
			key:     "Version",
			visible: true,
			width:   columnConfigs["Version"].width,
			style: func(s string) string {
				return cellStyleLeft.Width(columnConfigs["Version"].width).Render(s)
			},
		},
		{
			name:    "Status",
			key:     "Status",
			visible: true,
			width:   columnConfigs["Status"].width,
			style: func(s string) string {
				return cellStyleCenter.Width(columnConfigs["Status"].width).Render(s)
			},
		},
		{
			name:    "Branch",
			key:     "Branch",
			visible: m.visibleColumns["Branch"],
			width:   columnConfigs["Branch"].width,
			style: func(s string) string {
				return cellStyleCenter.Width(columnConfigs["Branch"].width).Render(s)
			},
		},
		{
			name:    "Type",
			key:     "Type",
			visible: m.visibleColumns["Type"],
			width:   columnConfigs["Type"].width,
			style: func(s string) string {
				return cellStyleCenter.Width(columnConfigs["Type"].width).Render(s)
			},
		},
		{
			name:    "Hash",
			key:     "Hash",
			visible: m.visibleColumns["Hash"],
			width:   columnConfigs["Hash"].width,
			style: func(s string) string {
				return cellStyleCenter.Width(columnConfigs["Hash"].width).Render(s)
			},
		},
		{
			name:    "Size",
			key:     "Size",
			visible: m.visibleColumns["Size"],
			width:   columnConfigs["Size"].width,
			style: func(s string) string {
				return cellStyleRight.Width(columnConfigs["Size"].width).Render(s)
			},
		},
		{
			name:    "Build Date",
			key:     "Build Date",
			visible: m.visibleColumns["Build Date"],
			width:   columnConfigs["Build Date"].width,
			style: func(s string) string {
				return cellStyleCenter.Width(columnConfigs["Build Date"].width).Render(s)
			},
		},
	}

	// Render each visible row
	for i := m.scrollOffset; i < endIdx; i++ {
		build := m.builds[i]
		var rowBuffer bytes.Buffer

		// Check if this is the selected row
		isSelected := i == m.cursor

		// Start collecting row content in a buffer
		for colIdx, col := range columns {
			if col.visible {
				var cellContent string

				switch col.key {
				case "Version":
					cellContent = build.Version
				case "Status":
					// Check if this build is currently downloading
					// Using Version + Hash as a unique identifier
					buildID := build.Version + "-" + build.Hash
					if state, exists := m.downloadStates[buildID]; exists && (state.BuildState == types.StateDownloading || state.BuildState == types.StateExtracting) {
						// For downloading builds, show progress
						if state.BuildState == types.StateDownloading {
							// Show download progress with percentage
							if state.Total > 0 {
								percent := (float64(state.Current) / float64(state.Total)) * 100
								speed := float64(0)
								if !state.StartTime.IsZero() {
									elapsedSecs := time.Since(state.StartTime).Seconds()
									if elapsedSecs > 0 {
										speed = float64(state.Current) / elapsedSecs
									}
								}
								cellContent = fmt.Sprintf("%.1f%% (%.1f MB/s)", percent, speed/1024/1024)
							} else {
								cellContent = "Downloading..."
							}
						} else if state.BuildState == types.StateExtracting {
							cellContent = "Extracting..."
						}
					} else {
						// For non-downloading builds, show the regular state
						cellContent = buildStateToString(build.Status)
					}
				case "Branch":
					cellContent = build.Branch
				case "Type":
					cellContent = build.ReleaseCycle
				case "Hash":
					// Truncate hash for display
					if len(build.Hash) > 8 {
						cellContent = build.Hash[:8]
					} else {
						cellContent = build.Hash
					}
				case "Size":
					cellContent = formatByteSize(build.Size)
				case "Build Date":
					// Get timestamp as proper time.Time
					buildTime := build.BuildDate.Time()
					if !buildTime.IsZero() {
						cellContent = buildTime.Format("2006-01-02")
					} else {
						cellContent = "Unknown"
					}
				}

				// Apply styling to cell content
				rowBuffer.WriteString(col.style(cellContent))

				// Add space between columns
				if colIdx < len(columns)-1 {
					rowBuffer.WriteString(" ")
				}
			}
		}

		// Apply row style (selected or normal)
		if isSelected {
			output.WriteString(selectedRowStyle.Render(rowBuffer.String()))
		} else {
			output.WriteString(regularRowStyle.Render(rowBuffer.String()))
		}
		output.WriteString("\n")
	}

	// If there are more items below what we're showing, add an indicator
	if len(m.builds) > endIdx {
		moreIndicator := fmt.Sprintf(" ↓ %d more ", len(m.builds)-endIdx)
		output.WriteString(lp.NewStyle().
			Foreground(lp.Color(colorInfo)).
			Align(lp.Right).
			Width(m.terminalWidth).
			Render(moreIndicator))
	}

	// If there are items above what we're showing, add an indicator
	if m.scrollOffset > 0 {
		moreIndicator := fmt.Sprintf(" ↑ %d more ", m.scrollOffset)
		// Replace the first line with the indicator
		lines := strings.Split(output.String(), "\n")
		if len(lines) > 0 {
			lines[0] = lp.NewStyle().
				Foreground(lp.Color(colorInfo)).
				Align(lp.Left).
				Width(m.terminalWidth).
				Render(moreIndicator)
			output.Reset()
			output.WriteString(strings.Join(lines, "\n"))
		}
	}

	return output.String()
}
