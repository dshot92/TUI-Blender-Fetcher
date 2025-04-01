package tui

import (
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

	// Dialog size constants
	deleteDialogWidth  = 50
	cleanupDialogWidth = 60
	quitDialogWidth    = 60

	// Safety limits
	maxTickCounter = 1000 // Maximum ticks to prevent infinite loops

	// Performance constants
	downloadTickRate    = 100 * time.Millisecond // How often to update download progress
	downloadStallTime   = 3 * time.Minute        // Default timeout for detecting stalled downloads
	extractionStallTime = 10 * time.Minute       // Longer timeout for extraction which can take longer

	// Environment variables
	envLaunchVariable = "TUI_BLENDER_LAUNCH"
)

// View states
type viewState int

const (
	viewList viewState = iota
	viewInitialSetup
	viewSettings
	viewDeleteConfirm  // New state for delete confirmation
	viewCleanupConfirm // Confirmation for cleaning up old builds
	viewQuitConfirm    // Confirmation for quitting during download
)

// Column Widths (adjust as needed)
const (
	colWidthVersion = 7  // Just enough for version numbers
	colWidthStatus  = 12 // For status text
	colWidthBranch  = 6  // Just for percentage
	colWidthType    = 12 // Release Cycle
	colWidthHash    = 9  // 8 chars + padding
	colWidthSize    = 10 // For formatted sizes
	colWidthDate    = 10 // YYYY-MM-DD
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
	footerStyle = lp.NewStyle().MarginTop(1).Padding(0, 1).Foreground(lp.Color(colorForeground))
	// Separator style (using box characters)
	separator = lp.NewStyle().SetString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━").Faint(true).String()

	// Define base styles for columns (can be customized further)
	cellStyleCenter = lp.NewStyle().Align(lp.Center)
	cellStyleRight  = lp.NewStyle().Align(lp.Right)
	cellStyleLeft   = lp.NewStyle() // Default
)

// Column configuration
type columnConfig struct {
	width    int
	priority int // Lower number = higher priority (will be shown first)
	visible  bool
}

// Column configurations
var (
	// Column configurations with priorities (lower = higher priority)
	columnConfigs = map[string]columnConfig{
		"Version":    {width: colWidthVersion, priority: 1},
		"Status":     {width: colWidthStatus, priority: 2},
		"Branch":     {width: colWidthBranch, priority: 5},
		"Type":       {width: colWidthType, priority: 4},
		"Hash":       {width: colWidthHash, priority: 6},
		"Size":       {width: colWidthSize, priority: 7},
		"Build Date": {width: colWidthDate, priority: 3},
	}
)
