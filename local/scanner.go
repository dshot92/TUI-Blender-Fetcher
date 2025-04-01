package local

import (
	"TUI-Blender-Launcher/model"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
)

const versionMetaFilename = "version.json" // Consistency with download package

var versionRegexFallback = regexp.MustCompile(`\b(\d+\.\d+(\.\d+)?)\b`) // More specific regex

// ReadBuildInfo tries to read version.json, falls back to parsing directory name.
// Exported version of readBuildInfo to be used from other packages.
func ReadBuildInfo(dirPath string) (*model.BlenderBuild, error) {
	metaPath := filepath.Join(dirPath, versionMetaFilename)
	build := &model.BlenderBuild{}

	if _, err := os.Stat(metaPath); err == nil {
		// version.json exists, try to read it
		data, err := os.ReadFile(metaPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to read %s: %v\n", metaPath, err)
			// Fall back to directory name parsing instead of returning error
			return fallbackToDirectoryParsing(dirPath)
		}
		if err := json.Unmarshal(data, build); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", metaPath, err)
			// Fall back to directory name parsing instead of returning error
			return fallbackToDirectoryParsing(dirPath)
		}
		build.Status = "Local"
		build.FileName = filepath.Base(dirPath)
		return build, nil
	}

	// version.json doesn't exist, fall back to regex parsing name
	return fallbackToDirectoryParsing(dirPath)
}

// Helper function to parse build info from directory name
func fallbackToDirectoryParsing(dirPath string) (*model.BlenderBuild, error) {
	dirName := filepath.Base(dirPath)
	match := versionRegexFallback.FindStringSubmatch(dirName)
	if len(match) > 1 {
		versionStr := match[1]
		build := &model.BlenderBuild{}
		build.Version = versionStr
		build.Status = "Local" // Indicate it lacks full metadata
		build.FileName = dirName
		return build, nil
	}

	// Didn't find metadata or parsable version in name
	return nil, nil // Return nil, nil to indicate not a recognized build dir
}

// ScanLocalBuilds scans the download directory for Blender installations using version.json or fallback name parsing.
func ScanLocalBuilds(downloadDir string) ([]model.BlenderBuild, error) {
	localBuilds := []model.BlenderBuild{}

	entries, err := os.ReadDir(downloadDir)
	if err != nil {
		if os.IsNotExist(err) {
			return localBuilds, nil
		}
		return nil, fmt.Errorf("failed to read download directory %s: %w", downloadDir, err)
	}

	for _, entry := range entries {
		// Skip temp download dir and oldbuilds dir
		if entry.IsDir() && entry.Name() != ".downloading" && entry.Name() != ".oldbuilds" {
			dirPath := filepath.Join(downloadDir, entry.Name())
			buildInfo, err := ReadBuildInfo(dirPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error processing directory %s: %v\n", dirPath, err)
				continue
			}
			if buildInfo != nil {
				localBuilds = append(localBuilds, *buildInfo)
			}
		}
	}

	// Sort local builds by version (descending)
	sort.Slice(localBuilds, func(i, j int) bool {
		// Basic version string comparison for now, consider semantic versioning later
		return localBuilds[i].Version > localBuilds[j].Version
	})

	return localBuilds, nil
}

// BuildLocalLookupMap creates a map of locally found versions (using version.json primarily).
func BuildLocalLookupMap(downloadDir string) (map[string]bool, error) {
	lookupMap := make(map[string]bool)

	entries, err := os.ReadDir(downloadDir)
	if err != nil {
		if os.IsNotExist(err) {
			return lookupMap, nil
		}
		return nil, fmt.Errorf("failed to read download directory %s: %w", downloadDir, err)
	}

	for _, entry := range entries {
		// Skip temp download dir and oldbuilds dir
		if entry.IsDir() && entry.Name() != ".downloading" && entry.Name() != ".oldbuilds" {
			dirPath := filepath.Join(downloadDir, entry.Name())
			buildInfo, err := ReadBuildInfo(dirPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error processing directory %s for map: %v\n", dirPath, err)
				continue
			}
			if buildInfo != nil {
				lookupMap[buildInfo.Version] = true
			}
		}
	}
	return lookupMap, nil
}

// DeleteBuild finds and deletes a local build by version.
// Returns true if successfully deleted, false if not found or error occurred.
func DeleteBuild(downloadDir string, version string) (bool, error) {
	entries, err := os.ReadDir(downloadDir)
	if err != nil {
		return false, fmt.Errorf("failed to read download directory %s: %w", downloadDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != ".downloading" {
			dirPath := filepath.Join(downloadDir, entry.Name())
			buildInfo, err := ReadBuildInfo(dirPath)
			if err != nil {
				// Error reading build info, but continue checking other directories
				continue
			}

			// Check if this is the build we want to delete
			if buildInfo != nil && buildInfo.Version == version {
				// Found the build to delete, remove the directory
				if err := os.RemoveAll(dirPath); err != nil {
					return false, fmt.Errorf("failed to delete build directory %s: %w", dirPath, err)
				}
				return true, nil
			}
		}
	}

	// Build not found
	return false, nil
}

// LaunchBlenderCmd creates a command to launch Blender for a specific version
func LaunchBlenderCmd(downloadDir string, version string) tea.Cmd {
	return func() tea.Msg {
		entries, err := os.ReadDir(downloadDir)
		if err != nil {
			return fmt.Errorf("failed to read download directory %s: %w", downloadDir, err)
		}

		for _, entry := range entries {
			if entry.IsDir() && entry.Name() != ".downloading" {
				dirPath := filepath.Join(downloadDir, entry.Name())
				buildInfo, err := ReadBuildInfo(dirPath)
				if err != nil {
					// Error reading build info, but continue checking other directories
					continue
				}

				// Check if this is the build we want to launch
				if buildInfo != nil && buildInfo.Version == version {
					// Find the blender executable
					blenderExe := findBlenderExecutable(dirPath)
					if blenderExe == "" {
						return fmt.Errorf("could not find Blender executable in %s", dirPath)
					}

					// Return a message to the TUI to exit gracefully and run Blender
					// This will allow Blender to take over the terminal completely
					return model.BlenderExecMsg{
						Version:    version,
						Executable: blenderExe,
					}
				}
			}
		}

		return fmt.Errorf("Blender version %s not found", version)
	}
}

// OpenDownloadDirCmd creates a command to open the download directory
func OpenDownloadDirCmd(downloadDir string) tea.Cmd {
	return func() tea.Msg {
		// Create the directory if it doesn't exist
		if _, err := os.Stat(downloadDir); os.IsNotExist(err) {
			if err := os.MkdirAll(downloadDir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", downloadDir, err)
			}
		}

		// Launch the file explorer
		if err := openFileExplorer(downloadDir); err != nil {
			return fmt.Errorf("failed to open directory: %w", err)
		}

		return nil // Success, no message needed
	}
}

// OpenDirCmd creates a command to open any directory
func OpenDirCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		// Create the directory if it doesn't exist
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, err)
			}
		}

		// Launch the file explorer
		if err := openFileExplorer(dir); err != nil {
			return fmt.Errorf("failed to open directory: %w", err)
		}

		return nil // Success, no message needed
	}
}

// findBlenderExecutable locates the Blender executable in the installation directory
func findBlenderExecutable(installDir string) string {
	// First, check for common locations based on platform
	// Linux: blender or blender.sh
	linuxCandidates := []string{
		filepath.Join(installDir, "blender"),
		filepath.Join(installDir, "blender.sh"),
	}

	for _, candidate := range linuxCandidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// If not found in common locations, search recursively
	var result string
	filepath.Walk(installDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip and continue
		}

		if !info.IsDir() && (info.Name() == "blender" || info.Name() == "blender.sh") {
			// Check if it's executable
			if info.Mode()&0111 != 0 {
				result = path
				return filepath.SkipDir // Stop walking once found
			}
		}
		return nil
	})

	return result
}

// OpenFileExplorer opens the default file explorer to the specified directory
// Exported version of openFileExplorer to be used from other packages.
func OpenFileExplorer(dir string) error {
	// For Linux, try xdg-open first
	var cmd *exec.Cmd
	if _, err := exec.LookPath("xdg-open"); err == nil {
		cmd = exec.Command("xdg-open", dir)
	} else if _, err := exec.LookPath("gnome-open"); err == nil {
		cmd = exec.Command("gnome-open", dir)
	} else if _, err := exec.LookPath("kde-open"); err == nil {
		cmd = exec.Command("kde-open", dir)
	} else {
		// Fallback: Just try xdg-open anyway
		cmd = exec.Command("xdg-open", dir)
	}

	cmd.Stdout = nil
	cmd.Stderr = nil

	// Detach process from terminal
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	return cmd.Start()
}

// openFileExplorer is a wrapper around OpenFileExplorer for backward compatibility
func openFileExplorer(dir string) error {
	return OpenFileExplorer(dir)
}

// DeleteAllOldBuilds removes all contents of the .oldbuilds directory
func DeleteAllOldBuilds(downloadDir string) error {
	oldBuildsDir := filepath.Join(downloadDir, ".oldbuilds")

	// Check if the directory exists first
	if _, err := os.Stat(oldBuildsDir); os.IsNotExist(err) {
		// Directory doesn't exist, nothing to do
		return nil
	}

	// Remove all contents of the .oldbuilds directory
	entries, err := os.ReadDir(oldBuildsDir)
	if err != nil {
		return fmt.Errorf("failed to read .oldbuilds directory: %w", err)
	}

	for _, entry := range entries {
		entryPath := filepath.Join(oldBuildsDir, entry.Name())
		if err := os.RemoveAll(entryPath); err != nil {
			return fmt.Errorf("failed to remove old build %s: %w", entry.Name(), err)
		}
	}

	return nil
}

// GetOldBuildsInfo returns information about backed up builds
func GetOldBuildsInfo(downloadDir string) (int, int64, error) {
	oldBuildsDir := filepath.Join(downloadDir, ".oldbuilds")

	// Check if the directory exists first
	if _, err := os.Stat(oldBuildsDir); os.IsNotExist(err) {
		// Directory doesn't exist
		return 0, 0, nil
	}

	// Get list of old builds
	entries, err := os.ReadDir(oldBuildsDir)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read .oldbuilds directory: %w", err)
	}

	count := len(entries)
	var totalSize int64 = 0

	// Calculate total size
	for _, entry := range entries {
		entryPath := filepath.Join(oldBuildsDir, entry.Name())
		size, err := getDirSize(entryPath)
		if err != nil {
			// Just log the error but continue
			fmt.Fprintf(os.Stderr, "Error calculating size of %s: %v\n", entryPath, err)
			continue
		}
		totalSize += size
	}

	return count, totalSize, nil
}

// Helper function to calculate directory size
func getDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// CheckUpdateAvailable determines if there's an update available for a local build
// by comparing the build dates of local and online versions.
func CheckUpdateAvailable(localBuild, onlineBuild model.BlenderBuild) bool {
	// If versions don't match, this is not an update for this build
	if localBuild.Version != onlineBuild.Version {
		return false
	}

	// If local build has no build date, consider it outdated
	if localBuild.BuildDate.Time().IsZero() {
		return true
	}

	// If online build has no build date, don't consider it an update
	if onlineBuild.BuildDate.Time().IsZero() {
		return false
	}

	// Compare build dates - if online is newer, an update is available
	return onlineBuild.BuildDate.Time().After(localBuild.BuildDate.Time())
}

// wrapper function for backward compatibility
func readBuildInfo(dirPath string) (*model.BlenderBuild, error) {
	return ReadBuildInfo(dirPath)
}
