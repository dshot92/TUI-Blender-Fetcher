package model

import (
	"encoding/json"
	"testing"
	"time"

	"TUI-Blender-Launcher/types"
)

func TestTimestampUnmarshalJSON(t *testing.T) {
	// Test cases
	testCases := []struct {
		name         string
		jsonData     string
		expectedTime time.Time
		expectError  bool
	}{
		{
			name:         "unix timestamp",
			jsonData:     `1633046400`,
			expectedTime: time.Unix(1633046400, 0),
			expectError:  false,
		},
		{
			name:         "string RFC3339",
			jsonData:     `"2021-10-01T00:00:00Z"`,
			expectedTime: time.Date(2021, 10, 1, 0, 0, 0, 0, time.UTC),
			expectError:  false,
		},
		{
			name:        "invalid format",
			jsonData:    `"not a timestamp"`,
			expectError: false, // Should not error, fallback to now
		},
		{
			name:        "non-time object",
			jsonData:    `{"some": "object"}`,
			expectError: false, // Should not error, fallback to now
		},
	}

	// Test each case
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var timestamp Timestamp

			// Unmarshal
			err := json.Unmarshal([]byte(tc.jsonData), &timestamp)

			// Check error result
			if tc.expectError && err == nil {
				t.Error("Expected an error, but got nil")
			} else if !tc.expectError && err != nil {
				t.Errorf("Expected no error, but got: %v", err)
			}

			// For valid cases, check the time value
			if !tc.expectError && err == nil && tc.expectedTime.Unix() > 0 {
				tsTime := time.Time(timestamp)
				if tsTime.Unix() != tc.expectedTime.Unix() {
					t.Errorf("Expected time %v, got %v", tc.expectedTime, tsTime)
				}
			}

			// For invalid cases that don't error, check we have a non-zero timestamp
			if !tc.expectError && err == nil && tc.expectedTime.Unix() == 0 {
				tsTime := time.Time(timestamp)
				if tsTime.IsZero() {
					t.Error("Expected non-zero time for invalid format, got zero time")
				}
			}
		})
	}
}

func TestTimestampMarshalJSON(t *testing.T) {
	// Test cases
	testCases := []struct {
		name         string
		timestamp    Timestamp
		expectedJSON string
	}{
		{
			name:         "normal time",
			timestamp:    Timestamp(time.Date(2021, 10, 1, 0, 0, 0, 0, time.UTC)),
			expectedJSON: `"2021-10-01T00:00:00Z"`,
		},
		{
			name:         "zero time",
			timestamp:    Timestamp(time.Time{}),
			expectedJSON: `"0001-01-01T00:00:00Z"`,
		},
	}

	// Test each case
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Marshal
			jsonData, err := json.Marshal(tc.timestamp)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Check the marshaled data
			if string(jsonData) != tc.expectedJSON {
				t.Errorf("Expected JSON %s, got %s", tc.expectedJSON, string(jsonData))
			}

			// Test round-trip
			var unmarshaledTimestamp Timestamp
			err = json.Unmarshal(jsonData, &unmarshaledTimestamp)
			if err != nil {
				t.Fatalf("Unexpected error in round-trip: %v", err)
			}

			// Compare times (allow 1 second difference for rounding)
			originalTime := time.Time(tc.timestamp)
			unmarshaledTime := time.Time(unmarshaledTimestamp)
			if originalTime.Sub(unmarshaledTime).Abs() > time.Second {
				t.Errorf("Round-trip failed. Original: %v, After: %v", originalTime, unmarshaledTime)
			}
		})
	}
}

func TestTimestampTime(t *testing.T) {
	// Create a test timestamp
	testTime := time.Date(2021, 10, 1, 0, 0, 0, 0, time.UTC)
	timestamp := Timestamp(testTime)

	// Get time.Time value
	timeValue := timestamp.Time()

	// Check the result
	if timeValue != testTime {
		t.Errorf("Expected time %v, got %v", testTime, timeValue)
	}
}

func TestBlenderBuildJsonMarshaling(t *testing.T) {
	// Create a test build
	build := BlenderBuild{
		Version:         "4.0.0",
		Branch:          "main",
		Hash:            "abc123",
		BuildDate:       Timestamp(time.Date(2021, 10, 1, 0, 0, 0, 0, time.UTC)),
		DownloadURL:     "https://example.com/blender-4.0.0.tar.xz",
		OperatingSystem: "linux",
		Architecture:    "x86_64",
		Size:            123456789,
		FileName:        "blender-4.0.0.tar.xz",
		FileExtension:   "tar.xz",
		ReleaseCycle:    "daily",
		Status:          types.StateOnline,
	}

	// Marshal
	jsonData, err := json.Marshal(build)
	if err != nil {
		t.Fatalf("Failed to marshal BlenderBuild: %v", err)
	}

	// Unmarshal into a new build
	var unmarshaled BlenderBuild
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal BlenderBuild: %v", err)
	}

	// Compare fields
	if build.Version != unmarshaled.Version {
		t.Errorf("Version mismatch: got %s, want %s", unmarshaled.Version, build.Version)
	}
	if build.Branch != unmarshaled.Branch {
		t.Errorf("Branch mismatch: got %s, want %s", unmarshaled.Branch, build.Branch)
	}
	if build.Hash != unmarshaled.Hash {
		t.Errorf("Hash mismatch: got %s, want %s", unmarshaled.Hash, build.Hash)
	}
	if time.Time(build.BuildDate).Format(time.RFC3339) != time.Time(unmarshaled.BuildDate).Format(time.RFC3339) {
		t.Errorf("BuildDate mismatch: got %v, want %v",
			time.Time(unmarshaled.BuildDate), time.Time(build.BuildDate))
	}
	if build.DownloadURL != unmarshaled.DownloadURL {
		t.Errorf("DownloadURL mismatch: got %s, want %s", unmarshaled.DownloadURL, build.DownloadURL)
	}
	if build.OperatingSystem != unmarshaled.OperatingSystem {
		t.Errorf("OperatingSystem mismatch: got %s, want %s", unmarshaled.OperatingSystem, build.OperatingSystem)
	}
	if build.Architecture != unmarshaled.Architecture {
		t.Errorf("Architecture mismatch: got %s, want %s", unmarshaled.Architecture, build.Architecture)
	}
	if build.Size != unmarshaled.Size {
		t.Errorf("Size mismatch: got %d, want %d", unmarshaled.Size, build.Size)
	}
	if build.FileName != unmarshaled.FileName {
		t.Errorf("FileName mismatch: got %s, want %s", unmarshaled.FileName, build.FileName)
	}
	if build.FileExtension != unmarshaled.FileExtension {
		t.Errorf("FileExtension mismatch: got %s, want %s", unmarshaled.FileExtension, build.FileExtension)
	}
	if build.ReleaseCycle != unmarshaled.ReleaseCycle {
		t.Errorf("ReleaseCycle mismatch: got %s, want %s", unmarshaled.ReleaseCycle, build.ReleaseCycle)
	}
	// Status is not included in JSON, so it will be empty in unmarshaled
}
