package tui

import (
	"TUI-Blender-Launcher/config"
	"TUI-Blender-Launcher/model"
	"sync"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
)

// Model represents the state of the TUI application.
type Model struct {
	builds           []model.BlenderBuild
	cursor           int
	config           config.Config
	err              error
	terminalWidth    int
	terminalHeight   int // Added: stores the terminal height for better layout control
	sortColumn       int
	sortReversed     bool
	isLoading        bool
	visibleColumns   map[string]bool
	currentView      viewState
	focusIndex       int
	editMode         bool
	settingsInputs   []textinput.Model
	progressBar      progress.Model
	commands         *Commands
	blenderRunning   string
	activeDownloadID string // Store the active download build ID for tracking
	downloadMutex    sync.Mutex
	downloadStates   map[string]*model.DownloadState
}

// InitialModel creates the initial state of the TUI model.
func InitialModel(cfg config.Config, needsSetup bool) *Model {
	// Configure the progress bar with fixed settings for consistent column display
	progModel := progress.New(
		progress.WithGradient("#FFAA00", "#FFD700"), // Orange gradient (was blue)
		progress.WithoutPercentage(),                // No percentage display
		progress.WithWidth(30),                      // Even wider progress bar
		progress.WithSolidFill("#FFAA00"),           // Orange fill for visibility (was blue)
	)

	m := &Model{
		config:         cfg,
		isLoading:      !needsSetup,
		commands:       NewCommands(cfg),
		progressBar:    progModel,
		sortColumn:     0,     // Default sort by Version
		sortReversed:   true,  // Default descending sort (newest versions first)
		blenderRunning: "",    // No Blender running initially
		editMode:       false, // Start in navigation mode, not edit mode
		downloadStates: make(map[string]*model.DownloadState),
	}

	if needsSetup {
		m.currentView = viewInitialSetup
		m.settingsInputs = make([]textinput.Model, 2)
		m.editMode = true // Enable edit mode immediately for initial setup

		var t textinput.Model
		// Download Dir input
		t = textinput.New()
		t.Placeholder = cfg.DownloadDir // Show default as placeholder
		t.SetValue(cfg.DownloadDir)     // Set initial value
		t.Focus()
		t.CharLimit = 256
		t.Width = 50
		m.settingsInputs[0] = t

		// Version Filter input (renamed from Cutoff)
		t = textinput.New()
		t.Placeholder = "e.g., 4.0, 3.6 (leave empty for none)"
		t.SetValue(cfg.VersionFilter)
		t.CharLimit = 10
		t.Width = 50
		m.settingsInputs[1] = t

		m.focusIndex = 0 // Start focus on the first input
	} else {
		m.currentView = viewList
		// Start loading local builds immediately
		m.isLoading = true
		// Trigger initial local scan via Init command
	}

	return m
}

// UpdateWindowSize updates the terminal dimensions and recalculates layout
func (m *Model) UpdateWindowSize(width, height int) {
	m.terminalWidth = width
	m.terminalHeight = height

	// Calculate and set column widths based on the terminal width
	calculateColumnWidths(width)
}

// calculateColumnWidths sets the width for each column based on the available terminal width
func calculateColumnWidths(terminalWidth int) {
	// Account for 1 space between each column
	availableWidth := terminalWidth - (len(columnConfigs) - 1)

	// First ensure all columns have at least their minimum width
	totalMinWidth := 0
	for _, config := range columnConfigs {
		totalMinWidth += config.minWidth
	}

	// If we have enough space for all columns at their minimum width
	if availableWidth >= totalMinWidth {
		// Distribute remaining space based on flex values
		remainingWidth := availableWidth - totalMinWidth
		totalFlex := 0.0

		for _, config := range columnConfigs {
			totalFlex += config.flex
		}

		// Set widths based on flex ratios
		for col, config := range columnConfigs {
			extraWidth := int((config.flex / totalFlex) * float64(remainingWidth))
			columnConfigs[col] = columnConfig{
				width:    config.minWidth + extraWidth,
				priority: config.priority,
				visible:  true,
				minWidth: config.minWidth,
				flex:     config.flex,
			}
		}
	} else {
		// Not enough space - just use minimum widths
		for col, config := range columnConfigs {
			columnConfigs[col] = columnConfig{
				width:    config.minWidth,
				priority: config.priority,
				visible:  true,
				minWidth: config.minWidth,
				flex:     config.flex,
			}
		}
	}
}

func (m *Model) View() string {
	// Render the page using the custom render function.
	return m.renderPageForView()
}
