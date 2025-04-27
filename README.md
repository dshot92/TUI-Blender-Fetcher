# TUI Blender Launcher (Go Version)

A Terminal User Interface (TUI) application written in Go for finding, downloading, managing, and launching Blender builds.

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

## Usage

### Navigation

The application uses keyboard shortcuts for navigation:

#### Builds Page
- `q`: Quit application
- `s`: Settings
- `r`: Reverse sort order
- `f`: Fetch online builds
- `d`: Download selected build (only for online/update builds)
- `Enter`: Launch selected build
- `o`: Open build directory
- `x`: Delete build (local builds) / Cancel download (for in-progress downloads)
- `↑/k`: Move cursor up
- `↓/j`: Move cursor down
- `←/h`: Previous sort column
- `→/l`: Next sort column
- `Page Up`: Scroll page up
- `Page Down`: Scroll page down
- `Home`: Go to first item
- `End`: Go to last item

#### Settings Page
- `s`: Save and return to builds page
- `q`: Quit application
- `Enter`: Edit selected setting
- `↑/k`: Move cursor up
- `↓/j`: Move cursor down
- `←/h`: Select previous option
- `→/l`: Select next option
- `c`: Clean up old builds

## Configuration

On first run, the application will guide you through an initial setup. You can configure:

- Download directory for Blender builds
- Version filter (e.g., "4.0", "3.6", or empty for no filter)
- Build type (daily, patch, experimental)

Settings are saved in your system's user configuration directory.

