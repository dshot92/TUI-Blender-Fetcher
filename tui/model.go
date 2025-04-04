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
	startIndex       int // Added: tracks the first visible row when scrolling
	config           config.Config
	err              error
	terminalWidth    int
	terminalHeight   int // Added: stores the terminal height for better layout control
	sortColumn       int
	sortReversed     bool
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
	}

	return m
}

// UpdateWindowSize updates the terminal dimensions and recalculates layout
func (m *Model) UpdateWindowSize(width, height int) {
	m.terminalWidth = width
	m.terminalHeight = height
}

func (m *Model) View() string {
	// Render the page using the custom render function.
	return m.renderPageForView()
}
