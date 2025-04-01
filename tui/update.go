package tui

import (
	"TUI-Blender-Launcher/local"
	"TUI-Blender-Launcher/model"
	"TUI-Blender-Launcher/types"
	"fmt"
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

	return tea.Batch(cmds...)
}

// Update updates the model based on messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// First handle the current view state
	switch m.currentView {
	case viewSettings:
		return m.updateSettingsView(msg)
	case viewDeleteConfirm:
		return m.updateDeleteConfirmView(msg)
	case viewCleanupConfirm:
		return m.updateCleanupConfirmView(msg)
	case viewQuitConfirm:
		return m.updateQuitConfirmView(msg)
	}

	// Then handle the type of message
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Update window dimensions
		m.terminalWidth = msg.Width
		m.terminalHeight = msg.Height

		// Calculate visible rows for table based on terminal height
		// Reserve space for header (3 lines) and footer (3 lines)
		reservedLines := 6
		m.visibleRows = m.terminalHeight - reservedLines
		if m.visibleRows < 5 {
			m.visibleRows = 5 // Ensure at least 5 rows for content
		}

		// Recalculate which columns should be visible
		m.visibleColumns = calculateVisibleColumns(m.terminalWidth)

		// Ensure Version and Status columns are always visible regardless of terminal width
		if m.visibleColumns != nil {
			m.visibleColumns["Version"] = true
			m.visibleColumns["Status"] = true
		}

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

	case tea.KeyMsg:
		// Handle key messages with the list view handler
		return m.updateListView(msg)

	case progress.FrameMsg:
		// Update the progress bar if we're displaying it
		progressModel, cmd := m.progressBar.Update(msg)
		m.progressBar = progressModel.(progress.Model)
		return m, cmd

	// Success messages
	case buildsFetchedMsg:
		return m.handleBuildsFetched(msg)

	case localBuildsScannedMsg:
		return m.handleLocalBuildsScanned(msg)

	case buildsUpdatedMsg:
		return m.handleBuildsUpdated(msg)

	case model.BlenderExecMsg:
		return m.handleBlenderExec(msg)

	case deleteBuildCompleteMsg:
		// Handle build deletion completion
		m.currentView = viewList
		return m, nil

	case startDownloadMsg:
		// Store the active download ID for UI rendering
		m.activeDownloadID = msg.buildID
		// Start download for the build
		return m, doDownloadCmd(msg.build, m.config, m.downloadStates, &m.downloadMutex)

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

	case errMsg:
		// A command returned an error
		m.err = msg
		return m, nil

	case tickMsg:
		// Tick events for updating downloads
		return m.handleDownloadProgress(msg)

	default:
		return m, nil
	}
}

// updateSettingsView handles key events in the settings view
func (m Model) updateSettingsView(msg tea.Msg) (tea.Model, tea.Cmd) {
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

		case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
			if m.editMode {
				// Exit edit mode and go back to navigation
				m.editMode = false
				updateFocusStyles(&m, m.focusIndex)
				return m, nil
			} else {
				// Esc does nothing when not in edit mode
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
				return &m, m.updateInputs(msg)
			}
		}
	}

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

				// Check if it's a download in progress first
				m.downloadMutex.Lock()
				downloadState, isDownloading := m.downloadStates[selectedBuild.Version]
				canCancel := isDownloading &&
					(downloadState.BuildState == types.StateDownloading ||
						downloadState.BuildState == types.StatePreparing ||
						downloadState.BuildState == types.StateExtracting)
				m.downloadMutex.Unlock()

				if canCancel {
					return m.handleCancelDownload()
				} else if selectedBuild.Status == types.StateLocal || selectedBuild.Status == types.StateUpdate {
					// Use x for deleting builds too
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

		case key.Matches(msg, key.NewBinding(key.WithKeys("c"))):
			return m.handleShowSettings()
		}
	}

	return m, nil
}

// updateDeleteConfirmView handles key events in the delete confirmation dialog
func (m Model) updateDeleteConfirmView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("esc", "n", "ctrl+c"))):
			// Cancel deletion and return to list view
			m.currentView = viewList
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("y", "enter"))):
			// Confirm deletion and execute it
			if m.deleteCandidate != "" {
				return m, func() tea.Msg {
					success, err := local.DeleteBuild(m.config.DownloadDir, m.deleteCandidate)
					if err != nil {
						return errMsg{err}
					}

					if !success {
						return errMsg{fmt.Errorf("failed to delete build %s", m.deleteCandidate)}
					}

					// Update build statuses after deletion
					for i := range m.builds {
						if m.builds[i].Version == m.deleteCandidate {
							m.builds[i].Status = types.StateOnline
							break
						}
					}
					m.builds = sortBuilds(m.builds, m.sortColumn, m.sortReversed)
					// Return a proper message instead of setting view directly
					return deleteBuildCompleteMsg{}
				}
			}
			m.currentView = viewList
			return m, nil
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
