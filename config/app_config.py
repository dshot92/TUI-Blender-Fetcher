from enum import Enum
from pathlib import Path


class AppConfig:
    """Configuration for the application."""

    DOWNLOAD_PATH = Path.home() / "blender/blender-build/"
    VERSION_CUTOFF = "2.80"  # Show only builds with version >= this value
    MAX_WORKERS = 4  # Maximum number of parallel downloads


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
