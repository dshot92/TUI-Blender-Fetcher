package tui

import (
	"TUI-Blender-Launcher/model"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
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
	downloadTickRate    = 100 * time.Millisecond
	downloadStallTime   = 30 * time.Second
	extractionStallTime = 2 * time.Minute
	uiRefreshRate       = 33 * time.Millisecond // How often to refresh the UI without user input (30 FPS)
	footerLineCount     = 2                     // Two line footer: contextual and general commands
)

// View states
type viewState int

const (
	viewList viewState = iota
	viewInitialSetup
	viewSettings
)

// Command types for key bindings
type CommandType int

const (
	CmdQuit CommandType = iota
	CmdShowSettings
	CmdToggleSortOrder
	CmdFetchBuilds
	CmdDownloadBuild
	CmdLaunchBuild
	CmdOpenBuildDir
	CmdDeleteBuild
	CmdMoveUp
	CmdMoveDown
	CmdMoveLeft
	CmdMoveRight
	CmdSaveSettings
	CmdToggleEditMode
	CmdCancelDownload
)

// KeyCommand defines a keyboard command with its key binding and description
type KeyCommand struct {
	Type        CommandType
	Keys        []string
	Description string
}

// Commands mapping for different views
var (
	// Common commands for all views
	CommonCommands = []KeyCommand{
		{Type: CmdQuit, Keys: []string{"q"}, Description: "Quit application"},
	}

	// List view commands
	ListCommands = []KeyCommand{
		{Type: CmdShowSettings, Keys: []string{"s"}, Description: "Show settings"},
		{Type: CmdToggleSortOrder, Keys: []string{"r"}, Description: "Toggle sort order"},
		{Type: CmdFetchBuilds, Keys: []string{"f"}, Description: "Fetch online builds"},
		{Type: CmdDownloadBuild, Keys: []string{"d"}, Description: "Download selected build"},
		{Type: CmdLaunchBuild, Keys: []string{"enter"}, Description: "Launch selected build"},
		{Type: CmdOpenBuildDir, Keys: []string{"o"}, Description: "Open build directory"},
		{Type: CmdDeleteBuild, Keys: []string{"x"}, Description: "Delete build/Cancel download"},
		{Type: CmdMoveUp, Keys: []string{"up", "k"}, Description: "Move cursor up"},
		{Type: CmdMoveDown, Keys: []string{"down", "j"}, Description: "Move cursor down"},
		{Type: CmdMoveLeft, Keys: []string{"left", "h"}, Description: "Previous sort column"},
		{Type: CmdMoveRight, Keys: []string{"right", "l"}, Description: "Next sort column"},
	}

	// Settings view commands
	SettingsCommands = []KeyCommand{
		{Type: CmdSaveSettings, Keys: []string{"s"}, Description: "Save settings and return"},
		{Type: CmdToggleEditMode, Keys: []string{"enter"}, Description: "Toggle edit mode"},
		{Type: CmdMoveUp, Keys: []string{"up", "k"}, Description: "Move cursor up"},
		{Type: CmdMoveDown, Keys: []string{"down", "j"}, Description: "Move cursor down"},
	}
)

// GetKeyBinding returns a tea key binding for the given command type
func GetKeyBinding(cmdType CommandType) key.Binding {
	var keys []string

	// Check in all command sets
	for _, cmd := range CommonCommands {
		if cmd.Type == cmdType {
			keys = cmd.Keys
			break
		}
	}

	if keys == nil {
		for _, cmd := range ListCommands {
			if cmd.Type == cmdType {
				keys = cmd.Keys
				break
			}
		}
	}

	if keys == nil {
		for _, cmd := range SettingsCommands {
			if cmd.Type == cmdType {
				keys = cmd.Keys
				break
			}
		}
	}

	return key.NewBinding(key.WithKeys(keys...))
}

// GetCommandsForView returns all commands available for a specific view
func GetCommandsForView(view viewState) []KeyCommand {
	result := make([]KeyCommand, len(CommonCommands))
	copy(result, CommonCommands)

	switch view {
	case viewList:
		result = append(result, ListCommands...)
	case viewSettings, viewInitialSetup:
		result = append(result, SettingsCommands...)
	}

	return result
}

// IsCommandAvailable checks if a command is available in the current view
func IsCommandAvailable(cmdType CommandType, view viewState) bool {
	for _, cmd := range GetCommandsForView(view) {
		if cmd.Type == cmdType {
			return true
		}
	}
	return false
}

// GenerateHelpText creates a formatted help message showing available keyboard shortcuts
func GenerateHelpText(view viewState) string {
	commands := GetCommandsForView(view)
	helpText := ""

	for i, cmd := range commands {
		// Format keys as a string (e.g. "up/k")
		keyText := cmd.Keys[0]
		for j := 1; j < len(cmd.Keys); j++ {
			keyText += "/" + cmd.Keys[j]
		}

		// Add command description
		helpText += keyText + ": " + cmd.Description

		// Add separator between commands
		if i < len(commands)-1 {
			helpText += " • "
		}
	}

	return helpText
}

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
