package tui

import (
	"TUI-Blender-Launcher/model"
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

	// Join cells horizontally for the row
	rowString := lp.JoinHorizontal(lp.Left, cells...)

	// Apply appropriate style consistently across the entire row
	if r.IsSelected {
		// Use selected style with explicit width to ensure alignment
		return selectedRowStyle.Width(sumColumnWidths(columns)).Render(rowString)
	}
	// Use regular style with explicit width to ensure alignment
	return regularRowStyle.Width(sumColumnWidths(columns)).Render(rowString)
}

// Helper function to calculate the sum of all column widths
func sumColumnWidths(columns []ColumnConfig) int {
	sum := 0
	for _, col := range columns {
		sum += col.Width
	}
	return sum
}

// renderStatus renders the status cell with appropriate formatting
// This is where download and extraction progress is displayed
func (r Row) renderStatus() string {
	return FormatBuildStatus(r.Build.Status, r.Status)
}

// ColumnConfig represents the configuration for a table column
type ColumnConfig struct {
	Name  string
	Key   string
	Width int
	Index int
	Style func(string) string
}

// Updated GetBuildColumns to accept terminalWidth and compute widths
func GetBuildColumns(terminalWidth int) []ColumnConfig {
	var cellStyleCenter = lp.NewStyle().Align(lp.Center)
	columns := []ColumnConfig{
		{Name: "Version", Key: "Version", Index: 0},
		{Name: "Status", Key: "Status", Index: 1},
		{Name: "Branch", Key: "Branch", Index: 2},
		{Name: "Type", Key: "Type", Index: 3},
		{Name: "Hash", Key: "Hash", Index: 4},
		{Name: "Size", Key: "Size", Index: 5},
		{Name: "Build Date", Key: "Build Date", Index: 6},
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

// Update RenderRows to pass terminalWidth and respect visibleRowsCount
func RenderRows(m *Model, visibleRowsCount int) string {
	var output strings.Builder

	// Get column configuration with computed widths
	columns := GetBuildColumns(m.terminalWidth)

	// Calculate visible range
	endIndex := m.startIndex + visibleRowsCount
	if endIndex > len(m.builds) {
		endIndex = len(m.builds)
	}

	// Only render rows in the visible range
	for i := m.startIndex; i < endIndex; i++ {
		build := m.builds[i]

		// Create a buildID to check for download state
		buildID := build.Version
		if build.Hash != "" {
			buildID = build.Version + "-" + build.Hash[:8]
		}

		// Get download state if exists
		var downloadState *model.DownloadState = m.commands.downloads.GetState(buildID)

		// Create and render row; highlight if this is the current row
		row := NewRow(build, i == m.cursor, downloadState)
		rowText := row.Render(columns)

		// Ensure each row has proper width
		output.WriteString(rowText)
		output.WriteString("\n")
	}

	return output.String()
}

// Update renderBuildContent to pass terminalWidth and handle scrolling
func (m *Model) renderBuildContent(availableHeight int) string {
	var output strings.Builder

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

	// Get column configuration with computed widths
	columns := GetBuildColumns(m.terminalWidth)

	// Build table header row first (without styling yet)
	var headerCells []string
	for _, col := range columns {
		headerText := col.Name
		if col.Index == m.sortColumn {
			if m.sortReversed {
				headerText += " ↓"
			} else {
				headerText += " ↑"
			}
		}
		// Use base styling first, add bold/underline separately
		headerContent := headerText
		headerCells = append(headerCells, col.Style(headerContent))
	}

	// Join header cells horizontally
	headerRow := lp.JoinHorizontal(lp.Left, headerCells...)

	// Now apply bold and underline to the entire row to keep alignment consistent
	styledHeader := lp.NewStyle().Bold(true).Underline(true).Render(headerRow)
	if !strings.HasSuffix(styledHeader, "\n") {
		styledHeader += "\n"
	}

	// Add the styled header to output
	output.WriteString(styledHeader)

	// Calculate how many rows can be displayed in the available height
	// Subtract 1 for the header row
	visibleRowsCount := availableHeight - 1
	if visibleRowsCount < 1 {
		visibleRowsCount = 1
	}

	// Render visible rows with scrolling
	rowsContent := RenderRows(m, visibleRowsCount)
	output.WriteString(rowsContent)

	// Create the final styled table with proper width
	finalOutput := lp.NewStyle().Width(m.terminalWidth).Render(output.String())

	return finalOutput
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
		// Use columnConfigs map to determine total column count
		if m.sortColumn < len(columnConfigs)-1 {
			m.sortColumn++
		}
	}
}
