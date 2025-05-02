package tui

import (
	"TUI-Blender-Launcher/api"
	"TUI-Blender-Launcher/config"
	"TUI-Blender-Launcher/download"
	"TUI-Blender-Launcher/local"
	"TUI-Blender-Launcher/model"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/cavaliergopher/grab/v3"
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

	// Clean up previous state if it was Failed or Cancelled before starting anew
	if state, exists := dm.states[buildID]; exists {
		if state.BuildState == model.StateFailed || state.BuildState == model.StateCancelled {
			// Remove the old failed/cancelled state to allow restart
			delete(dm.states, buildID)
		} else if state.BuildState == model.StateDownloading || state.BuildState == model.StateExtracting {
			// If already downloading/extracting this exact build, don't start another one
			return nil
		}
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

	// Create a temporary directory for downloads if it doesn't exist
	downloadTempDir := filepath.Join(dm.cfg.DownloadDir, download.DownloadingDir)
	if err := os.MkdirAll(downloadTempDir, 0750); err != nil {
		// Handle error creating download directory
		dm.states[buildID].BuildState = model.StateFailed
		programCh <- downloadCompleteMsg{
			buildVersion: build.Version,
			err:          fmt.Errorf("failed to create download directory: %w", err),
		}
		return nil
	}

	// Start the download in a goroutine
	go func() {
		// Get the filename from the download URL
		downloadFileName := filepath.Base(build.DownloadURL)
		downloadPath := filepath.Join(downloadTempDir, downloadFileName)

		// Set up the grab library context for cancellation
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Create a go routine to handle cancellation via our channel
		go func() {
			select {
			case <-cancelCh:
				cancel() // Cancel grab request if our channel is closed
			case <-ctx.Done():
				// Context done normally
			}
		}()

		// Create the grab client with extended timeouts
		client := grab.NewClient()
		client.UserAgent = "TUI-Blender-Launcher"

		// Set custom HTTP client with timeouts
		httpClient := &http.Client{
			Timeout: 5 * time.Minute,
			Transport: &http.Transport{
				IdleConnTimeout:     2 * time.Minute,
				DisableCompression:  false,
				TLSHandshakeTimeout: 1 * time.Minute,
			},
		}
		client.HTTPClient = httpClient

		// Create the request
		req, err := grab.NewRequest(downloadPath, build.DownloadURL)
		if err != nil {
			dm.states[buildID].BuildState = model.StateFailed
			programCh <- downloadCompleteMsg{
				buildVersion: build.Version,
				err:          fmt.Errorf("failed to create download request: %w", err),
			}
			return
		}
		req = req.WithContext(ctx)

		// Start download
		resp := client.Do(req)

		// Use a ticker to update the download state
		var lastBytes int64
		var lastTime time.Time
		var speedSamples []float64
		var speed float64
		var speedUpdateCounter int

		// Use a slightly longer interval for UI updates to reduce flickering
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

	downloadLoop:
		for {
			select {
			case <-ticker.C:
				// Update download state with grab response status
				now := time.Now()
				state := dm.states[buildID]
				if state == nil {
					break downloadLoop // State was deleted, exit loop
				}

				downloaded := resp.BytesComplete()
				total := resp.Size()

				// Calculate progress percentage
				percent := 0.0
				if total > 0 {
					percent = float64(downloaded) / float64(total)
				}

				// Calculate download speed with moving average for smoothing
				if !lastTime.IsZero() {
					// Only update speed calculation every 2 ticks to further reduce fluctuations
					speedUpdateCounter++
					if speedUpdateCounter >= 2 {
						speedUpdateCounter = 0

						bytesDiff := downloaded - lastBytes
						timeDiff := now.Sub(lastTime).Seconds()

						// Calculate current sample
						currentSpeed := float64(bytesDiff) / timeDiff

						// Add to samples for moving average (keep last 3 samples)
						speedSamples = append(speedSamples, currentSpeed)
						if len(speedSamples) > 3 {
							speedSamples = speedSamples[1:]
						}

						// Calculate average speed from samples
						speed = 0
						for _, s := range speedSamples {
							speed += s
						}
						speed /= float64(len(speedSamples))

						lastBytes = downloaded
						lastTime = now
					}
				} else if lastTime.IsZero() {
					lastBytes = downloaded
					lastTime = now
				}

				// Update state
				state.LastUpdated = now
				state.Progress = percent
				state.Current = downloaded
				state.Total = total
				state.Speed = speed

			case <-resp.Done:
				// Download completed or failed
				if err := resp.Err(); err != nil {
					// Handle download error
					state := dm.states[buildID]
					if state != nil {
						// Check if this was a cancellation
						if errors.Is(err, context.Canceled) {
							state.BuildState = model.StateCancelled
						} else {
							state.BuildState = model.StateFailed
							state.Progress = 0.0
						}
					}

					// Clean up partial download
					go func() {
						time.Sleep(500 * time.Millisecond) // Brief delay to allow UI update
						_ = os.RemoveAll(downloadPath)
					}()

					programCh <- downloadCompleteMsg{
						buildVersion: build.Version,
						err:          err,
					}
					return
				}

				// Download completed successfully, now proceed to extraction
				state := dm.states[buildID]
				if state != nil {
					state.BuildState = model.StateExtracting
					state.Progress = 0.0 // Reset progress for extraction phase
				}

				// Setup extraction progress callback
				extractionAdapter := func(downloadedBytes, totalBytes int64) {
					if totalBytes > 0 {
						// Convert to estimation progress (0.0-1.0)
						progress := float64(downloadedBytes) / float64(totalBytes)

						// Update state
						state := dm.states[buildID]
						if state == nil {
							return
						}

						select {
						case <-cancelCh:
							return
						default:
						}

						now := time.Now()
						state.LastUpdated = now
						state.Progress = progress
						state.Current = downloadedBytes
						state.Total = totalBytes
						state.BuildState = model.StateExtracting
					}
				}

				// Start extraction
				extractedPath, err := download.DownloadAndExtractBuild(build, dm.cfg.DownloadDir, extractionAdapter, cancelCh)

				// Update final state based on extraction result
				state = dm.states[buildID]
				if state == nil {
					return
				}

				if err != nil {
					// Check if this was a cancellation
					if errors.Is(err, download.ErrCancelled) {
						state.BuildState = model.StateCancelled
					} else {
						// Any other error should mark as failed
						state.BuildState = model.StateFailed
						state.Progress = 0.0
					}
				} else {
					state.BuildState = model.StateLocal
					state.Progress = 1.0
				}

				// Send completion message
				programCh <- downloadCompleteMsg{
					buildVersion:  build.Version,
					extractedPath: extractedPath,
					err:           err,
				}
				return

			case <-cancelCh:
				// Download was cancelled
				break downloadLoop
			}
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
	state.BuildState = model.StateCancelled
	state.Progress = 0.0 // Reset progress

	// Don't delete the state so we can track that it was cancelled
	// Keep it so it can be displayed with "Cancelled" status
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
		// Clean up download states, keeping only active ones
		newStates := make(map[string]*model.DownloadState)
		if c.downloads != nil && c.downloads.states != nil {
			for id, state := range c.downloads.states {
				// Only keep states that are actively in progress, discard terminal states like Failed/Cancelled.
				if state.BuildState == model.StateDownloading || state.BuildState == model.StateExtracting {
					newStates[id] = state
				}
			}
			c.downloads.states = newStates // Atomically replace the map
		}

		// Create API instance
		a := api.NewAPI()
		builds, err := a.FetchBuilds(c.cfg.VersionFilter, c.cfg.BuildType)
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

			// Determine status based *only* on comparison with local builds on disk
			if localBuild == nil {
				// Not found locally -> Online
				status = model.StateOnline
			} else {
				// Found locally -> Check for Update or just Local
				switch CheckUpdateAvailable(*localBuild, onlineBuild) {
				case model.StateUpdate:
					status = model.StateUpdate
				case model.StateLocal:
					status = model.StateLocal
				default: // Should not happen, default to Local
					status = model.StateLocal
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
