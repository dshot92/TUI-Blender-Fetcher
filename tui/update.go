package tui

import (
	"TUI-Blender-Launcher/model"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
)

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	var cmds []tea.Cmd

	// Create a Commands instance
	cmdManager := NewCommands(m.config)

	// Start with local build scan to get builds already on disk
	cmds = append(cmds, cmdManager.ScanLocalBuilds())

	// Add a program message listener to receive messages from background goroutines
	cmds = append(cmds, cmdManager.ProgramMsgListener())

	return tea.Batch(cmds...)
}

// Update updates the model based on messages
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle key messages first, routing based on the current view
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch m.currentView {
		case viewSettings, viewInitialSetup:
			return m.updateSettingsView(keyMsg)
		default:
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

	case startDownloadMsg:
		m.activeDownloadID = msg.buildID
		var cmds []tea.Cmd

		// Create a Commands instance and call DoDownload directly
		cmdManager := NewCommands(m.config)
		cmds = append(cmds, cmdManager.DoDownload(msg.build))

		return m, tea.Batch(cmds...)

	case downloadCompleteMsg:
		// Handle completion of download
		for i := range m.builds {
			// Find the build by version and update its status
			if m.builds[i].Version == msg.buildVersion {
				if msg.err != nil {
					// Handle download error
					m.builds[i].Status = model.StateFailed
					m.err = msg.err
				} else {
					// Update to local state on success
					m.builds[i].Status = model.StateLocal

					// Clear any error message
					m.err = nil
				}
				break
			}
		}

		// Re-sort the builds since status has changed
		m.builds = model.SortBuilds(m.builds, m.sortColumn, m.sortReversed)

		// Start listening for more program messages
		cmdManager := NewCommands(m.config)
		return m, cmdManager.ProgramMsgListener()

	case tickMsg:
		// Process tick messages for both views
		if m.currentView == viewSettings || m.currentView == viewInitialSetup {
			return m.updateSettingsView(msg)
		}
		return m.updateListView(msg)
	}

	return m, nil
}

// updateSettingsView handles key events in the settings view
func (m *Model) updateSettingsView(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle different message types
	switch msg := msg.(type) {
	case tickMsg:
		// Process tick messages for downloads but continue with other processing
		newModel, cmd := m.handleDownloadProgress(msg)
		// Return the updated model and command, but don't short-circuit other message handling
		return newModel, cmd

	case tea.KeyMsg:
		// Use centralized command handling
		for _, cmd := range GetCommandsForView(m.currentView) {
			if key.Matches(msg, GetKeyBinding(cmd.Type)) {
				switch cmd.Type {
				case CmdQuit:
					// Quit application
					return m, tea.Quit

				case CmdSaveSettings:
					// Save settings and return to main view
					m.currentView = viewList
					return saveSettings(m)

				case CmdToggleEditMode:
					// Toggle edit mode for the focused setting
					if m.editMode {
						// Exit edit mode and save settings
						m.editMode = false
						updateFocusStyles(m, m.focusIndex)
					} else {
						// Enter edit mode for the focused field
						m.editMode = true
						m.settingsInputs[m.focusIndex].Focus()
						updateFocusStyles(m, -1)
					}
					return m, nil

				case CmdMoveUp:
					if !m.editMode {
						oldFocus := m.focusIndex
						m.focusIndex = (m.focusIndex - 1 + len(m.settingsInputs)) % len(m.settingsInputs)
						updateFocusStyles(m, oldFocus)
					}
					return m, nil

				case CmdMoveDown:
					if !m.editMode {
						oldFocus := m.focusIndex
						m.focusIndex = (m.focusIndex + 1) % len(m.settingsInputs)
						updateFocusStyles(m, oldFocus)
					}
					return m, nil
				}
			}
		}

		// Pass other keys to the input field if in edit mode
		if m.editMode {
			// Create a copy of the model to avoid pointer issues
			updatedModel := m
			cmd := updatedModel.updateInputs(msg)
			return updatedModel, cmd
		}
	}

	// Default: keep the current model and continue the UI refresh
	return m, nil
}

// updateListView handles key events in the main list view
func (m *Model) updateListView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		// Process tick messages for downloads
		return m.handleDownloadProgress(msg)

	case tea.KeyMsg:
		// Calculate visible rows count for all navigation commands
		visibleRowsCount := m.terminalHeight - 7 // Approximate height for header, footer, separators
		if visibleRowsCount < 1 {
			visibleRowsCount = 1
		}

		// Use centralized command handling
		for _, cmd := range GetCommandsForView(viewList) {
			if key.Matches(msg, GetKeyBinding(cmd.Type)) {
				switch cmd.Type {
				case CmdQuit:
					// Quit application
					return m, tea.Quit

				case CmdShowSettings:
					// Switch to settings view
					return m.handleShowSettings()

				case CmdToggleSortOrder:
					// Toggle sort direction
					m.sortReversed = !m.sortReversed
					m.builds = model.SortBuilds(m.builds, m.sortColumn, m.sortReversed)
					m.ensureCursorVisible(visibleRowsCount)
					return m, nil

				case CmdMoveUp:
					m.updateCursor("up", visibleRowsCount)
					return m, nil

				case CmdMoveDown:
					m.updateCursor("down", visibleRowsCount)
					return m, nil

				case CmdMoveLeft:
					// Move sort column left
					m.updateSortColumn("left")
					m.builds = model.SortBuilds(m.builds, m.sortColumn, m.sortReversed)
					m.ensureCursorVisible(visibleRowsCount)
					return m, nil

				case CmdMoveRight:
					// Move sort column right
					m.updateSortColumn("right")
					m.builds = model.SortBuilds(m.builds, m.sortColumn, m.sortReversed)
					m.ensureCursorVisible(visibleRowsCount)
					return m, nil

				case CmdPageUp:
					m.updateCursor("pageup", visibleRowsCount)
					return m, nil

				case CmdPageDown:
					m.updateCursor("pagedown", visibleRowsCount)
					return m, nil

				case CmdHome:
					m.updateCursor("home", visibleRowsCount)
					return m, nil

				case CmdEnd:
					m.updateCursor("end", visibleRowsCount)
					return m, nil

				case CmdFetchBuilds:
					// Fetch online builds only
					if !m.isLoading {
						m.isLoading = true
						// Start with a clean slate for messages
						m.err = nil

						// Update the builds using the existing command manager
						return m, m.commands.FetchBuilds()
					}
					return m, nil

				case CmdDownloadBuild:
					// Start download for selected build
					return m.handleStartDownload()

				case CmdLaunchBuild:
					// Launch the selected build
					return m.handleLaunchBlender()

				case CmdOpenBuildDir:
					// Open the directory for the selected build
					return m.handleOpenBuildDir()

				case CmdDeleteBuild:
					build := m.builds[m.cursor]
					if build.Status == model.StateLocal {
						// Delete the build
						return m.handleDeleteBuild()
					} else if build.Status == model.StateDownloading || build.Status == model.StateExtracting {
						// Cancel the download
						return m.handleCancelDownload()
					}
					// For other states, do nothing
					return m, nil
				}
			}
		}
	}

	// If no specific action, return the model unchanged
	return m, nil
}

// Add this function to update cursor position with scrolling
func (m *Model) updateCursor(direction string, visibleRowsCount int) {
	if len(m.builds) == 0 {
		return
	}

	switch direction {
	case "up":
		m.cursor--
		if m.cursor < 0 {
			m.cursor = len(m.builds) - 1
		}
	case "down":
		m.cursor++
		if m.cursor >= len(m.builds) {
			m.cursor = 0
		}
	case "home":
		m.cursor = 0
	case "end":
		m.cursor = len(m.builds) - 1
	case "pageup":
		m.cursor -= visibleRowsCount
		if m.cursor < 0 {
			m.cursor = 0
		}
	case "pagedown":
		m.cursor += visibleRowsCount
		if m.cursor >= len(m.builds) {
			m.cursor = len(m.builds) - 1
		}
	}

	// Adjust startIndex to ensure cursor is visible
	if m.cursor < m.startIndex {
		// Cursor moved above visible area, scroll up
		m.startIndex = m.cursor
	} else if m.cursor >= m.startIndex+visibleRowsCount {
		// Cursor moved below visible area, scroll down
		m.startIndex = m.cursor - visibleRowsCount + 1
	}
}

// ensureCursorVisible ensures the cursor is visible within the scrolling window
func (m *Model) ensureCursorVisible(visibleRowsCount int) {
	if len(m.builds) == 0 {
		m.startIndex = 0
		return
	}

	// Ensure cursor is within bounds
	if m.cursor >= len(m.builds) {
		m.cursor = len(m.builds) - 1
	} else if m.cursor < 0 {
		m.cursor = 0
	}

	// Adjust startIndex to ensure cursor is visible
	if m.cursor < m.startIndex {
		// Cursor is above visible area
		m.startIndex = m.cursor
	} else if m.cursor >= m.startIndex+visibleRowsCount {
		// Cursor is below visible area
		m.startIndex = m.cursor - visibleRowsCount + 1
		if m.startIndex < 0 {
			m.startIndex = 0
		}
	}
}
