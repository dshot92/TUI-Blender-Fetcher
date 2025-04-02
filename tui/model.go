package tui

import (
	"TUI-Blender-Launcher/config"
	"TUI-Blender-Launcher/model"
	"TUI-Blender-Launcher/types"
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
)

// Model represents the state of the TUI application.
type Model struct {
	builds           []model.BlenderBuild
	cursor           int
	scrollOffset     int // Tracks the scroll position in the build list
	visibleRows      int // Number of rows that can be displayed at once
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
	downloadStates   map[string]*DownloadState // Key is now buildID instead of just version
	downloadMutex    sync.Mutex
	blenderRunning   string
	oldBuildsCount   int
	oldBuildsSize    int64
	deleteCandidate  string // Version of build being considered for deletion
	activeDownloadID string // Store the active download build ID for tracking
}

// DownloadState holds progress info for an active download
type DownloadState struct {
	BuildID       string           // Unique identifier for build (version + hash)
	Progress      float64          // Progress from 0.0 to 1.0
	Current       int64            // Bytes downloaded so far (renamed from CurrentBytes)
	Total         int64            // Total bytes to download (renamed from TotalBytes)
	Speed         float64          // Download speed in bytes/sec
	BuildState    types.BuildState // Changed from Message to BuildState
	LastUpdated   time.Time        // Timestamp of last progress update
	StartTime     time.Time        // When the download started
	StallDuration time.Duration    // How long download can stall before timeout
	CancelCh      chan struct{}    // Per-download cancel channel
}

// InitialModel creates the initial state of the TUI model.
func InitialModel(cfg config.Config, needsSetup bool) Model {
	// Configure the progress bar with fixed settings for consistent column display
	progModel := progress.New(
		progress.WithGradient("#FFAA00", "#FFD700"), // Orange gradient (was blue)
		progress.WithoutPercentage(),                // No percentage display
		progress.WithWidth(30),                      // Even wider progress bar
		progress.WithSolidFill("#FFAA00"),           // Orange fill for visibility (was blue)
	)

	m := Model{
		config:         cfg,
		isLoading:      !needsSetup,
		downloadStates: make(map[string]*DownloadState),
		progressBar:    progModel,
		sortColumn:     0,     // Default sort by Version
		sortReversed:   true,  // Default descending sort (newest versions first)
		blenderRunning: "",    // No Blender running initially
		editMode:       false, // Start in navigation mode, not edit mode
	}

	if needsSetup {
		m.currentView = viewInitialSetup
		m.settingsInputs = make([]textinput.Model, 3)
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

		// Manual Fetch input
		t = textinput.New()
		t.Placeholder = "true/false"
		t.SetValue(fmt.Sprintf("%t", cfg.ManualFetch))
		t.CharLimit = 5
		t.Width = 50
		m.settingsInputs[2] = t

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
