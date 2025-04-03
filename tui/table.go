package tui

import (
	"TUI-Blender-Launcher/model"
	"bytes"
	"strings"

	lp "github.com/charmbracelet/lipgloss"
)

// Row represents a single row in the builds table
type Row struct {
	Build      model.BlenderBuild
	IsSelected bool
	Status     *model.DownloadState
}

// NewRow creates a new row instance from a build
func NewRow(build model.BlenderBuild, isSelected bool, status *model.DownloadState) Row {
	return Row{
		Build:      build,
		IsSelected: isSelected,
		Status:     status,
	}
}

// Render renders a single row with the given column configuration
func (r Row) Render(columns []ColumnConfig) string {
	var cells []string
	for _, col := range columns {
		if col.Visible {
			var cellContent string
			switch col.Key {
			case "Version":
				cellContent = r.Build.Version
			case "Status":
				cellContent = r.renderStatus()
			case "Branch":
				cellContent = r.Build.Branch
			case "Type":
				cellContent = r.Build.ReleaseCycle
			case "Hash":
				cellContent = r.Build.Hash
			case "Size":
				cellContent = model.FormatByteSize(r.Build.Size)
			case "Build Date":
				cellContent = model.FormatBuildDate(r.Build.BuildDate)
			}
			cells = append(cells, col.Style(cellContent))
		}
	}
	rowString := lp.JoinHorizontal(lp.Left, cells...)
	if r.IsSelected {
		return selectedRowStyle.Render(rowString)
	}
	return regularRowStyle.Render(rowString)
}

// renderStatus renders the status cell with appropriate formatting
// This is where download and extraction progress is displayed
func (r Row) renderStatus() string {
	return FormatBuildStatus(r.Build.Status, r.Status)
}

// ColumnConfig represents the configuration for a table column
type ColumnConfig struct {
	Name    string
	Key     string
	Visible bool
	Width   int
	Index   int
	Style   func(string) string
}

// Updated GetBuildColumns to accept terminalWidth and compute widths
func GetBuildColumns(visibleColumns map[string]bool, terminalWidth int) []ColumnConfig {
	var cellStyleCenter = lp.NewStyle().Align(lp.Center)
	columns := []ColumnConfig{
		{Name: "Version", Key: "Version", Visible: true, Index: 0},
		{Name: "Status", Key: "Status", Visible: true, Index: 1},
		{Name: "Branch", Key: "Branch", Visible: true, Index: 2},
		{Name: "Type", Key: "Type", Visible: true, Index: 3},
		{Name: "Hash", Key: "Hash", Visible: true, Index: 4},
		{Name: "Size", Key: "Size", Visible: true, Index: 5},
		{Name: "Build Date", Key: "Build Date", Visible: true, Index: 6},
	}
	// Compute total flex for all columns
	totalFlex := 0.0
	for i := range columns {
		totalFlex += columnConfigs[columns[i].Key].flex
	}
	// Assign each column a width proportional to its flex value
	for i := range columns {
		flex := columnConfigs[columns[i].Key].flex
		colWidth := int((float64(terminalWidth) * flex) / totalFlex)
		columns[i].Width = colWidth
		columns[i].Style = func(width int) func(string) string {
			return func(s string) string {
				return cellStyleCenter.Width(width).Render(s)
			}
		}(colWidth)
	}
	return columns
}

// Update RenderRows to pass terminalWidth
func RenderRows(m *Model) string {
	var output bytes.Buffer

	// Get column configuration with computed widths
	columns := GetBuildColumns(m.visibleColumns, m.terminalWidth)

	// Render each row
	for i, build := range m.builds {
		// Create a buildID to check for download state
		buildID := build.Version
		if build.Hash != "" {
			buildID = build.Version + "-" + build.Hash[:8]
		}

		// Get download state if exists
		var downloadState *model.DownloadState = m.commands.downloads.GetState(buildID)

		// Create and render row; highlight if this is the current row
		row := NewRow(build, i == m.cursor, downloadState)
		output.WriteString(row.Render(columns))
		output.WriteString("\n")
	}

	return output.String()
}

// Update renderBuildContent to pass terminalWidth
func (m *Model) renderBuildContent(availableHeight int) string {
	var output bytes.Buffer

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
		var msg string = "No Blender builds found locally or online."

		return lp.Place(
			m.terminalWidth,
			availableHeight,
			lp.Center,
			lp.Top,
			lp.NewStyle().Foreground(lp.Color(colorInfo)).Render(msg),
		)
	}

	// Build header row using lipgloss.JoinHorizontal
	columns := GetBuildColumns(m.visibleColumns, m.terminalWidth)
	var headerCells []string
	for _, col := range columns {
		if col.Visible {
			headerText := col.Name

			// Add sort indicators for the currently sorted column
			if col.Index == m.sortColumn {
				if m.sortReversed {
					headerText += " ↓"
				} else {
					headerText += " ↑"
				}
			}
			headerContent := lp.NewStyle().Bold(true).Render(headerText)
			headerCells = append(headerCells, col.Style(headerContent))
		}
	}
	// Join header cells horizontally
	headerRow := lp.JoinHorizontal(lp.Left, headerCells...)
	if !strings.HasSuffix(headerRow, "\n") {
		headerRow += "\n"
	}
	output.WriteString(headerRow)

	// Render all rows without scrolling
	rowsContent := RenderRows(m)
	output.WriteString(rowsContent)

	// Force the table content to span the entire terminal width
	finalOutput := lp.NewStyle().Width(m.terminalWidth).Render(output.String())
	return lp.Place(m.terminalWidth, availableHeight, lp.Left, lp.Top, finalOutput)
}

// updateSortColumn handles lateral key events for sorting columns.
// It updates the Model's sortColumn value based on the key pressed.
// Allowed values range from 0 (Version) to 6 (Build Date).
func (m *Model) updateSortColumn(key string) {
	switch key {
	case "left":
		if m.sortColumn > 0 {
			m.sortColumn--
		}
	case "right":
		if m.sortColumn < 6 { // total columns are 7 (0 to 6)
			m.sortColumn++
		}
	}
}
