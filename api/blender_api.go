package api

import (
	"TUI-Blender-Launcher/model"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
)

// API endpoint
const blenderAPIURL = "https://builder.blender.org/download/daily/?format=json&v=1"

// FetchBuilds fetches the list of Blender builds from the official API,
// filtering for the current OS and architecture, and excluding checksum files.
func FetchBuilds() ([]model.BlenderBuild, error) {
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

	currentOS := runtime.GOOS
	currentArch := runtime.GOARCH

	// Map Go architecture names to API architecture names if needed
	apiArch := currentArch
	if currentOS == "linux" && currentArch == "amd64" {
		apiArch = "x86_64" // Map amd64 to x86_64 for Linux API calls
	}
	// Add other mappings if necessary (e.g., for darwin/arm64)

	// Define allowed file extensions (common archive/installer types)
	allowedExtensions := map[string]bool{
		"zip":     true,
		"tar.gz":  true,
		"tar.xz":  true,
		"tar.bz2": true,
		"xz":      true, // Add xz based on log output for Linux
		"dmg":     true,
		"pkg":     true,
		"msi":     true,
		"msix":    true,
	}

	var filteredBuilds []model.BlenderBuild
	for _, build := range allBuildEntries {
		// Check OS match (using Go's runtime.GOOS directly)
		if build.OperatingSystem != currentOS {
			continue
		}

		// Check Arch match (using the mapped API architecture name)
		if build.Architecture != apiArch {
			continue
		}

		// Check if the file extension is one of the allowed archive/installer types
		ext := strings.ToLower(build.FileExtension)
		if _, ok := allowedExtensions[ext]; !ok {
			continue
		}

		build.Status = "Online"
		build.Selected = false
		filteredBuilds = append(filteredBuilds, build)
	}

	// TODO: Sort builds (e.g., by date or version)

	return filteredBuilds, nil
}
