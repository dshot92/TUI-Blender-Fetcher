package model

import (
	"encoding/json"
	"fmt"
	"time"
)

// Timestamp is a custom type to handle Unix timestamp decoding from JSON numbers.
type Timestamp time.Time

// UnmarshalJSON implements the json.Unmarshaler interface for Timestamp.
func (t *Timestamp) UnmarshalJSON(b []byte) error {
	var timestamp int64
	// Try to unmarshal as an integer (Unix timestamp)
	if err := json.Unmarshal(b, &timestamp); err != nil {
		// If it's not an integer, maybe it's a quoted string timestamp?
		// Fallback or error handling could be added here if needed.
		// For now, we assume it must be a number based on the API example.
		return fmt.Errorf("timestamp field is not a valid number: %v", err)
	}
	// Convert Unix timestamp (seconds) to time.Time
	*t = Timestamp(time.Unix(timestamp, 0))
	return nil
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
	Status   string // e.g., "Online", "Downloading", "Downloaded", "Update Available", "Error"
	Selected bool   // Whether the user has selected this build
	// Add download progress fields later if needed
}
