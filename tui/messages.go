package tui

import (
	"TUI-Blender-Launcher/model"
	"time"
)

// Define messages for communication between components
// Group related message types together
type (
	// Data update messages
	buildsFetchedMsg struct { // Online builds fetched
		builds []model.BlenderBuild
		err    error // Add error field
	}
	localBuildsScannedMsg struct { // Initial local scan complete
		builds []model.BlenderBuild
		err    error // Include error from scanning
	}
	buildsUpdatedMsg struct { // Builds list updated (e.g., status change)
		builds []model.BlenderBuild
	}

	// Action messages
	startDownloadMsg struct { // Request to start download for a build
		build   model.BlenderBuild
		buildID string // Added unique build identifier
	}
	downloadCompleteMsg struct { // Download & extraction finished
		buildVersion  string // Version of the build that finished
		extractedPath string
		err           error
	}
	// Error message
	errMsg struct{ err error }

	// Timer message
	tickMsg time.Time

	// UI refresh message
	forceRenderMsg struct{} // Message used just to force UI rendering
)

// Implement the error interface for errMsg
func (e errMsg) Error() string { return e.err.Error() }
