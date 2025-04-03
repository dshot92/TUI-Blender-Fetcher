package tui

import (
	"TUI-Blender-Launcher/model"
	"fmt"
	"time"

	lp "github.com/charmbracelet/lipgloss"
)

// Constants for UI styling and configuration
const (
	// Color constants
	colorSuccess    = "10"  // Green for success states
	colorWarning    = "11"  // Yellow for warnings
	colorInfo       = "12"  // Blue for info
	colorError      = "9"   // Red for errors
	colorNeutral    = "15"  // White for neutral text
	colorBackground = "240" // Gray background
	colorForeground = "255" // White foreground

	// Performance constants
	downloadTickRate    = 50 * time.Millisecond // How often to update download progress (faster for smoother UI)
	downloadStallTime   = 30 * time.Second      // How long a download can stall before marking as failed
	extractionStallTime = 120 * time.Second     // Longer timeout for extraction phase
	uiRefreshRate       = 33 * time.Millisecond // How often to refresh the UI without user input (30 FPS)
)

// View states
type viewState int

const (
	viewList viewState = iota
	viewInitialSetup
	viewSettings
)

// Styles using lipgloss
var (
	// Updated header style to be more visible
	headerStyle = lp.NewStyle().Bold(true).Padding(1, 1).Foreground(lp.Color(colorForeground)).Background(lp.Color("236"))
	// Style for the selected row
	selectedRowStyle = lp.NewStyle().Background(lp.Color(colorBackground)).Foreground(lp.Color(colorForeground))
	// Style for regular rows (use default)
	regularRowStyle = lp.NewStyle()
	// Footer style
	footerStyle = lp.NewStyle().MarginTop(1).Padding(1, 1).Foreground(lp.Color(colorForeground))
	// Define base styles for columns (can be customized further)

)

// Column configuration
type columnConfig struct {
	width    int
	priority int // Lower number = higher priority (will be shown first)
	visible  bool
	minWidth int     // Minimum width for the column
	flex     float64 // Flex ratio for dynamic width calculation
}

// Column configurations
var (
	// Column configurations with priorities and flex values
	columnConfigs = map[string]columnConfig{
		"Version":    {width: 0, priority: 1, minWidth: 7, flex: 1.0},  // Version gets more space
		"Status":     {width: 0, priority: 2, minWidth: 12, flex: 1.0}, // Status needs room for different states
		"Branch":     {width: 0, priority: 5, minWidth: 6, flex: 1.0},
		"Type":       {width: 0, priority: 4, minWidth: 10, flex: 1.0},
		"Hash":       {width: 0, priority: 6, minWidth: 9, flex: 1.0},
		"Size":       {width: 0, priority: 7, minWidth: 8, flex: 1.0},
		"Build Date": {width: 0, priority: 3, minWidth: 10, flex: 1.0},
	}
)

// FormatBuildStatus converts a build state to a human-readable string with proper formatting
// including download progress information if available
func FormatBuildStatus(buildState model.BuildState, downloadState *model.DownloadState) string {
	// If there's an active download, show progress information
	if downloadState != nil && (downloadState.BuildState == model.StateDownloading || downloadState.BuildState == model.StateExtracting) {
		if downloadState.BuildState == model.StateDownloading {
			// Show download progress with percentage and speed
			if downloadState.Total > 0 {
				percent := (float64(downloadState.Current) / float64(downloadState.Total)) * 100
				speed := downloadState.Speed
				if speed == 0 && !downloadState.StartTime.IsZero() {
					elapsedSecs := time.Since(downloadState.StartTime).Seconds()
					if elapsedSecs > 0 {
						speed = float64(downloadState.Current) / elapsedSecs
					}
				}
				return fmt.Sprintf("%.1f%% (%.1f MB/s)", percent, speed/1024/1024)
			}
			return "Downloading..."
		} else if downloadState.BuildState == model.StateExtracting {
			return "Extracting..."
		}
	}

	// For non-downloading builds, show the regular state
	return buildState.String()
}
