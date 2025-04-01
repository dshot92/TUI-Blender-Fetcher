package local

import (
	"TUI-Blender-Launcher/model"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

const versionMetaFilename = "version.json" // Consistency with download package

var versionRegexFallback = regexp.MustCompile(`\b(\d+\.\d+(\.\d+)?)\b`) // More specific regex

// Tries to read version.json, falls back to parsing directory name.
func readBuildInfo(dirPath string) (*model.BlenderBuild, error) {
	metaPath := filepath.Join(dirPath, versionMetaFilename)
	build := &model.BlenderBuild{}

	if _, err := os.Stat(metaPath); err == nil {
		// version.json exists, try to read it
		data, err := os.ReadFile(metaPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to read %s: %v\n", metaPath, err)
			return nil, fmt.Errorf("read %s failed: %w", metaPath, err)
		}
		if err := json.Unmarshal(data, build); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", metaPath, err)
			return nil, fmt.Errorf("parse %s failed: %w", metaPath, err)
		}
		build.Status = "Downloaded"
		build.FileName = filepath.Base(dirPath)
		return build, nil
	}

	// version.json doesn't exist, fall back to regex parsing name
	dirName := filepath.Base(dirPath)
	match := versionRegexFallback.FindStringSubmatch(dirName)
	if len(match) > 1 {
		versionStr := match[1]
		build.Version = versionStr
		build.Status = "Downloaded (legacy)" // Indicate it lacks full metadata
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
		if entry.IsDir() && entry.Name() != ".downloading" { // Skip temp download dir
			dirPath := filepath.Join(downloadDir, entry.Name())
			buildInfo, err := readBuildInfo(dirPath)
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
		if entry.IsDir() && entry.Name() != ".downloading" {
			dirPath := filepath.Join(downloadDir, entry.Name())
			buildInfo, err := readBuildInfo(dirPath)
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
