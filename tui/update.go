package tui

import (
	"TUI-Blender-Launcher/model"
	"TUI-Blender-Launcher/types"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
)

// deleteBuildCompleteMsg is sent when a build is successfully deleted
type deleteBuildCompleteMsg struct{}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	var cmds []tea.Cmd

	// Start with local build scan to get builds already on disk
	cmds = append(cmds, scanLocalBuildsCmd(m.config))

	// Get information about old builds
	cmds = append(cmds, getOldBuildsInfoCmd(m.config))

	// Start the continuous tick system for UI updates
	cmds = append(cmds, tickCmd())

	// Start a dedicated UI refresh cycle
	cmds = append(cmds, uiRefreshCmd())

	return tea.Batch(cmds...)
}

// Update updates the model based on messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Special priority handling for view-specific cases
	switch m.currentView {
	case viewSettings, viewInitialSetup:
		// In settings screens, let the settings handler take priority for key events
		if _, ok := msg.(tea.KeyMsg); ok {
			return m.updateSettingsView(msg)
		}
	case viewCleanupConfirm:
		// In cleanup confirm dialog, handle keys there first
		if _, ok := msg.(tea.KeyMsg); ok {
			return m.updateCleanupConfirmView(msg)
		}
	case viewQuitConfirm:
		// In quit confirm dialog, handle keys there first
		if _, ok := msg.(tea.KeyMsg); ok {
			return m.updateQuitConfirmView(msg)
		}
	}

	// Handle global messages
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Update window dimensions and layout using our new method
		m.UpdateWindowSize(msg.Width, msg.Height)

		// Make sure cursor position is still valid
		if len(m.builds) > 0 && m.cursor >= len(m.builds) {
			m.cursor = len(m.builds) - 1
		}

		// Adjust scroll position to keep cursor visible
		if m.cursor < m.scrollOffset {
			m.scrollOffset = m.cursor
		} else if m.cursor >= m.scrollOffset+m.visibleRows {
			m.scrollOffset = m.cursor - m.visibleRows + 1
		}

		// Ensure scroll offset is never negative
		if m.scrollOffset < 0 {
			m.scrollOffset = 0
		}

		return m, nil

	case progress.FrameMsg:
		// Update the progress bar if we're displaying it
		progressModel, cmd := m.progressBar.Update(msg)
		m.progressBar = progressModel.(progress.Model)
		return m, cmd

	case forceRenderMsg:
		// This is just to force the UI to refresh
		// Return the model with another UI refresh command to keep the cycle going
		return m, uiRefreshCmd()

	// Handle errors first, showing them in any view
	case errMsg:
		m.err = msg.err
		return m, nil

	// Handle various update message types
	case localBuildsScannedMsg:
		return m.handleLocalBuildsScanned(msg)

	case buildsFetchedMsg:
		return m.handleBuildsFetched(msg)

	case buildsUpdatedMsg:
		return m.handleBuildsUpdated(msg)

	case model.BlenderExecMsg:
		return m.handleBlenderExec(msg)

	case deleteBuildCompleteMsg:
		// When build deletion is complete, return to list view
		m.currentView = viewList
		return m, fetchBuildsCmd(m.config)

	case startDownloadMsg:
		// Store the active download ID for UI rendering
		m.activeDownloadID = msg.buildID

		// Ensure we have a continuous UI refresh during download/extraction operations
		// so the user can still interact with the TUI
		var cmds []tea.Cmd

		// Add download command
		cmds = append(cmds, doDownloadCmd(msg.build, m.config, m.downloadStates, &m.downloadMutex))

		// Add continuous UI refresh command during active processes
		cmds = append(cmds, uiRefreshCmd())

		return m, tea.Batch(cmds...)

	case downloadCompleteMsg:
		// Post-download processing is now handled in the download command
		return m, nil

	case resetStatusMsg:
		// Handle resetting status after cancellation
		for i := range m.builds {
			if m.builds[i].Version == msg.version {
				// Reset to Online or Update based on previous status
				localPath := filepath.Join(m.config.DownloadDir, m.builds[i].Version)
				if _, err := os.Stat(localPath); err == nil {
					// If we have a local version, mark as update
					m.builds[i].Status = types.StateUpdate
				} else {
					// If no local version, mark as online
					m.builds[i].Status = types.StateOnline
				}
				m.builds = sortBuilds(m.builds, m.sortColumn, m.sortReversed)
				break
			}
		}
		return m, nil

	case oldBuildsInfo:
		if msg.err != nil {
			log.Printf("Error getting old builds info: %v", msg.err)
		} else {
			m.oldBuildsCount = msg.count
			m.oldBuildsSize = msg.size
		}
		return m, nil

	case cleanupOldBuildsMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			// Get updated old builds count
			return m, getOldBuildsInfoCmd(m.config)
		}
		return m, nil

	case tickMsg:
		// Tick events for updating downloads
		return m.handleDownloadProgress(msg)

	default:
		return m, nil
	}

	// Based on the current view, update accordingly
	switch m.currentView {
	case viewList:
		return m.updateListView(msg)
	case viewSettings:
		return m.updateSettingsView(msg)
	case viewInitialSetup:
		return m.updateSettingsView(msg)
	case viewCleanupConfirm:
		return m.updateCleanupConfirmView(msg)
	case viewQuitConfirm:
		return m.updateQuitConfirmView(msg)
	}

	return m, nil
}

// updateSettingsView handles key events in the settings view
func (m Model) updateSettingsView(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle different message types
	switch msg := msg.(type) {
	case tickMsg:
		// Process tick messages for downloads but continue with other processing
		newModel, cmd := m.handleDownloadProgress(msg)
		// Return the updated model and command, but don't short-circuit other message handling
		return newModel, cmd

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+c", "q"))):
			// Check if there are any active downloads before quitting
			hasActiveDownloads := false
			m.downloadMutex.Lock()
			for _, state := range m.downloadStates {
				if state.BuildState == types.StateDownloading ||
					state.BuildState == types.StatePreparing ||
					state.BuildState == types.StateExtracting {
					hasActiveDownloads = true
					break
				}
			}
			m.downloadMutex.Unlock()

			if hasActiveDownloads {
				// Show confirmation dialog
				m.currentView = viewQuitConfirm
				return m, nil
			}
			// No active downloads, quit immediately
			return m, tea.Quit

		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			if m.editMode {
				// Exit edit mode and go back to navigation
				m.editMode = false
				updateFocusStyles(&m, m.focusIndex)
				return m, nil
			} else {
				// When not in edit mode, ESC returns to the main view
				m.currentView = viewList
				return m, nil
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("s"))):
			// If not in edit mode, 's' takes us back to the build list and saves settings
			if !m.editMode {
				m.currentView = viewList
				return saveSettings(&m)
			}
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("tab"))):
			// In edit mode, tab cycles between fields
			if m.editMode {
				oldFocus := m.focusIndex
				m.focusIndex = (m.focusIndex + 1) % len(m.settingsInputs)
				updateFocusStyles(&m, oldFocus)
				m.settingsInputs[m.focusIndex].Focus()
				return m, nil
			}
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("shift+tab"))):
			// In edit mode, shift+tab cycles between fields in reverse
			if m.editMode {
				oldFocus := m.focusIndex
				m.focusIndex = (m.focusIndex - 1 + len(m.settingsInputs)) % len(m.settingsInputs)
				updateFocusStyles(&m, oldFocus)
				m.settingsInputs[m.focusIndex].Focus()
				return m, nil
			}
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
			if !m.editMode {
				oldFocus := m.focusIndex
				m.focusIndex = (m.focusIndex - 1 + len(m.settingsInputs)) % len(m.settingsInputs)
				updateFocusStyles(&m, oldFocus)
				return m, nil
			}
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
			if !m.editMode {
				oldFocus := m.focusIndex
				m.focusIndex = (m.focusIndex + 1) % len(m.settingsInputs)
				updateFocusStyles(&m, oldFocus)
				return m, nil
			}
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			if m.editMode {
				// Save settings and return to navigation mode
				m.editMode = false
				updateFocusStyles(&m, m.focusIndex)
				return saveSettings(&m)
			} else {
				// Focus the current field for editing
				m.editMode = true
				updateFocusStyles(&m, -1)
				return m, nil
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("c"))):
			// Only in settings view, c will clean up old builds
			return m.handleCleanupOldBuilds()

		default:
			// Pass other keys to the input field if in edit mode
			if m.editMode {
				// Create a copy of the model to avoid pointer issues
				updatedModel := m
				cmd := updatedModel.updateInputs(msg)
				return updatedModel, cmd
			}
		}
	}

	// Default: keep the current model and continue the UI refresh
	return m, nil
}

// updateListView handles key events in the main list view
func (m Model) updateListView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+c", "q"))):
			// Check if there are any active downloads before quitting
			hasActiveDownloads := false
			m.downloadMutex.Lock()
			for _, state := range m.downloadStates {
				if state.BuildState == types.StateDownloading ||
					state.BuildState == types.StatePreparing ||
					state.BuildState == types.StateExtracting {
					hasActiveDownloads = true
					break
				}
			}
			m.downloadMutex.Unlock()

			if hasActiveDownloads {
				// Show confirmation dialog
				m.currentView = viewQuitConfirm
				return m, nil
			}
			// No active downloads, quit immediately
			return m, tea.Quit

		case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
			if m.cursor > 0 {
				m.cursor--

				// Handle scrolling: if cursor moves above visible area, scroll up
				if m.cursor < m.scrollOffset {
					m.scrollOffset = m.cursor
				}
			} else if len(m.builds) > 0 {
				// Wrap around to bottom
				m.cursor = len(m.builds) - 1

				// When wrapping to bottom, set scroll to show the last page of items
				lastPageStart := len(m.builds) - m.visibleRows
				if lastPageStart < 0 {
					lastPageStart = 0
				}
				m.scrollOffset = lastPageStart
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
			if len(m.builds) > 0 && m.cursor < len(m.builds)-1 {
				m.cursor++

				// Handle scrolling: if cursor moves below visible area, scroll down
				if m.cursor >= m.scrollOffset+m.visibleRows {
					m.scrollOffset = m.cursor - m.visibleRows + 1
				}
			} else {
				// Wrap around to top
				m.cursor = 0
				m.scrollOffset = 0 // Reset scroll to top when wrapping
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("page_up"))):
			// Move cursor and scroll up a full page
			if len(m.builds) > 0 {
				// Calculate new cursor position (move up by visible rows)
				m.cursor -= m.visibleRows
				if m.cursor < 0 {
					m.cursor = 0
				}

				// Also move scroll position up by visible rows
				m.scrollOffset -= m.visibleRows
				if m.scrollOffset < 0 {
					m.scrollOffset = 0
				}
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("page_down"))):
			// Move cursor and scroll down a full page
			if len(m.builds) > 0 {
				// Calculate new cursor position (move down by visible rows)
				m.cursor += m.visibleRows
				if m.cursor >= len(m.builds) {
					m.cursor = len(m.builds) - 1
				}

				// Also move scroll position down by visible rows
				m.scrollOffset += m.visibleRows
				maxScroll := len(m.builds) - m.visibleRows
				if maxScroll < 0 {
					maxScroll = 0
				}
				if m.scrollOffset > maxScroll {
					m.scrollOffset = maxScroll
				}
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("home"))):
			// Move to the first build
			if len(m.builds) > 0 {
				m.cursor = 0
				m.scrollOffset = 0
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("end"))):
			// Move to the last build
			if len(m.builds) > 0 {
				m.cursor = len(m.builds) - 1

				// Set scroll position to show last item
				m.scrollOffset = len(m.builds) - m.visibleRows
				if m.scrollOffset < 0 {
					m.scrollOffset = 0
				}
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("right", "l"))):
			// Cycle sort column forward
			lastCol := getLastVisibleColumn()
			m.sortColumn = (m.sortColumn + 1) % (lastCol + 1)
			m.builds = sortBuilds(m.builds, m.sortColumn, m.sortReversed)

		case key.Matches(msg, key.NewBinding(key.WithKeys("left", "h"))):
			// Cycle sort column backward
			lastCol := getLastVisibleColumn()
			m.sortColumn = (m.sortColumn - 1 + (lastCol + 1)) % (lastCol + 1)
			m.builds = sortBuilds(m.builds, m.sortColumn, m.sortReversed)

		case key.Matches(msg, key.NewBinding(key.WithKeys("r"))):
			// Toggle sort order
			m.sortReversed = !m.sortReversed
			m.builds = sortBuilds(m.builds, m.sortColumn, m.sortReversed)

		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			// Only handle launch if state is local
			if len(m.builds) > 0 && m.cursor < len(m.builds) {
				selectedBuild := m.builds[m.cursor]
				if selectedBuild.Status == types.StateLocal {
					return m.handleLaunchBlender()
				}
			}
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("o"))):
			// Only handle open dir if state is local or update
			if len(m.builds) > 0 && m.cursor < len(m.builds) {
				selectedBuild := m.builds[m.cursor]
				if selectedBuild.Status == types.StateLocal || selectedBuild.Status == types.StateUpdate {
					return m.handleOpenBuildDir()
				}
			}
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("d"))):
			// Can download online builds or updates
			if len(m.builds) > 0 && m.cursor < len(m.builds) {
				selectedBuild := m.builds[m.cursor]
				if selectedBuild.Status == types.StateOnline || selectedBuild.Status == types.StateUpdate {
					return m.handleStartDownload()
				}
			}
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("x"))):
			// Handle both cancelling downloads and deleting builds
			if len(m.builds) > 0 && m.cursor < len(m.builds) {
				selectedBuild := m.builds[m.cursor]

				// First priority: cancel any active download
				m.downloadMutex.Lock()
				canCancel := false
				// Search for any download state that matches this version
				// Need to check all keys since buildID might include hash
				for id, state := range m.downloadStates {
					// Extract version from buildID
					downloadVersion := id
					if strings.Contains(id, "-") {
						downloadVersion = strings.Split(id, "-")[0]
					}

					if downloadVersion == selectedBuild.Version &&
						(state.BuildState == types.StateDownloading ||
							state.BuildState == types.StatePreparing ||
							state.BuildState == types.StateExtracting) {
						canCancel = true
						// Set the activeDownloadID to ensure we're canceling the right one
						m.activeDownloadID = id
						break
					}
				}
				m.downloadMutex.Unlock()

				if canCancel {
					return m.handleCancelDownload()
				} else if selectedBuild.Status == types.StateLocal || selectedBuild.Status == types.StateUpdate {
					// Secondary action: delete builds
					return m.handleDeleteBuild()
				}
			}
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("f"))):
			// Refresh builds list
			m.isLoading = true
			return m, tea.Batch(
				fetchBuildsCmd(m.config),
				scanLocalBuildsCmd(m.config),
				getOldBuildsInfoCmd(m.config),
			)

		case key.Matches(msg, key.NewBinding(key.WithKeys("s"))):
			return m.handleShowSettings()
		}
	}

	return m, nil
}

// updateCleanupConfirmView handles key events in the cleanup confirmation dialog
func (m Model) updateCleanupConfirmView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc", "n", "ctrl+c"))):
			// Cancel cleanup and return to list view
			m.currentView = viewList
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("y", "enter"))):
			// Confirm cleanup and execute it
			m.currentView = viewList
			return m, cleanupOldBuildsCmd(m.config)
		}
	}

	return m, nil
}

// updateQuitConfirmView handles key events in the quit confirmation dialog
func (m Model) updateQuitConfirmView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc", "n", "ctrl+c"))):
			// Cancel quit and return to list view
			m.currentView = viewList
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("y", "enter"))):
			// Confirm quit and cancel all downloads
			var versionsToCleanup []string

			m.downloadMutex.Lock()
			for version, state := range m.downloadStates {
				if state.BuildState == types.StateDownloading ||
					state.BuildState == types.StatePreparing ||
					state.BuildState == types.StateExtracting {
					// Signal cancellation by closing the channel
					select {
					case <-state.CancelCh:
						// Already closed, do nothing
					default:
						// Close the channel to signal cancellation
						close(state.CancelCh)
					}

					// Add this version to cleanup list
					versionsToCleanup = append(versionsToCleanup, version)
				}
			}
			m.downloadMutex.Unlock()

			// Clean up any partial downloads
			for _, version := range versionsToCleanup {
				// Clean up downloaded file in .downloading directory
				downloadDir := filepath.Join(m.config.DownloadDir, ".downloading")
				files, err := os.ReadDir(downloadDir)
				if err != nil {
					log.Printf("Warning: couldn't read .downloading directory: %v", err)
				} else {
					for _, file := range files {
						if strings.Contains(file.Name(), version) {
							filePath := filepath.Join(downloadDir, file.Name())
							if err := os.Remove(filePath); err != nil {
								log.Printf("Warning: failed to remove partial download %s: %v", filePath, err)
							} else {
								log.Printf("Cleaned up partial download: %s", filePath)
							}
						}
					}
				}

				// Also clean up any partially extracted directories
				entries, err := os.ReadDir(m.config.DownloadDir)
				if err != nil {
					log.Printf("Warning: couldn't read download directory: %v", err)
				} else {
					for _, entry := range entries {
						if entry.IsDir() && strings.Contains(entry.Name(), version) {
							dirPath := filepath.Join(m.config.DownloadDir, entry.Name())

							// Check if this is a complete build by looking for version.json
							metaPath := filepath.Join(dirPath, "version.json")
							if _, err := os.Stat(metaPath); os.IsNotExist(err) {
								// No version.json, so this is likely a partial extraction
								if err := os.RemoveAll(dirPath); err != nil {
									log.Printf("Warning: failed to remove partial extraction %s: %v", dirPath, err)
								} else {
									log.Printf("Cleaned up partial extraction: %s", dirPath)
								}
							}
						}
					}
				}
			}

			// Quit the application
			return m, tea.Quit
		}
	}

	return m, nil
}
