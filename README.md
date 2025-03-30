# Blender Fetcher

A utility for finding, downloading, and managing Blender builds.

## Installation

1. Clone the repository:
```bash
git clone https://github.com/yourusername/sh-dotfiles.git
cd sh-dotfiles/shell/blender_fetcher
```

2. Install the required dependencies:
```bash
pip install -r requirements.txt
```

3. Install the package:
```bash
pip install -e .
```

## Usage

Run the application:
```bash
fetch-blender-build
```

Or use the shorter alias:
```bash
fbb
```

### Features

- Browse and download Blender builds from the official Blender site
- Manage local Blender installations
- Launch installed Blender versions

### Controls

- **Arrow keys** or **HJKL**: Navigate
- **Space**: Select/deselect a build
- **F**: Fetch online builds
- **D**: Download selected build(s)
- **Enter**: Launch selected Blender version
- **R**: Reverse sort order
- **S**: Open settings
- **Q**: Quit

## Requirements

- Python 3.6+
- Rich 13.9.4 or higher 

### Links
[Rich Docs](https://rich.readthedocs.io/en/latest/#)
[Rich GitHub](https://github.com/Textualize/rich)
