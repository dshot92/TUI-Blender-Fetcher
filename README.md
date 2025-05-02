# TUI Blender Launcher (Go Version)

A Terminal User Interface (TUI) application written in Go for finding, downloading, managing, and launching Blender builds.

![Video Example](readme_assets/example.gif)

## Features

- Browse available Blender builds (daily, patch, and experimental builds)
- Filter builds by version number and build type
- Sort and navigate through builds with keyboard shortcuts
- Download Blender builds with real-time progress tracking
- Manage locally downloaded Blender installations
- Launch installed Blender versions directly from the TUI
- Clean up old builds to free disk space
- Configurable download directory
- Multi-platform support (Linux, Windows, macOS)

## Blender BuildBot API

This application uses the official Blender BuildBot API to fetch available builds. The API provides access to daily, patch, and experimental builds in JSON format:

```
https://builder.blender.org/download/<category>/?format=json&v=1
```

Where `<category>` can be:
- `daily` - Latest development builds from the master branch
- `patch` - Patch builds from pull requests
- `experimental` - Experimental builds from specific branches

For more information about the Blender BuildBot and its API, visit the [official documentation](https://developer.blender.org/docs/handbook/tooling/buildbot/#builds-listing-api).


## Installation

### Prerequisites

- Go 1.18 or higher
- Git

### Building from Source

```bash
# Clone the repository
git clone https://github.com/dshot92/TUI-Blender-Launcher.git
cd TUI-Blender-Launcher

# Install dependencies
go mod tidy

# Run directly
go run main.go

# Or build the executable
go build -o tui-blender-launcher

# Run the built executable
./tui-blender-launcher
```

## Configuration

On first run, the application will guide you through an initial setup. You can configure:

- Download directory for Blender builds
- Version filter (e.g., "4.0", "3.6", or empty for no filter)
- Build type (daily, patch, experimental)

Settings are saved in your system's user configuration directory:
- **Linux**: `~/.config/tui-blender-launcher/config.toml`
- **macOS**: `~/Library/Application Support/tui-blender-launcher/config.toml`
- **Windows**: `%AppData%\tui-blender-launcher\config.toml`

Default config.toml:
```toml
download_dir = [HOME-DIR]"/blender/blender-build"
version_filter = ""
build_type = "daily"
uuid = "e9b26094-0ecc-4177-8d9e-d13a440ab51e" # Random UUID generated on first run
```

## Usage

### Navigation

The application uses keyboard shortcuts for navigation:

- <kbd>⬆</kbd> / <kbd>k</kbd>: Move cursor up
- <kbd>⬇</kbd> / <kbd>j</kbd>: Move cursor down
- <kbd>⬅</kbd> / <kbd>h</kbd>: Previous sort column
- <kbd>⮕</kbd> / <kbd>l</kbd>: Next sort column

#### Builds Page

- <kbd>f</kbd>: Fetch online builds

- <kbd>Enter</kbd>: Launch selected build
- <kbd>o</kbd>: Open build directory
- <kbd>x</kbd>: Delete build (local builds) / Cancel download (for in-progress downloads)
- <kbd>d</kbd>: Download selected build (only for online/update builds)

- <kbd>r</kbd>: Reverse sort order
- <kbd>s</kbd>: Settings
- <kbd>q</kbd>: Quit application

#### Settings Page
- <kbd>Enter</kbd>: Edit selected setting
- <kbd>s</kbd>: Save and return to builds page

- <kbd>c</kbd>: Clean up old builds
- <kbd>q</kbd>: Quit application

