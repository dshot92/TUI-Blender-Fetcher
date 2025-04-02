# TUI Blender Launcher (Go Version)

A Terminal User Interface (TUI) application written in Go for finding, downloading, managing, and launching Blender builds.

## Features

- Browse available Blender builds.
- Manage locally downloaded Blender installations.
- Launch installed Blender versions directly from the TUI.

## Installation

### Prerequisites

- Go
- Git

```bash
git clone https://github.com/dshot92/TUI-Blender-Launcher.git
cd TUI-Blender-Launcher

go mod tidy

# Run
go run main

# Build
go build
```

## Usage

Run the application from your terminal:

```bash
./TUI-Blender-Launcher
```

Builds page:
q:Quit
s:Settings
r:Reverse Sort
f:Fecth builds
d:download build (only for online builds)
enter:Launch Build
o:Open build Dir
x: delete build (only local builds)
x:cancel build download (downloading/extracting builds)

Settings Page: 
s:save and back to build page
q:Quit
enter:edit setting
c:cleanup old builds

