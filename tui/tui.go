package tui

import (
	"TUI-Blender-Launcher/api" // Import the api package
	"TUI-Blender-Launcher/config"
	"TUI-Blender-Launcher/download" // Import download package
	"TUI-Blender-Launcher/local"    // Import local package
	"TUI-Blender-Launcher/model"    // Import the model package
	"TUI-Blender-Launcher/util"     // Import util package
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings" // Import strings
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput" // Import textinput
	tea "github.com/charmbracelet/bubbletea"
	lp "github.com/charmbracelet/lipgloss" // Import lipgloss
	"github.com/mattn/go-runewidth"        // Import runewidth
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

	// Safety limits
	maxTickCounter = 1000 // Maximum ticks to prevent infinite loops

	// Performance constants
	downloadTickRate    = 100 * time.Millisecond // How often to update download progress
	downloadStallTime   = 3 * time.Minute        // Default timeout for detecting stalled downloads
	extractionStallTime = 10 * time.Minute       // Longer timeout for extraction which can take longer

	// Path constants
	launcherPathFile = "blender_launch_command.txt"

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
)

// Define messages for communication between components
// Group related message types together
type (
	// Data update messages
	buildsFetchedMsg struct { // Online builds fetched
		builds []model.BlenderBuild
		err    error // Add error field
	}
	localBuildsScannedMsg struct { // Initial local scan complete
		builds []model.BlenderBuild
		err    error // Include error from scanning
	}
	buildsUpdatedMsg struct { // Builds list updated (e.g., status change)
		builds []model.BlenderBuild
	}
	oldBuildsInfo struct { // Information about old builds
		count int
		size  int64
		err   error
	}

	// Action messages
	startDownloadMsg struct { // Request to start download for a build
		build model.BlenderBuild
	}
	downloadCompleteMsg struct { // Download & extraction finished
		buildVersion  string // Version of the build that finished
		extractedPath string
		err           error
	}
	cleanupOldBuildsMsg struct { // Result of cleaning up old builds
		err error
	}

	// Progress updates
	downloadProgressMsg struct { // Reports download progress
		BuildVersion string // Identifier for the build being downloaded
		CurrentBytes int64
		TotalBytes   int64
		Percent      float64 // Calculated percentage 0.0 to 1.0
		Speed        float64 // Bytes per second
	}

	// Message to reset a build's status after cancellation cleanup
	resetStatusMsg struct {
		version string
	}

	// Error message
	errMsg struct{ err error }

	// Timer message
	tickMsg time.Time
)

// Implement the error interface for errMsg
func (e errMsg) Error() string { return e.err.Error() }

// Column visibility configuration
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
		"Build Date": 10, // Dates
	}

	// All possible columns in priority order (lower index = higher priority)
	allColumns := []struct {
		name     string
		priority int
	}{
		{"Version", 1},
		{"Status", 2},
		{"Build Date", 3},
		{"Type", 4},
		{"Branch", 5},
		{"Hash", 6},
		{"Size", 7},
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

	// Calculate minimum required width for a functional table
	// Start with a clean slate - no columns visible by default

	// First, strictly calculate how many columns can fit
	// Account for column gaps (1 character between each column)
	remainingWidth := terminalWidth

	// Sort all columns by priority
	sortedColumns := make([]string, len(allColumns))
	for i, col := range allColumns {
		sortedColumns[i] = col.name
	}
	sort.Slice(sortedColumns, func(i, j int) bool {
		return newConfigs[sortedColumns[i]].priority < newConfigs[sortedColumns[j]].priority
	})

	// Start adding columns by priority until we run out of space
	visibleCount := 0
	for _, name := range sortedColumns {
		colWidth := minColumnWidths[name]

		// For each column we need its width plus one space for the gap
		// (except for the first column which doesn't need a leading gap)
		neededWidth := colWidth
		if visibleCount > 0 {
			neededWidth += 1 // Add gap width
		}

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
		visibleCols := []string{}
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

	// Ensure Version is always visible if there's room
	if terminalWidth >= minColumnWidths["Version"] && !newConfigs["Version"].visible {
		// Force Version to be visible
		config := newConfigs["Version"]
		config.visible = true
		config.width = minColumnWidths["Version"]
		newConfigs["Version"] = config
	}

	// Build visibility map for return
	visible := make(map[string]bool)
	for name, config := range newConfigs {
		visible[name] = config.visible
	}

	// Remove debug logging
	// Update global config
	columnConfigs = newConfigs

	return visible
}

// Model represents the state of the TUI application.
type Model struct {
	builds          []model.BlenderBuild
	cursor          int
	config          config.Config
	err             error
	terminalWidth   int
	sortColumn      int
	sortReversed    bool
	isLoading       bool
	visibleColumns  map[string]bool
	currentView     viewState
	focusIndex      int
	editMode        bool
	settingsInputs  []textinput.Model
	progressBar     progress.Model
	downloadStates  map[string]*DownloadState
	downloadMutex   sync.Mutex
	blenderRunning  string
	oldBuildsCount  int
	oldBuildsSize   int64
	deleteCandidate string // Version of build being considered for deletion
}

// DownloadState holds progress info for an active download
type DownloadState struct {
	Progress      float64 // 0.0 to 1.0
	Current       int64
	Total         int64
	Speed         float64       // Bytes per second
	Message       string        // e.g., "Preparing", "Downloading", "Extracting", "Local", "Failed: "
	LastUpdated   time.Time     // Timestamp of last progress update
	StartTime     time.Time     // When the download started
	StallDuration time.Duration // How long download can stall before timeout
	CancelCh      chan struct{} // Per-download cancel channel
}

// Styles using lipgloss
var (
	// Using default terminal colors
	headerStyle = lp.NewStyle().Bold(true).Padding(0, 1)
	// Style for the selected row
	selectedRowStyle = lp.NewStyle().Background(lp.Color(colorBackground)).Foreground(lp.Color(colorForeground))
	// Style for regular rows (use default)
	regularRowStyle = lp.NewStyle()
	// Footer style
	footerStyle = lp.NewStyle().MarginTop(1).Faint(true)
	// Separator style (using box characters)
	separator = lp.NewStyle().SetString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━").Faint(true).String()

	// Column Widths (adjust as needed)
	colWidthSelect  = 0 // Removed selection column
	colWidthVersion = 18
	colWidthStatus  = 18
	colWidthBranch  = 12
	colWidthType    = 18 // Release Cycle
	colWidthHash    = 15
	colWidthSize    = 12
	colWidthDate    = 20 // YYYY-MM-DD HH:MM

	// Define base styles for columns (can be customized further)
	cellStyleCenter = lp.NewStyle().Align(lp.Center)
	cellStyleRight  = lp.NewStyle().Align(lp.Right)
	cellStyleLeft   = lp.NewStyle() // Default
)

// InitialModel creates the initial state of the TUI model.
func InitialModel(cfg config.Config, needsSetup bool) Model {
	// Configure the progress bar with fixed settings for consistent column display
	progModel := progress.New(
		progress.WithGradient("#FFFFFF", "#FFFFFF"), // Solid white
		progress.WithoutPercentage(),                // No percentage display
		progress.WithWidth(10),                      // Fixed width that fits in columns
		progress.WithSolidFill("#FFFFFF"),           // Ensure fill is visible
		progress.WithDefaultGradient(),              // Use default gradient for better visibility
	)

	m := Model{
		config:         cfg,
		isLoading:      !needsSetup,
		downloadStates: make(map[string]*DownloadState),
		progressBar:    progModel,
		sortColumn:     0,                     // Default sort by Version
		sortReversed:   true,                  // Default descending sort (newest versions first)
		blenderRunning: "",                    // No Blender running initially
		editMode:       false,                 // Start in navigation mode, not edit mode
		visibleColumns: make(map[string]bool), // Initialize visible columns map
	}

	if needsSetup {
		m.currentView = viewInitialSetup
		m.settingsInputs = make([]textinput.Model, 2)

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

// command to fetch builds
// Now accepts the model to access config
func fetchBuildsCmd(cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		// Pass config (specifically VersionFilter) to FetchBuilds
		builds, err := api.FetchBuilds(cfg.VersionFilter)
		if err != nil {
			return errMsg{err}
		}
		return buildsFetchedMsg{builds: builds, err: nil}
	}
}

// Command to scan for LOCAL builds
func scanLocalBuildsCmd(cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		builds, err := local.ScanLocalBuilds(cfg.DownloadDir)
		// Return specific message for local scan results
		return localBuildsScannedMsg{builds: builds, err: err}
	}
}

// Command to re-scan local builds and update status of the provided (online) list
func updateStatusFromLocalScanCmd(onlineBuilds []model.BlenderBuild, cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		// Get all local builds - use full scan to compare hash values
		localBuilds, err := local.ScanLocalBuilds(cfg.DownloadDir)
		if err != nil {
			// Propagate error if scanning fails
			return errMsg{fmt.Errorf("failed local scan during status update: %w", err)}
		}

		// Create a map of local builds by version for easy lookup
		localBuildMap := make(map[string]model.BlenderBuild)
		for _, build := range localBuilds {
			localBuildMap[build.Version] = build
		}

		updatedBuilds := make([]model.BlenderBuild, len(onlineBuilds))
		copy(updatedBuilds, onlineBuilds) // Work on a copy

		for i := range updatedBuilds {
			if localBuild, found := localBuildMap[updatedBuilds[i].Version]; found {
				// We found a matching version locally
				if local.CheckUpdateAvailable(localBuild, updatedBuilds[i]) {
					// Using our new function to check if update is available based on build date
					updatedBuilds[i].Status = "Update"
				} else {
					updatedBuilds[i].Status = "Local"
				}
			} else {
				updatedBuilds[i].Status = "Online" // Not installed
			}
		}
		return buildsUpdatedMsg{builds: updatedBuilds}
	}
}

// tickCmd sends a tickMsg after a short delay.
// Now it supports dynamic tick rates based on download activity
func tickCmd() tea.Cmd {
	return tea.Tick(downloadTickRate, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Create a new function for adaptive tick rate
func adaptiveTickCmd(activeCount int, isExtracting bool) tea.Cmd {
	// Base rate is our standard download tick rate
	rate := downloadTickRate

	// If there are no active downloads, we can slow down the tick rate
	if activeCount == 0 {
		rate = 500 * time.Millisecond // Slower when idle
	} else if isExtracting {
		// During extraction, we can use a slightly slower rate
		rate = 250 * time.Millisecond
	} else if activeCount > 1 {
		// With multiple downloads, we can use a slightly faster rate
		// to make the UI more responsive, but not too fast to cause system load
		rate = 80 * time.Millisecond
	}

	return tea.Tick(rate, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// doDownloadCmd starts the download in a goroutine which updates shared state.
func doDownloadCmd(build model.BlenderBuild, cfg config.Config, downloadMap map[string]*DownloadState, mutex *sync.Mutex) tea.Cmd {
	now := time.Now()

	// Create a cancel channel specific to this download
	downloadCancelCh := make(chan struct{})

	mutex.Lock()
	if _, exists := downloadMap[build.Version]; !exists {
		// Initialize download state with defaults
		downloadMap[build.Version] = &DownloadState{
			Message:       "Preparing",
			StartTime:     now,
			LastUpdated:   now,
			Progress:      0.0,
			StallDuration: downloadStallTime, // Initial stall timeout
			CancelCh:      downloadCancelCh,
		}
	} else {
		mutex.Unlock()
		return nil
	}
	mutex.Unlock()

	// Create a done channel for this download
	done := make(chan struct{})

	go func() {
		// Variables to track progress for speed calculation
		var lastUpdateTime time.Time
		var lastUpdateBytes int64
		var currentSpeed float64

		// Define progress callback function
		progressCallback := func(downloaded, total int64) {
			// Check for cancellation
			select {
			case <-downloadCancelCh:
				return
			default:
				// Continue with progress update
			}

			currentTime := time.Now()
			percent := 0.0
			if total > 0 {
				percent = float64(downloaded) / float64(total)
			}

			// Calculate speed (only if enough time has passed)
			if !lastUpdateTime.IsZero() {
				elapsed := currentTime.Sub(lastUpdateTime).Seconds()
				if elapsed > 0.2 {
					bytesSinceLast := downloaded - lastUpdateBytes
					if elapsed > 0 {
						currentSpeed = float64(bytesSinceLast) / elapsed
					}
					lastUpdateBytes = downloaded
					lastUpdateTime = currentTime
				}
			} else {
				// First call, initialize time/bytes
				lastUpdateBytes = downloaded
				lastUpdateTime = currentTime
			}

			// Check again for cancellation before trying to lock
			select {
			case <-downloadCancelCh:
				return
			default:
				// Continue updating state
			}

			// Try to lock, skip update if contended
			if !mutex.TryLock() {
				return
			}
			defer mutex.Unlock()

			if state, ok := downloadMap[build.Version]; ok {
				// If already cancelled, don't update progress
				if state.Message == "Cancelled" {
					return
				}

				// Always update the last update timestamp
				state.LastUpdated = currentTime

				// Use a virtual size threshold to detect extraction phase
				const extractionVirtualSize int64 = 100 * 1024 * 1024

				// Determine state based on progress info
				if total == extractionVirtualSize {
					// Extraction phase
					state.Message = "Extracting"
					state.Progress = percent
					state.Speed = 0                           // No download speed during extraction
					state.StallDuration = extractionStallTime // Longer timeout for extraction
				} else if state.Message == "Extracting" {
					// Continue extraction progress updates
					state.Progress = percent
				} else {
					// Normal download progress
					state.Progress = percent
					state.Current = downloaded
					state.Total = total
					state.Speed = currentSpeed
					state.Message = "Downloading"
					state.StallDuration = downloadStallTime
				}
			}
		}

		// Check for cancellation before starting download
		select {
		case <-downloadCancelCh:
			// Download canceled before starting
			mutex.Lock()
			if state, ok := downloadMap[build.Version]; ok {
				state.Message = "Cancelled"
			}
			mutex.Unlock()
			close(done)
			return
		default:
			// Proceed with download
		}

		// Download and extract the build
		_, err := download.DownloadAndExtractBuild(build, cfg.DownloadDir, progressCallback, downloadCancelCh)

		// Check for cancellation or handle completion
		select {
		case <-downloadCancelCh:
			// Download was cancelled during execution
			mutex.Lock()
			if state, ok := downloadMap[build.Version]; ok {
				state.Message = "Cancelled"
			}
			mutex.Unlock()
			close(done)
			return
		default:
			// Continue processing result
		}

		// Update final status based on result
		mutex.Lock()
		if state, ok := downloadMap[build.Version]; ok {
			if state.Message == "Cancelled" {
				// Keep as cancelled
			} else if err != nil {
				if errors.Is(err, download.ErrCancelled) {
					state.Message = "Cancelled"
				} else {
					state.Message = fmt.Sprintf("Failed: %v", err)
				}
			} else {
				state.Message = "Local"
			}
		}
		mutex.Unlock()

		// Signal completion
		close(done)
	}()

	// Start with a single active download
	return adaptiveTickCmd(1, false)
}

// Init initializes the TUI model.
func (m Model) Init() tea.Cmd {
	// Store the program reference when Init is called by Bubble Tea runtime
	// This is a bit of a hack, relies on Init being called once with the Program.
	// A dedicated message might be cleaner if issues arise.
	// NOTE: This won't work as Program is not passed here. Alternative needed.
	// We'll set it in Update on the first FrameMsg instead.
	var cmds []tea.Cmd

	if m.currentView == viewList {
		cmds = append(cmds, scanLocalBuildsCmd(m.config))
		// Get info about old builds
		cmds = append(cmds, getOldBuildsInfoCmd(m.config))
	}
	if m.currentView == viewInitialSetup && len(m.settingsInputs) > 0 {
		cmds = append(cmds, textinput.Blink)
	}

	if len(cmds) > 0 {
		return tea.Batch(cmds...)
	}
	return nil
}

// Helper to update focused input
func (m *Model) updateInputs(msg tea.Msg) tea.Cmd {
	// Make sure we have inputs to update
	if len(m.settingsInputs) == 0 {
		return nil
	}

	var cmds []tea.Cmd
	for i := range m.settingsInputs {
		m.settingsInputs[i], cmds[i] = m.settingsInputs[i].Update(msg)
	}
	return tea.Batch(cmds...)
}

// Update handles messages and updates the model state.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle global events first for better responsiveness
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Global key handler for exit (works regardless of view)
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			// No need to close a global cancel channel anymore
			// Signal all active downloads to cancel
			m.downloadMutex.Lock()
			for _, state := range m.downloadStates {
				close(state.CancelCh)
			}
			m.downloadMutex.Unlock()

			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		// Handle window size globally (avoid duplicate handlers)
		m.terminalWidth = msg.Width
		// Do NOT update progress bar width to terminal width
		// Keep it small for column-based display

		// Recalculate visible columns and their widths based on new terminal width
		m.visibleColumns = calculateVisibleColumns(m.terminalWidth)

		// Remove debug logging

		return m, nil
	case tea.MouseMsg:
		// Process mouse events which can help maintain focus
		return m, nil
	}

	// Now handle view-specific events and messages
	switch m.currentView {
	case viewInitialSetup, viewSettings:
		return m.updateSettingsView(msg)
	case viewList:
		return m.updateListView(msg)
	case viewDeleteConfirm:
		return m.updateDeleteConfirmView(msg)
	case viewCleanupConfirm:
		return m.updateCleanupConfirmView(msg)
	}

	return m, nil
}

// updateSettingsView handles updating the settings/setup view
func (m Model) updateSettingsView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		s := msg.String()
		if m.editMode {
			// In edit mode - handle exiting edit mode and input-specific keys
			switch s {
			case "enter":
				// Toggle out of edit mode
				m.editMode = false
				// Blur the current input
				if m.focusIndex >= 0 && m.focusIndex < len(m.settingsInputs) {
					m.settingsInputs[m.focusIndex].Blur()
				}
				return m, nil
			case "esc", "escape":
				// Also exit edit mode with Escape
				m.editMode = false
				// Blur the current input
				if m.focusIndex >= 0 && m.focusIndex < len(m.settingsInputs) {
					m.settingsInputs[m.focusIndex].Blur()
				}
				return m, nil
			default:
				// Pass other keys to the focused input
				if m.focusIndex >= 0 && m.focusIndex < len(m.settingsInputs) {
					m.settingsInputs[m.focusIndex], cmd = m.settingsInputs[m.focusIndex].Update(msg)
				}
				return m, cmd
			}
		} else {
			// In navigation mode - handle navigation and entering edit mode
			switch s {
			case "s", "S":
				// Save settings and go back to list view
				return saveSettings(m)
			case "o", "O":
				// Open download directory
				return m, local.OpenDirCmd(m.config.DownloadDir)
			case "j", "down":
				// Move focus down
				oldFocus := m.focusIndex
				m.focusIndex++
				if m.focusIndex >= len(m.settingsInputs) {
					m.focusIndex = 0
				}
				updateFocusStyles(&m, oldFocus)
				return m, nil
			case "k", "up":
				// Move focus up
				oldFocus := m.focusIndex
				m.focusIndex--
				if m.focusIndex < 0 {
					m.focusIndex = len(m.settingsInputs) - 1
				}
				updateFocusStyles(&m, oldFocus)
				return m, nil
			case "tab":
				// Tab navigates between inputs
				oldFocus := m.focusIndex
				m.focusIndex++
				if m.focusIndex >= len(m.settingsInputs) {
					m.focusIndex = 0
				}
				updateFocusStyles(&m, oldFocus)
				return m, nil
			case "shift+tab":
				// Shift+Tab navigates backwards
				oldFocus := m.focusIndex
				m.focusIndex--
				if m.focusIndex < 0 {
					m.focusIndex = len(m.settingsInputs) - 1
				}
				updateFocusStyles(&m, oldFocus)
				return m, nil
			case "enter":
				// Enter edit mode
				m.editMode = true
				if m.focusIndex >= 0 && m.focusIndex < len(m.settingsInputs) {
					m.settingsInputs[m.focusIndex].Focus()
				}
				return m, textinput.Blink
			case "c", "C":
				// Add cleanup functionality in settings
				if m.oldBuildsCount > 0 {
					m.currentView = viewCleanupConfirm
					return m, nil
				}
				return m, nil
			case "h", "left":
				// Go back to list view without saving
				m.currentView = viewList
				return m, nil
			}
			return m, nil
		}
	}

	// Only pass message to the focused input if in edit mode
	if m.editMode {
		currentFocus := m.focusIndex
		if len(m.settingsInputs) > 0 && currentFocus >= 0 && currentFocus < len(m.settingsInputs) {
			m.settingsInputs[currentFocus], cmd = m.settingsInputs[currentFocus].Update(msg)
		}
	}
	return m, cmd
}

// updateListView handles updating the main list view
func (m Model) updateListView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	// Handle key presses
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			// Move cursor up
			if len(m.builds) > 0 {
				m.cursor--
				if m.cursor < 0 {
					m.cursor = len(m.builds) - 1
				}
			}
			return m, nil
		case "down", "j":
			// Move cursor down
			if len(m.builds) > 0 {
				m.cursor++
				if m.cursor >= len(m.builds) {
					m.cursor = 0
				}
			}
			return m, nil
		case "left", "h":
			// Move to previous column for sorting
			if m.sortColumn > 0 {
				m.sortColumn--
				// Skip hidden columns
				for m.sortColumn > 0 && !isColumnVisible(m.sortColumn) {
					m.sortColumn--
				}
			} else {
				// Wrap to the last visible column
				m.sortColumn = getLastVisibleColumn()
			}
			// Re-sort the list
			m.builds = sortBuilds(m.builds, m.sortColumn, m.sortReversed)
			return m, nil
		case "right", "l":
			// Move to next column for sorting
			m.sortColumn++
			// Skip hidden columns
			for m.sortColumn < 7 && !isColumnVisible(m.sortColumn) {
				m.sortColumn++
			}
			if m.sortColumn >= 7 {
				m.sortColumn = 0 // Wrap to first column
			}
			// Re-sort the list
			m.builds = sortBuilds(m.builds, m.sortColumn, m.sortReversed)
			return m, nil
		case "r":
			// Toggle sort order
			m.sortReversed = !m.sortReversed
			// Re-sort the list
			m.builds = sortBuilds(m.builds, m.sortColumn, m.sortReversed)
			return m, nil
		case "enter":
			// Handle enter key for launching Blender
			return m.handleLaunchBlender()
		case "d", "D":
			// Start download of the selected build
			return m.handleStartDownload()
		case "c", "C":
			// Cancel download of the selected build
			return m.handleCancelDownload()
		case "o", "O":
			// Open build directory for selected build
			return m.handleOpenBuildDir()
		case "s", "S":
			// Show settings
			return m.handleShowSettings()
		case "f", "F":
			// Fetch from Builder API
			m.isLoading = true
			return m, fetchBuildsCmd(m.config)
		case "x", "X":
			// Delete a build
			return m.handleDeleteBuild()
		}
	// Handle initial local scan results
	case localBuildsScannedMsg:
		return m.handleLocalBuildsScanned(msg)
	// Handle online builds fetched
	case buildsFetchedMsg:
		return m.handleBuildsFetched(msg)
	// Handle builds list after status update
	case buildsUpdatedMsg:
		return m.handleBuildsUpdated(msg)
	case model.BlenderLaunchedMsg:
		// Record that Blender is running
		m.blenderRunning = msg.Version
		// Update the footer message
		m.err = nil
		return m, nil
	case model.BlenderExecMsg:
		return m.handleBlenderExec(msg)
	case errMsg:
		m.isLoading = false
		m.err = msg.err
		return m, nil
	// Handle Download Start Request
	case startDownloadMsg:
		cmd = doDownloadCmd(msg.build, m.config, m.downloadStates, &m.downloadMutex)
		return m, cmd
	case tickMsg:
		return m.handleDownloadProgress(msg)
	case downloadCompleteMsg:
		// Just trigger a refresh of local files
		cmd = scanLocalBuildsCmd(m.config)
		// Also refresh old builds info after download completes
		return m, tea.Batch(cmd, getOldBuildsInfoCmd(m.config))
	case oldBuildsInfo:
		m.oldBuildsCount = msg.count
		m.oldBuildsSize = msg.size
		if msg.err != nil {
			m.err = msg.err
		}
		return m, nil
	case cleanupOldBuildsMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.oldBuildsCount = 0
			m.oldBuildsSize = 0
		}
		m.currentView = viewList
		return m, nil
	case resetStatusMsg:
		// Find the build and reset its status
		needsSort := false
		for i := range m.builds {
			if m.builds[i].Version == msg.version {
				// Only reset if it's still marked as Cancelled
				if m.builds[i].Status == "Cancelled" {
					m.builds[i].Status = "Online" // Or potentially "Update" if applicable?
					// TODO: Re-check if an update is available for this build?
					// For now, just set to Online. If user fetches again, it will update.
					needsSort = true // Re-sort if status changed
				}
				break
			}
		}
		if needsSort {
			m.builds = sortBuilds(m.builds, m.sortColumn, m.sortReversed)
		}
		return m, nil // No further command needed
	}

	return m, nil
}

// updateDeleteConfirmView handles updating the delete confirmation view
func (m Model) updateDeleteConfirmView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			// User confirmed deletion
			// Implement actual deletion logic using the DeleteBuild function
			success, err := local.DeleteBuild(m.config.DownloadDir, m.deleteCandidate)
			if err != nil {
				log.Printf("Error deleting build %s: %v", m.deleteCandidate, err)
				m.err = fmt.Errorf("Failed to delete build: %w", err)
			} else if !success {
				log.Printf("Build %s not found for deletion", m.deleteCandidate)
				m.err = fmt.Errorf("Build %s not found", m.deleteCandidate)
			} else {
				log.Printf("Successfully deleted build: %s", m.deleteCandidate)
				// Clear any previous error
				m.err = nil
			}

			// Return to builds view and refresh the builds list
			m.deleteCandidate = ""
			m.currentView = viewList
			m.isLoading = true
			return m, scanLocalBuildsCmd(m.config)

		case "n", "N", "esc", "escape":
			// User cancelled deletion
			m.deleteCandidate = ""
			m.currentView = viewList
			return m, nil
		}
	}

	return m, nil
}

// updateCleanupConfirmView handles updating the cleanup confirmation view
func (m Model) updateCleanupConfirmView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			// User confirmed cleanup
			m.currentView = viewList
			return m, cleanupOldBuildsCmd(m.config)

		case "n", "N", "esc", "escape":
			// User cancelled cleanup
			m.currentView = viewList
			return m, nil
		}
	}

	return m, nil
}

// Helper functions for handling specific actions in list view
func (m Model) handleLaunchBlender() (tea.Model, tea.Cmd) {
	if len(m.builds) > 0 && m.cursor < len(m.builds) {
		selectedBuild := m.builds[m.cursor]
		// Only attempt to launch if it's a local build
		if selectedBuild.Status == "Local" {
			// Add launch logic here
			log.Printf("Launching Blender %s", selectedBuild.Version)
			cmd := local.LaunchBlenderCmd(m.config.DownloadDir, selectedBuild.Version)
			return m, cmd
		}
	}
	return m, nil
}

// New function to handle opening the build directory for a specific version
func (m Model) handleOpenBuildDir() (tea.Model, tea.Cmd) {
	if len(m.builds) > 0 && m.cursor < len(m.builds) {
		selectedBuild := m.builds[m.cursor]
		// Only open dir if it's a local build or has an update available
		if selectedBuild.Status == "Local" || selectedBuild.Status == "Update" {
			// Create a command that locates the correct build directory by version
			return m, func() tea.Msg {
				entries, err := os.ReadDir(m.config.DownloadDir)
				if err != nil {
					return errMsg{fmt.Errorf("failed to read download directory %s: %w", m.config.DownloadDir, err)}
				}

				version := selectedBuild.Version
				for _, entry := range entries {
					if entry.IsDir() && entry.Name() != ".downloading" && entry.Name() != ".oldbuilds" {
						dirPath := filepath.Join(m.config.DownloadDir, entry.Name())
						buildInfo, err := local.ReadBuildInfo(dirPath)
						if err != nil {
							// Error reading build info, but continue checking other directories
							continue
						}

						// Check if this is the build we want to open
						if buildInfo != nil && buildInfo.Version == version {
							// Open this directory
							if err := local.OpenFileExplorer(dirPath); err != nil {
								return errMsg{fmt.Errorf("failed to open directory: %w", err)}
							}
							return nil // Success
						}
					}
				}

				return errMsg{fmt.Errorf("build directory for Blender version %s not found", version)}
			}
		}
	}
	return m, nil
}

func (m Model) handleStartDownload() (tea.Model, tea.Cmd) {
	if len(m.builds) > 0 && m.cursor < len(m.builds) {
		selectedBuild := m.builds[m.cursor]
		// Allow downloading both Online builds and Updates
		if selectedBuild.Status == "Online" || selectedBuild.Status == "Update" {
			// Update status to avoid duplicate downloads
			selectedBuild.Status = "Preparing"
			m.builds[m.cursor] = selectedBuild
			// Send message to start download
			return m, func() tea.Msg {
				return startDownloadMsg{build: selectedBuild}
			}
		}
	}
	return m, nil
}

func (m Model) handleShowSettings() (tea.Model, tea.Cmd) {
	m.currentView = viewSettings
	m.editMode = false // Ensure we start in navigation mode

	// Initialize settings inputs if not already done
	if len(m.settingsInputs) == 0 {
		m.settingsInputs = make([]textinput.Model, 2)

		// Download Dir input
		var t textinput.Model
		t = textinput.New()
		t.Placeholder = m.config.DownloadDir
		t.CharLimit = 256
		t.Width = 50
		m.settingsInputs[0] = t

		// Version Filter input
		t = textinput.New()
		t.Placeholder = "e.g., 4.0, 3.6 (leave empty for none)"
		t.CharLimit = 10
		t.Width = 50
		m.settingsInputs[1] = t
	}

	// Copy current config values
	m.settingsInputs[0].SetValue(m.config.DownloadDir)
	m.settingsInputs[1].SetValue(m.config.VersionFilter)

	// Focus first input (but don't focus for editing yet)
	m.focusIndex = 0
	updateFocusStyles(&m, -1)

	return m, nil
}

func (m Model) handleDeleteBuild() (tea.Model, tea.Cmd) {
	if len(m.builds) > 0 && m.cursor < len(m.builds) {
		selectedBuild := m.builds[m.cursor]
		// Only allow deleting local builds or builds that can be updated
		if selectedBuild.Status == "Local" || selectedBuild.Status == "Update" {
			m.deleteCandidate = selectedBuild.Version
			m.currentView = viewDeleteConfirm
			return m, nil
		}
	}
	return m, nil
}

func (m Model) handleCleanupOldBuilds() (tea.Model, tea.Cmd) {
	if m.oldBuildsCount > 0 {
		// Prompt for confirmation
		m.currentView = viewCleanupConfirm
		return m, nil
	}
	return m, nil
}

func (m Model) handleLocalBuildsScanned(msg localBuildsScannedMsg) (tea.Model, tea.Cmd) {
	m.isLoading = false
	if msg.err != nil {
		m.err = msg.err
	} else {
		m.builds = msg.builds
		// Sort the builds based on current sort settings
		m.builds = sortBuilds(m.builds, m.sortColumn, m.sortReversed)
		m.err = nil
	}
	// Adjust cursor if necessary
	if m.cursor >= len(m.builds) {
		m.cursor = 0
		if len(m.builds) > 0 {
			m.cursor = len(m.builds) - 1
		}
	}
	return m, nil
}

func (m Model) handleBuildsFetched(msg buildsFetchedMsg) (tea.Model, tea.Cmd) {
	m.isLoading = false
	if msg.err != nil {
		m.err = msg.err
		return m, nil
	}

	// Store the updated builds
	m.builds = msg.builds

	// Re-apply sort settings
	m.builds = sortBuilds(m.builds, m.sortColumn, m.sortReversed)

	// Ensure cursor doesn't go out of bounds
	if m.cursor >= len(m.builds) {
		m.cursor = len(m.builds) - 1
		if m.cursor < 0 {
			m.cursor = 0
		}
	}

	// Now update the status of the builds based on local scan
	return m, updateStatusFromLocalScanCmd(m.builds, m.config)
}

func (m Model) handleBuildsUpdated(msg buildsUpdatedMsg) (tea.Model, tea.Cmd) {
	m.isLoading = false // Now loading is complete
	m.builds = msg.builds
	// Sort the builds based on current sort settings
	m.builds = sortBuilds(m.builds, m.sortColumn, m.sortReversed)
	m.err = nil
	// Adjust cursor
	if m.cursor >= len(m.builds) {
		m.cursor = 0
		if len(m.builds) > 0 {
			m.cursor = len(m.builds) - 1
		}
	}
	return m, nil
}

func (m Model) handleBlenderExec(msg model.BlenderExecMsg) (tea.Model, tea.Cmd) {
	// Store Blender info
	execInfo := msg

	// Write a command file that the main.go program will execute after the TUI exits
	// This ensures Blender runs in the same terminal session after the TUI is fully terminated
	launcherPath := filepath.Join(os.TempDir(), "blender_launch_command.txt")

	// First try to save the command
	err := os.WriteFile(launcherPath, []byte(execInfo.Executable), 0644)
	if err != nil {
		return m, func() tea.Msg {
			return errMsg{fmt.Errorf("failed to save launch info: %w", err)}
		}
	}

	// Set an environment variable to tell the main program to run Blender on exit
	os.Setenv("TUI_BLENDER_LAUNCH", launcherPath)

	// Display exit message with info about Blender launch
	m.err = nil
	m.blenderRunning = execInfo.Version

	// Simply quit - the main program will handle launching Blender
	return m, tea.Quit
}

func (m Model) handleDownloadProgress(msg tickMsg) (tea.Model, tea.Cmd) {
	var commands []tea.Cmd
	now := time.Now()

	m.downloadMutex.Lock() // Lock early

	activeDownloads := 0
	var progressCmds []tea.Cmd
	// Lists to store versions identified for state change/cleanup
	completedDownloads := make([]string, 0)
	stalledDownloads := make([]string, 0)
	cancelledDownloads := make([]string, 0)
	extractingInProgress := false

	// Temporary copy of download states for use after unlock
	tempStates := make(map[string]DownloadState)

	// Process download states while holding the lock
	for version, state := range m.downloadStates {
		tempStates[version] = *state // Store a copy

		if state.Message == "Local" || strings.HasPrefix(state.Message, "Failed") {
			// Download completed or failed
			completedDownloads = append(completedDownloads, version)
		} else if state.Message == "Cancelled" {
			// Download was cancelled
			cancelledDownloads = append(cancelledDownloads, version)
		} else if state.Message == "Downloading" ||
			state.Message == "Preparing" ||
			state.Message == "Extracting" {
			// Active download
			timeSinceUpdate := now.Sub(state.LastUpdated)
			if timeSinceUpdate > state.StallDuration {
				// Download appears stalled
				log.Printf("WARNING: Download for %s stalled (no updates for %v), marking as failed",
					version, timeSinceUpdate.Round(time.Second))

				// Update status in our temporary copy
				tempStateCopy := *state
				tempStateCopy.Message = fmt.Sprintf("Failed: Download stalled for %v",
					timeSinceUpdate.Round(time.Second))
				tempStates[version] = tempStateCopy
				stalledDownloads = append(stalledDownloads, version)
			} else {
				// Active download that's not stalled
				activeDownloads++
				if state.Message == "Extracting" {
					extractingInProgress = true
				}
				// Queue progress bar update
				progressCmds = append(progressCmds, m.progressBar.SetPercent(state.Progress))
			}
		}
	}

	// Clean up completed/stalled/cancelled downloads
	for _, version := range completedDownloads {
		delete(m.downloadStates, version)
	}
	for _, version := range stalledDownloads {
		delete(m.downloadStates, version)
	}
	for _, version := range cancelledDownloads {
		delete(m.downloadStates, version)
	}

	m.downloadMutex.Unlock() // Unlock after map modifications

	// Update build statuses based on download states
	needsSort := false
	for _, version := range completedDownloads {
		for i := range m.builds {
			if m.builds[i].Version == version {
				if tempState, ok := tempStates[version]; ok {
					m.builds[i].Status = tempState.Message
					needsSort = true
				}
				break
			}
		}
	}
	for _, version := range stalledDownloads {
		for i := range m.builds {
			if m.builds[i].Version == version {
				if tempState, ok := tempStates[version]; ok {
					m.builds[i].Status = tempState.Message
					needsSort = true
				}
				break
			}
		}
	}
	for _, version := range cancelledDownloads {
		for i := range m.builds {
			if m.builds[i].Version == version {
				m.builds[i].Status = "Cancelled"
				needsSort = true
				break
			}
		}
		// Schedule the status reset command after cancellation
		commands = append(commands, tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg {
			return resetStatusMsg{version: version}
		}))
	}

	// Re-sort if any status changed
	if needsSort {
		m.builds = sortBuilds(m.builds, m.sortColumn, m.sortReversed)
	}

	// Create any needed commands
	if activeDownloads > 0 {
		// Continue ticking for active downloads
		commands = append(commands, adaptiveTickCmd(activeDownloads, extractingInProgress))
		// Add progress bar updates
		if len(progressCmds) > 0 {
			commands = append(commands, progressCmds...)
		}
	} else if len(progressCmds) > 0 {
		// Handle final progress updates if any
		commands = append(commands, progressCmds...)
	}

	if len(commands) > 0 {
		return m, tea.Batch(commands...)
	}

	return m, nil
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

// View renders the UI based on the model state.
func (m Model) View() string {
	switch m.currentView {
	case viewInitialSetup, viewSettings:
		return m.renderSettingsView()
	case viewList:
		return m.renderListView()
	case viewDeleteConfirm:
		return m.renderDeleteConfirmView()
	case viewCleanupConfirm:
		return m.renderCleanupConfirmView()
	}

	// Fallback empty view
	return ""
}

// renderSettingsView handles rendering the settings and initial setup views
func (m Model) renderSettingsView() string {
	var viewBuilder strings.Builder

	title := "Initial Setup"
	if m.currentView == viewSettings {
		title = "Settings"
	}
	viewBuilder.WriteString(fmt.Sprintf("%s\n\n", title))
	viewBuilder.WriteString("Download Directory:\n")

	// Only render inputs if they exist
	if len(m.settingsInputs) >= 2 {
		viewBuilder.WriteString(m.settingsInputs[0].View() + "\n\n")
		viewBuilder.WriteString("Minimum Blender Version Filter (e.g., 4.0, 3.6 - empty for none):\n")
		viewBuilder.WriteString(m.settingsInputs[1].View() + "\n\n")
	} else {
		// Fallback if inputs aren't initialized
		viewBuilder.WriteString(m.config.DownloadDir + "\n\n")
		viewBuilder.WriteString("Minimum Blender Version Filter (e.g., 4.0, 3.6 - empty for none):\n")
		viewBuilder.WriteString(m.config.VersionFilter + "\n\n")
	}

	if m.err != nil {
		viewBuilder.WriteString(lp.NewStyle().Foreground(lp.Color(colorError)).Render(fmt.Sprintf("Error: %v\n\n", m.err)))
	}

	// Use the same footer style as the main list view
	var footerKeybinds1, footerKeybinds2 string

	if m.editMode {
		footerKeybinds1 = "Enter:Save  Esc:Cancel"
		footerKeybinds2 = "Tab:Next Field"
	} else {
		footerKeybinds1 = "Enter:Edit Field  S:Save & Back  O:Open Dir"
		if m.oldBuildsCount > 0 {
			footerKeybinds2 = fmt.Sprintf("C:Cleanup old Builds (%d)", m.oldBuildsCount)
		}
	}

	// Always render two lines for the footer
	footerContent := footerKeybinds1 + "\n" + footerKeybinds2
	viewBuilder.WriteString(footerStyle.Render(footerContent))

	return viewBuilder.String()
}

// renderListView handles rendering the main builds list view
func (m Model) renderListView() string {
	var sb strings.Builder

	// Handle error state
	if m.err != nil {
		errorStyle := lp.NewStyle().Foreground(lp.Color(colorError))
		sb.WriteString(errorStyle.Render("Error: "+m.err.Error()) + "\n\n")
		sb.WriteString("Press f to try fetching online builds, s for settings, q to quit.")
		return sb.String()
	}

	// --- DEFINE BASE STYLES ---
	headerStyle := lp.NewStyle().Bold(true)
	cellStyle := lp.NewStyle().PaddingRight(1)
	selectedStyle := lp.NewStyle().Background(lp.Color(colorBackground)).Foreground(lp.Color(colorForeground))

	// Status-specific styles
	localStyle := lp.NewStyle().Foreground(lp.Color(colorSuccess))
	updateStyle := lp.NewStyle().Foreground(lp.Color(colorInfo))
	errorStyle := lp.NewStyle().Foreground(lp.Color(colorError))
	warningStyle := lp.NewStyle().Foreground(lp.Color(colorWarning))

	// --- DETERMINE VISIBLE COLUMNS ---
	// Create a list of column definitions with width and visibility
	columns := []struct {
		name    string
		width   int
		visible bool
		index   int // Sort column index
	}{
		{"Version", columnConfigs["Version"].width, m.visibleColumns["Version"], 0},
		{"Status", columnConfigs["Status"].width, m.visibleColumns["Status"], 1},
		{"Branch", columnConfigs["Branch"].width, m.visibleColumns["Branch"], 2},
		{"Type", columnConfigs["Type"].width, m.visibleColumns["Type"], 3},
		{"Hash", columnConfigs["Hash"].width, m.visibleColumns["Hash"], 4},
		{"Size", columnConfigs["Size"].width, m.visibleColumns["Size"], 5},
		{"Build Date", columnConfigs["Build Date"].width, m.visibleColumns["Build Date"], 6},
	}

	// --- RENDER TABLE HEADER ---
	// Create header cells
	var headerCells []string
	for _, col := range columns {
		if col.visible {
			title := col.name
			if m.sortColumn == col.index {
				if m.sortReversed {
					title += " ↓"
				} else {
					title += " ↑"
				}

				// Center align all columns including Version
				headerCells = append(headerCells, headerStyle.Copy().Underline(true).Align(lp.Center).Width(col.width).Render(title))
			} else {
				// Center align all columns including Version
				headerCells = append(headerCells, headerStyle.Copy().Align(lp.Center).Width(col.width).Render(title))
			}
		}
	}

	// Join and render the header
	sb.WriteString(lp.JoinHorizontal(lp.Left, headerCells...) + "\n")

	// Render separator
	separatorStyle := lp.NewStyle().Faint(true)
	totalWidth := 0
	visibleColumnCount := 0
	for _, col := range columns {
		if col.visible {
			totalWidth += col.width
			visibleColumnCount++
		}
	}
	// Add padding between columns
	totalWidth += visibleColumnCount - 1

	// Ensure totalWidth is never negative before using strings.Repeat
	if totalWidth < 1 {
		totalWidth = 1 // Minimum width of 1 to avoid panic
	}

	sb.WriteString(separatorStyle.Render(strings.Repeat("─", totalWidth)) + "\n")

	// --- HANDLE EMPTY TABLE ---
	if len(m.builds) == 0 {
		noBuildsMsg := "No builds found. Press 'f' to fetch online builds."
		sb.WriteString(lp.PlaceHorizontal(totalWidth, lp.Center, noBuildsMsg) + "\n")
	} else {
		// --- RENDER TABLE ROWS ---
		for i, build := range m.builds {
			isSelected := m.cursor == i
			downloadState, isDownloading := m.downloadStates[build.Version]

			// Collect cells for this row
			var cells []string

			// Helper function to add a cell with appropriate styling
			addCell := func(content string, width int, baseStyle lp.Style, colIndex int) {
				// Center align all columns
				style := baseStyle.Copy().Align(lp.Center)

				// Apply selection styling if selected
				if isSelected {
					cells = append(cells, style.Inherit(selectedStyle).Width(width).Render(content))
				} else {
					cells = append(cells, style.Width(width).Render(content))
				}
			}

			// Determine status style
			statusStyle := cellStyle.Copy()
			switch build.Status {
			case "Local":
				statusStyle = statusStyle.Copy().Inherit(localStyle)
			case "Update":
				statusStyle = statusStyle.Copy().Inherit(updateStyle)
			case "Cancelled":
				statusStyle = statusStyle.Copy().Inherit(warningStyle)
			default:
				if strings.HasPrefix(build.Status, "Failed") {
					statusStyle = statusStyle.Copy().Inherit(errorStyle)
				} else if isDownloading {
					statusStyle = statusStyle.Copy().Inherit(warningStyle)
				}
			}

			// Special handling for downloading builds
			if isDownloading {
				// For downloading builds, create a completely different row with full control over width
				var rowContent string

				// Version (always visible)
				versionText := "Blender " + build.Version
				versionCell := cellStyle.Copy().Align(lp.Center).Width(columns[0].width).Render(versionText)

				// Status (always visible)
				statusCell := statusStyle.Copy().Align(lp.Center).Width(columns[1].width).Render(downloadState.Message)

				// Branch (speed display for downloads)
				branchContent := ""
				if columns[2].visible {
					if downloadState.Message == "Downloading" {
						branchContent = util.FormatSpeed(downloadState.Speed)
					}
					branchCell := cellStyle.Copy().Align(lp.Center).Width(columns[2].width).Render(branchContent)
					rowContent = lp.JoinHorizontal(lp.Left, versionCell, statusCell, branchCell)
				} else {
					rowContent = lp.JoinHorizontal(lp.Left, versionCell, statusCell)
				}

				// Calculate progress bar width precisely
				remainingWidth := 0
				for i := 3; i < len(columns); i++ {
					if columns[i].visible {
						remainingWidth += columns[i].width + 1 // Include gap
					}
				}
				// Remove the last gap space (after the last column)
				if remainingWidth > 0 {
					remainingWidth -= 1
				}

				// Only render progress bar if we have space
				if remainingWidth > 0 && (downloadState.Message == "Downloading" || downloadState.Message == "Extracting") {
					// Create the progress bar
					pb := m.progressBar
					pb.Width = remainingWidth - 2 // Subtract 2 for safety
					progressBar := pb.ViewAs(downloadState.Progress)

					progressCell := cellStyle.Copy().Width(remainingWidth).Render(progressBar)
					rowContent = lp.JoinHorizontal(lp.Left, rowContent, progressCell)
				}

				// Apply selection styling if needed
				if isSelected {
					rowContent = selectedStyle.Render(rowContent)
				}

				// Add the entire row directly
				sb.WriteString(rowContent + "\n")
			} else {
				// Regular row rendering for non-downloading builds

				// Version
				if columns[0].visible {
					addCell("Blender "+build.Version, columns[0].width, cellStyle, 0)
				}

				// Status
				if columns[1].visible {
					addCell(build.Status, columns[1].width, statusStyle, 1)
				}

				// Branch
				if columns[2].visible {
					addCell(build.Branch, columns[2].width, cellStyle, 2)
				}

				// Type
				if columns[3].visible {
					addCell(build.ReleaseCycle, columns[3].width, cellStyle, 3)
				}

				// Hash
				if columns[4].visible {
					addCell(build.Hash, columns[4].width, cellStyle, 4)
				}

				// Size
				if columns[5].visible {
					addCell(util.FormatSize(build.Size), columns[5].width, cellStyle, 5)
				}

				// Build Date
				if columns[6].visible {
					addCell(build.BuildDate.Time().Format("2006-01-02"), columns[6].width, cellStyle, 6)
				}

				// Join cells and add to output
				sb.WriteString(lp.JoinHorizontal(lp.Left, cells...) + "\n")
			}
		}
	}

	// --- ADD LOADING INDICATOR ---
	if m.isLoading {
		loadingStyle := lp.NewStyle().Foreground(lp.Color(colorInfo))
		if len(m.builds) == 0 {
			sb.WriteString(loadingStyle.Render("Scanning local builds...\n"))
		} else {
			sb.WriteString(loadingStyle.Render("\nFetching online builds...\n"))
		}
	}

	// --- RUNNING BLENDER NOTICE ---
	if m.blenderRunning != "" {
		noticeStyle := lp.NewStyle().Foreground(lp.Color(colorSuccess)).Bold(true)
		notice := fmt.Sprintf("\n⚠ Blender %s is running - this terminal will display its console output", m.blenderRunning)
		sb.WriteString(noticeStyle.Render(notice) + "\n")
	}

	// --- FOOTER KEYBINDS ---
	var footerKeybinds1 string
	var footerKeybinds2 string

	// First line: contextual commands for selected build
	if len(m.builds) > 0 && m.cursor < len(m.builds) {
		selectedBuild := m.builds[m.cursor]
		status := selectedBuild.Status

		var commands []string

		// Launch: only for Local builds
		if status == "Local" {
			commands = append(commands, "Enter:Launch")
		}

		// Download: for Online or Update builds
		if status == "Online" || status == "Update" {
			commands = append(commands, "D:Download")
		}

		// Open directory: for Local or Update builds
		if status == "Local" || status == "Update" {
			commands = append(commands, "O:Open Dir")
		}

		// Delete: for Local or Update builds
		if status == "Local" || status == "Update" {
			commands = append(commands, "X:Delete")
		}

		// Cancel: for in-progress downloads
		if strings.HasPrefix(status, "Downloading") ||
			status == "Preparing" ||
			status == "Extracting" {
			commands = append(commands, "C:Cancel")
		}

		footerKeybinds1 = strings.Join(commands, "  ")
	}

	// Second line: always the same global commands
	footerKeybinds2 = "F:Fetch  S:Settings  Q:Quit  R:Sort  ←→:Column"

	// Render footer
	footerStyle := lp.NewStyle().MarginTop(1).Faint(true)
	footerContent := footerKeybinds1 + "\n" + footerKeybinds2
	sb.WriteString(footerStyle.Render(footerContent))

	return sb.String()
}

// renderConfirmationDialog creates a standard confirmation dialog
func (m Model) renderConfirmationDialog(title string, messageLines []string, yesText string, noText string, width int) string {
	var viewBuilder strings.Builder

	// Create a styled border box
	boxStyle := lp.NewStyle().
		BorderStyle(lp.RoundedBorder()).
		BorderForeground(lp.Color("11")). // Yellow border
		Padding(1, 2)

	// Title with warning styling
	titleStyle := lp.NewStyle().
		Foreground(lp.Color("11")). // Yellow text
		Bold(true)

	// Create the content
	var contentBuilder strings.Builder
	contentBuilder.WriteString(titleStyle.Render(title) + "\n\n")

	// Add all message lines
	for _, line := range messageLines {
		contentBuilder.WriteString(line + "\n")
	}
	contentBuilder.WriteString("\n")

	// Button styling
	yesStyle := lp.NewStyle().
		Foreground(lp.Color("9")). // Red for delete
		Bold(true)
	noStyle := lp.NewStyle().
		Foreground(lp.Color("10")). // Green for cancel
		Bold(true)

	contentBuilder.WriteString(yesStyle.Render(yesText) + "    ")
	contentBuilder.WriteString(noStyle.Render(noText))

	// Combine everything in the box
	confirmBox := boxStyle.Width(width).Render(contentBuilder.String())

	// Center the box in the terminal
	viewBuilder.WriteString("\n\n") // Add some top spacing
	viewBuilder.WriteString(lp.Place(m.terminalWidth, 20,
		lp.Center, lp.Center,
		confirmBox))
	viewBuilder.WriteString("\n\n")

	return viewBuilder.String()
}

// renderDeleteConfirmView handles rendering the delete confirmation view
func (m Model) renderDeleteConfirmView() string {
	// Build version styling
	buildStyle := lp.NewStyle().
		Foreground(lp.Color("15")). // White text
		Bold(true)

	// Create the message with styled build name
	buildText := buildStyle.Render("Blender " + m.deleteCandidate)
	messageLines := []string{
		"Are you sure you want to delete " + buildText + "?",
		"This will permanently remove this build from your system.",
	}

	return m.renderConfirmationDialog(
		"Confirm Deletion",
		messageLines,
		"[Y] Yes, delete it",
		"[N] No, cancel",
		50, // Width of the dialog
	)
}

// renderCleanupConfirmView handles rendering the cleanup confirmation view
func (m Model) renderCleanupConfirmView() string {
	messageLines := []string{
		fmt.Sprintf("Are you sure you want to clean up %d old builds?", m.oldBuildsCount),
		fmt.Sprintf("This will free up %s of disk space.", util.FormatSize(m.oldBuildsSize)),
		"All backed up builds in the .oldbuilds directory will be permanently deleted.",
	}

	return m.renderConfirmationDialog(
		"Confirm Cleanup",
		messageLines,
		"[Y] Yes, delete them",
		"[N] No, cancel",
		60, // Width of the dialog
	)
}

// Define a sort function type for better organization
type sortFunc func(a, b model.BlenderBuild) bool

// sortBuilds sorts the builds based on the selected column and sort order
func sortBuilds(builds []model.BlenderBuild, column int, reverse bool) []model.BlenderBuild {
	// Create a copy of builds to avoid modifying the original
	sortedBuilds := make([]model.BlenderBuild, len(builds))
	copy(sortedBuilds, builds)

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

// Command to get info about old builds
func getOldBuildsInfoCmd(cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		count, size, err := local.GetOldBuildsInfo(cfg.DownloadDir)
		return oldBuildsInfo{
			count: count,
			size:  size,
			err:   err,
		}
	}
}

// Command to clean up old builds
func cleanupOldBuildsCmd(cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		err := local.DeleteAllOldBuilds(cfg.DownloadDir)
		return cleanupOldBuildsMsg{err: err}
	}
}

// Helper function to update focus styling for settings inputs
func updateFocusStyles(m *Model, oldFocus int) {
	// Update the prompt style of all inputs
	for i := 0; i < len(m.settingsInputs); i++ {
		if i == m.focusIndex {
			// Just update the style, don't focus in navigation mode
			m.settingsInputs[i].PromptStyle = selectedRowStyle
		} else {
			m.settingsInputs[i].PromptStyle = regularRowStyle
		}
	}
}

// Helper function to save settings
func saveSettings(m Model) (tea.Model, tea.Cmd) {
	m.config.DownloadDir = m.settingsInputs[0].Value()
	m.config.VersionFilter = m.settingsInputs[1].Value()
	err := config.SaveConfig(m.config)
	if err != nil {
		m.err = fmt.Errorf("failed to save config: %w", err)
	} else {
		m.err = nil
		m.currentView = viewList
		// If list is empty, trigger initial local scan now
		if len(m.builds) == 0 {
			m.isLoading = true
			return m, scanLocalBuildsCmd(m.config)
		}
	}
	return m, nil
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

// Handler for canceling download of selected build
func (m Model) handleCancelDownload() (tea.Model, tea.Cmd) {
	if len(m.builds) == 0 || m.cursor >= len(m.builds) {
		return m, nil
	}

	selectedBuild := m.builds[m.cursor]
	buildVersion := selectedBuild.Version

	// Lock *only* to safely read the map
	m.downloadMutex.Lock()
	downloadState, isDownloading := m.downloadStates[buildVersion]
	// Check if it's in a cancellable state *while holding the lock*
	canCancel := isDownloading &&
		(downloadState.Message == "Downloading" ||
			downloadState.Message == "Preparing" ||
			downloadState.Message == "Extracting")
	m.downloadMutex.Unlock() // Unlock immediately after reading

	// If not downloading or not in a cancellable state, do nothing
	if !canCancel {
		return m, nil
	}

	// Signal cancellation by closing the channel (thread-safe)
	// No mutex needed here.
	select {
	case <-downloadState.CancelCh:
		// Already closed, do nothing
	default:
		// Close the channel to signal cancellation
		close(downloadState.CancelCh)
	}

	// The download goroutine will see this and update its state message to "Cancelled"
	// The tick handler will then pick up that state change.
	// Return immediately, no command, no UI update here.
	return m, nil
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
