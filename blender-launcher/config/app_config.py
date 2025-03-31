from enum import Enum
from pathlib import Path

import sys

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
    VERSION_CUTOFF = "3.1"  # Show only builds with version >= this value

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
            else:
                # If config file doesn't exist, create it with defaults
                print("Config file doesn't exist, creating with defaults")
                cls.save_config()
        except Exception as e:
            print(f"Failed to load config: {e}")
            # Ensure we create a new config file with defaults
            cls.save_config()

    @classmethod
    def save_config(cls):
        """Save configuration to a TOML file."""
        # Create config directory if it doesn't exist
        cls.CONFIG_DIR.mkdir(parents=True, exist_ok=True)
        print(
            f"Debug: Config directory at {cls.CONFIG_DIR} exists: {cls.CONFIG_DIR.exists()}"
        )

        # Prepare config data
        config_data = {
            "download_path": str(cls.DOWNLOAD_PATH),
            "version_cutoff": cls.VERSION_CUTOFF,
        }
        print(f"Debug: Preparing to save config data: {config_data}")

        # Save config
        try:
            print(f"Debug: Attempting to write to {cls.CONFIG_FILE}")
            with open(cls.CONFIG_FILE, "w") as f:
                toml.dump(config_data, f)
            print(f"Debug: Config saved successfully to {cls.CONFIG_FILE}")
        except Exception as e:
            print(f"Failed to save config: {e}")
            # Print more detailed error information
            import traceback

            traceback.print_exc()


# Load config on module import
print("Loading AppConfig module")
AppConfig.load_config()
# Ensure config file exists even if load_config didn't create it
if not AppConfig.CONFIG_FILE.exists():
    print("Explicitly creating config file")
    AppConfig.save_config()


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
