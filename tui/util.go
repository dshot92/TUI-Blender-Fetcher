package tui

import (
	"TUI-Blender-Launcher/model"
	"TUI-Blender-Launcher/types"
	"fmt"
	"sort"

	"github.com/mattn/go-runewidth"
)

// formatByteSize converts bytes to human-readable sizes
func formatByteSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// buildStateToString converts build state to display string
func buildStateToString(state types.BuildState) string {
	switch state {
	case types.StateDownloading:
		return "Downloading"
	case types.StateExtracting:
		return "Extracting"
	case types.StatePreparing:
		return "Preparing"
	case types.StateLocal:
		return "Local"
	case types.StateOnline:
		return "Online"
	case types.StateUpdate:
		return "Update"
	case types.StateFailed:
		return "Failed"
	case types.StateNone:
		return "Cancelled"
	default:
		return state.String()
	}
}

// calculateSplitIndex finds the rune index to split a string for a given visual width.
func calculateSplitIndex(s string, targetWidth int) int {
	currentWidth := 0
	for i, r := range s {
		runeWidth := runewidth.RuneWidth(r)
		if currentWidth+runeWidth > targetWidth {
			return i // Split before this rune
		}
		currentWidth += runeWidth
	}
	return len(s) // Target width is >= string width
}

// calculateVisibleColumns determines which columns should be visible based on terminal width
// and calculates appropriate widths to use full available space
func calculateVisibleColumns(terminalWidth int) map[string]bool {
	// Constants
	const columnGap = 1 // Space between columns

	// All possible columns in priority order (lower index = higher priority)
	allColumns := []struct {
		name   string
		config columnConfig
	}{
		{"Version", columnConfigs["Version"]},
		{"Status", columnConfigs["Status"]},
		{"Branch", columnConfigs["Branch"]},
		{"Type", columnConfigs["Type"]},
		{"Hash", columnConfigs["Hash"]},
		{"Size", columnConfigs["Size"]},
		{"Build Date", columnConfigs["Build Date"]},
	}

	// Sort columns by priority
	sort.Slice(allColumns, func(i, j int) bool {
		return allColumns[i].config.priority < allColumns[j].config.priority
	})

	// Initialize visibility map
	visibleColumns := make(map[string]bool)

	// Step 1: Calculate how many columns can fit with their minimum widths
	remainingWidth := terminalWidth
	var visibleCols []string

	// Always include Version and Status
	visibleColumns["Version"] = true
	visibleColumns["Status"] = true
	visibleCols = append(visibleCols, "Version", "Status")

	// Reserve space for the required columns and their gaps
	remainingWidth -= columnConfigs["Version"].minWidth + columnConfigs["Status"].minWidth + columnGap

	// Add additional columns by priority if they fit
	for _, col := range allColumns {
		// Skip already added columns
		if col.name == "Version" || col.name == "Status" {
			continue
		}

		// Check if this column fits
		if remainingWidth >= (col.config.minWidth + columnGap) {
			visibleColumns[col.name] = true
			visibleCols = append(visibleCols, col.name)
			remainingWidth -= (col.config.minWidth + columnGap)
		} else {
			visibleColumns[col.name] = false
		}
	}

	// Step 2: Distribute available width proportionally using flex values
	// Calculate total flex for visible columns
	totalFlex := 0.0
	for _, colName := range visibleCols {
		totalFlex += columnConfigs[colName].flex
	}

	// Calculate exact distributed width (including fractional part)
	availableWidth := terminalWidth - (len(visibleCols)-1)*columnGap
	var distributedWidth float64
	var widthAssignments = make(map[string]float64)

	// First pass: calculate ideal width based on flex proportion
	for _, colName := range visibleCols {
		// Calculate proportional width
		proportion := columnConfigs[colName].flex / totalFlex
		idealWidth := float64(availableWidth) * proportion

		// Ensure minimum width
		if idealWidth < float64(columnConfigs[colName].minWidth) {
			idealWidth = float64(columnConfigs[colName].minWidth)
		}

		widthAssignments[colName] = idealWidth
		distributedWidth += idealWidth
	}

	// Second pass: adjust for integer widths and distribute remaining pixels
	// We need to convert to integers, which might leave some pixels unallocated
	remainingPixels := availableWidth - int(distributedWidth)

	// Update the actual column configs with new widths
	for colName, width := range widthAssignments {
		config := columnConfigs[colName]
		config.width = int(width)

		// Distribute any remaining pixels to columns by priority
		if remainingPixels > 0 {
			config.width++
			remainingPixels--
		}

		// Update column config
		columnConfigs[colName] = config
	}

	return visibleColumns
}

// sortBuilds sorts the builds based on the selected column and sort order
func sortBuilds(builds []model.BlenderBuild, column int, reverse bool) []model.BlenderBuild {
	// Create a copy of builds to avoid modifying the original
	sortedBuilds := make([]model.BlenderBuild, len(builds))
	copy(sortedBuilds, builds)

	// Define sort function type for better organization
	type sortFunc func(a, b model.BlenderBuild) bool

	// Define the sort functions for each column
	sortFuncs := map[int]sortFunc{
		0: func(a, b model.BlenderBuild) bool { // Version
			return a.Version < b.Version
		},
		1: func(a, b model.BlenderBuild) bool { // Status
			return a.Status < b.Status
		},
		2: func(a, b model.BlenderBuild) bool { // Branch
			return a.Branch < b.Branch
		},
		3: func(a, b model.BlenderBuild) bool { // Type/ReleaseCycle
			return a.ReleaseCycle < b.ReleaseCycle
		},
		4: func(a, b model.BlenderBuild) bool { // Hash
			return a.Hash < b.Hash
		},
		5: func(a, b model.BlenderBuild) bool { // Size
			return a.Size < b.Size
		},
		6: func(a, b model.BlenderBuild) bool { // Date
			return a.BuildDate.Time().Before(b.BuildDate.Time())
		},
	}

	// Check if we have a sort function for this column
	if sortFunc, ok := sortFuncs[column]; ok {
		sort.SliceStable(sortedBuilds, func(i, j int) bool {
			// Apply the sort function, handling the reverse flag
			if reverse {
				return !sortFunc(sortedBuilds[i], sortedBuilds[j])
			}
			return sortFunc(sortedBuilds[i], sortedBuilds[j])
		})
	}

	return sortedBuilds
}

// getSortIndicator returns a string indicating the sort direction for a given column
func getSortIndicator(m Model, column int, title string) string {
	if m.sortColumn == column {
		if m.sortReversed {
			return "↓ " + title
		} else {
			return "↑ " + title
		}
	}
	return title
}

// Helper function to check if a column is visible
func isColumnVisible(column int) bool {
	switch column {
	case 0:
		return true // Version is always visible
	case 1:
		return true // Status is always visible
	case 2:
		return columnConfigs["Branch"].visible
	case 3:
		return columnConfigs["Type"].visible
	case 4:
		return columnConfigs["Hash"].visible
	case 5:
		return columnConfigs["Size"].visible
	case 6:
		return columnConfigs["Build Date"].visible
	default:
		return false
	}
}

// Helper function to get the last visible column index
func getLastVisibleColumn() int {
	for i := 6; i >= 0; i-- {
		if isColumnVisible(i) {
			return i
		}
	}
	return 0 // Fallback to first column (should never happen as Version is always visible)
}

// Helper function to count visible columns
func countVisibleColumns(columns []struct {
	name    string
	width   int
	visible bool
	index   int
}) int {
	count := 0
	for _, col := range columns {
		if col.visible {
			count++
		}
	}
	return count
}

// Utility function to create plural words
func pluralize(word string, count int) string {
	if count == 1 {
		return word
	}
	return word + "s"
}
