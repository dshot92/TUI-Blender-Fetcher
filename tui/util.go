package tui

import (
	"TUI-Blender-Launcher/model"
	"sort"

	"github.com/mattn/go-runewidth"
)

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
	// Minimum column widths for readability
	minColumnWidths := map[string]int{
		"Version":    12, // Need space for "Blender X.Y.Z"
		"Status":     8,  // Status messages like "Local", "Online"
		"Branch":     6,  // Branch names
		"Type":       8,  // Types like "stable", "alpha"
		"Hash":       10, // Commit hashes
		"Size":       8,  // File sizes
		"Build Date": 10, // Dates (YYYY-MM-DD)
	}

	// All possible columns in priority order (lower index = higher priority)
	allColumns := []struct {
		name     string
		priority int
	}{
		{"Version", 1},
		{"Status", 2},
		{"Branch", 3},
		{"Build Date", 4},
		{"Type", 5},
		{"Size", 6},
		{"Hash", 7},
	}

	// Initialize configs with minimum width values
	newConfigs := make(map[string]columnConfig)
	for _, col := range allColumns {
		newConfigs[col.name] = columnConfig{
			width:    minColumnWidths[col.name],
			priority: col.priority,
			visible:  false,
		}
	}

	// Always make Version and Status visible regardless of terminal width
	newConfigs["Version"] = columnConfig{
		width:    minColumnWidths["Version"],
		priority: 1,
		visible:  true,
	}
	newConfigs["Status"] = columnConfig{
		width:    minColumnWidths["Status"],
		priority: 2,
		visible:  true,
	}

	// Calculate minimum required width for a functional table
	// Start with Version and Status already visible
	remainingWidth := terminalWidth - minColumnWidths["Version"] - minColumnWidths["Status"] - 1 // -1 for gap between them

	// Sort all columns by priority, skipping Version and Status which are already handled
	sortedColumns := make([]string, 0, len(allColumns)-2)
	for _, col := range allColumns {
		if col.name != "Version" && col.name != "Status" {
			sortedColumns = append(sortedColumns, col.name)
		}
	}
	sort.Slice(sortedColumns, func(i, j int) bool {
		return newConfigs[sortedColumns[i]].priority < newConfigs[sortedColumns[j]].priority
	})

	// Start adding columns by priority until we run out of space
	visibleCount := 2 // Version and Status are already visible
	for _, name := range sortedColumns {
		colWidth := minColumnWidths[name]

		// For each column we need its width plus one space for the gap
		neededWidth := colWidth + 1 // Always add gap width after the first two columns

		// Check if this column fits
		if remainingWidth >= neededWidth {
			// It fits, mark it visible
			config := newConfigs[name]
			config.visible = true
			config.width = colWidth
			newConfigs[name] = config

			remainingWidth -= neededWidth
			visibleCount++
		} else {
			// No more space - this and remaining columns stay hidden
			break
		}
	}

	// Now distribute any remaining width to make visible columns wider
	if remainingWidth > 0 && visibleCount > 0 {
		// Get list of visible columns
		visibleCols := []string{"Version", "Status"}
		for _, name := range sortedColumns {
			if newConfigs[name].visible {
				visibleCols = append(visibleCols, name)
			}
		}

		// Calculate equal distribution
		extraPerCol := remainingWidth / visibleCount
		remainder := remainingWidth % visibleCount

		// First pass: give each column its fair share
		for _, name := range visibleCols {
			config := newConfigs[name]
			config.width += extraPerCol
			newConfigs[name] = config
		}

		// Second pass: distribute remainder from highest to lowest priority
		for i := 0; i < remainder && i < len(visibleCols); i++ {
			config := newConfigs[visibleCols[i]]
			config.width++
			newConfigs[visibleCols[i]] = config
		}
	}

	// Build visibility map for return
	visible := make(map[string]bool)
	for name, config := range newConfigs {
		visible[name] = config.visible
	}

	// Ensure Version and Status are always visible
	visible["Version"] = true
	visible["Status"] = true

	// Update global config
	columnConfigs = newConfigs

	return visible
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
