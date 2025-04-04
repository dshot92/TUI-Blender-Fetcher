package tui

import (
	"TUI-Blender-Launcher/model"
	"fmt"
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

// Column configuration
type columnConfig struct {
	width    int
	priority int     // Lower number = higher priority (will be shown first)
	flex     float64 // Flex ratio for dynamic width calculation
}

// Column configurations
var (
	// Column configurations with priorities and flex values
	columnConfigs = map[string]columnConfig{
		"Version":    {width: 0, priority: 1, flex: 1.0}, // Version gets more space
		"Status":     {width: 0, priority: 2, flex: 1.0}, // Status needs room for different states
		"Branch":     {width: 0, priority: 5, flex: 1.0},
		"Type":       {width: 0, priority: 4, flex: 1.0},
		"Hash":       {width: 0, priority: 6, flex: 1.0},
		"Size":       {width: 0, priority: 7, flex: 1.0},
		"Build Date": {width: 0, priority: 3, flex: 1.0},
	}
)

// Render renders a single row with the given column configuration
func (r Row) Render(columns []ColumnConfig) string {
	var cells []string

	// Special handling for downloads and extractions
	isDownloading := r.Build.Status == model.StateDownloading && r.Status != nil
	isExtracting := r.Build.Status == model.StateExtracting && r.Status != nil

	// Handle special case for download/extract - we'll render empty cells for Type, Hash, Size, Build Date
	// and only display content in Version, Status, and Branch columns
	if isDownloading || isExtracting {
		for _, col := range columns {
			var cellContent string

			switch col.Key {
			case "Version":
				cellContent = r.Build.Version
			case "Status":
				if isDownloading {
					cellContent = model.StateDownloading.String()
				} else if isExtracting {
					cellContent = model.StateExtracting.String()
				}
			case "Branch":
				// Show download speed in Branch column when downloading
				if isDownloading && r.Status.Speed > 0 {
					cellContent = fmt.Sprintf("%.1f MB/s", r.Status.Speed/1024/1024)
				} else if isExtracting {
					// Show percentage in Branch column for extraction
					cellContent = fmt.Sprintf("%.1f%%", r.Status.Progress*100)
				}
			case "Type", "Hash", "Size", "Build Date":
				// These columns will be replaced by progress bar
				cellContent = ""
			}

			cells = append(cells, col.Style(cellContent))
		}
	} else {
		// Normal rendering for non-downloading builds
		for _, col := range columns {
			var cellContent string
			switch col.Key {
			case "Version":
				cellContent = r.Build.Version
			case "Status":
				cellContent = r.Build.Status.String()
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

	// Join cells horizontally for the row
	rowString := lp.JoinHorizontal(lp.Left, cells...)

	// Apply a progress bar for downloading/extracting across Type to Build Date columns
	if (isDownloading || isExtracting) && r.Status != nil {
		// Find the beginning of the Type column
		typeColIndex := -1
		typePosition := 0

		// Calculate the position of where to insert the progress bar
		for i, col := range columns {
			if i < 3 { // Version, Status, Branch columns
				typePosition += col.Width
			}
			if col.Key == "Type" {
				typeColIndex = i
				break
			}
		}

		if typeColIndex >= 0 {
			// Calculate progress bar width - rest of the columns
			progressBarWidth := 0
			for i := typeColIndex; i < len(columns); i++ {
				progressBarWidth += columns[i].Width
			}

			// Create a progress bar
			progress := r.Status.Progress
			if progress < 0 {
				progress = 0
			}
			if progress > 1 {
				progress = 1
			}

			// Create progress bar visual
			completedWidth := int(float64(progressBarWidth) * progress)
			if completedWidth > progressBarWidth {
				completedWidth = progressBarWidth
			}

			remainingWidth := progressBarWidth - completedWidth

			// Create the progress bar with orange color for the completed portion
			progressBar := ""
			if completedWidth > 0 {
				progressBar += lp.NewStyle().
					Background(lp.Color(highlightColor)).
					Foreground(lp.Color(textColor)).
					Width(completedWidth).
					Render("")
			}

			if remainingWidth > 0 {
				progressBar += lp.NewStyle().
					Background(lp.Color(backgroundColor)).
					Width(remainingWidth).
					Render("")
			}

			// Create a new row string with the progress bar inserted at the Type column
			if typePosition < len(rowString) {
				// Replace from Type column onward with progress bar
				rowString = rowString[:typePosition] + progressBar
			}
		}
	}

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

// FormatBuildStatus converts a build state to a human-readable string with proper formatting
// including download progress information if available
func renderStatus(buildState model.BuildState, downloadState *model.DownloadState) string {
	// If there's an active download, show simple status
	if downloadState != nil && (downloadState.BuildState == model.StateDownloading || downloadState.BuildState == model.StateExtracting) {
		if downloadState.BuildState == model.StateDownloading {
			// Show simple "Downloading" status since details are in other columns
			return model.StateDownloading.String()
		} else if downloadState.BuildState == model.StateExtracting {
			// Show simple "Extracting" status since details are in other columns
			return model.StateExtracting.String()
		}
	}

	// For non-downloading builds, show the regular state
	return buildState.String()
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
	newlineStyle := lp.NewStyle().Render("\n")

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
		var downloadState *model.DownloadState = nil

		// Check in current model's download states
		if state, exists := m.downloadStates[buildID]; exists {
			downloadState = state
		} else {
			// Fallback to checking in commands downloads manager
			downloadState = m.commands.downloads.GetState(buildID)
		}

		// Create and render row; highlight if this is the current row
		row := NewRow(build, i == m.cursor, downloadState)
		rowText := row.Render(columns)

		// Ensure each row has proper width
		output.WriteString(rowText)
		if i < endIndex-1 {
			output.WriteString(newlineStyle)
		}
	}

	return output.String()
}

// Update renderBuildContent to pass terminalWidth and handle scrolling
func (m *Model) renderBuildContent(availableHeight int) string {
	var output strings.Builder
	newlineStyle := lp.NewStyle().Render("\n")

	if len(m.builds) == 0 {
		// No builds to display
		var msg string = "No Blender builds found locally or online."

		return lp.Place(
			m.terminalWidth,
			availableHeight,
			lp.Center,
			lp.Top,
			lp.NewStyle().Foreground(lp.Color(highlightColor)).Render(msg),
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
		styledHeader += newlineStyle
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
