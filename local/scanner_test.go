package local

import (
	"TUI-Blender-Launcher/model"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFindBlenderExecutable(t *testing.T) {
	// Create a temporary directory for the test
	tempDir, err := os.MkdirTemp("", "blender-executable-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir) // Clean up at the end

	// Test cases
	testCases := []struct {
		name          string
		setup         func(string) string // Function to set up the test case, returns expected path
		expectedFound bool                // Whether we expect to find an executable
	}{
		{
			name: "blender at root",
			setup: func(dir string) string {
				path := filepath.Join(dir, "blender")
				createExecutableFile(t, path)
				return path
			},
			expectedFound: true,
		},
		{
			name: "blender.sh at root",
			setup: func(dir string) string {
				path := filepath.Join(dir, "blender.sh")
				createExecutableFile(t, path)
				return path
			},
			expectedFound: true,
		},
		{
			name: "blender in subdirectory",
			setup: func(dir string) string {
				subdir := filepath.Join(dir, "bin")
				err := os.Mkdir(subdir, 0755)
				if err != nil {
					t.Fatalf("Failed to create subdirectory: %v", err)
				}
				path := filepath.Join(subdir, "blender")
				createExecutableFile(t, path)
				return path
			},
			expectedFound: true,
		},
		{
			name: "no blender executable",
			setup: func(dir string) string {
				// Create a non-executable file
				path := filepath.Join(dir, "blender.txt")
				err := os.WriteFile(path, []byte("not an executable"), 0644)
				if err != nil {
					t.Fatalf("Failed to create non-executable file: %v", err)
				}
				return ""
			},
			expectedFound: false,
		},
	}

	// Run the test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup the test case
			testDir := filepath.Join(tempDir, tc.name)
			err := os.Mkdir(testDir, 0755)
			if err != nil {
				t.Fatalf("Failed to create test directory: %v", err)
			}

			expectedPath := tc.setup(testDir)

			// Call the function
			foundPath := findBlenderExecutable(testDir)

			// Check results
			if tc.expectedFound {
				if foundPath == "" {
					t.Errorf("Expected to find executable at %s, but none found", expectedPath)
				} else if foundPath != expectedPath {
					t.Errorf("Found incorrect executable: got %s, want %s", foundPath, expectedPath)
				}
			} else {
				if foundPath != "" {
					t.Errorf("Expected no executable to be found, but found %s", foundPath)
				}
			}
		})
	}
}

// Helper function to create an executable file
func createExecutableFile(t *testing.T, path string) {
	err := os.WriteFile(path, []byte("#!/bin/sh\necho 'mock blender'"), 0755)
	if err != nil {
		t.Fatalf("Failed to create executable file: %v", err)
	}
}

func TestReadBuildInfo(t *testing.T) {
	// Create a temporary directory for the test
	tempDir, err := os.MkdirTemp("", "blender-buildinfo-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir) // Clean up at the end

	// Test cases
	testCases := []struct {
		name           string
		dirName        string
		setupMetadata  bool                // Whether to create version.json
		metadataJSON   string              // JSON content for version.json
		expectedBuild  *model.BlenderBuild // Expected build info
		expectError    bool                // Whether an error is expected
		expectedStatus string              // Expected status in the build info
	}{
		{
			name:          "valid metadata file",
			dirName:       "blender-4.0.0-valid",
			setupMetadata: true,
			metadataJSON: `{
				"version": "4.0.0",
				"branch": "main",
				"hash": "abc123",
				"file_mtime": 1633046400,
				"url": "https://example.com/blender-4.0.0.tar.xz",
				"platform": "linux",
				"architecture": "x86_64",
				"file_size": 123456789,
				"file_name": "blender-4.0.0.tar.xz",
				"file_extension": "tar.xz",
				"release_cycle": "daily"
			}`,
			expectedBuild: &model.BlenderBuild{
				Version:         "4.0.0",
				Branch:          "main",
				Hash:            "abc123",
				BuildDate:       model.Timestamp(time.Unix(1633046400, 0)),
				DownloadURL:     "https://example.com/blender-4.0.0.tar.xz",
				OperatingSystem: "linux",
				Architecture:    "x86_64",
				Size:            123456789,
				FileName:        "blender-4.0.0-valid",
				FileExtension:   "tar.xz",
				ReleaseCycle:    "daily",
				Status:          "Local",
			},
			expectError:    false,
			expectedStatus: "Local",
		},
		{
			name:          "invalid JSON in metadata file",
			dirName:       "blender-4.0.0-invalid",
			setupMetadata: true,
			metadataJSON:  "{invalid json",
			expectedBuild: &model.BlenderBuild{
				Version:  "4.0.0",
				FileName: "blender-4.0.0-invalid",
				Status:   "Local",
			},
			expectError:    false,
			expectedStatus: "Local",
		},
		{
			name:          "fallback to directory name parsing",
			dirName:       "blender-3.6.0",
			setupMetadata: false,
			expectedBuild: &model.BlenderBuild{
				Version:  "3.6.0",
				FileName: "blender-3.6.0",
				Status:   "Local",
			},
			expectError:    false,
			expectedStatus: "Local",
		},
		{
			name:           "unrecognized directory name",
			dirName:        "some-random-dir",
			setupMetadata:  false,
			expectedBuild:  nil,
			expectError:    false,
			expectedStatus: "",
		},
	}

	// Run the test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup the test case
			testDir := filepath.Join(tempDir, tc.dirName)
			err := os.Mkdir(testDir, 0755)
			if err != nil {
				t.Fatalf("Failed to create test directory: %v", err)
			}

			if tc.setupMetadata {
				metadataPath := filepath.Join(testDir, versionMetaFilename)
				err := os.WriteFile(metadataPath, []byte(tc.metadataJSON), 0644)
				if err != nil {
					t.Fatalf("Failed to create metadata file: %v", err)
				}
			}

			// Call the function
			build, err := readBuildInfo(testDir)

			// Check error result
			if tc.expectError && err == nil {
				t.Errorf("Expected an error, but got nil")
			} else if !tc.expectError && err != nil {
				t.Errorf("Expected no error, but got: %v", err)
			}

			// Check build info result
			if tc.expectedBuild == nil {
				if build != nil {
					t.Errorf("Expected nil build info, but got: %+v", build)
				}
			} else {
				if build == nil {
					t.Errorf("Expected non-nil build info, but got nil")
				} else {
					// Check critical fields
					if build.Version != tc.expectedBuild.Version {
						t.Errorf("Version mismatch: got %s, want %s", build.Version, tc.expectedBuild.Version)
					}
					if build.Status != tc.expectedStatus {
						t.Errorf("Status mismatch: got %s, want %s", build.Status, tc.expectedStatus)
					}
					if build.FileName != tc.expectedBuild.FileName {
						t.Errorf("FileName mismatch: got %s, want %s", build.FileName, tc.expectedBuild.FileName)
					}
				}
			}
		})
	}
}

func TestScanLocalBuilds(t *testing.T) {
	// Create a temporary directory for the test
	tempDir, err := os.MkdirTemp("", "blender-scan-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir) // Clean up at the end

	// Create test build directories
	buildDirs := []struct {
		name    string
		version string
	}{
		{"blender-4.0.0", "4.0.0"},
		{"blender-3.6.2", "3.6.2"},
		{"blender-3.5.1", "3.5.1"},
		{".downloading", ""}, // Should be skipped
		{".oldbuilds", ""},   // Should be skipped
		{"not-a-build", ""},  // Should be skipped
	}

	for _, bd := range buildDirs {
		dirPath := filepath.Join(tempDir, bd.name)
		err := os.Mkdir(dirPath, 0755)
		if err != nil {
			t.Fatalf("Failed to create test directory %s: %v", bd.name, err)
		}

		// Create a version.json file for valid build directories
		if bd.version != "" {
			metadataJSON := fmt.Sprintf(`{
				"version": "%s",
				"branch": "main",
				"hash": "abc123",
				"file_mtime": 1633046400,
				"platform": "linux",
				"architecture": "x86_64",
				"file_name": "%s.tar.xz",
				"file_extension": "tar.xz",
				"release_cycle": "daily"
			}`, bd.version, bd.name)

			metadataPath := filepath.Join(dirPath, versionMetaFilename)
			err := os.WriteFile(metadataPath, []byte(metadataJSON), 0644)
			if err != nil {
				t.Fatalf("Failed to create metadata file: %v", err)
			}
		}
	}

	// Run the scan
	builds, err := ScanLocalBuilds(tempDir)
	if err != nil {
		t.Fatalf("ScanLocalBuilds returned an error: %v", err)
	}

	// Check that we got the right number of builds
	expectedCount := 3 // The number of valid build directories
	if len(builds) != expectedCount {
		t.Errorf("Expected %d builds, got %d", expectedCount, len(builds))
	}

	// Verify that the builds are sorted by version (descending)
	if len(builds) >= 3 {
		if builds[0].Version != "4.0.0" || builds[1].Version != "3.6.2" || builds[2].Version != "3.5.1" {
			t.Errorf("Builds not sorted correctly: %v, %v, %v",
				builds[0].Version, builds[1].Version, builds[2].Version)
		}
	}

	// Verify that all builds have "Local" status
	for _, build := range builds {
		if build.Status != "Local" {
			t.Errorf("Build %s has status %s, expected 'Local'", build.Version, build.Status)
		}
	}

	// Test non-existent directory
	nonExistentDir := filepath.Join(tempDir, "non-existent")
	nonExistentBuilds, nonExistentErr := ScanLocalBuilds(nonExistentDir)
	if nonExistentErr != nil {
		t.Errorf("Expected no error for non-existent dir, got: %v", nonExistentErr)
	}
	if len(nonExistentBuilds) != 0 {
		t.Errorf("Expected empty slice for non-existent dir, got %d builds", len(nonExistentBuilds))
	}
}

func TestDeleteBuild(t *testing.T) {
	// Create a temporary directory for the test
	tempDir, err := os.MkdirTemp("", "blender-delete-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir) // Clean up at the end

	// Create test build directories
	buildDirs := []struct {
		name    string
		version string
	}{
		{"blender-4.0.0", "4.0.0"},
		{"blender-3.6.2", "3.6.2"},
	}

	for _, bd := range buildDirs {
		dirPath := filepath.Join(tempDir, bd.name)
		err := os.Mkdir(dirPath, 0755)
		if err != nil {
			t.Fatalf("Failed to create test directory %s: %v", bd.name, err)
		}

		// Create a version.json file
		metadataJSON := fmt.Sprintf(`{
			"version": "%s",
			"branch": "main",
			"hash": "abc123",
			"file_mtime": 1633046400,
			"platform": "linux",
			"architecture": "x86_64",
			"file_name": "%s.tar.xz",
			"file_extension": "tar.xz",
			"release_cycle": "daily"
		}`, bd.version, bd.name)

		metadataPath := filepath.Join(dirPath, versionMetaFilename)
		err = os.WriteFile(metadataPath, []byte(metadataJSON), 0644)
		if err != nil {
			t.Fatalf("Failed to create metadata file: %v", err)
		}
	}

	// Test cases
	testCases := []struct {
		name          string
		version       string
		expectSuccess bool
	}{
		{"delete existing version", "4.0.0", true},
		{"delete non-existent version", "5.0.0", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Verify directory exists before delete if we expect success
			if tc.expectSuccess {
				dirToDelete := ""
				for _, bd := range buildDirs {
					if bd.version == tc.version {
						dirToDelete = filepath.Join(tempDir, bd.name)
						break
					}
				}
				if dirToDelete == "" {
					t.Fatalf("Test setup error: could not find directory for version %s", tc.version)
				}

				if _, err := os.Stat(dirToDelete); os.IsNotExist(err) {
					t.Fatalf("Test setup error: directory %s does not exist", dirToDelete)
				}
			}

			// Call the function
			success, err := DeleteBuild(tempDir, tc.version)
			if err != nil {
				t.Errorf("DeleteBuild returned an error: %v", err)
			}

			// Check success result
			if success != tc.expectSuccess {
				t.Errorf("Expected success=%v, got %v", tc.expectSuccess, success)
			}

			// Verify directory no longer exists if we expected success
			if tc.expectSuccess {
				dirToCheck := ""
				for _, bd := range buildDirs {
					if bd.version == tc.version {
						dirToCheck = filepath.Join(tempDir, bd.name)
						break
					}
				}

				if _, err := os.Stat(dirToCheck); !os.IsNotExist(err) {
					t.Errorf("Directory %s still exists after deletion", dirToCheck)
				}
			}
		})
	}
}

// Note: Tests for LaunchBlenderCmd and OpenDownloadDirCmd are more complex
// as they involve system calls and process execution. They might require
// more sophisticated mocking of OS functions.
