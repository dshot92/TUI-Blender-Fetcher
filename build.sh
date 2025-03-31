#!/bin/bash
set -e

echo "=== Blender Launcher Build Script ==="

# Check if virtual environment exists, create if not
if [ ! -d "venv" ]; then
  echo "Creating virtual environment..."
  python -m venv venv
fi

# Activate virtual environment
echo "Activating virtual environment..."
source venv/bin/activate

# Install dependencies
echo "Installing dependencies..."
pip install --upgrade pip
pip install -r requirements.txt
pip install pyinstaller

# Create a direct standalone entry point file
echo "Creating standalone entry point..."
cat > launcher_entry.py << 'EOF'
#!/usr/bin/env python3
"""
Direct entry point for Blender Launcher executable
"""

# Skip all package import issues by directly importing what we need
from blender_launcher.ui.tui import BlenderTUI

def main():
    tui = BlenderTUI()
    tui.run()

if __name__ == "__main__":
    main()
EOF

# Create folder structure matching what the app expects
mkdir -p build/temp/blender_launcher/{api,config,models,ui,utils}
cp -r blender-launcher/api/* build/temp/blender_launcher/api/ 2>/dev/null || true
cp -r blender-launcher/config/* build/temp/blender_launcher/config/ 2>/dev/null || true
cp -r blender-launcher/models/* build/temp/blender_launcher/models/ 2>/dev/null || true
cp -r blender-launcher/ui/* build/temp/blender_launcher/ui/ 2>/dev/null || true
cp -r blender-launcher/utils/* build/temp/blender_launcher/utils/ 2>/dev/null || true
touch build/temp/blender_launcher/__init__.py
touch build/temp/blender_launcher/api/__init__.py
touch build/temp/blender_launcher/config/__init__.py
touch build/temp/blender_launcher/models/__init__.py
touch build/temp/blender_launcher/ui/__init__.py
touch build/temp/blender_launcher/utils/__init__.py

echo "Building standalone executable..."
pyinstaller \
  --name="blender-launcher" \
  --onefile \
  --add-data="build/temp/blender_launcher:blender_launcher" \
  --clean \
  --path="build/temp" \
  --collect-all=rich \
  --collect-all=toml \
  --collect-all=requests \
  launcher_entry.py

# Clean up temporary files
rm -rf build/temp
rm launcher_entry.py

echo "Making executable..."
chmod +x dist/blender-launcher

echo "Build complete! Executable available at dist/blender-launcher" 