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
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// DownloadManager handles all download operations with thread-safe state access
type DownloadManager struct {
	states map[string]*model.DownloadState
	cfg    config.Config
}

// NewDownloadManager creates a new download manager
func NewDownloadManager(cfg config.Config) *DownloadManager {
	return &DownloadManager{
		states: make(map[string]*model.DownloadState),
		cfg:    cfg,
	}
}

// GetState safely retrieves state for a build
func (dm *DownloadManager) GetState(buildID string) *model.DownloadState {
	return dm.states[buildID]
}

// GetAllStates returns a copy of all download states
func (dm *DownloadManager) GetAllStates() map[string]*model.DownloadState {
	result := make(map[string]*model.DownloadState)
	for k, v := range dm.states {
		result[k] = v
	}
	return result
}

// StartDownload begins a new download for a build
func (dm *DownloadManager) StartDownload(build model.BlenderBuild) tea.Msg {
	// Create a unique build ID
	buildID := build.Version
	if build.Hash != "" {
		buildID = build.Version + "-" + build.Hash[:8]
	}

	// Don't start if already downloading
	if _, exists := dm.states[buildID]; exists {
		return nil
	}

	// Setup download state
	now := time.Now()
	cancelCh := make(chan struct{})
	dm.states[buildID] = &model.DownloadState{
		BuildID:     buildID,
		BuildState:  model.StateDownloading,
		StartTime:   now,
		LastUpdated: now,
		Progress:    0.0,
		CancelCh:    cancelCh,
	}

	// Start the download in a goroutine
	go func() {
		var lastBytes int64
		var lastTime time.Time
		var speed float64

		// Progress callback
		progressCallback := func(downloaded, total int64) {
			select {
			case <-cancelCh:
				return
			default:
			}

			now := time.Now()
			percent := 0.0
			if total > 0 {
				percent = float64(downloaded) / float64(total)
			}

			// Calculate download speed
			if !lastTime.IsZero() && now.Sub(lastTime).Seconds() > 0.2 {
				bytesDiff := downloaded - lastBytes
				timeDiff := now.Sub(lastTime).Seconds()
				speed = float64(bytesDiff) / timeDiff
				lastBytes = downloaded
				lastTime = now
			} else if lastTime.IsZero() {
				lastBytes = downloaded
				lastTime = now
			}

			// Update state based on progress
			state := dm.states[buildID]
			if state == nil {
				return
			}

			state.LastUpdated = now
			state.Progress = percent
			state.Current = downloaded
			state.Total = total
			state.Speed = speed

			// Handle extraction phase
			const extractionVirtualSize int64 = 100 * 1024 * 1024
			if total == extractionVirtualSize {
				state.BuildState = model.StateExtracting
			}
		}

		// Perform the download
		extractPath, err := download.DownloadAndExtractBuild(build, dm.cfg.DownloadDir, progressCallback, cancelCh)

		// Update final state
		state := dm.states[buildID]
		if state == nil {
			return
		}

		if err != nil {
			state.BuildState = model.StateFailed
			// Clean up partial download after showing error
			go func() {
				time.Sleep(3 * time.Second)
				deletePath := filepath.Join(dm.cfg.DownloadDir, ".downloading", build.Version)
				os.RemoveAll(deletePath)
			}()
		} else {
			state.BuildState = model.StateLocal
			state.Progress = 1.0
		}

		// Send completion message
		programCh <- downloadCompleteMsg{
			buildVersion:  build.Version,
			extractedPath: extractPath,
			err:           err,
		}
	}()

	return nil
}

// CancelDownload stops an in-progress download
func (dm *DownloadManager) CancelDownload(buildID string) {
	state := dm.states[buildID]
	if state == nil {
		return
	}

	close(state.CancelCh)
	state.BuildState = model.StateNone
	delete(dm.states, buildID)
}

// Commands generates tea commands for the TUI
type Commands struct {
	cfg       config.Config
	downloads *DownloadManager
}

// NewCommands creates a new Commands instance
func NewCommands(cfg config.Config) *Commands {
	return &Commands{
		cfg:       cfg,
		downloads: NewDownloadManager(cfg),
	}
}

// FetchBuilds creates a command to fetch builds from the API
func (c *Commands) FetchBuilds() tea.Cmd {
	return func() tea.Msg {
		builds, err := api.FetchBuilds(c.cfg.VersionFilter)
		if err != nil {
			return errMsg{err}
		}
		return buildsFetchedMsg{builds: builds, err: nil}
	}
}

// ScanLocalBuilds creates a command to scan for local builds
func (c *Commands) ScanLocalBuilds() tea.Cmd {
	return func() tea.Msg {
		builds, err := local.ScanLocalBuilds(c.cfg.DownloadDir)
		return localBuildsScannedMsg{builds: builds, err: err}
	}
}

// UpdateBuildStatus creates a command to update status of builds based on local scan
func (c *Commands) UpdateBuildStatus(onlineBuilds []model.BlenderBuild) tea.Cmd {
	return func() tea.Msg {
		localBuilds, err := local.ScanLocalBuilds(c.cfg.DownloadDir)
		if err != nil {
			return errMsg{fmt.Errorf("failed local scan during status update: %w", err)}
		}

		// Create map for quick lookup
		localBuildMap := make(map[string]model.BlenderBuild)
		for _, build := range localBuilds {
			localBuildMap[build.Version] = build
		}

		// Copy builds to avoid modifying originals, showing all builds
		updatedBuilds := make([]model.BlenderBuild, 0, len(onlineBuilds))

		// Update status for each build
		for _, build := range onlineBuilds {
			// Create a copy of the build to update
			updatedBuild := build

			if localBuild, found := localBuildMap[build.Version]; found {
				if local.CheckUpdateAvailable(localBuild, build) {
					updatedBuild.Status = model.StateUpdate
				} else {
					updatedBuild.Status = model.StateLocal
				}
			} else {
				updatedBuild.Status = model.StateOnline
			}

			updatedBuilds = append(updatedBuilds, updatedBuild)
		}

		return buildsUpdatedMsg{builds: updatedBuilds}
	}
}

// DoDownload creates a command to download and extract a build
func (c *Commands) DoDownload(build model.BlenderBuild) tea.Cmd {
	return func() tea.Msg {
		return c.downloads.StartDownload(build)
	}
}

// Global channel for program messages - kept for compatibility
var programCh = make(chan tea.Msg)

// ProgramMsgListener returns a command that listens for program messages
func (c *Commands) ProgramMsgListener() tea.Cmd {
	return func() tea.Msg {
		return <-programCh
	}
}

// UIRefresh creates a command that forces a UI refresh
func (c *Commands) UIRefresh() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(time.Millisecond * 200)
		return forceRenderMsg{}
	}
}
