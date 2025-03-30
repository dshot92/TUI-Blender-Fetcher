from enum import Enum
from pathlib import Path
import os
import sys
import shutil

# Use standard toml package
try:
    import toml
except ImportError:
    print(
        "toml package is required. Install with: pip install toml or sudo pacman -S python-toml"
    )
    sys.exit(1)


class AppConfig:
    """Configuration for the application."""

    # Default values
    DOWNLOAD_PATH = Path.home() / "blender/blender-build/"
    VERSION_CUTOFF = "2.80"  # Show only builds with version >= this value

    # Config file location
    CONFIG_DIR = Path.home() / ".config" / "blender-launcher"
    CONFIG_FILE = CONFIG_DIR / "config.toml"

    @classmethod
    def load_config(cls):
        """Load configuration from TOML file.

        If the configuration file doesn't exist or loading fails,
        default values will be used and a new configuration file will be created.
        """
        try:
            if cls.CONFIG_FILE.exists():
                with open(cls.CONFIG_FILE, "r") as f:
                    config_data = toml.load(f)

                # Update config values from file
                if "download_path" in config_data:
                    cls.DOWNLOAD_PATH = Path(config_data["download_path"])
                if "version_cutoff" in config_data:
                    cls.VERSION_CUTOFF = config_data["version_cutoff"]

                # No need to validate settings now that download_tool is removed
        except Exception as e:
            print(f"Failed to load config: {e}")
            # Ensure we create a new config file with defaults
            cls.save_config()

    @classmethod
    def save_config(cls):
        """Save configuration to a TOML file."""
        # Create config directory if it doesn't exist
        cls.CONFIG_DIR.mkdir(parents=True, exist_ok=True)

        # Prepare config data
        config_data = {
            "download_path": str(cls.DOWNLOAD_PATH),
            "version_cutoff": cls.VERSION_CUTOFF,
        }

        # Save config
        try:
            with open(cls.CONFIG_FILE, "w") as f:
                toml.dump(config_data, f)
        except Exception as e:
            print(f"Failed to save config: {e}")


# Load config on module import
AppConfig.load_config()


class Colors(str, Enum):
    """ANSI color codes for terminal output."""

    GREEN = "green"
    YELLOW = "yellow"
    BLUE = "blue"
    MAGENTA = "magenta"
    CYAN = "cyan"
    RED = "red"
    WHITE = "white"
    RESET = ""


class BuilderConfig:
    """Configuration for the Blender builder API."""

    USER_AGENT = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/11q5.0 Safari/537.36"
    API_URL = "https://builder.blender.org/download/daily/?format=json&v=1"
    TIMEOUT = 20
