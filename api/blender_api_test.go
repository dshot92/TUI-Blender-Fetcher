package api

import (
	"TUI-Blender-Launcher/model"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchBuilds(t *testing.T) {
	// Set up a test HTTP client and server
	originalClient := http.DefaultClient
	defer func() { http.DefaultClient = originalClient }()

	// Setup a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check the request method and path
		if r.Method != http.MethodGet {
			t.Errorf("Expected GET request, got %s", r.Method)
		}

		// Return a mock JSON response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `[
			{
				"version": "4.0.0",
				"branch": "main",
				"hash": "abc123",
				"file_mtime": 1633046400,
				"url": "https://example.com/blender-4.0.0-linux-x86_64.tar.xz",
				"platform": "linux",
				"architecture": "x86_64",
				"file_size": 123456789,
				"file_name": "blender-4.0.0-linux-x86_64.tar.xz",
				"file_extension": "tar.xz",
				"release_cycle": "daily"
			},
			{
				"version": "3.6.2",
				"branch": "main",
				"hash": "def456",
				"file_mtime": 1633046300,
				"url": "https://example.com/blender-3.6.2-linux-x86_64.tar.xz",
				"platform": "linux",
				"architecture": "x86_64", 
				"file_size": 123456789,
				"file_name": "blender-3.6.2-linux-x86_64.tar.xz",
				"file_extension": "tar.xz",
				"release_cycle": "daily"
			},
			{
				"version": "3.5.1",
				"branch": "main",
				"hash": "ghi789",
				"file_mtime": 1633046200,
				"url": "https://example.com/blender-3.5.1-windows-x86_64.zip",
				"platform": "windows",
				"architecture": "x86_64",
				"file_size": 123456789,
				"file_name": "blender-3.5.1-windows-x86_64.zip",
				"file_extension": "zip",
				"release_cycle": "daily"
			},
			{
				"version": "4.0.0",
				"branch": "main",
				"hash": "jkl012",
				"file_mtime": 1633046100,
				"url": "https://example.com/blender-4.0.0-linux-x86_64.sha256",
				"platform": "linux",
				"architecture": "x86_64",
				"file_size": 123,
				"file_name": "blender-4.0.0-linux-x86_64.sha256",
				"file_extension": "sha256",
				"release_cycle": "daily"
			}
		]`)
	}))
	defer server.Close()

	// Create a custom client that redirects requests to our test server
	http.DefaultClient = &http.Client{
		Transport: &mockTransport{
			apiURL: blenderAPIURL,
			server: server,
		},
	}

	// Test cases
	testCases := []struct {
		name            string
		versionFilter   string
		expectError     bool
		expectedCount   int
		checkFirstBuild func(*testing.T, model.BlenderBuild)
	}{
		{
			name:          "no filter",
			versionFilter: "",
			expectError:   false,
			expectedCount: 2, // Only the two Linux x86_64 with allowed extensions
			checkFirstBuild: func(t *testing.T, build model.BlenderBuild) {
				// First build should be the 4.0.0 Linux build
				if build.Version != "4.0.0" {
					t.Errorf("Expected version 4.0.0, got %s", build.Version)
				}
				if build.OperatingSystem != "linux" {
					t.Errorf("Expected OS linux, got %s", build.OperatingSystem)
				}
				if build.Architecture != "x86_64" {
					t.Errorf("Expected architecture x86_64, got %s", build.Architecture)
				}
				if build.FileExtension != "tar.xz" {
					t.Errorf("Expected file extension tar.xz, got %s", build.FileExtension)
				}
				if build.Status != "Online" {
					t.Errorf("Expected status Online, got %s", build.Status)
				}
			},
		},
		{
			name:          "filter to 4.0",
			versionFilter: "4.0",
			expectError:   false,
			expectedCount: 1, // Only the 4.0.0 Linux build
			checkFirstBuild: func(t *testing.T, build model.BlenderBuild) {
				if build.Version != "4.0.0" {
					t.Errorf("Expected version 4.0.0, got %s", build.Version)
				}
			},
		},
		{
			name:            "filter to 5.0",
			versionFilter:   "5.0",
			expectError:     false,
			expectedCount:   0, // No builds match
			checkFirstBuild: nil,
		},
		{
			name:            "invalid filter",
			versionFilter:   "invalid",
			expectError:     true,
			expectedCount:   0,
			checkFirstBuild: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Call the function
			builds, err := FetchBuilds(tc.versionFilter)

			// Check error result
			if tc.expectError && err == nil {
				t.Error("Expected an error, but got nil")
			} else if !tc.expectError && err != nil {
				t.Errorf("Expected no error, but got: %v", err)
			}

			// If we don't expect an error, check the builds
			if !tc.expectError {
				if len(builds) != tc.expectedCount {
					t.Errorf("Expected %d builds, got %d", tc.expectedCount, len(builds))
				}

				// If expected to find at least one build and we have a check function
				if tc.expectedCount > 0 && tc.checkFirstBuild != nil && len(builds) > 0 {
					tc.checkFirstBuild(t, builds[0])
				}
			}
		})
	}
}

func TestFetchBuildsServerError(t *testing.T) {
	// Set up a test HTTP client and server
	originalClient := http.DefaultClient
	defer func() { http.DefaultClient = originalClient }()

	// Setup a mock HTTP server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	// Create a custom client that redirects requests to our test server
	http.DefaultClient = &http.Client{
		Transport: &mockTransport{
			apiURL: blenderAPIURL,
			server: server,
		},
	}

	// Call the function
	builds, err := FetchBuilds("")

	// Should return an error
	if err == nil {
		t.Error("Expected an error for server error, but got nil")
	}

	// Builds should be nil or empty
	if builds != nil && len(builds) > 0 {
		t.Errorf("Expected no builds for server error, got %d", len(builds))
	}
}

func TestFetchBuildsInvalidJSON(t *testing.T) {
	// Set up a test HTTP client and server
	originalClient := http.DefaultClient
	defer func() { http.DefaultClient = originalClient }()

	// Setup a mock HTTP server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("This is not valid JSON"))
	}))
	defer server.Close()

	// Create a custom client that redirects requests to our test server
	http.DefaultClient = &http.Client{
		Transport: &mockTransport{
			apiURL: blenderAPIURL,
			server: server,
		},
	}

	// Call the function
	builds, err := FetchBuilds("")

	// Should return an error
	if err == nil {
		t.Error("Expected an error for invalid JSON, but got nil")
	}

	// Builds should be nil or empty
	if builds != nil && len(builds) > 0 {
		t.Errorf("Expected no builds for invalid JSON, got %d", len(builds))
	}
}

// mockTransport is a custom http.RoundTripper that redirects requests
// from the real API URL to our test server
type mockTransport struct {
	apiURL string
	server *httptest.Server
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// If the request is for the Blender API, redirect to our test server
	if req.URL.String() == m.apiURL {
		// Create a new request to the test server
		testReq, err := http.NewRequest(req.Method, m.server.URL, req.Body)
		if err != nil {
			return nil, err
		}

		// Copy headers
		testReq.Header = req.Header

		// Send the request to our test server
		return http.DefaultTransport.RoundTrip(testReq)
	}

	// For other requests, use the default transport
	return http.DefaultTransport.RoundTrip(req)
}
