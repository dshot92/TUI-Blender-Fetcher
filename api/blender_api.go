package api

import (
	"TUI-Blender-Launcher/model"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"

	version "github.com/hashicorp/go-version" // Import version library
)

// API endpoint
const blenderAPIURL = "https://builder.blender.org/download/daily/?format=json&v=1"

// FetchBuilds fetches the list of Blender builds from the official API,
// filtering for the current OS/architecture, file extensions, and minimum version.
func FetchBuilds(versionFilter string) ([]model.BlenderBuild, error) { // Added versionFilter param
	resp, err := http.Get(blenderAPIURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch data: status code %d", resp.StatusCode)
	}

	var allBuildEntries []model.BlenderBuild
	if err := json.NewDecoder(resp.Body).Decode(&allBuildEntries); err != nil {
		return nil, fmt.Errorf("failed to decode JSON (check API response structure): %w", err)
	}

	// --- Filtering Setup ---
	currentOS := runtime.GOOS
	currentArch := runtime.GOARCH
	apiArch := currentArch
	if currentOS == "linux" && currentArch == "amd64" {
		apiArch = "x86_64"
	}

	allowedExtensions := map[string]bool{
		"zip": true, "tar.gz": true, "tar.xz": true, "tar.bz2": true,
		"xz": true, "dmg": true, "pkg": true, "msi": true, "msix": true,
	}

	// Parse the version filter if provided
	var minVersion *version.Version
	if versionFilter != "" {
		minVersion, err = version.NewVersion(versionFilter)
		if err != nil {
			// Handle invalid filter format - maybe log and ignore?
			// For now, return error to notify user via TUI
			return nil, fmt.Errorf("invalid version filter format '%s': %w", versionFilter, err)
		}
	}

	// --- Filtering Loop ---
	var filteredBuilds []model.BlenderBuild
	for _, build := range allBuildEntries {
		// Check OS
		if build.OperatingSystem != currentOS {
			continue
		}
		// Check Arch
		if build.Architecture != apiArch {
			continue
		}
		// Check Extension
		ext := strings.ToLower(build.FileExtension)
		if _, ok := allowedExtensions[ext]; !ok {
			continue
		}

		// Check Version Filter
		if minVersion != nil {
			buildVersion, err := version.NewVersion(build.Version)
			if err != nil {
				// Skip builds with unparseable versions if filter is active
				continue
			}
			if buildVersion.LessThan(minVersion) {
				continue // Skip if build version is less than filter
			}
		}

		// Passed all filters
		build.Status = "Online"
		build.Selected = false
		filteredBuilds = append(filteredBuilds, build)
	}

	// TODO: Sort builds (e.g., by date or version)

	return filteredBuilds, nil
}
