package tui

import (
	"TUI-Blender-Launcher/config"
	"TUI-Blender-Launcher/local"
	"TUI-Blender-Launcher/model"
	"fmt"
	"log"
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
		// Only attempt to launch if it's a local build
		if selectedBuild.Status == model.StateLocal {
			// Add launch logic here
			log.Printf("Launching Blender %s", selectedBuild.Version)
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
		// Allow downloading both Online builds and Updates
		if selectedBuild.Status == model.StateOnline || selectedBuild.Status == model.StateUpdate {
			// Generate a unique build ID using version and hash
			buildID := selectedBuild.Version
			if selectedBuild.Hash != "" {
				buildID = selectedBuild.Version + "-" + selectedBuild.Hash[:8]
			}

			// Update status to avoid duplicate downloads
			selectedBuild.Status = model.StateDownloading
			m.builds[m.cursor] = selectedBuild

			// Store the active download ID for UI rendering
			m.activeDownloadID = buildID

			// Send message to start download
			return m, func() tea.Msg {
				return startDownloadMsg{
					build:   selectedBuild,
					buildID: buildID,
				}
			}
		}
	}
	return m, nil
}

// handleCancelDownload cancels an active download
func (m *Model) handleCancelDownload() (tea.Model, tea.Cmd) {
	if len(m.builds) == 0 || m.cursor >= len(m.builds) {
		return m, nil
	}

	// Use the activeDownloadID that was set when detecting the cancellable download
	buildID := m.activeDownloadID
	if buildID == "" {
		// Fallback to build version if activeDownloadID isn't set
		selectedBuild := m.builds[m.cursor]
		buildID = selectedBuild.Version
		// Try to recreate the buildID format
		if selectedBuild.Hash != "" {
			buildID = selectedBuild.Version + "-" + selectedBuild.Hash[:8]
		}
	}

	// Get download state from the manager
	downloadState := m.commands.downloads.GetState(buildID)
	if downloadState == nil {
		return m, nil // Nothing to cancel
	}

	canCancel := downloadState.BuildState == model.StateDownloading ||
		downloadState.BuildState == model.StateExtracting

	// If not downloading or not in a cancellable state, do nothing
	if !canCancel {
		return m, nil
	}

	// Cancel the download
	m.commands.downloads.CancelDownload(buildID)

	// We've already used activeDownloadID, now clear it
	m.activeDownloadID = ""

	return m, nil
}

// handleShowSettings shows the settings screen
func (m *Model) handleShowSettings() (tea.Model, tea.Cmd) {
	m.currentView = viewSettings
	m.editMode = false // Ensure we start in navigation mode

	// Initialize settings inputs if not already done
	if len(m.settingsInputs) == 0 {
		m.settingsInputs = make([]textinput.Model, 3)

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
		// Only allow deleting local builds or builds that can be updated
		if selectedBuild.Status == model.StateLocal || selectedBuild.Status == model.StateUpdate {
			// Directly delete the build without confirmation
			return m, func() tea.Msg {
				success, err := local.DeleteBuild(m.config.DownloadDir, selectedBuild.Version)
				if err != nil {
					return errMsg{err}
				}

				if !success {
					return errMsg{fmt.Errorf("failed to delete build %s", selectedBuild.Version)}
				}

				// Update build statuses after deletion
				for i := range m.builds {
					if m.builds[i].Version == selectedBuild.Version {
						m.builds[i].Status = model.StateOnline
						break
					}
				}
				m.builds = sortBuilds(m.builds, m.sortColumn, m.sortReversed)
				// Return a proper message instead of setting view directly
				return deleteBuildCompleteMsg{}
			}
		}
	}
	return m, nil
}

// handleLocalBuildsScanned processes the result of scanning local builds
func (m *Model) handleLocalBuildsScanned(msg localBuildsScannedMsg) (tea.Model, tea.Cmd) {
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

// handleBuildsFetched processes the result of fetching builds from the API
func (m *Model) handleBuildsFetched(msg buildsFetchedMsg) (tea.Model, tea.Cmd) {
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
	cmdManager := NewCommands(m.config)
	return m, cmdManager.UpdateBuildStatus(m.builds)
}

// handleBuildsUpdated processes the result of updating build statuses
func (m *Model) handleBuildsUpdated(msg buildsUpdatedMsg) (tea.Model, tea.Cmd) {
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

// handleBlenderExec handles launching Blender after selecting it
func (m *Model) handleBlenderExec(msg model.BlenderExecMsg) (tea.Model, tea.Cmd) {
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

// handleDownloadProgress processes tick messages for download progress updates
func (m *Model) handleDownloadProgress(msg tickMsg) (tea.Model, tea.Cmd) {
	now := time.Now()

	// Get all download states
	states := m.commands.downloads.GetAllStates()

	activeDownloads := 0
	var progressCmds []tea.Cmd
	// Lists to track completed, stalled, and cancelled downloads
	completedDownloads := make([]string, 0)
	stalledDownloads := make([]string, 0)
	cancelledDownloads := make([]string, 0)

	// Temporary copy of states for use
	tempStates := make(map[string]model.DownloadState)

	// Process download states
	for id, state := range states {
		tempStates[id] = *state // Store a copy

		if state.BuildState == model.StateLocal || strings.HasPrefix(state.BuildState.String(), "Failed") {
			// Download completed or failed
			completedDownloads = append(completedDownloads, id)
		} else if state.BuildState == model.StateNone {
			// Download was cancelled
			cancelledDownloads = append(cancelledDownloads, id)
		} else if state.BuildState == model.StateDownloading ||
			state.BuildState == model.StateExtracting {
			// Active download
			timeSinceUpdate := now.Sub(state.LastUpdated)
			if timeSinceUpdate > state.StallDuration {
				// Download appears stalled
				log.Printf("WARNING: Download for %s stalled (no updates for %v), marking as failed",
					id, timeSinceUpdate.Round(time.Second))

				// Update status in our temporary copy
				tempStateCopy := *state
				tempStateCopy.BuildState = model.StateFailed
				tempStates[id] = tempStateCopy
				stalledDownloads = append(stalledDownloads, id)

				// Cancel the stalled download
				m.commands.downloads.CancelDownload(id)
			} else {
				// Active download that's not stalled
				activeDownloads++

				// Only update progress bar for the active download
				if id == m.activeDownloadID {
					// Queue progress bar update
					progressCmds = append(progressCmds, m.progressBar.SetPercent(state.Progress))
				}
			}
		}
	}

	// Update build statuses based on download states
	needsSort := false
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
					m.builds[i].Status = model.StateOnline
					needsSort = true
					break
				}
			}
		}
	}

	// Sort if needed
	if needsSort {
		m.builds = sortBuilds(m.builds, m.sortColumn, m.sortReversed)
	}

	// Return any progress bar update commands
	return m, tea.Batch(progressCmds...)
}

// Helper function to update focus styling for settings inputs
func updateFocusStyles(m *Model, oldFocus int) {
	// Update the prompt style of all inputs
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

	// Validate and sanitize inputs
	if downloadDir == "" {
		// Don't allow empty download dir
		m.err = fmt.Errorf("download directory cannot be empty")
		return m, nil
	}

	// Update config values
	m.config.DownloadDir = downloadDir
	m.config.VersionFilter = versionFilter

	// Save the config
	err := config.SaveConfig(m.config)
	if err != nil {
		m.err = fmt.Errorf("failed to save config: %w", err)
		return m, nil
	}

	// Clear any errors and trigger rescans if needed
	m.err = nil

	// If returning to list view, trigger a new scan
	if m.currentView == viewList {
		m.isLoading = true
		cmdManager := NewCommands(m.config)
		return m, tea.Batch(
			cmdManager.ScanLocalBuilds(),
			cmdManager.FetchBuilds(),
		)
	}

	return m, nil
}

// handleCleanupOldBuilds handles cleaning up old Blender builds
func (m *Model) handleCleanupOldBuilds() (tea.Model, tea.Cmd) {
	return m, func() tea.Msg {
		// Create .oldbuilds directory if it doesn't exist
		oldBuildsDir := filepath.Join(m.config.DownloadDir, ".oldbuilds")
		if err := os.MkdirAll(oldBuildsDir, 0755); err != nil {
			return errMsg{fmt.Errorf("failed to create .oldbuilds directory: %w", err)}
		}

		// Get local builds and group by major version
		builds, err := local.ScanLocalBuilds(m.config.DownloadDir)
		if err != nil {
			return errMsg{fmt.Errorf("failed to scan local builds: %w", err)}
		}

		// Group builds by major version (e.g., "3.6", "4.0")
		buildsByVersion := make(map[string][]model.BlenderBuild)
		for _, build := range builds {
			// Extract major version (e.g., "3.6" from "3.6.1")
			parts := strings.Split(build.Version, ".")
			if len(parts) >= 2 {
				majorVersion := parts[0] + "." + parts[1]
				buildsByVersion[majorVersion] = append(buildsByVersion[majorVersion], build)
			}
		}

		// For each major version, keep only the latest build
		for majorVersion, versionBuilds := range buildsByVersion {
			// Skip if there's only one build for this major version
			if len(versionBuilds) <= 1 {
				continue
			}

			// Sort builds by version (newest first)
			// We can use the sortBuilds function to do this
			sortedBuilds := sortBuilds(versionBuilds, 0, true)

			// Keep the newest build, move others to .oldbuilds
			for i := 1; i < len(sortedBuilds); i++ {
				oldBuild := sortedBuilds[i]

				// Get the build directory by version
				entries, err := os.ReadDir(m.config.DownloadDir)
				if err != nil {
					continue
				}

				for _, entry := range entries {
					if !entry.IsDir() || entry.Name() == ".downloading" || entry.Name() == ".oldbuilds" {
						continue
					}

					dirPath := filepath.Join(m.config.DownloadDir, entry.Name())
					buildInfo, err := local.ReadBuildInfo(dirPath)
					if err != nil || buildInfo == nil {
						continue
					}

					// Found the build directory for this version
					if buildInfo.Version == oldBuild.Version {
						// Move to .oldbuilds directory
						target := filepath.Join(oldBuildsDir, entry.Name())
						if err := os.Rename(dirPath, target); err != nil {
							log.Printf("Failed to move old build %s (major version %s): %v",
								oldBuild.Version, majorVersion, err)
						} else {
							log.Printf("Moved old build %s (major version %s) to .oldbuilds",
								oldBuild.Version, majorVersion)
						}
						break
					}
				}
			}
		}

		return cleanupCompleteMsg{}
	}
}
