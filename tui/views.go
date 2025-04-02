package tui

import (
	"TUI-Blender-Launcher/types"
	"bytes"
	"fmt"
	"strings"

	lp "github.com/charmbracelet/lipgloss"
)

// Utility function for converting bytes to human-readable sizes
func formatByteSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// Utility function for displaying download progress
func buildStateToString(state types.BuildState) string {
	switch state {
	case types.StateDownloading:
		return "Downloading"
	case types.StateExtracting:
		return "Extracting"
	case types.StatePreparing:
		return "Preparing"
	case types.StateLocal:
		return "Local"
	case types.StateOnline:
		return "Online"
	case types.StateUpdate:
		return "Update"
	case types.StateFailed:
		return "Failed"
	case types.StateNone:
		return "Cancelled"
	default:
		return state.String()
	}
}

// View renders the current view of the model
func (m Model) View() string {
	// First determine if we need to show an overlay dialog
	var isDialog bool
	var dialogContent string

	switch m.currentView {
	case viewDeleteConfirm:
		isDialog = true
		dialogContent = m.renderDeleteConfirmDialog()
	case viewCleanupConfirm:
		isDialog = true
		dialogContent = m.renderCleanupConfirmDialog()
	case viewQuitConfirm:
		isDialog = true
		dialogContent = m.renderQuitConfirmDialog()
	default:
		isDialog = false
	}

	// Calculate the visible rows for table content based on terminal height
	// Reserve space for header (3 lines) and footer (3 lines)
	reservedLines := 6

	// Update VisibleRows property based on actual terminal height
	m.visibleRows = m.terminalHeight - reservedLines
	if m.visibleRows < 5 {
		m.visibleRows = 5 // Ensure at least 5 rows for content
	}

	// Render the appropriate base view
	var baseView string
	if m.currentView == viewInitialSetup || m.currentView == viewSettings {
		baseView = m.renderSettingsView()
	} else {
		// For dialog views, we still want to see the list underneath
		baseView = m.renderListView()
	}

	// If we're showing a dialog, place it on top of the base view
	if isDialog {
		return lp.JoinVertical(
			lp.Top,
			baseView,
			lp.Place(
				m.terminalWidth,
				0, // Use 0 height to avoid extra spacing
				lp.Center,
				lp.Center,
				dialogContent,
			),
		)
	}

	// Otherwise just return the base view
	return baseView
}

// renderSettingsView renders the settings screen
func (m Model) renderSettingsView() string {
	var b strings.Builder

	// Header - Match the same style as the build list view
	headerText := "TUI Blender Launcher - Settings"
	b.WriteString(headerStyle.Width(m.terminalWidth).AlignHorizontal(lp.Center).Render(headerText))
	b.WriteString("\n\n")

	// Calculate the width for consistency
	contentWidth := m.terminalWidth

	// Settings content
	var settingsContent strings.Builder

	// Settings Fields with proper styling
	labels := []string{"Download Directory:", "Version Filter:", "Manual Fetch:"}

	for i, input := range m.settingsInputs {
		// Create a row for the setting
		var rowContent strings.Builder

		// Show selection indicator for the focused item
		if i == m.focusIndex {
			// Apply selected style to the entire row with the arrow
			rowContent.WriteString("→ " + lp.NewStyle().Bold(true).Render(labels[i]))
			settingsContent.WriteString(selectedRowStyle.Render(rowContent.String()) + "\n")

			// Add the input field with proper indentation - highlight the input field too
			if m.editMode {
				settingsContent.WriteString(selectedRowStyle.Render("  "+input.View()) + "\n")
			} else {
				settingsContent.WriteString("  " + input.View() + "\n")
			}
		} else {
			// Regular styling for non-selected rows
			rowContent.WriteString("  " + lp.NewStyle().Bold(true).Render(labels[i]))
			settingsContent.WriteString(rowContent.String() + "\n")

			// Add the input field with proper indentation - normal style
			settingsContent.WriteString("  " + input.View() + "\n")
		}

		// Add spacing between settings
		settingsContent.WriteString("\n")
	}

	// Add a description for Manual Fetch setting
	manualFetchDesc := "When Manual Fetch is set to true, builds will only be fetched when you press 'f'"
	settingsContent.WriteString("  " + lp.NewStyle().Italic(true).Foreground(lp.Color("12")).Render(manualFetchDesc) + "\n\n")

	// Add cleanup old builds option if there are old builds to clean
	if m.oldBuildsCount > 0 {
		cleanupText := fmt.Sprintf(
			"Press 'c' to clean up %d old %s (%s)",
			m.oldBuildsCount,
			pluralize("build", m.oldBuildsCount),
			formatByteSize(m.oldBuildsSize),
		)
		settingsContent.WriteString("\n" + lp.NewStyle().Foreground(lp.Color("11")).Render(cleanupText))
	}

	// Add error message if any
	if m.err != nil {
		errStyle := lp.NewStyle().Foreground(lp.Color("9")).Bold(true).MarginTop(1)
		settingsContent.WriteString("\n\n" + errStyle.Render("Error: "+m.err.Error()))
	}

	// Render the settings content with appropriate margin
	settingsStyle := lp.NewStyle().
		MarginLeft(2).
		MarginRight(2)

	b.WriteString(settingsStyle.Render(settingsContent.String()))

	// Add separator before footer - match width from the list view
	b.WriteString("\n" + separator[:contentWidth] + "\n")

	// Add command legends like in build list view
	// Context-specific commands
	var contextText strings.Builder
	contextText.WriteString("Enter: ")
	if m.editMode {
		contextText.WriteString("Save edits  ")
	} else {
		contextText.WriteString("Edit field  ")
	}

	if m.editMode {
		contextText.WriteString("Tab/Shift+Tab: Switch fields  ")
		contextText.WriteString("Esc: Cancel editing")
	} else {
		contextText.WriteString("↑/↓: Navigate  ")
		contextText.WriteString("s: Back to builds")
	}

	b.WriteString("\n")
	b.WriteString(footerStyle.Render(contextText.String()))
	b.WriteString("\n")

	// General commands at the bottom
	var generalText strings.Builder
	if m.oldBuildsCount > 0 {
		generalText.WriteString("c: Clean old builds  ")
	}
	generalText.WriteString("q: Quit")

	b.WriteString(footerStyle.Render(generalText.String()))

	return b.String()
}

// renderListView renders the builds list view
func (m Model) renderListView() string {
	var b strings.Builder

	// ===== HEADER SECTION - ALWAYS VISIBLE =====

	// Column headers - create the header row with column names
	headerRow := bytes.Buffer{}

	// Define column order, widths, visibility and labels
	columns := []struct {
		name    string
		width   int
		visible bool
		index   int
	}{
		{name: "Blender", width: columnConfigs["Version"].width, visible: true, index: 0}, // Always show Version
		{name: "Status", width: columnConfigs["Status"].width, visible: true, index: 1},   // Always show Status
		{name: "Branch", width: columnConfigs["Branch"].width, visible: m.visibleColumns["Branch"], index: 2},
		{name: "Type", width: columnConfigs["Type"].width, visible: m.visibleColumns["Type"], index: 3},
		{name: "Hash", width: columnConfigs["Hash"].width, visible: m.visibleColumns["Hash"], index: 4},
		{name: "Size", width: columnConfigs["Size"].width, visible: m.visibleColumns["Size"], index: 5},
		{name: "Build Date", width: columnConfigs["Build Date"].width, visible: m.visibleColumns["Build Date"], index: 6},
	}

	// Ensure Version and Status are always visible regardless of terminal width
	columns[0].visible = true
	columns[1].visible = true

	// Add header columns with sort indicators - use bold style to make them more visible
	for i, col := range columns {
		if col.visible {
			// Add sort indicator to column name if this is the sort column
			colTitle := getSortIndicator(m, col.index, col.name)

			// Apply style and padding based on column type
			switch i {
			case 0: // Version - left align
				headerRow.WriteString(cellStyleLeft.Copy().Bold(true).Width(col.width).Render(colTitle))
			case 5: // Size - right align
				headerRow.WriteString(cellStyleRight.Copy().Bold(true).Width(col.width).Render(colTitle))
			case 6: // Date - center
				headerRow.WriteString(cellStyleCenter.Copy().Bold(true).Width(col.width).Render(colTitle))
			default: // Others - center
				headerRow.WriteString(cellStyleCenter.Copy().Bold(true).Width(col.width).Render(colTitle))
			}

			// Add space between columns
			if i < len(columns)-1 {
				headerRow.WriteString(" ")
			}
		}
	}

	// Ensure the header row is rendered - make it bold and with a higher contrast background
	headerBgStyle := lp.NewStyle().Background(lp.Color("236")).Bold(true).Width(m.terminalWidth)
	b.WriteString(headerBgStyle.Render(headerRow.String()))
	b.WriteString("\n")
	b.WriteString(separator[:headerRow.Len()])
	b.WriteString("\n")
	// ===== END HEADER SECTION =====

	// ===== CONTENT SECTION - BUILD LIST =====
	// Create a content buffer to hold build rows
	var contentBuilder strings.Builder

	// Calculate available height for the table content
	// Reserve space for:
	// - Header: 3 lines (title + columns + separator)
	// - Footer: 3 lines (separator + command row 1 + command row 2)
	reservedLines := 6
	availableRows := m.terminalHeight - reservedLines

	if availableRows < 5 {
		availableRows = 5 // Ensure at least 5 rows for content
	}

	// Build rows
	if m.isLoading {
		loadingStyle := lp.NewStyle().Foreground(lp.Color("11")).Bold(true).Italic(true)
		contentBuilder.WriteString(loadingStyle.Render("Loading builds list... Please wait."))
		contentBuilder.WriteString("\n")
	} else if len(m.builds) == 0 {
		emptyStyle := lp.NewStyle().Foreground(lp.Color("8")).Italic(true)
		contentBuilder.WriteString(emptyStyle.Render("No builds found. Press 'f' to refresh."))
		contentBuilder.WriteString("\n")
	} else {
		// Calculate which builds to display based on scroll position
		totalBuilds := len(m.builds)

		// If we have fewer builds than visible rows, just show all of them
		if totalBuilds <= availableRows {
			m.scrollOffset = 0 // Reset scroll offset if not needed
		}

		// Calculate start and end indices
		startIdx := m.scrollOffset
		endIdx := startIdx + availableRows
		if endIdx > totalBuilds {
			endIdx = totalBuilds
		}

		// Add scroll indicator at top if needed
		if m.scrollOffset > 0 {
			scrollUpIndicator := lp.NewStyle().Foreground(lp.Color("7")).
				Render(fmt.Sprintf("↑ %d more %s",
					m.scrollOffset,
					pluralize("build", m.scrollOffset)))
			contentBuilder.WriteString(scrollUpIndicator)
			contentBuilder.WriteString("\n")

			// Reduce available rows by one for the indicator
			if endIdx-startIdx > availableRows-1 {
				endIdx--
			}
		}

		// Only render the visible subset of builds
		for i := startIdx; i < endIdx; i++ {
			build := m.builds[i]

			// Highlight if this is the selected row
			var rowStyle lp.Style
			if i == m.cursor {
				rowStyle = selectedRowStyle
			} else {
				rowStyle = regularRowStyle
			}

			// Build the row content cell by cell
			rowContent := bytes.Buffer{}

			// Version column
			if columns[0].visible {
				versionText := build.Version
				rowContent.WriteString(cellStyleLeft.Width(columns[0].width).Render(versionText))
				rowContent.WriteString(" ")
			}

			// Status column with potential download progress
			if columns[1].visible {
				var statusText string

				// Get download state if applicable
				m.downloadMutex.Lock()

				// Generate the build ID in the same way as we do for download and other operations
				buildID := build.Version
				if build.Hash != "" {
					buildID = build.Version + "-" + build.Hash[:8]
				}

				// Only check download state for builds matching the activeDownloadID
				var isDownloading bool
				var downloadState *DownloadState
				var isActiveDownload bool

				// Check if this is the active download
				isActiveDownload = (m.activeDownloadID != "" && buildID == m.activeDownloadID)
				if isActiveDownload {
					downloadState, isDownloading = m.downloadStates[m.activeDownloadID]
				}

				m.downloadMutex.Unlock()

				// Format download progress if actively downloading
				if isActiveDownload && isDownloading && downloadState != nil &&
					(downloadState.BuildState == types.StateDownloading ||
						downloadState.BuildState == types.StateExtracting ||
						downloadState.BuildState == types.StatePreparing) {
					// Just show the state name, details will be in the progress bar
					statusText = buildStateToString(downloadState.BuildState)
				} else {
					// Regular status display (not downloading)
					statusText = buildStateToString(build.Status)
				}

				// Apply color based on status
				var statusStyle lp.Style
				switch build.Status {
				case types.StateLocal:
					statusStyle = lp.NewStyle().Foreground(lp.Color(colorSuccess))
				case types.StateOnline:
					statusStyle = lp.NewStyle().Foreground(lp.Color(colorNeutral))
				case types.StateUpdate:
					statusStyle = lp.NewStyle().Foreground(lp.Color(colorInfo))
				case types.StateFailed:
					statusStyle = lp.NewStyle().Foreground(lp.Color(colorError))
				case types.StateDownloading, types.StatePreparing, types.StateExtracting, types.StateNone:
					statusStyle = lp.NewStyle().Foreground(lp.Color(colorWarning))
				default:
					// For active statuses (downloading, extracting)
					if isDownloading {
						statusStyle = lp.NewStyle().Foreground(lp.Color(colorWarning))
					} else {
						statusStyle = lp.NewStyle() // Default
					}
				}

				rowContent.WriteString(cellStyleCenter.Width(columns[1].width).
					Render(statusStyle.Render(statusText)))
				rowContent.WriteString(" ")
			}

			// Check if we're downloading this specific build - if so, show progress bar spanning multiple columns
			// Generate or reuse the buildID
			buildID := build.Version
			if build.Hash != "" {
				buildID = build.Version + "-" + build.Hash[:8]
			}

			// Check if this is the active download
			isActiveDownload := (m.activeDownloadID != "" && buildID == m.activeDownloadID)

			m.downloadMutex.Lock()
			var downloadState *DownloadState
			var isDownloading bool

			if isActiveDownload {
				downloadState, isDownloading = m.downloadStates[m.activeDownloadID]
			}
			m.downloadMutex.Unlock()

			// If this is the active download being downloaded, show progress
			if isActiveDownload && isDownloading && downloadState != nil &&
				(downloadState.BuildState == types.StateDownloading ||
					downloadState.BuildState == types.StateExtracting ||
					downloadState.BuildState == types.StatePreparing) {

				// Branch column - show percentage
				if columns[2].visible {
					// Format percentage for Branch column
					percentage := int(downloadState.Progress * 100)
					// Simpler display - just percentage
					speedInfo := fmt.Sprintf("%d%%", percentage)

					// Apply style to percentage text
					branchStyle := lp.NewStyle().
						Foreground(lp.Color(colorWarning)).
						Bold(true)

					rowContent.WriteString(cellStyleCenter.Width(columns[2].width).
						Render(branchStyle.Render(speedInfo)))
					rowContent.WriteString(" ")
				}

				// Type column - show download speed
				if columns[3].visible {
					// Format and display download speed in Type column
					var speedText string
					if downloadState.BuildState == types.StateDownloading && downloadState.Speed > 0 {
						speedText = formatByteSize(int64(downloadState.Speed)) + "/s"
					} else {
						speedText = buildStateToString(downloadState.BuildState)
					}

					// Apply style to speed text
					speedStyle := lp.NewStyle().
						Foreground(lp.Color(colorWarning)).
						Bold(true)

					rowContent.WriteString(cellStyleCenter.Width(columns[3].width).
						Render(speedStyle.Render(speedText)))
					rowContent.WriteString(" ")
				}

				// Calculate total width for Hash, Size, and Build Date columns combined
				totalMiddleWidth := 0

				// Count the width of Hash, Size, and Build Date columns plus spacing
				if columns[4].visible { // Hash
					totalMiddleWidth += columns[4].width + 1
				}
				if columns[5].visible { // Size
					totalMiddleWidth += columns[5].width + 1 // Add spacing after Size
				}
				if columns[6].visible { // Build Date
					totalMiddleWidth += columns[6].width
				}

				// Adjust spacing
				if totalMiddleWidth > 0 {
					// Set the progress bar to take up the available width
					// Save current progress bar width
					savedWidth := m.progressBar.Width

					// Temporarily adjust width to fill the available space
					m.progressBar.Width = totalMiddleWidth - 3 // Small buffer for display

					// Get progress bar as text
					progressText := m.progressBar.ViewAs(downloadState.Progress)

					// Restore original width
					m.progressBar.Width = savedWidth

					// Create a style for the progress display
					progressStyle := lp.NewStyle().
						Foreground(lp.Color(colorWarning)).
						Bold(true).
						Width(totalMiddleWidth).
						Align(lp.Left) // Left align for better visibility of progress

					// Render the progress bar across Hash, Size, and Build Date columns
					rowContent.WriteString(progressStyle.Render(progressText))

					// After drawing the progress bar, add the row to the output
					contentBuilder.WriteString(rowStyle.Render(rowContent.String()))
					contentBuilder.WriteString("\n")
					continue // Skip to next row
				}
			}

			// If not downloading or progress bar couldn't be shown, display normal columns

			// Branch column
			if columns[2].visible {
				rowContent.WriteString(cellStyleCenter.Width(columns[2].width).
					Render(build.Branch))
				rowContent.WriteString(" ")
			}

			// Type column (release cycle)
			if columns[3].visible {
				rowContent.WriteString(cellStyleCenter.Width(columns[3].width).
					Render(build.ReleaseCycle))
				rowContent.WriteString(" ")
			}

			// Hash column
			if columns[4].visible {
				hashDisplay := build.Hash
				if len(hashDisplay) > 8 {
					hashDisplay = hashDisplay[:8]
				}
				rowContent.WriteString(cellStyleCenter.Width(columns[4].width).
					Render(hashDisplay))
				rowContent.WriteString(" ")
			}

			// Size column
			if columns[5].visible {
				sizeStr := formatByteSize(build.Size)
				rowContent.WriteString(cellStyleRight.Width(columns[5].width).
					Render(sizeStr))
				rowContent.WriteString(" ")
			}

			// Date column
			if columns[6].visible {
				dateText := build.BuildDate.Time().Format("2006-01-02")
				rowContent.WriteString(cellStyleCenter.Width(columns[6].width).
					Render(dateText))
			}

			// Apply the row style and add to the output
			contentBuilder.WriteString(rowStyle.Render(rowContent.String()))
			contentBuilder.WriteString("\n")
		}

		// Add scroll indicator at bottom if needed
		if endIdx < totalBuilds {
			scrollDownIndicator := lp.NewStyle().Foreground(lp.Color("7")).
				Render(fmt.Sprintf("↓ %d more %s",
					totalBuilds-endIdx,
					pluralize("build", totalBuilds-endIdx)))
			contentBuilder.WriteString(scrollDownIndicator)
			contentBuilder.WriteString("\n")
		}
	}

	// Add the content to the main view
	b.WriteString(contentBuilder.String())

	// Fill any remaining space to push footer to bottom
	// Calculate how many empty lines we need to add
	fillerLines := availableRows
	// Count lines in content
	contentLines := strings.Count(contentBuilder.String(), "\n")
	if contentLines < fillerLines {
		for i := 0; i < fillerLines-contentLines; i++ {
			b.WriteString("\n")
		}
	}

	// ===== FOOTER SECTION - ALWAYS AT BOTTOM =====
	// Add separator before footer - match width of header

	// First footer row - Contextual actions based on selected build
	var footerTopRow strings.Builder

	// Add contextual commands based on the selected build
	if len(m.builds) > 0 && m.cursor < len(m.builds) {
		selectedBuild := m.builds[m.cursor]

		// Different commands based on build status
		switch selectedBuild.Status {
		case types.StateLocal:
			footerTopRow.WriteString("Enter: Launch  ")
			footerTopRow.WriteString("o: Open folder  ")
			footerTopRow.WriteString("x: Delete  ")
		case types.StateOnline:
			footerTopRow.WriteString("d: Download  ")
		case types.StateUpdate:
			footerTopRow.WriteString("d: Download update  ")
			footerTopRow.WriteString("o: Open folder  ")
			footerTopRow.WriteString("x: Delete  ")
		}

		// Check if we're downloading this build (to offer cancel)
		m.downloadMutex.Lock()
		downloadState, isDownloading := m.downloadStates[selectedBuild.Version]
		canCancel := isDownloading &&
			(downloadState.BuildState == types.StateDownloading ||
				downloadState.BuildState == types.StatePreparing ||
				downloadState.BuildState == types.StateExtracting)
		m.downloadMutex.Unlock()

		if canCancel {
			footerTopRow.WriteString("x: Cancel download  ")
		}
	}

	// If no contextual commands were added, show a helpful message
	if footerTopRow.Len() == 0 {
		footerTopRow.WriteString("Select a build to see available actions  |  ")
	}

	b.WriteString(footerStyle.Render(footerTopRow.String()))
	// b.WriteString("\n")

	// Second footer row - General commands
	var footerBottomRow strings.Builder
	footerBottomRow.WriteString("f: Fetch Builds  ")
	footerBottomRow.WriteString("r: Reverse sort  ")
	footerBottomRow.WriteString("s: Settings  ")
	footerBottomRow.WriteString("q: Quit")

	b.WriteString(footerStyle.Render(footerBottomRow.String()))
	// ===== END FOOTER SECTION =====

	return b.String()
}

// Helper function for pluralizing words
func pluralize(word string, count int) string {
	if count == 1 {
		return word
	}
	return word + "s"
}

// renderDialogBox creates a styled dialog box for confirmation dialogs
func (m Model) renderDialogBox(content string, width int) string {
	boxStyle := lp.NewStyle().
		BorderStyle(lp.NormalBorder()).   // Use normal border instead of thick
		BorderForeground(lp.Color("12")). // Regular blue border
		Padding(1, 2).
		Width(width)

	return boxStyle.Render(content)
}

// renderDeleteConfirmDialog creates the delete confirmation dialog
func (m Model) renderDeleteConfirmDialog() string {
	var contentBuilder strings.Builder

	// Find the build being deleted to get its status
	var buildStatus types.BuildState
	for _, build := range m.builds {
		if build.Version == m.deleteCandidate {
			buildStatus = build.Status
			break
		}
	}

	// Add title
	titleStyle := lp.NewStyle().
		Bold(true)

	contentBuilder.WriteString(titleStyle.Render("Confirm Deletion"))
	contentBuilder.WriteString("\n\n")

	// Build version styling
	buildStyle := lp.NewStyle().
		Foreground(lp.Color("15")). // White text
		Bold(true)

	// Create the message with styled build name
	buildText := buildStyle.Render("Blender " + m.deleteCandidate)

	statusText := ""
	if buildStatus == types.StateUpdate {
		statusText = " (with available update)"
	}

	// Add message lines
	contentBuilder.WriteString("Are you sure you want to delete " + buildText + statusText + "?\n")
	contentBuilder.WriteString("This will permanently remove this build from your system.\n\n")

	// Add simplified instructions
	contentBuilder.WriteString("[y] Yes, delete it    [n] No, cancel")

	// Apply the box styling
	return m.renderDialogBox(contentBuilder.String(), deleteDialogWidth)
}

// renderCleanupConfirmDialog creates the cleanup confirmation dialog
func (m Model) renderCleanupConfirmDialog() string {
	var contentBuilder strings.Builder

	// Add title
	titleStyle := lp.NewStyle().
		Bold(true)

	contentBuilder.WriteString(titleStyle.Render("Confirm Cleanup"))
	contentBuilder.WriteString("\n\n")

	// Add message lines
	contentBuilder.WriteString(fmt.Sprintf("Are you sure you want to clean up %d old %s?\n",
		m.oldBuildsCount,
		pluralize("build", m.oldBuildsCount)))
	contentBuilder.WriteString(fmt.Sprintf("This will free up %s of disk space.\n",
		formatByteSize(m.oldBuildsSize)))
	contentBuilder.WriteString("These builds were identified as incomplete or older versions.\n\n")

	// Add simplified instructions
	contentBuilder.WriteString("[y] Yes, clean up    [n] No, cancel")

	// Apply the box styling
	return m.renderDialogBox(contentBuilder.String(), cleanupDialogWidth)
}

// renderQuitConfirmDialog creates the quit confirmation dialog when downloads are in progress
func (m Model) renderQuitConfirmDialog() string {
	var contentBuilder strings.Builder

	// Add title
	titleStyle := lp.NewStyle().
		Bold(true)

	contentBuilder.WriteString(titleStyle.Render("Confirm Exit"))
	contentBuilder.WriteString("\n\n")

	// Count active downloads
	m.downloadMutex.Lock()
	activeDownloads := 0
	extractingInProgress := false

	for _, state := range m.downloadStates {
		if state.BuildState == types.StateDownloading ||
			state.BuildState == types.StatePreparing ||
			state.BuildState == types.StateExtracting {
			activeDownloads++
			if state.BuildState == types.StateExtracting {
				extractingInProgress = true
			}
		}
	}
	m.downloadMutex.Unlock()

	// Build message content
	if extractingInProgress {
		contentBuilder.WriteString(fmt.Sprintf("There %s %d active %s or extraction%s in progress.\n",
			pluralize("is", activeDownloads),
			activeDownloads,
			pluralize("download", activeDownloads),
			pluralize("", activeDownloads)))
		contentBuilder.WriteString("Extraction is a CPU-intensive process and can't be resumed if interrupted.\n")
	} else {
		contentBuilder.WriteString(fmt.Sprintf("There %s %d active %s in progress.\n",
			pluralize("is", activeDownloads),
			activeDownloads,
			pluralize("download", activeDownloads)))
	}

	contentBuilder.WriteString("\nDo you want to quit and cancel the operations?\n\n")

	// Add simplified instructions
	contentBuilder.WriteString("[y] Yes, quit    [n] No, continue")

	// Apply the box styling
	return m.renderDialogBox(contentBuilder.String(), quitDialogWidth)
}
