package tui

import (
	"TUI-Blender-Launcher/model"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
)

// deleteBuildCompleteMsg is sent when a build is successfully deleted
type deleteBuildCompleteMsg struct{}

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	var cmds []tea.Cmd

	// Start with local build scan to get builds already on disk
	cmds = append(cmds, m.scanLocalBuildsCmd())

	// Add a program message listener to receive messages from background goroutines
	cmdManager := NewCommands(m.config)
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
		// Use centralized command handling
		for _, cmd := range GetCommandsForView(viewList) {
			if key.Matches(msg, GetKeyBinding(cmd.Type)) {
				switch cmd.Type {
				case CmdQuit:
					// Quit application
					return m, tea.Quit

				case CmdShowSettings:
					// Show settings
					return m.handleShowSettings()

				case CmdToggleSortOrder:
					// Toggle sort order (reverse)
					m.sortReversed = !m.sortReversed
					m.builds = model.SortBuilds(m.builds, m.sortColumn, m.sortReversed)
					return m, nil

				case CmdFetchBuilds:
					// Fetch online builds only
					m.isLoading = true
					return m, m.fetchBuildsCmd()

				case CmdDownloadBuild:
					// Download build (only for online builds)
					if len(m.builds) > 0 && m.cursor < len(m.builds) {
						selectedBuild := m.builds[m.cursor]
						if selectedBuild.Status == model.StateOnline || selectedBuild.Status == model.StateUpdate {
							return m.handleStartDownload()
						}
					}
					return m, nil

				case CmdLaunchBuild:
					// Launch build
					if len(m.builds) > 0 && m.cursor < len(m.builds) {
						selectedBuild := m.builds[m.cursor]
						if selectedBuild.Status == model.StateLocal {
							return m.handleLaunchBlender()
						}
					}
					return m, nil

				case CmdOpenBuildDir:
					// Open build directory
					if len(m.builds) > 0 && m.cursor < len(m.builds) {
						selectedBuild := m.builds[m.cursor]
						if selectedBuild.Status == model.StateLocal || selectedBuild.Status == model.StateUpdate {
							return m.handleOpenBuildDir()
						}
					}
					return m, nil

				case CmdDeleteBuild:
					// Delete build (local) or cancel download (downloading/extracting)
					if len(m.builds) > 0 && m.cursor < len(m.builds) {
						selectedBuild := m.builds[m.cursor]

						// Check if the build is currently downloading or extracting
						canCancel := false
						buildID := selectedBuild.Version
						if selectedBuild.Hash != "" {
							buildID = selectedBuild.Version + "-" + selectedBuild.Hash[:8]
						}

						// Check if this build has an active download
						state := m.commands.downloads.GetState(buildID)
						if state != nil && (state.BuildState == model.StateDownloading ||
							state.BuildState == model.StateExtracting) {
							canCancel = true
							m.activeDownloadID = buildID
						}

						if canCancel {
							return m.handleCancelDownload()
						} else if selectedBuild.Status == model.StateLocal {
							// Delete local build
							return m.handleDeleteBuild()
						}
					}
					return m, nil

				case CmdMoveUp:
					if m.cursor > 0 {
						m.cursor--
					} else if len(m.builds) > 0 {
						m.cursor = len(m.builds) - 1
					}
					return m, nil

				case CmdMoveDown:
					if len(m.builds) > 0 && m.cursor < len(m.builds)-1 {
						m.cursor++
					} else {
						m.cursor = 0
					}
					return m, nil

				case CmdMoveLeft:
					// Change sort column to the left
					m.updateSortColumn("left")
					m.builds = model.SortBuilds(m.builds, m.sortColumn, m.sortReversed)
					return m, nil

				case CmdMoveRight:
					// Change sort column to the right
					m.updateSortColumn("right")
					m.builds = model.SortBuilds(m.builds, m.sortColumn, m.sortReversed)
					return m, nil
				}
			}
		}
	}

	return m, nil
}
