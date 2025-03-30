from typing import List, Tuple

import json
import requests

from ..config.app_config import BuilderConfig
from ..models.build_info import BlenderBuild


def fetch_builds() -> List[BlenderBuild]:
    """Fetch and parse build information from the Blender API.

    Returns:
        List of BlenderBuild objects filtered for Linux x86_64 and by version cutoff.

    Raises:
        Exception: Various exceptions for API connection errors.
    """
    try:
        headers = {"User-Agent": BuilderConfig.USER_AGENT}

        with requests.get(
            BuilderConfig.API_URL,
            headers=headers,
            timeout=BuilderConfig.TIMEOUT,
        ) as response:
            response.raise_for_status()
            builds_data = response.json()

            # Filter for Linux x86_64 builds and exclude .sha256 files
            linux_builds = [
                BlenderBuild.from_dict(build)
                for build in builds_data
                if build["platform"] == "linux"
                and build["architecture"] == "x86_64"
                and not build["file_extension"] == "sha256"
            ]

            # Filter by version cutoff
            from ..config.app_config import AppConfig

            version_min = tuple(map(int, AppConfig.VERSION_CUTOFF.split(".")))
            filtered_builds = [
                build
                for build in linux_builds
                if version_meets_minimum(build.version, version_min)
            ]

            return filtered_builds

    except requests.exceptions.Timeout:
        raise Exception(
            f"Connection timed out after {BuilderConfig.TIMEOUT} seconds"
        ) from None
    except requests.exceptions.ConnectionError:
        raise Exception(
            "Failed to connect to the Blender API. Check your internet connection."
        ) from None
    except requests.exceptions.HTTPError as e:
        raise Exception(
            f"HTTP error: {e.response.status_code} - {e.response.reason}"
        ) from e
    except json.JSONDecodeError:
        raise Exception("Failed to parse API response: Invalid JSON format") from None
    except KeyError as e:
        raise Exception(f"Failed to parse API response: Missing key {e}") from e
    except Exception as e:
        raise Exception(f"Unexpected error: {e}") from e


def version_meets_minimum(version: str, min_version: Tuple[int, ...]) -> bool:
    """Check if a version string meets the minimum version requirement.

    Args:
        version: The version string to check (e.g., "2.83")
        min_version: A tuple of integers representing the minimum version

    Returns:
        True if the version meets or exceeds the minimum, False otherwise
    """
    try:
        version_tuple = tuple(map(int, version.split(".")))
        return version_tuple >= min_version
    except (ValueError, AttributeError):
        # If the version can't be parsed, assume it doesn't meet requirements
        return False
