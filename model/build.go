package model

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// BuildState represents the current state of a Blender build
type BuildState int

const (
	// StateNone is the default state
	StateNone BuildState = iota
	// StateInstalled indicates the build is installed locally
	StateDownloading
	// StateExtracting indicates the build is being extracted
	StateExtracting
	// StateRunning indicates Blender is currently running
	StateLocal
	// StateOnline indicates the build is available online
	StateOnline
	// StateUpdate indicates a newer version is available online
	StateUpdate
	// StateFailed indicates a failed operation
	StateFailed
)

// String returns the string representation of the BuildState
func (s BuildState) String() string {
	switch s {
	case StateNone:
		return "Cancelled"
	case StateDownloading:
		return "Downloading"
	case StateExtracting:
		return "Extracting"
	case StateLocal:
		return "Local"
	case StateOnline:
		return "Online"
	case StateUpdate:
		return "Update"
	case StateFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}

// Timestamp is a custom type to handle Unix timestamp decoding from JSON numbers.
type Timestamp time.Time

// UnmarshalJSON implements the json.Unmarshaler interface for Timestamp.
func (t *Timestamp) UnmarshalJSON(b []byte) error {
	// Try to unmarshal as an integer (Unix timestamp)
	var timestamp int64
	if err := json.Unmarshal(b, &timestamp); err == nil {
		// It's a Unix timestamp (seconds)
		*t = Timestamp(time.Unix(timestamp, 0))
		return nil
	}

	// If not an integer, try a string with RFC3339 format
	var timeStr string
	if err := json.Unmarshal(b, &timeStr); err == nil {
		parsedTime, err := time.Parse(time.RFC3339, timeStr)
		if err == nil {
			*t = Timestamp(parsedTime)
			return nil
		}
	}

	// If neither worked, it might be an object, we'll use current time
	// This is a fallback to prevent breaking the whole program
	*t = Timestamp(time.Now())
	return nil
}

// MarshalJSON implements the json.Marshaler interface for Timestamp.
// This ensures the timestamp is properly saved in version.json as RFC3339 formatted string.
func (t Timestamp) MarshalJSON() ([]byte, error) {
	// Convert to RFC3339 string format for consistent serialization
	return json.Marshal(time.Time(t).Format(time.RFC3339))
}

// Time returns the underlying time.Time value.
// This provides convenience for using the value as a standard time.Time.
func (t Timestamp) Time() time.Time {
	return time.Time(t)
}

// BlenderBuild represents the structure of a single build entry from the API
// plus internal state for the TUI.
type BlenderBuild struct {
	// Fields from API
	Version         string    `json:"version"`
	Branch          string    `json:"branch"`
	Hash            string    `json:"hash"`           // Git commit hash short identifier
	BuildDate       Timestamp `json:"file_mtime"`     // Use custom Timestamp type
	DownloadURL     string    `json:"url"`            // URL for the specific file (can be build or checksum)
	OperatingSystem string    `json:"platform"`       // e.g., "linux", "windows", "macos"
	Architecture    string    `json:"architecture"`   // e.g., "amd64", "arm64"
	Size            int64     `json:"file_size"`      // File size in bytes
	FileName        string    `json:"file_name"`      // Full name of the downloadable file
	FileExtension   string    `json:"file_extension"` // e.g., "zip", "tar.gz", "sha256", "msi"
	ReleaseCycle    string    `json:"release_cycle"`  // e.g., "daily", "stable", "candidate" (replaces previous 'Type')

	// Internal state (not from API)
	Status BuildState // Changed from types.BuildState to BuildState
	// Selected field removed - we only work with highlighted builds now
}

// BlenderLaunchedMsg is sent when Blender is successfully launched
// This allows the UI to handle launched state appropriately
type BlenderLaunchedMsg struct {
	Version string // The version of Blender that was launched
}

// BlenderExecMsg is sent when Blender should be executed directly
// This will cause the TUI to exit and exec Blender in its place
type BlenderExecMsg struct {
	Version    string // The version of Blender to launch
	Executable string // The path to the Blender executable
}

// DownloadState holds progress info for an active download
type DownloadState struct {
	BuildID       string        // Unique identifier for build (version + hash)
	Progress      float64       // Progress from 0.0 to 1.0
	Current       int64         // Bytes downloaded so far (renamed from CurrentBytes)
	Total         int64         // Total bytes to download (renamed from TotalBytes)
	Speed         float64       // Download speed in bytes/sec
	BuildState    BuildState    // Changed from Message to BuildState
	LastUpdated   time.Time     // Timestamp of last progress update
	StartTime     time.Time     // When the download started
	CancelCh      chan struct{} // Per-download cancel channel
}

// FormatByteSize converts bytes to human-readable sizes
func FormatByteSize(bytes int64) string {
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

// FormatBuildDate formats a build date in yyyy-mm-dd-hh-mm format
func FormatBuildDate(t Timestamp) string {
	return t.Time().Format("2006-01-02-15:04")
}

// SortBuilds sorts the builds based on the selected column and sort order
func SortBuilds(builds []BlenderBuild, column int, reverse bool) []BlenderBuild {
	// Create a copy of builds to avoid modifying the original
	sortedBuilds := make([]BlenderBuild, len(builds))
	copy(sortedBuilds, builds)

	// Define sort function type for better organization
	type sortFunc func(a, b BlenderBuild) bool

	// Define the sort functions for each column based on the column index
	sortFuncs := map[int]sortFunc{
		0: func(a, b BlenderBuild) bool { // Version
			return a.Version < b.Version
		},
		1: func(a, b BlenderBuild) bool { // Status
			return a.Status < b.Status
		},
		2: func(a, b BlenderBuild) bool { // Branch
			return a.Branch < b.Branch
		},
		3: func(a, b BlenderBuild) bool { // Type/ReleaseCycle
			return a.ReleaseCycle < b.ReleaseCycle
		},
		4: func(a, b BlenderBuild) bool { // Hash
			return a.Hash < b.Hash
		},
		5: func(a, b BlenderBuild) bool { // Size
			return a.Size < b.Size
		},
		6: func(a, b BlenderBuild) bool { // Build Date
			return a.BuildDate.Time().Before(b.BuildDate.Time())
		},
	}

	// Check if we have a sort function for this column
	if sortFunc, ok := sortFuncs[column]; ok {
		sort.SliceStable(sortedBuilds, func(i, j int) bool {
			if reverse {
				return !sortFunc(sortedBuilds[i], sortedBuilds[j])
			}
			return sortFunc(sortedBuilds[i], sortedBuilds[j])
		})
	}

	return sortedBuilds
}
