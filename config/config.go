package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// AppName is used for the config directory
const AppName = "tui-blender-launcher" // Use lowercase app name

// Config holds the application settings.
type Config struct {
	DownloadDir   string `toml:"download_dir"`
	VersionFilter string `toml:"version_filter"` // e.g., "4.0", "3.6", or empty for no filter
}

// DefaultConfig returns a Config struct with default values.
func DefaultConfig() Config {
	// Sensible default download path (e.g., ~/blender-builds)
	// We will expand the ~ later
	homeDir, _ := os.UserHomeDir() // Use UserHomeDir for safety
	defaultDownloadPath := filepath.Join(homeDir, "blender/blender-builds")

	return Config{
		DownloadDir:   defaultDownloadPath,
		VersionFilter: "", // No filter by default
	}
}

// GetConfigPath returns the full path to the config file.
// Exported version of getConfigPath.
func GetConfigPath() (string, error) {
	configDir, err := os.UserConfigDir() // Gets ~/.config on Linux, appropriate paths on other OS
	if err != nil {
		return "", fmt.Errorf("could not get user config directory: %w", err)
	}

	appConfigDir := filepath.Join(configDir, AppName)
	configFilePath := filepath.Join(appConfigDir, "config.toml")

	return configFilePath, nil
}

// LoadConfig loads the configuration from the default path.
// If the file doesn't exist, it returns default settings without error.
func LoadConfig() (Config, error) {
	cfgPath, err := GetConfigPath()
	if err != nil {
		return Config{}, err // Return zero Config and the error
	}

	cfg := DefaultConfig() // Start with defaults

	// Check if config file exists
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		// Config file doesn't exist, return defaults quietly
		// We will prompt/create it later if needed
		return cfg, nil
	} else if err != nil {
		// Other error reading file stat
		return Config{}, fmt.Errorf("could not stat config file %s: %w", cfgPath, err)
	}

	// File exists, try to load it
	if _, err := toml.DecodeFile(cfgPath, &cfg); err != nil {
		return Config{}, fmt.Errorf("could not decode config file %s: %w", cfgPath, err)
	}

	// Expand ~ in DownloadDir if present
	if cfg.DownloadDir != "" && cfg.DownloadDir[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return cfg, fmt.Errorf("could not get home directory to expand path: %w", err)
		}
		cfg.DownloadDir = filepath.Join(homeDir, cfg.DownloadDir[1:])
	}

	return cfg, nil
}

// SaveConfig saves the configuration to the default path.
// It creates the config directory if it doesn't exist.
func SaveConfig(cfg Config) error {
	cfgPath, err := GetConfigPath()
	if err != nil {
		return err
	}

	appConfigDir := filepath.Dir(cfgPath)

	// Create the config directory if it doesn't exist
	if err := os.MkdirAll(appConfigDir, 0750); err != nil { // Use 0750 for permissions
		return fmt.Errorf("could not create config directory %s: %w", appConfigDir, err)
	}

	// Create and open the file for writing
	file, err := os.Create(cfgPath)
	if err != nil {
		return fmt.Errorf("could not create config file %s: %w", cfgPath, err)
	}
	defer file.Close()

	// Encode the config to the file
	encoder := toml.NewEncoder(file)
	if err := encoder.Encode(cfg); err != nil {
		return fmt.Errorf("could not encode config to file %s: %w", cfgPath, err)
	}

	return nil
}
