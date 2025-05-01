package tui

import (
	"TUI-Blender-Launcher/config"
	"TUI-Blender-Launcher/launch"
	"TUI-Blender-Launcher/local"
	"TUI-Blender-Launcher/model"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Helper to update focused input
func (m *Model) updateInputs(msg tea.Msg) tea.Cmd {
	// Make sure we have inputs to update
	if len(m.settingsInputs) == 0 {
		return nil
	}

	var cmds []tea.Cmd = make([]tea.Cmd, len(m.settingsInputs))

	// Only update the currently focused input
	if m.focusIndex >= 0 && m.focusIndex < len(m.settingsInputs) {
		// Update only the focused input field
		var cmd tea.Cmd
		m.settingsInputs[m.focusIndex], cmd = m.settingsInputs[m.focusIndex].Update(msg)
		cmds[m.focusIndex] = cmd
	}

	return tea.Batch(cmds...)
}

// Helper functions for handling specific actions in list view
func (m *Model) handleLaunchBlender() (tea.Model, tea.Cmd) {
	if len(m.builds) > 0 && m.cursor < len(m.builds) {
		selectedBuild := m.builds[m.cursor]
		// Only attempt to launch if it's a local build or has an update available
		if selectedBuild.Status == model.StateLocal || selectedBuild.Status == model.StateUpdate {
			cmd := local.LaunchBlenderCmd(m.config.DownloadDir, selectedBuild.Version)
			return m, cmd
		}
	}
	return m, nil
}

// handleOpenBuildDir opens the build directory for a specific version
func (m *Model) handleOpenBuildDir() (tea.Model, tea.Cmd) {
	if len(m.builds) > 0 && m.cursor < len(m.builds) {
		selectedBuild := m.builds[m.cursor]
		// Only open dir if it's a local build or has an update available
		if selectedBuild.Status == model.StateLocal || selectedBuild.Status == model.StateUpdate {
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

// handleStartDownload initiates a download for the selected build
func (m *Model) handleStartDownload() (tea.Model, tea.Cmd) {
	if len(m.builds) > 0 && m.cursor < len(m.builds) {
		selectedBuild := m.builds[m.cursor]
		// Allow downloading Online, Update, Failed, and Cancelled builds
		if selectedBuild.Status == model.StateOnline ||
			selectedBuild.Status == model.StateUpdate ||
			selectedBuild.Status == model.StateFailed ||
			selectedBuild.Status == model.StateCancelled { // StateNone == Cancelled
			// Generate a unique build ID using version and hash
			buildID := selectedBuild.Version
			if selectedBuild.Hash != "" {
				buildID = selectedBuild.Version + "-" + selectedBuild.Hash[:8]
			}

			// Update status to Downloading immediately for UI feedback
			selectedBuild.Status = model.StateDownloading
			m.builds[m.cursor] = selectedBuild

			// Store the active download ID for UI rendering
			m.activeDownloadID = buildID

			// Start the download using the download manager command
			return m, m.commands.DoDownload(selectedBuild)
		}
	}
	return m, nil
}

// handleCancelDownload cancels an active download
func (m *Model) handleCancelDownload() (tea.Model, tea.Cmd) {
	if len(m.builds) == 0 || m.cursor >= len(m.builds) {
		return m, nil
	}

	// Create buildID for the selected build first
	selectedBuild := m.builds[m.cursor]
	selectedBuildID := selectedBuild.Version
	if selectedBuild.Hash != "" {
		selectedBuildID = selectedBuild.Version + "-" + selectedBuild.Hash[:8]
	}

	// Use activeDownloadID if set; otherwise, use the selected build ID
	buildID := m.activeDownloadID
	if buildID == "" {
		buildID = selectedBuildID
	}

	// Cancel the download using the download manager
	m.commands.downloads.CancelDownload(buildID)

	// Update the build status to Cancelled (StateNone) after cancellation
	// so it shows as cancelled until next fetch
	for i, build := range m.builds {
		buildID := build.Version
		if build.Hash != "" {
			buildID = build.Version + "-" + build.Hash[:8]
		}

		// Update the status of both the selected build and any build matching the active download
		if buildID == m.activeDownloadID || buildID == selectedBuildID {
			// Only update if it's in a downloading or extracting state
			if m.builds[i].Status == model.StateDownloading ||
				m.builds[i].Status == model.StateExtracting {
				m.builds[i].Status = model.StateCancelled // Set to Cancelled
			}
		}
	}

	// Clear the active download ID
	m.activeDownloadID = ""

	return m, nil
}

// handleShowSettings shows the settings screen
func (m *Model) handleShowSettings() (tea.Model, tea.Cmd) {
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

	// Update build type selection with current build type
	for i, opt := range m.buildTypeOptions {
		if opt == m.config.BuildType {
			m.buildTypeIndex = i
			m.buildType = opt
			break
		}
	}

	// Focus first input (but don't focus for editing yet)
	m.focusIndex = 0

	// Ensure all inputs are properly styled based on focus state
	for i := range m.settingsInputs {
		if i == m.focusIndex {
			m.settingsInputs[i].PromptStyle = selectedRowStyle
		} else {
			m.settingsInputs[i].PromptStyle = regularRowStyle
		}
		// Ensure all are blurred initially
		m.settingsInputs[i].Blur()
	}

	return m, nil
}

// handleDeleteBuild prepares to delete a build
func (m *Model) handleDeleteBuild() (tea.Model, tea.Cmd) {
	if len(m.builds) > 0 && m.cursor < len(m.builds) {
		selectedBuild := m.builds[m.cursor]
		if selectedBuild.Status == model.StateDownloading || selectedBuild.Status == model.StateExtracting {
			return m.handleCancelDownload()
		}
		// Only allow deleting local builds or builds that can be updated
		if selectedBuild.Status == model.StateLocal || selectedBuild.Status == model.StateUpdate {
			return m, func() tea.Msg {
				success, err := local.DeleteBuild(m.config.DownloadDir, selectedBuild.Version)
				if err != nil {
					return errMsg{err}
				}
				if !success {
					return errMsg{fmt.Errorf("failed to delete build %s", selectedBuild.Version)}
				}
				// Remove the deleted build from the list
				indexToRemove := -1
				for i, b := range m.builds {
					if b.Version == selectedBuild.Version {
						indexToRemove = i
						break
					}
				}
				if indexToRemove != -1 {
					m.builds = append(m.builds[:indexToRemove], m.builds[indexToRemove+1:]...)
					if len(m.builds) == 0 {
						m.cursor = 0
					} else if m.cursor >= len(m.builds) {
						m.cursor = len(m.builds) - 1
					}
				}
				m.builds = model.SortBuilds(m.builds, m.sortColumn, m.sortReversed)
				return nil
			}
		}
	}
	return m, nil
}

// handleLocalBuildsScanned processes the result of scanning local builds
func (m *Model) handleLocalBuildsScanned(msg localBuildsScannedMsg) (tea.Model, tea.Cmd) {
	// If there was an error scanning builds, store it but continue with empty list
	if msg.err != nil {
		m.err = msg.err
		m.builds = []model.BlenderBuild{}
		return m, nil
	}

	// Set builds to local builds only, don't fetch online builds automatically
	m.builds = msg.builds

	// Apply version filter if set
	if m.config.VersionFilter != "" {
		m.builds = m.applyVersionFilter(m.builds)
	}

	// Sort builds immediately for better visual feedback
	m.builds = model.SortBuilds(m.builds, m.sortColumn, m.sortReversed)

	// Reset cursor and startIndex when loading new builds
	if len(m.builds) > 0 {
		m.cursor = 0
		m.startIndex = 0
	}

	return m, nil
}

// handleBuildsFetched processes the result of fetching builds from the API
func (m *Model) handleBuildsFetched(msg buildsFetchedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err
		return m, nil
	}

	// Preserve only local builds from the current list.
	// Failed/Cancelled states are reset by the fetch command itself.
	var localBuilds []model.BlenderBuild
	for _, build := range m.builds {
		if build.Status == model.StateLocal {
			localBuilds = append(localBuilds, build)
		}
	}

	// Start with local builds + newly fetched builds.
	m.builds = localBuilds
	m.builds = append(m.builds, msg.builds...)

	// Deduplicate and sort (will be handled by UpdateBuildStatus)
	// We call UpdateBuildStatus which will determine the final statuses (Local, Online, Update)
	// based on comparison between local and the combined list.

	// Apply version filter if set *before* updating status
	if m.config.VersionFilter != "" {
		m.builds = m.applyVersionFilter(m.builds)
	}

	// Reset cursor and startIndex for a consistent view
	if len(m.builds) > 0 {
		m.cursor = 0
		m.startIndex = 0
	} else {
		m.cursor = 0
		m.startIndex = 0
	}

	// Update the status based on what's available locally vs online.
	// This command now receives the combined list (local + fetched)
	// and should correctly assign Local, Online, or Update status.
	return m, m.commands.UpdateBuildStatus(m.builds)
}

// applyVersionFilter filters builds by version, keeping only builds with version >= filter value
func (m *Model) applyVersionFilter(builds []model.BlenderBuild) []model.BlenderBuild {
	if m.config.VersionFilter == "" {
		return builds
	}

	filtered := make([]model.BlenderBuild, 0)
	for _, build := range builds {
		// Always keep local builds regardless of version filter
		if build.Status == model.StateLocal {
			filtered = append(filtered, build)
			continue
		}

		// Compare versions (simple string comparison works for Blender's versioning scheme)
		if build.Version >= m.config.VersionFilter {
			filtered = append(filtered, build)
		}
	}
	return filtered
}

// handleBuildsUpdated finalizes the build list after determining local/online status
func (m *Model) handleBuildsUpdated(msg buildsUpdatedMsg) (tea.Model, tea.Cmd) {
	// Replace builds with updated ones that have correct status
	m.builds = msg.builds

	// Create a set of build IDs that are currently downloading or extracting
	// according to the *final* build list we just received.
	activeDownloadIDs := make(map[string]bool)
	for _, build := range m.builds {
		if build.Status == model.StateDownloading || build.Status == model.StateExtracting {
			buildID := build.Version
			if build.Hash != "" {
				buildID = build.Version + "-" + build.Hash[:8]
			}
			activeDownloadIDs[buildID] = true
		}
	}
	// Remove any state from m.downloadStates that isn't in the active set.
	// This ensures Failed/Cancelled states lingering in the TUI cache are removed
	// once the fetch/update cycle completes.
	for id := range m.downloadStates {
		if !activeDownloadIDs[id] {
			delete(m.downloadStates, id)
		}
	}

	// Apply version filter if set
	if m.config.VersionFilter != "" {
		m.builds = m.applyVersionFilter(m.builds)
	}

	m.builds = model.SortBuilds(m.builds, m.sortColumn, m.sortReversed)

	// Ensure cursor is within bounds and visible
	visibleRowsCount := m.terminalHeight - 7
	if visibleRowsCount < 1 {
		visibleRowsCount = 1
	}

	if len(m.builds) > 0 {
		if m.cursor >= len(m.builds) {
			m.cursor = len(m.builds) - 1
		}

		// Ensure cursor is visible
		if m.cursor < m.startIndex || m.cursor >= m.startIndex+visibleRowsCount {
			// If cursor is outside visible area, adjust startIndex
			m.startIndex = m.cursor - visibleRowsCount/2
			if m.startIndex < 0 {
				m.startIndex = 0
			}
		}
	}

	// No further commands needed here, just update the UI state.
	return m, nil
}

// handleBlenderExec handles launching Blender after selecting it
func (m *Model) handleBlenderExec(msg model.BlenderExecMsg) (tea.Model, tea.Cmd) {
	// Store Blender info
	execInfo := msg

	// Launch Blender directly using the launch package
	return m, func() tea.Msg {
		blenderExe := execInfo.Executable

		// Import the launch package at the top of the file if needed
		err := launch.BlenderInNewTerminal(blenderExe)
		if err != nil {
			return errMsg{fmt.Errorf("failed to launch Blender: %w", err)}
		}

		// Return a message indicating Blender was launched successfully
		return nil
	}
}

// handleDownloadProgress processes tick messages for download progress updates
func (m *Model) handleDownloadProgress(msg tickMsg) (tea.Model, tea.Cmd) {

	activeDownloads := 0
	var progressCmds []tea.Cmd
	// Lists to store IDs identified for state change/cleanup
	completedDownloads := make([]string, 0)
	stalledDownloads := make([]string, 0)
	cancelledDownloads := make([]string, 0)

	// If commands exists, sync download states from it
	if m.commands != nil && m.commands.downloads != nil {
		// Get states from download manager
		states := m.commands.downloads.GetAllStates()

		// Update our local copy - always update for downloads
		for id, state := range states {
			// For downloads and extractions, always update state to ensure UI reflects latest
			if state.BuildState == model.StateDownloading || state.BuildState == model.StateExtracting {
				m.downloadStates[id] = state

				// Check for stalled downloads - detect if a download hasn't progressed in 15 seconds
				if state.BuildState == model.StateDownloading && time.Since(state.LastUpdated) > 15*time.Second {
					// Mark as stalled (will transition to failed)
					stalledDownloads = append(stalledDownloads, id)

					// Set the state to failed
					state.BuildState = model.StateFailed
					state.Progress = 0.0
					m.downloadStates[id] = state

					// Cancel the download in the download manager
					m.commands.downloads.CancelDownload(id)
				}
			} else {
				// For other states, only update when changed significantly
				existingState, exists := m.downloadStates[id]
				if !exists ||
					existingState.BuildState != state.BuildState ||
					math.Abs(existingState.Progress-state.Progress) >= 0.01 {
					m.downloadStates[id] = state
				}
			}
		}
	}

	// Temporary copy of download states for use after unlock
	tempStates := make(map[string]model.DownloadState)

	// Process download states while holding the lock
	for id, state := range m.downloadStates {
		tempStates[id] = *state // Store a copy

		if state.BuildState == model.StateLocal || strings.HasPrefix(state.BuildState.String(), "Failed") {
			// Download completed or failed
			completedDownloads = append(completedDownloads, id)
		} else if state.BuildState == model.StateCancelled {
			// Download was cancelled
			cancelledDownloads = append(cancelledDownloads, id)
		} else if state.BuildState == model.StateDownloading ||
			state.BuildState == model.StateExtracting {
			// Active download
			activeDownloads++

			// Only update progress bar for the active download
			if id == m.activeDownloadID {
				// Always update progress bar for active downloads
				progressCmds = append(progressCmds, m.progressBar.SetPercent(state.Progress))
				m.lastRenderState[id+"_progressbar"] = state.Progress
			}
		}
	}

	// If we have no active download ID but there are active downloads, pick the first one
	if m.activeDownloadID == "" && activeDownloads > 0 {
		for id, state := range m.downloadStates {
			if state.BuildState == model.StateDownloading || state.BuildState == model.StateExtracting {
				m.activeDownloadID = id
				progressCmds = append(progressCmds, m.progressBar.SetPercent(state.Progress))
				m.lastRenderState[id+"_progressbar"] = state.Progress
				break
			}
		}
	}

	// Update build statuses for downloads/extractions to ensure they display correctly
	needsSort := false
	for i := range m.builds {
		buildID := m.builds[i].Version
		if m.builds[i].Hash != "" {
			buildID = m.builds[i].Version + "-" + m.builds[i].Hash[:8]
		}

		// Update status for active downloads - force update for any active download
		if state, ok := tempStates[buildID]; ok {
			if state.BuildState == model.StateDownloading || state.BuildState == model.StateExtracting {
				// Always update build status for downloads/extractions
				oldStatus := m.builds[i].Status
				m.builds[i].Status = state.BuildState
				if oldStatus != state.BuildState {
					needsSort = true
				}
			}
		}
	}

	// Process other state changes (completed/etc.)
	var buildsChanged bool = len(completedDownloads) > 0 ||
		len(stalledDownloads) > 0 ||
		len(cancelledDownloads) > 0

	if !buildsChanged && !needsSort {
		// If only progress changed but not statuses, no need for additional updates
		return m, tea.Batch(progressCmds...)
	}

	// Process completed, stalled, and cancelled downloads
	// For each completed download, find the matching build and update its status
	for _, id := range completedDownloads {
		if state, ok := tempStates[id]; ok {
			// Extract the version from the BuildID (before the hash if present)
			version := state.BuildID
			if strings.Contains(version, "-") {
				version = strings.Split(version, "-")[0]
			}

			for i := range m.builds {
				if m.builds[i].Version == version {
					m.builds[i].Status = state.BuildState
					needsSort = true
					break
				}
			}
		}
	}

	// Same for stalled downloads
	for _, id := range stalledDownloads {
		if state, ok := tempStates[id]; ok {
			// Extract the version from the BuildID
			version := state.BuildID
			if strings.Contains(version, "-") {
				version = strings.Split(version, "-")[0]
			}

			for i := range m.builds {
				if m.builds[i].Version == version {
					m.builds[i].Status = state.BuildState
					needsSort = true
					break
				}
			}
		}
	}

	// And for cancelled downloads
	for _, id := range cancelledDownloads {
		if state, ok := tempStates[id]; ok {
			// Extract the version from the BuildID
			version := state.BuildID
			if strings.Contains(version, "-") {
				version = strings.Split(version, "-")[0]
			}

			for i := range m.builds {
				if m.builds[i].Version == version {
					// Keep the build with Cancelled status (StateNone)
					// Don't convert to online immediately - wait for explicit fetch
					m.builds[i].Status = model.StateCancelled
					needsSort = true
					break
				}
			}
		}
	}

	// Sort if needed
	if needsSort {
		m.builds = model.SortBuilds(m.builds, m.sortColumn, m.sortReversed)
	}

	// Return any progress bar update commands
	return m, tea.Batch(progressCmds...)
}

// Helper function to update focus styling for settings inputs
func updateFocusStyles(m *Model, oldFocus int) {
	// Update the prompt style of text inputs
	for i := 0; i < len(m.settingsInputs); i++ {
		if i == m.focusIndex {
			// For the selected item, use a highlighted prompt style
			m.settingsInputs[i].PromptStyle = selectedRowStyle

			// For edit mode, focus the input
			if m.editMode && i == m.focusIndex {
				m.settingsInputs[i].Focus()
			} else if oldFocus == i && !m.editMode {
				// When exiting edit mode, blur the input
				m.settingsInputs[i].Blur()
			}
		} else {
			// Normal style for unselected items
			m.settingsInputs[i].PromptStyle = regularRowStyle

			// Ensure non-focused inputs are blurred
			m.settingsInputs[i].Blur()
		}
	}

	// No need to handle build type focus specifically - it's handled by the render function

	// Special case when entering edit mode
	if m.editMode && m.focusIndex >= 0 && m.focusIndex < len(m.settingsInputs) {
		// Make sure the focused input is actually focused
		m.settingsInputs[m.focusIndex].Focus()
	}
}

// Helper function to save settings
func saveSettings(m *Model) (tea.Model, tea.Cmd) {
	// Ensure we get the current values from the inputs
	downloadDir := m.settingsInputs[0].Value()
	versionFilter := m.settingsInputs[1].Value()
	buildType := m.buildType

	// Validate and sanitize inputs
	if downloadDir == "" {
		// Don't allow empty download dir
		m.err = fmt.Errorf("download directory cannot be empty")
		return m, nil
	}

	// Build type validation is not needed as dropdown guarantees valid values

	// Check if version filter changed
	versionFilterChanged := m.config.VersionFilter != versionFilter
	buildTypeChanged := m.config.BuildType != buildType

	// Update config values
	m.config.DownloadDir = downloadDir
	m.config.VersionFilter = versionFilter
	m.config.BuildType = buildType

	// Save the config
	err := config.SaveConfig(m.config)
	if err != nil {
		m.err = fmt.Errorf("failed to save config: %w", err)
		return m, nil
	}

	// Recreate commands with updated config
	m.commands = NewCommands(m.config)

	// Clear any errors and trigger rescans if needed
	m.err = nil

	// If returning to list view, apply version filter if it changed
	if m.currentView == viewList {
		if (versionFilterChanged || buildTypeChanged) && len(m.builds) > 0 {
			// Re-apply version filter and sort
			if m.config.VersionFilter != "" {
				m.builds = m.applyVersionFilter(m.builds)
			}
			m.builds = model.SortBuilds(m.builds, m.sortColumn, m.sortReversed)

			// Reset cursor if needed
			if len(m.builds) > 0 && m.cursor >= len(m.builds) {
				m.cursor = len(m.builds) - 1
				m.startIndex = 0
			}
		} else if len(m.builds) == 0 {
			return m, m.commands.ScanLocalBuilds()
		}
		return m, nil
	}

	return m, nil
}
