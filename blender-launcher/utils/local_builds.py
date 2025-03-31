import json
import shutil
from datetime import datetime
from pathlib import Path
from typing import Dict, Optional

from ..config.app_config import AppConfig
from ..models.build_info import LocalBuildInfo


def get_local_builds() -> Dict[str, LocalBuildInfo]:
    """Scan for and return information about local Blender builds.

    Returns:
        Dictionary mapping version strings to LocalBuildInfo objects
    """
    local_builds = {}

    # Ensure download directory exists
    download_dir = Path(AppConfig.DOWNLOAD_PATH)
    if not download_dir.exists():
        return local_builds

    for dir_path in download_dir.glob("blender-*"):
        if not dir_path.is_dir():
            continue

        build_info = _extract_build_info_from_directory(dir_path)
        if build_info and build_info.version:
            # If we already have this version, only update if the new one is newer
            if build_info.version not in local_builds or (
                build_info.time
                and local_builds.get(build_info.version)
                and local_builds[build_info.version].time
                and build_info.time > local_builds[build_info.version].time
            ):
                local_builds[build_info.version] = build_info

    return local_builds


def _extract_build_info_from_directory(dir_path: Path) -> Optional[LocalBuildInfo]:
    """Extract build information from a directory.

    First tries to read from version.json, then falls back to directory name parsing.

    Args:
        dir_path: Path to the directory containing a Blender build

    Returns:
        LocalBuildInfo object if information can be extracted, None otherwise
    """
    # First check if version.json exists
    version_file = dir_path / "version.json"
    if version_file.exists():
        try:
            with open(version_file, "r") as f:
                build_info = json.load(f)
                version = build_info.get("version")
                build_time = build_info.get("build_time")

                if version:
                    return LocalBuildInfo(
                        version=version,
                        time=build_time,
                        branch=build_info.get("branch"),
                        risk_id=build_info.get("risk_id"),
                        build_date=build_info.get(
                            "mtime_formatted",
                            datetime.fromtimestamp(
                                build_info.get("file_mtime", 0)
                            ).strftime("%Y-%m-%d %H:%M"),
                        ),
                        download_date=build_info.get("download_date"),
                        directory_size=build_info.get("directory_size"),
                    )
        except (json.JSONDecodeError, IOError, KeyError):
            # If there's an error reading the json, fall back to directory name parsing
            pass

    # Fallback to parsing directory name
    dir_name = dir_path.name
    if "blender-" in dir_name and "-" in dir_name[8:]:
        version = dir_name.split("-")[1]

        # Check if timestamp exists at the end (like _20250330_0415)
        if "_" in dir_name:
            timestamp_parts = dir_name.split("_")[-2:]
            # Verify that these actually look like a timestamp (first part should be numeric)
            build_time = (
                "_".join(timestamp_parts)
                if len(timestamp_parts) == 2 and timestamp_parts[0].isdigit()
                else None
            )
        else:
            # No timestamp in directory name, but still a valid build
            build_time = None

        # Calculate directory size for builds without version.json
        directory_size = None
        try:
            # Import here to avoid circular imports
            from .download import _calculate_directory_size

            directory_size = _calculate_directory_size(dir_path)
        except Exception:
            pass  # Skip size calculation if it fails

        return LocalBuildInfo(
            version=version, time=build_time, directory_size=directory_size
        )

    return None


def delete_local_build(version: str) -> bool:
    """Delete a local Blender build by version.

    Args:
        version: The version string of the build to delete

    Returns:
        True if the build was deleted successfully, False otherwise
    """
    download_dir = Path(AppConfig.DOWNLOAD_PATH)
    if not download_dir.exists():
        return False

    # Search for directories containing this version
    build_dirs = list(download_dir.glob(f"blender-{version}-*"))

    if not build_dirs:
        return False

    success = True
    # Delete all matching directories (usually just one)
    for dir_path in build_dirs:
        try:
            shutil.rmtree(dir_path)
        except (PermissionError, OSError):
            success = False

    return success
