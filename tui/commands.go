package tui

import (
	"TUI-Blender-Launcher/api"
	"TUI-Blender-Launcher/config"
	"TUI-Blender-Launcher/download"
	"TUI-Blender-Launcher/local"
	"TUI-Blender-Launcher/model"
	"TUI-Blender-Launcher/types"
	"errors"
	"fmt"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// fetchBuildsCmd fetches builds from the API
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

// scanLocalBuildsCmd scans for local builds
func scanLocalBuildsCmd(cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		builds, err := local.ScanLocalBuilds(cfg.DownloadDir)
		// Return specific message for local scan results
		return localBuildsScannedMsg{builds: builds, err: err}
	}
}

// updateStatusFromLocalScanCmd updates status of builds based on local scan
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
					updatedBuilds[i].Status = types.StateUpdate
				} else {
					updatedBuilds[i].Status = types.StateLocal
				}
			} else {
				updatedBuilds[i].Status = types.StateOnline // Not installed
			}
		}
		return buildsUpdatedMsg{builds: updatedBuilds}
	}
}

// tickCmd sends a tickMsg after a short delay
func tickCmd() tea.Cmd {
	return tea.Tick(downloadTickRate, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// adaptiveTickCmd creates a tick with adaptive rate based on download activity
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

// doDownloadCmd starts the download in a goroutine
func doDownloadCmd(build model.BlenderBuild, cfg config.Config, downloadMap map[string]*DownloadState, mutex *sync.Mutex) tea.Cmd {
	now := time.Now()

	// Create a unique build ID using version and hash
	buildID := build.Version
	if build.Hash != "" {
		buildID = build.Version + "-" + build.Hash[:8]
	}

	// Create a cancel channel specific to this download
	downloadCancelCh := make(chan struct{})

	mutex.Lock()
	if _, exists := downloadMap[buildID]; !exists {
		// Initialize download state with defaults
		downloadMap[buildID] = &DownloadState{
			BuildID:       buildID,
			BuildState:    types.StatePreparing,
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

			if state, ok := downloadMap[buildID]; ok {
				// If already cancelled, don't update progress
				if state.BuildState == types.StateNone {
					return
				}

				// Always update the last update timestamp
				state.LastUpdated = currentTime

				// Use a virtual size threshold to detect extraction phase
				const extractionVirtualSize int64 = 100 * 1024 * 1024

				// Determine state based on progress info
				if total == extractionVirtualSize {
					// Extraction phase
					state.BuildState = types.StateExtracting
					state.Progress = percent
					state.Speed = 0                           // No download speed during extraction
					state.StallDuration = extractionStallTime // Longer timeout for extraction
				} else if state.BuildState == types.StateExtracting {
					// Continue extraction progress updates
					state.Progress = percent
				} else {
					// Normal download progress
					state.Progress = percent
					state.Current = downloaded
					state.Total = total
					state.Speed = currentSpeed
					state.BuildState = types.StateDownloading
					state.StallDuration = downloadStallTime
				}
			}
		}

		// Check for cancellation before starting download
		select {
		case <-downloadCancelCh:
			// Download canceled before starting
			mutex.Lock()
			if state, ok := downloadMap[buildID]; ok {
				state.BuildState = types.StateNone
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
			if state, ok := downloadMap[buildID]; ok {
				state.BuildState = types.StateNone
			}
			mutex.Unlock()
			close(done)
			return
		default:
			// Continue processing result
		}

		// Update final status based on result
		mutex.Lock()
		if state, ok := downloadMap[buildID]; ok {
			if state.BuildState == types.StateNone {
				// Keep as cancelled
			} else if err != nil {
				if errors.Is(err, download.ErrCancelled) {
					state.BuildState = types.StateNone
				} else {
					// StateNone for error states, will be displayed with additional context in UI
					state.BuildState = types.StateNone
				}
			} else {
				state.BuildState = types.StateLocal
			}
		}
		mutex.Unlock()

		// Signal completion
		close(done)
	}()

	// Return the buildID with the command so the model can track the active download
	return func() tea.Msg {
		return startDownloadMsg{
			build:   build,
			buildID: buildID,
		}
	}
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
