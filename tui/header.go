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

	// Add header columns with sort indicators - use bold style to make them more visible
	for i, col := range columns {
		if col.visible {
			// Add sort indicator to column name if this is the sort column
			colTitle := getSortIndicator(m, col.index, col.name)

			// Apply style and padding based on column type
			switch i {
			case 0: // Version - left align
				headerRow.WriteString(cellStyleLeft.Copy().Bold(true).Width(col.width).Render(colTitle))
			case 5: // Size - right align
				headerRow.WriteString(cellStyleRight.Copy().Bold(true).Width(col.width).Render(colTitle))
			case 6: // Date - center
				headerRow.WriteString(cellStyleCenter.Copy().Bold(true).Width(col.width).Render(colTitle))
			default: // Others - center
				headerRow.WriteString(cellStyleCenter.Copy().Bold(true).Width(col.width).Render(colTitle))
			}

			// Add space between columns
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
