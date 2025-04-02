package tui

import (
	"bytes"
	"fmt"
	"strings"

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
			lp.Top,
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
			lp.Top,
			lp.NewStyle().Foreground(lp.Color(colorInfo)).Render(msg),
		)
	}

	var output bytes.Buffer

	// Add the table header to the top of the table
	output.WriteString(m.renderBuildTableHeader())

	// Add a line separator between header and content
	separator := strings.Repeat("─", m.terminalWidth)
	output.WriteString(lp.NewStyle().
		Foreground(lp.Color("240")).
		Render(separator) + "\n")

	// Calculate how many builds we can show in the available space
	// Subtract 4 for the top/bottom padding, table header, and separator line
	maxShownBuilds := availableHeight - 4

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

	// Render all rows using our new row.go functionality
	rowsContent := RenderRows(m, maxShownBuilds)
	output.WriteString(rowsContent)

	// If there are more items below what we're showing, add an indicator
	if len(m.builds) > endIdx {
		moreIndicator := fmt.Sprintf(" ↓ %d more ", len(m.builds)-endIdx)
		output.WriteString(lp.NewStyle().
			Foreground(lp.Color(colorInfo)).
			Align(lp.Right).
			Width(m.terminalWidth).
			Render(moreIndicator))
	}

	return lp.Place(m.terminalWidth, availableHeight, lp.Left, lp.Top, output.String())
}
