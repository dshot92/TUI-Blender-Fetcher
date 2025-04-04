package tui

import (
	"github.com/charmbracelet/bubbles/key"
	lp "github.com/charmbracelet/lipgloss"
)

// Constants for UI styling and configuration
const (
	// Color constants
	textColor       = "255" // White for text
	backgroundColor = "24"  // Gray background
	highlightColor  = "12"  // Blue for highlights
	orangeColor     = "208" // Orange for local builds
	greenColor      = "46"  // Green for updated builds
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
	CmdPageUp   // Add PageUp command
	CmdPageDown // Add PageDown command
	CmdHome     // Add Home command
	CmdEnd      // Add End command
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
		{Type: CmdPageUp, Keys: []string{"pgup"}, Description: "Page up"},
		{Type: CmdPageDown, Keys: []string{"pgdown"}, Description: "Page down"},
		{Type: CmdHome, Keys: []string{"home"}, Description: "Go to first item"},
		{Type: CmdEnd, Keys: []string{"end"}, Description: "Go to last item"},
	}

	// Settings view commands
	SettingsCommands = []KeyCommand{
		{Type: CmdSaveSettings, Keys: []string{"s"}, Description: "Save settings and return"},
		{Type: CmdToggleEditMode, Keys: []string{"enter"}, Description: "Toggle edit mode"},
		{Type: CmdMoveUp, Keys: []string{"up", "k"}, Description: "Move cursor up"},
		{Type: CmdMoveDown, Keys: []string{"down", "j"}, Description: "Move cursor down"},
		{Type: CmdMoveLeft, Keys: []string{"left", "h"}, Description: "Select previous option"},
		{Type: CmdMoveRight, Keys: []string{"right", "l"}, Description: "Select next option"},
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

// Styles using lipgloss
var (
	// Style for the selected row
	selectedRowStyle = lp.NewStyle().Background(lp.Color(backgroundColor)).Foreground(lp.Color(textColor)).Align(lp.Left)
	// Style for regular rows (use default)
	regularRowStyle = lp.NewStyle().Align(lp.Left)
	// Footer style - remove margin and use minimal padding
	footerStyle = lp.NewStyle().Padding(0, 0).Foreground(lp.Color(textColor))
	// Define base styles for columns (can be customized further)

)
