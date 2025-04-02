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
	cmds = append(cmds, m.scanLocalBuildsCmd())

	// Start the continuous tick system for UI updates
	cmds = append(cmds, m.tickCmd())

	// Start a dedicated UI refresh cycle
	cmds = append(cmds, m.uiRefreshCmd())

	return tea.Batch(cmds...)
}

// Update updates the model based on messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle key messages first, routing based on the current view
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch m.currentView {
		case viewSettings, viewInitialSetup:
			return m.updateSettingsView(keyMsg)
		case viewQuitConfirm:
			return m.updateQuitConfirmView(keyMsg)
		default:
			// For viewList and any other views, use the list view handler
			return m.updateListView(keyMsg)
		}
	}

	// Handle non-key messages
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.UpdateWindowSize(msg.Width, msg.Height)
		if len(m.builds) > 0 && m.cursor >= len(m.builds) {
			m.cursor = len(m.builds) - 1
		}
		return m, nil

	case progress.FrameMsg:
		progressModel, cmd := m.progressBar.Update(msg)
		m.progressBar = progressModel.(progress.Model)
		return m, cmd

	case forceRenderMsg:
		return m, m.uiRefreshCmd()

	case errMsg:
		m.err = msg.err
		return m, nil

	case localBuildsScannedMsg:
		return m.handleLocalBuildsScanned(msg)

	case buildsFetchedMsg:
		return m.handleBuildsFetched(msg)

	case buildsUpdatedMsg:
		return m.handleBuildsUpdated(msg)

	case model.BlenderExecMsg:
		return m.handleBlenderExec(msg)

	case deleteBuildCompleteMsg:
		m.currentView = viewList
		return m, m.fetchBuildsCmd()

	case startDownloadMsg:
		m.activeDownloadID = msg.buildID
		var cmds []tea.Cmd
		cmds = append(cmds, m.doDownloadCmd(msg.build))
		cmds = append(cmds, m.uiRefreshCmd())
		return m, tea.Batch(cmds...)

	case downloadCompleteMsg:
		return m, nil

	case resetStatusMsg:
		for i := range m.builds {
			if m.builds[i].Version == msg.version {
				localPath := filepath.Join(m.config.DownloadDir, m.builds[i].Version)
				if _, err := os.Stat(localPath); err == nil {
					m.builds[i].Status = types.StateUpdate
				} else {
					m.builds[i].Status = types.StateOnline
				}
				m.builds = sortBuilds(m.builds, m.sortColumn, m.sortReversed)
				break
			}
		}
		return m, nil
	}

	// Default catch-all return
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
			} else if len(m.builds) > 0 {
				m.cursor = len(m.builds) - 1
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
			if len(m.builds) > 0 && m.cursor < len(m.builds)-1 {
				m.cursor++
			} else {
				m.cursor = 0
			}

		case key.Matches(msg, key.NewBinding(key.WithKeys("right", "l"))):
			// Cycle sort column forward
			lastCol := len(columnConfigs) - 1
			m.sortColumn = (m.sortColumn + 1) % (lastCol + 1)
			m.builds = sortBuilds(m.builds, m.sortColumn, m.sortReversed)

		case key.Matches(msg, key.NewBinding(key.WithKeys("left", "h"))):
			// Cycle sort column backward
			lastCol := len(columnConfigs) - 1
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
				m.fetchBuildsCmd(),
				m.scanLocalBuildsCmd(),
			)

		case key.Matches(msg, key.NewBinding(key.WithKeys("s"))):
			return m.handleShowSettings()
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
