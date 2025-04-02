package tui

import (
	"bytes"
	"strings"

	lp "github.com/charmbracelet/lipgloss"
)

// Modified renderTitleHeader to use a default width if m.terminalWidth is not set
func (m Model) renderTitleHeader(text string) string {
	width := m.terminalWidth
	// if width <= 0 {
	// 	width = 80 // default width
	// }
	// if width < len(text) {
	// 	width = len(text) + 4 // add some padding
	// }
	return headerStyle.Width(width).AlignHorizontal(lp.Center).Render(text) + "\n"
}

// renderCommonHeader returns the common header (title) for both builds and settings pages.
func (m Model) renderCommonHeader() string {
	return m.renderTitleHeader("TUI Blender Launcher")
}

// renderBuildTableHeader returns the table header portion for the builds page.
func (m Model) renderBuildTableHeader() string {
	// Column headers - create the header row with column names
	headerRow := bytes.Buffer{}

	// Define column order, widths, visibility and labels
	columns := []struct {
		name    string
		width   int
		visible bool
		index   int
	}{
		{name: "Version", width: columnConfigs["Version"].width, visible: true, index: 0}, // Always show Version
		{name: "Status", width: columnConfigs["Status"].width, visible: true, index: 1},   // Always show Status
		{name: "Branch", width: columnConfigs["Branch"].width, visible: true, index: 2},
		{name: "Type", width: columnConfigs["Type"].width, visible: true, index: 3},
		{name: "Hash", width: columnConfigs["Hash"].width, visible: true, index: 4},
		{name: "Size", width: columnConfigs["Size"].width, visible: true, index: 5},
		{name: "Build Date", width: columnConfigs["Build Date"].width, visible: true, index: 6},
	}

	// Ensure Version and Status are always visible regardless of terminal width
	columns[0].visible = true
	columns[1].visible = true

	// Set default width if zero
	for i := range columns {
		if columns[i].width == 0 {
			switch columns[i].name {
			case "Version":
				columns[i].width = columnConfigs["Version"].minWidth
			case "Status":
				columns[i].width = columnConfigs["Status"].minWidth
			case "Branch":
				columns[i].width = columnConfigs["Branch"].minWidth
			case "Type":
				columns[i].width = columnConfigs["Type"].minWidth
			case "Hash":
				columns[i].width = columnConfigs["Hash"].minWidth
			case "Size":
				columns[i].width = columnConfigs["Size"].minWidth
			case "Build Date":
				columns[i].width = columnConfigs["Build Date"].minWidth
			}
		}
	}

	// Add header columns with sort indicators - use bold style to make them more visible
	for i, col := range columns {
		if col.visible {
			colTitle := getSortIndicator(m, col.index, col.name)
			if colTitle == "" {
				colTitle = col.name
			}
			displayWidth := col.width
			if len(colTitle) > displayWidth {
				displayWidth = len(colTitle)
			}

			switch i {
			case 0: // Blender - left align
				headerRow.WriteString(cellStyleLeft.Copy().Bold(true).Width(displayWidth).Render(colTitle))
			case 5: // Size - right align
				headerRow.WriteString(cellStyleRight.Copy().Bold(true).Width(displayWidth).Render(colTitle))
			case 6: // Build Date - center align
				headerRow.WriteString(cellStyleCenter.Copy().Bold(true).Width(displayWidth).Render(colTitle))
			default: // Others - center align
				headerRow.WriteString(cellStyleCenter.Copy().Bold(true).Width(displayWidth).Render(colTitle))
			}

			if i < len(columns)-1 {
				headerRow.WriteString(" ")
			}
		}
	}

	return headerRow.String() + "\n" + lp.NewStyle().Foreground(lp.Color("240")).Render(strings.Repeat("─", m.terminalWidth))
}
