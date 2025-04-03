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
// const blenderAPIURL = "https://builder.blender.org/download/experimental/?format=json&v=1"

// const blenderAPIURL = "https://builder.blender.org/download/patch/?format=json&v=1"

const blenderAPIURL = "https://builder.blender.org/download/daily/?format=json&v=1"

// API represents the Blender API client
type API struct {
	client *http.Client
}

// NewAPI creates a new API client
func NewAPI() *API {
	return &API{
		client: &http.Client{},
	}
}

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
	// Initialize apiArch explicitly within the switch block below
	var apiArch string

	// Map architecture names from Go runtime format (GOARCH) to Blender API format.
	// GOOS values (linux, windows, darwin) match the API 'platform' field directly.
	switch currentOS {
	case "linux":
		switch currentArch {
		case "amd64":
			apiArch = "x86_64" // Map Go's amd64 to API's x86_64
		case "arm64":
			// Assuming API uses "arm64" for Linux ARM (like other OS).
			// Verified data did not contain Linux ARM builds from this endpoint.
			// Adjust if other endpoints use "aarch64" or similar for Linux ARM.
			apiArch = "arm64"
		default:
			// For unknown/unsupported arch, use Go's name; will likely be filtered out later.
			apiArch = currentArch
		}
	case "darwin": // macOS
		switch currentArch {
		case "amd64":
			apiArch = "x86_64" // Map Go's amd64 to API's x86_64
		case "arm64":
			apiArch = "arm64" // Go's arm64 matches API's arm64
		default:
			apiArch = currentArch
		}
	case "windows":
		switch currentArch {
		case "amd64":
			apiArch = "amd64" // Go's amd64 matches API's amd64
		case "arm64":
			apiArch = "arm64" // Go's arm64 matches API's arm64
		default:
			apiArch = currentArch
		}
	default:
		// For unknown OS, use Go's arch name; OS filter check later will handle it.
		apiArch = currentArch
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
		// Check Arch: Use the explicitly mapped apiArch
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
		build.Status = model.StateOnline
		filteredBuilds = append(filteredBuilds, build)
	}

	// DEBUG: Duplicate builds multiple times for testing
	// duplicatedBuilds := make([]model.BlenderBuild, 0, len(filteredBuilds)*5)
	// for i := 0; i < 5; i++ {
	// 	for _, build := range filteredBuilds {
	// 		// Create a copy of the build to avoid pointer issues
	// 		buildCopy := build
	// 		// Add a suffix to make each copy unique
	// 		buildCopy.Version = fmt.Sprintf("%s (Copy %d)", build.Version, i+1)
	// 		duplicatedBuilds = append(duplicatedBuilds, buildCopy)
	// 	}
	// }
	// filteredBuilds = duplicatedBuilds

	return filteredBuilds, nil
}
