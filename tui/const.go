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
	downloadTickRate    = 50 * time.Millisecond // How often to update download progress (faster for smoother UI)
	downloadStallTime   = 30 * time.Second      // How long a download can stall before marking as failed
	extractionStallTime = 120 * time.Second     // Longer timeout for extraction phase
	uiRefreshRate       = 33 * time.Millisecond // How often to refresh the UI without user input (30 FPS)

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
	minWidth int     // Minimum width for the column
	flex     float64 // Flex ratio for dynamic width calculation
}

// Column configurations
var (
	// Column configurations with priorities and flex values
	columnConfigs = map[string]columnConfig{
		"Version":    {width: 0, priority: 1, minWidth: 7, flex: 1.0},  // Version gets more space
		"Status":     {width: 0, priority: 2, minWidth: 12, flex: 1.2}, // Status needs room for different states
		"Branch":     {width: 0, priority: 5, minWidth: 6, flex: 0.8},
		"Type":       {width: 0, priority: 4, minWidth: 10, flex: 1.0},
		"Hash":       {width: 0, priority: 6, minWidth: 9, flex: 0.8},
		"Size":       {width: 0, priority: 7, minWidth: 8, flex: 0.8},
		"Build Date": {width: 0, priority: 3, minWidth: 10, flex: 1.0},
	}
)
