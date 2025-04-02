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

// sortBuilds sorts the builds based on the selected column and sort order
func sortBuilds(builds []model.BlenderBuild, column int, reverse bool) []model.BlenderBuild {
	// Create a copy of builds to avoid modifying the original
	sortedBuilds := make([]model.BlenderBuild, len(builds))
	copy(sortedBuilds, builds)

	// Define sort function type for better organization
	type sortFunc func(a, b model.BlenderBuild) bool

	// Define the sort functions for each column based on the column index
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
		6: func(a, b model.BlenderBuild) bool { // Build Date
			return a.BuildDate.Time().Before(b.BuildDate.Time())
		},
	}

	// Check if we have a sort function for this column
	if sortFunc, ok := sortFuncs[column]; ok {
		sort.SliceStable(sortedBuilds, func(i, j int) bool {
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
