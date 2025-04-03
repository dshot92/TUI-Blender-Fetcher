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
	var rowBuffer bytes.Buffer

	// Render each cell in the row
	for colIdx, col := range columns {
		if col.Visible {
			var cellContent string

			// Determine cell content based on column key
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

			// Apply column-specific style to cell content
			rowBuffer.WriteString(col.Style(cellContent))

			// Add space between columns
			if colIdx < len(columns)-1 {
				rowBuffer.WriteString(" ")
			}
		}
	}

	// Apply row styling (selected or regular)
	if r.IsSelected {
		return selectedRowStyle.Render(rowBuffer.String())
	}
	return regularRowStyle.Render(rowBuffer.String())
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

// GetBuildColumns returns the column configuration for the build table
func GetBuildColumns(visibleColumns map[string]bool) []ColumnConfig {
	var cellStyleCenter = lp.NewStyle().Align(lp.Center)
	return []ColumnConfig{
		{
			Name:    "Version",
			Key:     "Version",
			Visible: true,
			Width:   columnConfigs["Version"].width,
			Index:   0,
			Style: func(s string) string {
				return cellStyleCenter.Width(columnConfigs["Version"].width).Render(s)
			},
		},
		{
			Name:    "Status",
			Key:     "Status",
			Visible: true,
			Width:   columnConfigs["Status"].width,
			Index:   1,
			Style: func(s string) string {
				return cellStyleCenter.Width(columnConfigs["Status"].width).Render(s)
			},
		},
		{
			Name:    "Branch",
			Key:     "Branch",
			Visible: true, // Always visible
			Width:   columnConfigs["Branch"].width,
			Index:   2,
			Style: func(s string) string {
				return cellStyleCenter.Width(columnConfigs["Branch"].width).Render(s)
			},
		},
		{
			Name:    "Type",
			Key:     "Type",
			Visible: true, // Always visible
			Width:   columnConfigs["Type"].width,
			Index:   3,
			Style: func(s string) string {
				return cellStyleCenter.Width(columnConfigs["Type"].width).Render(s)
			},
		},
		{
			Name:    "Hash",
			Key:     "Hash",
			Visible: true, // Always visible
			Width:   columnConfigs["Hash"].width,
			Index:   4,
			Style: func(s string) string {
				return cellStyleCenter.Width(columnConfigs["Hash"].width).Render(s)
			},
		},
		{
			Name:    "Size",
			Key:     "Size",
			Visible: true, // Always visible
			Width:   columnConfigs["Size"].width,
			Index:   5,
			Style: func(s string) string {
				return cellStyleCenter.Width(columnConfigs["Size"].width).Render(s)
			},
		},
		{
			Name:    "Build Date",
			Key:     "Build Date",
			Visible: true, // Always visible
			Width:   columnConfigs["Build Date"].width,
			Index:   6,
			Style: func(s string) string {
				return cellStyleCenter.Width(columnConfigs["Build Date"].width).Render(s)
			},
		},
	}
}

// RenderRows renders all rows without scrolling
func RenderRows(m *Model) string {
	var output bytes.Buffer

	// Get column configuration
	columns := GetBuildColumns(m.visibleColumns)

	// Render each row
	for i, build := range m.builds {
		// Create buildID to check for download state
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

// renderBuildContent renders the table content
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

	// Build header row
	var headerBuffer bytes.Buffer
	columns := GetBuildColumns(m.visibleColumns)
	for colIdx, col := range columns {
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
			headerBuffer.WriteString(col.Style(headerContent))

			// Add space between columns
			if colIdx < len(columns)-1 {
				headerBuffer.WriteString(" ")
			}
		}
	}
	headerStr := headerBuffer.String()
	if !strings.HasSuffix(headerStr, "\n") {
		headerStr += "\n"
	}
	output.WriteString(headerStr)

	// Render all rows without scrolling (simple navigation)
	rowsContent := RenderRows(m)
	output.WriteString(rowsContent)

	return lp.Place(m.terminalWidth, availableHeight, lp.Left, lp.Top, output.String())
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
