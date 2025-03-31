from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from typing import Dict, Optional


@dataclass
class LocalBuildInfo:
    """Information about a local Blender build."""

    version: str
    time: Optional[str]
    branch: Optional[str] = None
    risk_id: Optional[str] = None
    build_date: Optional[str] = None
    download_date: Optional[str] = None
    directory_size: Optional[int] = None  # Size of the build directory in bytes

    @property
    def size_mb(self) -> Optional[float]:
        """Get the directory size in megabytes, if available."""
        return (
            self.directory_size / (1024 * 1024)
            if self.directory_size is not None
            else None
        )


@dataclass
class BlenderBuild:
    """Represents a Blender build from the API."""

    version: str
    branch: str
    risk_id: str
    file_size: int
    file_mtime: int
    file_name: str
    url: str
    platform: str
    architecture: str

    @property
    def size_mb(self) -> float:
        """Get the file size in megabytes."""
        return self.file_size / (1024 * 1024)

    @property
    def mtime_formatted(self) -> str:
        """Get the formatted modification time."""
        return datetime.fromtimestamp(self.file_mtime).strftime("%Y-%m-%d %H:%M")

    @property
    def build_time(self) -> str:
        """Get the build time formatted for directory naming."""
        return datetime.fromtimestamp(self.file_mtime).strftime("%Y%m%d_%H%M")

    @classmethod
    def from_dict(cls, data: Dict) -> BlenderBuild:
        """Create a BlenderBuild instance from API response data."""
        return cls(
            version=data["version"],
            branch=data["branch"],
            risk_id=data["risk_id"],
            file_size=data["file_size"],
            file_mtime=data["file_mtime"],
            file_name=data["file_name"],
            url=data["url"],
            platform=data["platform"],
            architecture=data["architecture"],
        )
