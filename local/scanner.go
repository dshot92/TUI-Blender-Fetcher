package local

import (
	"TUI-Blender-Launcher/model"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
)

const versionMetaFilename = "version.json"

// ReadBuildInfo reads build information from version.json in the given directory.
// Returns nil if version.json does not exist.
func ReadBuildInfo(dirPath string) (*model.BlenderBuild, error) {
	metaPath := filepath.Join(dirPath, versionMetaFilename)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read %s: %w", metaPath, err)
	}

	var build model.BlenderBuild
	if err := json.Unmarshal(data, &build); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", metaPath, err)
	}
	build.Status = model.StateLocal
	build.FileName = filepath.Base(dirPath)
	return &build, nil
}

// ScanLocalBuilds scans the download directory for local Blender builds using version.json.
func ScanLocalBuilds(downloadDir string) ([]model.BlenderBuild, error) {
	var localBuilds []model.BlenderBuild
	entries, err := os.ReadDir(downloadDir)
	if err != nil {
		if os.IsNotExist(err) {
			return localBuilds, nil
		}
		return nil, fmt.Errorf("failed to read download directory %s: %w", downloadDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != ".oldbuilds" {
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

	sort.Slice(localBuilds, func(i, j int) bool {
		return localBuilds[i].Version > localBuilds[j].Version
	})

	return localBuilds, nil
}

// BuildLocalLookupMap creates a map of available local build versions.
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
		if entry.IsDir() && entry.Name() != ".oldbuilds" {
			dirPath := filepath.Join(downloadDir, entry.Name())
			buildInfo, err := ReadBuildInfo(dirPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error processing directory %s: %v\n", dirPath, err)
				continue
			}
			if buildInfo != nil {
				lookupMap[buildInfo.Version] = true
			}
		}
	}

	return lookupMap, nil
}

// DeleteBuild finds and deletes a local build by version. Returns true if deletion was successful.
func DeleteBuild(downloadDir string, version string) (bool, error) {
	entries, err := os.ReadDir(downloadDir)
	if err != nil {
		return false, fmt.Errorf("failed to read download directory %s: %w", downloadDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			dirPath := filepath.Join(downloadDir, entry.Name())
			buildInfo, err := ReadBuildInfo(dirPath)
			if err != nil {
				continue
			}
			if buildInfo != nil && buildInfo.Version == version {
				if err := os.RemoveAll(dirPath); err != nil {
					return false, fmt.Errorf("failed to delete build directory %s: %w", dirPath, err)
				}
				return true, nil
			}
		}
	}

	return false, nil
}

// LaunchBlenderCmd creates a command to launch Blender for a specific version.
func LaunchBlenderCmd(downloadDir string, version string) tea.Cmd {
	return func() tea.Msg {
		entries, err := os.ReadDir(downloadDir)
		if err != nil {
			return fmt.Errorf("failed to read download directory %s: %w", downloadDir, err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				dirPath := filepath.Join(downloadDir, entry.Name())
				buildInfo, err := ReadBuildInfo(dirPath)
				if err != nil {
					continue
				}
				if buildInfo != nil && buildInfo.Version == version {
					blenderExe := findBlenderExecutable(dirPath)
					if blenderExe == "" {
						return fmt.Errorf("could not find Blender executable in %s", dirPath)
					}
					return model.BlenderExecMsg{
						Version:    version,
						Executable: blenderExe,
					}
				}
			}
		}

		return fmt.Errorf("blender version %s not found", version)
	}
}

// OpenDownloadDirCmd creates a command to open the download directory.
func OpenDownloadDirCmd(downloadDir string) tea.Cmd {
	return func() tea.Msg {
		if _, err := os.Stat(downloadDir); os.IsNotExist(err) {
			if err := os.MkdirAll(downloadDir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", downloadDir, err)
			}
		}

		if err := openFileExplorer(downloadDir); err != nil {
			return fmt.Errorf("failed to open directory: %w", err)
		}

		return nil
	}
}

// OpenDirCmd creates a command to open any directory.
func OpenDirCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, err)
			}
		}

		if err := openFileExplorer(dir); err != nil {
			return fmt.Errorf("failed to open directory: %w", err)
		}

		return nil
	}
}

// findBlenderExecutable locates the Blender executable in the installation directory.
func findBlenderExecutable(installDir string) string {
	var candidate string
	switch runtime.GOOS {
	case "windows":
		candidate = filepath.Join(installDir, "blender-launcher.exe")
	case "linux":
		candidate = filepath.Join(installDir, "blender")
	default:
		candidate = filepath.Join(installDir, "blender")
	}

	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return ""
}

// OpenFileExplorer opens the default file explorer to the specified directory.
func OpenFileExplorer(dir string) error {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("explorer", dir)
	} else {
		if _, err := exec.LookPath("xdg-open"); err == nil {
			cmd = exec.Command("xdg-open", dir)
		} else if _, err := exec.LookPath("gnome-open"); err == nil {
			cmd = exec.Command("gnome-open", dir)
		} else if _, err := exec.LookPath("kde-open"); err == nil {
			cmd = exec.Command("kde-open", dir)
		} else {
			cmd = exec.Command("xdg-open", dir)
		}
	}

	cmd.Stdout = nil
	cmd.Stderr = nil

	// Detach the process (implementation provided elsewhere)
	detachProcess(cmd)

	return cmd.Start()
}

// openFileExplorer is a simple wrapper for OpenFileExplorer.
func openFileExplorer(dir string) error {
	return OpenFileExplorer(dir)
}
