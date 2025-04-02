package tui

import (
	"bytes"
	"strings"

	lp "github.com/charmbracelet/lipgloss"
)

// renderBuildHeader renders the header for the build list view
func (m Model) renderBuildHeader() string {
	var b strings.Builder

	// Title bar
	headerText := "TUI Blender Launcher"
	b.WriteString(headerStyle.Width(m.terminalWidth).AlignHorizontal(lp.Center).Render(headerText))
	b.WriteString("\n")

	// Column headers - create the header row with column names
	headerRow := bytes.Buffer{}

	// Define column order, widths, visibility and labels
	columns := []struct {
		name    string
		width   int
		visible bool
		index   int
	}{
		{name: "Blender", width: columnConfigs["Version"].width, visible: true, index: 0}, // Always show Version
		{name: "Status", width: columnConfigs["Status"].width, visible: true, index: 1},   // Always show Status
		{name: "Branch", width: columnConfigs["Branch"].width, visible: m.visibleColumns["Branch"], index: 2},
		{name: "Type", width: columnConfigs["Type"].width, visible: m.visibleColumns["Type"], index: 3},
		{name: "Hash", width: columnConfigs["Hash"].width, visible: m.visibleColumns["Hash"], index: 4},
		{name: "Size", width: columnConfigs["Size"].width, visible: m.visibleColumns["Size"], index: 5},
		{name: "Build Date", width: columnConfigs["Build Date"].width, visible: m.visibleColumns["Build Date"], index: 6},
	}

	// Ensure Version and Status are always visible regardless of terminal width
	columns[0].visible = true
	columns[1].visible = true

	// After the columns slice is defined, insert the following code block:

	for i := range columns {
		if columns[i].width == 0 {
			switch columns[i].name {
			case "Blender":
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

	// This block ensures that each column gets a default width if its computed width is zero

	// Add header columns with sort indicators - use bold style to make them more visible
	for i, col := range columns {
		if col.visible {
			// Get the header label with sort indicator
			colTitle := getSortIndicator(m, col.index, col.name)
			// Ensure the cell width is at least as wide as the header label
			displayWidth := col.width
			if len(colTitle) > displayWidth {
				displayWidth = len(colTitle)
			}

			switch i {
			case 0: // Version - left align
				headerRow.WriteString(cellStyleLeft.Copy().Bold(true).Width(displayWidth).Render(colTitle))
			case 5: // Size - right align
				headerRow.WriteString(cellStyleRight.Copy().Bold(true).Width(displayWidth).Render(colTitle))
			case 6: // Build Date - center align
				headerRow.WriteString(cellStyleCenter.Copy().Bold(true).Width(displayWidth).Render(colTitle))
			default: // Others - center align
				headerRow.WriteString(cellStyleCenter.Copy().Bold(true).Width(displayWidth).Render(colTitle))
			}

			// Add a space between columns if not the last column
			if i < len(columns)-1 {
				headerRow.WriteString(" ")
			}
		}
	}

	// Ensure the header row is rendered - make it bold and with a higher contrast background
	headerBgStyle := lp.NewStyle().Background(lp.Color("236")).Bold(true).Width(m.terminalWidth)
	b.WriteString(headerBgStyle.Render(headerRow.String()))
	b.WriteString("\n")

	return b.String()
}

// renderSettingsHeader renders the header for the settings view
func (m Model) renderSettingsHeader() string {
	var b strings.Builder

	// Header - Match the same style as the build list view
	headerText := "TUI Blender Launcher - Settings"
	b.WriteString(headerStyle.Width(m.terminalWidth).AlignHorizontal(lp.Center).Render(headerText))
	b.WriteString("\n")

	return b.String()
}
