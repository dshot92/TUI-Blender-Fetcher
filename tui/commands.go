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

// FetchBuilds fetches the list of builds from the API.
func (c *Commands) FetchBuilds() tea.Cmd {
	return func() tea.Msg {
		builds, err := api.FetchBuilds(c.cfg.VersionFilter, c.cfg.BuildType)
		return buildsFetchedMsg{builds, err}
	}
}

// ScanLocalBuilds creates a command to scan for local builds
func (c *Commands) ScanLocalBuilds() tea.Cmd {
	return func() tea.Msg {
		builds, err := local.ScanLocalBuilds(c.cfg.DownloadDir)
		return localBuildsScannedMsg{builds: builds, err: err}
	}
}

// CheckUpdateAvailable determines if an update is available for a local build by comparing build dates, branch, and release_cycle.
func CheckUpdateAvailable(localBuild, onlineBuild model.BlenderBuild) model.BuildState {
	// If online build hash is present and matches local build hash, treat as identical (no update)
	if onlineBuild.Hash != "" && onlineBuild.Hash == localBuild.Hash {
		return model.StateLocal
	}

	// Ensure version, branch, and release_cycle all match; if not, treat as no local match
	if localBuild.Version != onlineBuild.Version || localBuild.Branch != onlineBuild.Branch || localBuild.ReleaseCycle != onlineBuild.ReleaseCycle {
		return model.StateOnline
	}

	// If local build date is not set, assume update is available
	if localBuild.BuildDate.Time().IsZero() {
		return model.StateUpdate
	}
	if onlineBuild.BuildDate.Time().IsZero() {
		return model.StateOnline
	}

	if onlineBuild.BuildDate.Time().After(localBuild.BuildDate.Time()) {
		return model.StateUpdate
	}
	return model.StateLocal
}

// UpdateBuildStatus creates a command to update status of builds based on local scan
func (c *Commands) UpdateBuildStatus(onlineBuilds []model.BlenderBuild) tea.Cmd {
	return func() tea.Msg {
		localBuilds, err := local.ScanLocalBuilds(c.cfg.DownloadDir)
		if err != nil {
			return errMsg{fmt.Errorf("failed local scan during status update: %w", err)}
		}

		// Create maps for quick lookup by version and hash
		localBuildMap := make(map[string]model.BlenderBuild)
		localBuildHashMap := make(map[string]model.BlenderBuild)
		for _, build := range localBuilds {
			localBuildMap[build.Version] = build
			if build.Hash != "" {
				localBuildHashMap[build.Hash] = build
			}
		}

		// Group online builds by composite key: version|branch|releaseCycle
		grouped := make(map[string]model.BlenderBuild)
		for _, onlineBuild := range onlineBuilds {
			var localBuild *model.BlenderBuild
			// Try to find a matching local build by hash
			if onlineBuild.Hash != "" {
				if lb, found := localBuildHashMap[onlineBuild.Hash]; found {
					localBuild = &lb
				}
			}
			// Fallback to matching by version
			if localBuild == nil {
				if lb, found := localBuildMap[onlineBuild.Version]; found {
					localBuild = &lb
				}
			}

			var status model.BuildState
			if localBuild == nil {
				status = model.StateOnline
			} else {
				switch CheckUpdateAvailable(*localBuild, onlineBuild) {
				case model.StateUpdate:
					status = model.StateUpdate
				case model.StateLocal:
					status = model.StateLocal
				default:
					status = model.StateOnline
				}
			}

			updated := onlineBuild
			updated.Status = status

			// Composite key: version|branch|releaseCycle
			key := onlineBuild.Version + "|" + onlineBuild.Branch + "|" + onlineBuild.ReleaseCycle

			// If an entry already exists, prefer the one with StateUpdate over StateLocal
			if existing, exists := grouped[key]; exists {
				if existing.Status == model.StateLocal && updated.Status == model.StateUpdate {
					grouped[key] = updated
				}
			} else {
				grouped[key] = updated
			}
		}

		// Build final list
		finalBuilds := make([]model.BlenderBuild, 0, len(grouped))
		for _, b := range grouped {
			finalBuilds = append(finalBuilds, b)
		}

		return buildsUpdatedMsg{builds: finalBuilds}
	}
}

// DoDownload creates a command to download and extract a build
func (c *Commands) DoDownload(build model.BlenderBuild) tea.Cmd {
	return func() tea.Msg {
		return c.downloads.StartDownload(build)
	}
}

// StartTicker starts a ticker to regularly update the UI during downloads
func (c *Commands) StartTicker() tea.Cmd {
	return func() tea.Msg {
		ticker := time.NewTicker(500 * time.Millisecond)
		done := make(chan bool)

		go func() {
			for {
				select {
				case <-done:
					return
				case t := <-ticker.C:
					programCh <- tickMsg(t)
				}
			}
		}()

		return nil
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
