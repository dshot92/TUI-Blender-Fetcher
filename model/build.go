package model

import (
	"TUI-Blender-Launcher/types"
	"encoding/json"
	"time"
)

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
	Status types.BuildState // Changed from string to types.BuildState
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
