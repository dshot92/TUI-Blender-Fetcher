package tui

import (
	"TUI-Blender-Launcher/api"
	"TUI-Blender-Launcher/config"
	"TUI-Blender-Launcher/download"
	"TUI-Blender-Launcher/local"
	"TUI-Blender-Launcher/model"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// CommandManager encapsulates all the command generation logic for the TUI
type CommandManager struct {
	Config        config.Config
	DownloadMap   map[string]*model.DownloadState
	DownloadMutex *sync.Mutex
}

// NewCommandManager creates a new CommandManager
func NewCommandManager(cfg config.Config, downloadMap map[string]*model.DownloadState, mutex *sync.Mutex) *CommandManager {
	return &CommandManager{
		Config:        cfg,
		DownloadMap:   downloadMap,
		DownloadMutex: mutex,
	}
}

// FetchBuilds creates a command to fetch builds from the API
func (cm *CommandManager) FetchBuilds() tea.Cmd {
	return func() tea.Msg {
		builds, err := api.FetchBuilds(cm.Config.VersionFilter)
		if err != nil {
			return errMsg{err}
		}
		return buildsFetchedMsg{builds: builds, err: nil}
	}
}

// ScanLocalBuilds creates a command to scan for local builds
func (cm *CommandManager) ScanLocalBuilds() tea.Cmd {
	return func() tea.Msg {
		builds, err := local.ScanLocalBuilds(cm.Config.DownloadDir)
		return localBuildsScannedMsg{builds: builds, err: err}
	}
}

// UpdateStatusFromLocalScan creates a command to update status of builds based on local scan
func (cm *CommandManager) UpdateStatusFromLocalScan(onlineBuilds []model.BlenderBuild) tea.Cmd {
	return func() tea.Msg {
		localBuilds, err := local.ScanLocalBuilds(cm.Config.DownloadDir)
		if err != nil {
			return errMsg{fmt.Errorf("failed local scan during status update: %w", err)}
		}

		localBuildMap := make(map[string]model.BlenderBuild)
		for _, build := range localBuilds {
			localBuildMap[build.Version] = build
		}

		updatedBuilds := make([]model.BlenderBuild, len(onlineBuilds))
		copy(updatedBuilds, onlineBuilds) // Work on a copy

		for i := range updatedBuilds {
			if localBuild, found := localBuildMap[updatedBuilds[i].Version]; found {
				if local.CheckUpdateAvailable(localBuild, updatedBuilds[i]) {
					updatedBuilds[i].Status = model.StateUpdate
				} else {
					updatedBuilds[i].Status = model.StateLocal
				}
			} else {
				updatedBuilds[i].Status = model.StateOnline // Not installed
			}
		}
		return buildsUpdatedMsg{builds: updatedBuilds}
	}
}

// Tick creates a tick command with fixed rate
func (cm *CommandManager) Tick() tea.Cmd {
	return tea.Tick(downloadTickRate, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// AdaptiveTick creates a tick command with adaptive rate based on download activity
func (cm *CommandManager) AdaptiveTick(activeCount int, isExtracting bool) tea.Cmd {
	rate := downloadTickRate

	if activeCount == 0 {
		rate = 500 * time.Millisecond // Slower when idle
	} else if isExtracting {
		rate = 250 * time.Millisecond // During extraction
	} else if activeCount > 1 {
		rate = 80 * time.Millisecond // Multiple downloads
	}

	return tea.Tick(rate, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// DoDownload creates a command to download and extract a build
func (cm *CommandManager) DoDownload(build model.BlenderBuild) tea.Cmd {
	now := time.Now()

	// Create a unique build ID
	buildID := build.Version
	if build.Hash != "" {
		buildID = build.Version + "-" + build.Hash[:8]
	}

	// Create a cancel channel specific to this download
	downloadCancelCh := make(chan struct{})

	cm.DownloadMutex.Lock()
	if _, exists := cm.DownloadMap[buildID]; !exists {
		cm.DownloadMap[buildID] = &model.DownloadState{
			BuildID:       buildID,
			BuildState:    model.StateDownloading,
			StartTime:     now,
			LastUpdated:   now,
			Progress:      0.0,
			StallDuration: downloadStallTime,
			CancelCh:      downloadCancelCh,
		}
	} else {
		cm.DownloadMutex.Unlock()
		return nil
	}
	cm.DownloadMutex.Unlock()

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
			if !cm.DownloadMutex.TryLock() {
				return
			}
			defer cm.DownloadMutex.Unlock()

			if state, ok := cm.DownloadMap[buildID]; ok {
				// If already cancelled, don't update progress
				if state.BuildState == model.StateNone {
					return
				}

				// Always update the last update timestamp
				state.LastUpdated = currentTime

				// Use a virtual size threshold to detect extraction phase
				const extractionVirtualSize int64 = 100 * 1024 * 1024

				// Determine state based on progress info
				if total == extractionVirtualSize {
					// Extraction phase
					state.BuildState = model.StateExtracting
					state.Progress = percent
					state.Speed = 0                           // No download speed during extraction
					state.StallDuration = extractionStallTime // Longer timeout for extraction
				} else if state.BuildState == model.StateExtracting {
					// Continue extraction progress updates
					state.Progress = percent
				} else {
					// Normal download progress
					state.Progress = percent
					state.Current = downloaded
					state.Total = total
					state.Speed = currentSpeed
					state.BuildState = model.StateDownloading
					state.StallDuration = downloadStallTime
				}

				// Update the download state
				updated := UpdateDownloadProgress(cm.DownloadMap, buildID, downloaded, total, model.StateDownloading)

				if updated {
					// Also update speed if we have a new value
					if currentSpeed > 0 {
						if state, ok := cm.DownloadMap[buildID]; ok {
							state.Speed = currentSpeed
						}
					}
				}
			}
		}

		// Check for cancellation before starting download
		select {
		case <-downloadCancelCh:
			// Download canceled before starting
			cm.DownloadMutex.Lock()
			if state, ok := cm.DownloadMap[buildID]; ok {
				state.BuildState = model.StateNone
			}
			cm.DownloadMutex.Unlock()
			close(done)
			return
		default:
			// Proceed with download
		}

		// Download and extract the build
		extractPath, err := download.DownloadAndExtractBuild(build, cm.Config.DownloadDir, progressCallback, downloadCancelCh)

		// Check for cancellation or handle completion
		select {
		case <-downloadCancelCh:
			// Download was cancelled during execution
			cm.DownloadMutex.Lock()
			if state, ok := cm.DownloadMap[buildID]; ok {
				state.BuildState = model.StateNone
			}
			cm.DownloadMutex.Unlock()
			close(done)
			return
		default:
			// Download completed (success or error)
			cm.DownloadMutex.Lock()
			defer cm.DownloadMutex.Unlock()

			if state, ok := cm.DownloadMap[buildID]; ok {
				if err != nil {
					// Handle download error
					state.BuildState = model.StateFailed
					// Send completion message with error
					go func() {
						time.Sleep(3 * time.Second) // Keep error visible for a moment
						deletePath := filepath.Join(cm.Config.DownloadDir, ".downloading", build.Version)
						os.RemoveAll(deletePath) // Clean up partial download
						programCh <- downloadCompleteMsg{
							buildVersion:  build.Version,
							extractedPath: "",
							err:           err,
						}
						close(done)
					}()
				} else {
					// Handle download success
					state.BuildState = model.StateLocal
					state.Progress = 1.0 // Ensure progress is shown as complete
					// Send completion message
					go func() {
						time.Sleep(1 * time.Second) // Brief pause to show completion
						programCh <- downloadCompleteMsg{
							buildVersion:  build.Version,
							extractedPath: extractPath,
							err:           nil,
						}
						close(done)
					}()
				}
			} else {
				close(done)
			}
		}
	}()

	// Return a no-op command - real messages will be sent via programCh
	return nil
}

// UIRefresh creates a command that forces a UI refresh
func (cm *CommandManager) UIRefresh() tea.Cmd {
	return func() tea.Msg {
		// Short pause to avoid excessive refreshes
		time.Sleep(time.Millisecond * 500)
		return forceRenderMsg{}
	}
}

// Global channel for program messages
var programCh = make(chan tea.Msg)

// ProgramMsgListener returns a command that listens for program messages
func (cm *CommandManager) ProgramMsgListener() tea.Cmd {
	return func() tea.Msg {
		return <-programCh
	}
}
