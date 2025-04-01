package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	// Get the default config
	cfg := DefaultConfig()

	// Check that the version filter is empty
	if cfg.VersionFilter != "" {
		t.Errorf("Expected empty version filter, got %s", cfg.VersionFilter)
	}

	// Check that the download dir is set to a reasonable default
	homeDir, _ := os.UserHomeDir()
	expectedPath := filepath.Join(homeDir, "blender/blender-build")

	if cfg.DownloadDir != expectedPath {
		t.Errorf("Expected download dir %s, got %s", expectedPath, cfg.DownloadDir)
	}
}

func TestGetConfigPath(t *testing.T) {
	// Get the config path
	path, err := GetConfigPath()
	if err != nil {
		t.Fatalf("GetConfigPath returned an error: %v", err)
	}

	// Check that it's not empty
	if path == "" {
		t.Error("GetConfigPath returned an empty path")
	}

	// Check that it ends with the expected path components
	expected := filepath.Join(AppName, "config.toml")
	if !filepath.IsAbs(path) {
		t.Error("GetConfigPath did not return an absolute path")
	}
	if !strings.HasSuffix(path, expected) {
		t.Errorf("Expected path to end with %s, got %s", expected, path)
	}
}

func TestLoadConfig(t *testing.T) {
	// Create a temporary directory for the test
	tempDir, err := os.MkdirTemp("", "blender-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir) // Clean up at the end

	// Save the original XDG_CONFIG_HOME
	oldConfigHome := os.Getenv("XDG_CONFIG_HOME")
	defer os.Setenv("XDG_CONFIG_HOME", oldConfigHome) // Restore at the end

	// Set XDG_CONFIG_HOME to our temp directory
	os.Setenv("XDG_CONFIG_HOME", tempDir)

	// Create the config directory structure
	configDir := filepath.Join(tempDir, AppName)
	err = os.MkdirAll(configDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Test cases
	testCases := []struct {
		name          string
		configContent string
		expectError   bool
		checkConfig   func(*testing.T, Config) // Function to validate the loaded config
	}{
		{
			name:          "valid config",
			configContent: "download_dir = \"/custom/path\"\nversion_filter = \"4.0\"\n",
			expectError:   false,
			checkConfig: func(t *testing.T, cfg Config) {
				if cfg.DownloadDir != "/custom/path" {
					t.Errorf("Expected download dir /custom/path, got %s", cfg.DownloadDir)
				}
				if cfg.VersionFilter != "4.0" {
					t.Errorf("Expected version filter 4.0, got %s", cfg.VersionFilter)
				}
			},
		},
		{
			name:          "invalid toml",
			configContent: "download_dir = /custom/path\" version_filter = \"4.0\"\n", // Invalid TOML syntax
			expectError:   true,
			checkConfig:   nil, // Not needed for error case
		},
		{
			name:          "missing config file",
			configContent: "", // No content, file will be deleted
			expectError:   false,
			checkConfig: func(t *testing.T, cfg Config) {
				// Should return default config
				homeDir, _ := os.UserHomeDir()
				expectedPath := filepath.Join(homeDir, "blender/blender-build")
				if cfg.DownloadDir != expectedPath {
					t.Errorf("Expected download dir %s, got %s", expectedPath, cfg.DownloadDir)
				}
				if cfg.VersionFilter != "" {
					t.Errorf("Expected empty version filter, got %s", cfg.VersionFilter)
				}
			},
		},
		{
			name:          "path with tilde",
			configContent: "download_dir = \"~/custom/path\"\nversion_filter = \"3.6\"\n",
			expectError:   false,
			checkConfig: func(t *testing.T, cfg Config) {
				homeDir, _ := os.UserHomeDir()
				expectedPath := filepath.Join(homeDir, "custom/path")
				if cfg.DownloadDir != expectedPath {
					t.Errorf("Expected download dir %s, got %s", expectedPath, cfg.DownloadDir)
				}
				if cfg.VersionFilter != "3.6" {
					t.Errorf("Expected version filter 3.6, got %s", cfg.VersionFilter)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configPath := filepath.Join(configDir, "config.toml")

			// Set up or clear the config file
			if tc.configContent == "" {
				// Remove the file if it exists
				os.Remove(configPath)
			} else {
				// Write the test content
				err := os.WriteFile(configPath, []byte(tc.configContent), 0644)
				if err != nil {
					t.Fatalf("Failed to write config file: %v", err)
				}
			}

			// Call the function
			cfg, err := LoadConfig()

			// Check error result
			if tc.expectError && err == nil {
				t.Error("Expected an error, but got nil")
			} else if !tc.expectError && err != nil {
				t.Errorf("Expected no error, but got: %v", err)
			}

			// If a config check function is provided and no error occurred, check the config
			if tc.checkConfig != nil && err == nil {
				tc.checkConfig(t, cfg)
			}
		})
	}
}

func TestSaveConfig(t *testing.T) {
	// Create a temporary directory for the test
	tempDir, err := os.MkdirTemp("", "blender-config-save-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir) // Clean up at the end

	// Save the original XDG_CONFIG_HOME
	oldConfigHome := os.Getenv("XDG_CONFIG_HOME")
	defer os.Setenv("XDG_CONFIG_HOME", oldConfigHome) // Restore at the end

	// Set XDG_CONFIG_HOME to our temp directory
	os.Setenv("XDG_CONFIG_HOME", tempDir)

	// Create a test config
	cfg := Config{
		DownloadDir:   "/test/path",
		VersionFilter: "3.5",
	}

	// Save the config
	err = SaveConfig(cfg)
	if err != nil {
		t.Fatalf("SaveConfig returned an error: %v", err)
	}

	// Check that the config file was created
	configPath, _ := GetConfigPath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Errorf("Config file was not created at %s", configPath)
	}

	// Read the config file and check its content
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	content := string(data)
	// Check that the config contains our values
	if !containsStr(content, "download_dir = \"/test/path\"") {
		t.Errorf("Config file doesn't contain expected download_dir, got: %s", content)
	}
	if !containsStr(content, "version_filter = \"3.5\"") {
		t.Errorf("Config file doesn't contain expected version_filter, got: %s", content)
	}

	// Load the config and verify values
	loadedCfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	if loadedCfg.DownloadDir != cfg.DownloadDir {
		t.Errorf("Loaded download_dir doesn't match saved value. Got %s, want %s",
			loadedCfg.DownloadDir, cfg.DownloadDir)
	}
	if loadedCfg.VersionFilter != cfg.VersionFilter {
		t.Errorf("Loaded version_filter doesn't match saved value. Got %s, want %s",
			loadedCfg.VersionFilter, cfg.VersionFilter)
	}
}

// Helper function to check if a string contains a substring
// (Simplified string check for TOML fields)
func containsStr(s, substr string) bool {
	return strings.HasPrefix(s, substr) || strings.Contains(s, "\n"+substr) || strings.Contains(s, substr+"\n")
}
