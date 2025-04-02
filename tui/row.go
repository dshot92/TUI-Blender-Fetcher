package tui

import (
	"TUI-Blender-Launcher/model"
	"TUI-Blender-Launcher/types"
	"bytes"
	"fmt"
	"time"
)

// Row represents a single row in the builds table
type Row struct {
	Build         model.BlenderBuild
	IsSelected    bool
	DownloadState *DownloadState
}

// NewRow creates a new row instance from a build
func NewRow(build model.BlenderBuild, isSelected bool, downloadState *DownloadState) Row {
	return Row{
		Build:         build,
		IsSelected:    isSelected,
		DownloadState: downloadState,
	}
}

// Render renders a single row with the given column configuration
func (r Row) Render(columns []ColumnConfig) string {
	var rowBuffer bytes.Buffer

	// Render each cell in the row
	for colIdx, col := range columns {
		if col.Visible {
			var cellContent string

			// Determine cell content based on column key
			switch col.Key {
			case "Version":
				cellContent = r.Build.Version
			case "Status":
				cellContent = r.renderStatus()
			case "Branch":
				cellContent = r.Build.Branch
			case "Type":
				cellContent = r.Build.ReleaseCycle
			case "Hash":
				// Truncate hash for display
				if len(r.Build.Hash) > 8 {
					cellContent = r.Build.Hash[:8]
				} else {
					cellContent = r.Build.Hash
				}
			case "Size":
				cellContent = formatByteSize(r.Build.Size)
			case "Build Date":
				// Get timestamp as proper time.Time
				buildTime := r.Build.BuildDate.Time()
				if !buildTime.IsZero() {
					cellContent = buildTime.Format("2006-01-02")
				} else {
					cellContent = "Unknown"
				}
			}

			// Apply column-specific style to cell content
			rowBuffer.WriteString(col.Style(cellContent))

			// Add space between columns
			if colIdx < len(columns)-1 {
				rowBuffer.WriteString(" ")
			}
		}
	}

	// Apply row styling (selected or regular)
	if r.IsSelected {
		return selectedRowStyle.Render(rowBuffer.String())
	}
	return regularRowStyle.Render(rowBuffer.String())
}

// renderStatus renders the status cell with appropriate formatting
// This is where download and extraction progress is displayed
func (r Row) renderStatus() string {
	// If there's an active download for this build, show progress information
	if r.DownloadState != nil && (r.DownloadState.BuildState == types.StateDownloading || r.DownloadState.BuildState == types.StateExtracting) {
		if r.DownloadState.BuildState == types.StateDownloading {
			// Show download progress with percentage and speed
			if r.DownloadState.Total > 0 {
				percent := (float64(r.DownloadState.Current) / float64(r.DownloadState.Total)) * 100
				speed := r.DownloadState.Speed
				if speed == 0 && !r.DownloadState.StartTime.IsZero() {
					elapsedSecs := time.Since(r.DownloadState.StartTime).Seconds()
					if elapsedSecs > 0 {
						speed = float64(r.DownloadState.Current) / elapsedSecs
					}
				}
				return fmt.Sprintf("%.1f%% (%.1f MB/s)", percent, speed/1024/1024)
			}
			return "Downloading..."
		} else if r.DownloadState.BuildState == types.StateExtracting {
			return "Extracting..."
		}
	}

	// For non-downloading builds, show the regular state
	return buildStateToString(r.Build.Status)
}

// ColumnConfig represents the configuration for a table column
type ColumnConfig struct {
	Name    string
	Key     string
	Visible bool
	Width   int
	Style   func(string) string
}

// GetBuildColumns returns the column configuration for the build table
func GetBuildColumns(visibleColumns map[string]bool) []ColumnConfig {
	return []ColumnConfig{
		{
			Name:    "Blender",
			Key:     "Version",
			Visible: true, // Always visible
			Width:   columnConfigs["Version"].width,
			Style: func(s string) string {
				return cellStyleLeft.Width(columnConfigs["Version"].width).Render(s)
			},
		},
		{
			Name:    "Status",
			Key:     "Status",
			Visible: true, // Always visible
			Width:   columnConfigs["Status"].width,
			Style: func(s string) string {
				return cellStyleCenter.Width(columnConfigs["Status"].width).Render(s)
			},
		},
		{
			Name:    "Branch",
			Key:     "Branch",
			Visible: visibleColumns["Branch"],
			Width:   columnConfigs["Branch"].width,
			Style: func(s string) string {
				return cellStyleCenter.Width(columnConfigs["Branch"].width).Render(s)
			},
		},
		{
			Name:    "Type",
			Key:     "Type",
			Visible: visibleColumns["Type"],
			Width:   columnConfigs["Type"].width,
			Style: func(s string) string {
				return cellStyleCenter.Width(columnConfigs["Type"].width).Render(s)
			},
		},
		{
			Name:    "Hash",
			Key:     "Hash",
			Visible: visibleColumns["Hash"],
			Width:   columnConfigs["Hash"].width,
			Style: func(s string) string {
				return cellStyleCenter.Width(columnConfigs["Hash"].width).Render(s)
			},
		},
		{
			Name:    "Size",
			Key:     "Size",
			Visible: visibleColumns["Size"],
			Width:   columnConfigs["Size"].width,
			Style: func(s string) string {
				return cellStyleRight.Width(columnConfigs["Size"].width).Render(s)
			},
		},
		{
			Name:    "Build Date",
			Key:     "Build Date",
			Visible: visibleColumns["Build Date"],
			Width:   columnConfigs["Build Date"].width,
			Style: func(s string) string {
				return cellStyleCenter.Width(columnConfigs["Build Date"].width).Render(s)
			},
		},
	}
}

// RenderRows renders all visible rows based on current scroll position
func RenderRows(m Model, maxShownBuilds int) string {
	var output bytes.Buffer

	// Calculate end index for iteration
	endIdx := m.scrollOffset + maxShownBuilds
	if endIdx > len(m.builds) {
		endIdx = len(m.builds)
	}

	// Get column configuration
	columns := GetBuildColumns(m.visibleColumns)

	// Render each visible row
	for i := m.scrollOffset; i < endIdx; i++ {
		build := m.builds[i]

		// Create buildID to check for download state
		buildID := build.Version + "-" + build.Hash

		// Get download state if exists
		var downloadState *DownloadState
		if state, exists := m.downloadStates[buildID]; exists {
			downloadState = state
		}

		// Create and render row
		row := NewRow(build, i == m.cursor, downloadState)
		output.WriteString(row.Render(columns))
		output.WriteString("\n")
	}

	return output.String()
}

// UpdateDownloadProgress updates the progress for a specific build row
// Returns true if the UI should be refreshed
func UpdateDownloadProgress(states map[string]*DownloadState, buildID string, current, total int64, buildState types.BuildState) bool {
	state, exists := states[buildID]
	if !exists {
		// Create a new download state if one doesn't exist
		state = &DownloadState{
			BuildID:       buildID,
			BuildState:    buildState,
			Current:       current,
			Total:         total,
			StartTime:     time.Now(),
			LastUpdated:   time.Now(),
			StallDuration: downloadStallTime,
			CancelCh:      make(chan struct{}),
		}
		states[buildID] = state
		return true
	}

	// Check if state actually changed to avoid unnecessary updates
	if state.Current == current && state.BuildState == buildState {
		return false
	}

	// Update the state
	state.Current = current
	state.Total = total
	state.BuildState = buildState

	// Calculate progress percentage
	if total > 0 {
		state.Progress = float64(current) / float64(total)
	}

	// Calculate download speed
	elapsed := time.Since(state.LastUpdated).Seconds()
	if elapsed > 0 && state.BuildState == types.StateDownloading {
		bytesDownloaded := current - state.Current
		state.Speed = float64(bytesDownloaded) / elapsed
	}

	// Update timestamps
	state.LastUpdated = time.Now()

	// Update stall duration based on state
	if buildState == types.StateExtracting {
		state.StallDuration = extractionStallTime
	} else {
		state.StallDuration = downloadStallTime
	}

	return true
}

// CancelDownload signals cancellation for a specific build row
// Returns true if a download was actually cancelled
func CancelDownload(states map[string]*DownloadState, buildID string) bool {
	state, exists := states[buildID]
	if !exists {
		return false
	}

	// Only cancel if the build is in a cancellable state
	if state.BuildState != types.StateDownloading &&
		state.BuildState != types.StatePreparing &&
		state.BuildState != types.StateExtracting {
		return false
	}

	// Signal cancellation by closing the channel if it hasn't been closed already
	select {
	case <-state.CancelCh:
		// Channel already closed (cancel already called)
		return false
	default:
		// Close the channel to signal cancellation
		close(state.CancelCh)

		// Update the state
		state.BuildState = types.StateNone
		return true
	}
}
