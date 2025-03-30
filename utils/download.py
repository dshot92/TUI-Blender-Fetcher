import json
import shutil
import subprocess
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime
from pathlib import Path
from typing import List, Optional

from rich.console import Console
from rich.prompt import Confirm

from ..config.app_config import AppConfig
from ..models.build_info import BlenderBuild


def download_build(build: BlenderBuild, console: Console) -> Optional[str]:
    """Download and extract a specific build.

    Args:
        build: The build to download
        console: Console for output

    Returns:
        The version string if successful, None otherwise
    """
    # Ensure download directory exists
    download_dir = Path(AppConfig.DOWNLOAD_PATH)
    download_dir.mkdir(parents=True, exist_ok=True)

    filename = build.file_name
    extracted_dir_name = filename.replace(".tar.xz", "")

    # Get absolute paths
    download_path = download_dir / filename
    extract_path = download_dir / extracted_dir_name

    # Check for and remove existing builds of this version
    if not _cleanup_existing_builds(download_dir, build.version, console):
        return None

    try:
        console.print(f"\nStarting download of {filename}...")

        # Download the file
        if not _download_file(build.url, download_dir, console):
            return None

        # Extract the archive
        if not _extract_archive(download_path, download_dir, console):
            return None

        # Create version information file
        _create_version_info(extract_path, build)

        # Clean up the archive file
        download_path.unlink()
        console.print(f"Cleaned up archive file for {build.version}")

        console.print(
            f"Download and extraction of Blender {build.version} completed successfully"
        )

        return build.version

    except Exception as e:
        console.print(
            f"Failed to download/extract {build.version}: {e}", style="bold red"
        )
        return None


def download_multiple_builds(
    builds: List[BlenderBuild], selected_indices: List[int], console: Console
) -> None:
    """Download multiple builds in parallel.

    Args:
        builds: List of available builds
        selected_indices: Indices of builds to download
        console: Console for output
    """
    selected_builds = [builds[idx] for idx in selected_indices]

    # Ensure download directory exists
    download_dir = Path(AppConfig.DOWNLOAD_PATH)
    download_dir.mkdir(parents=True, exist_ok=True)

    # Ask confirmation for removing existing builds
    all_existing_builds = []
    for build in selected_builds:
        existing = list(download_dir.glob(f"blender-{build.version}*"))
        all_existing_builds.extend(existing)

    if all_existing_builds:
        console.print("\nExisting builds found:")
        for build_dir in all_existing_builds:
            console.print(f"  - {build_dir}")

        if not Confirm.ask(
            "Do you want to remove existing builds and download the new ones?"
        ):
            console.print("Download cancelled")
            return

    console.print(
        f"\nStarting parallel download of {len(selected_builds)} builds with {AppConfig.MAX_WORKERS} workers...\n"
    )
    console.print(f"Files will be downloaded to: {download_dir}\n")
    console.print("Press Ctrl+C to cancel downloads\n", style="bold yellow")

    # Use ThreadPoolExecutor to download and extract in parallel
    completed_versions = []

    try:
        with ThreadPoolExecutor(max_workers=AppConfig.MAX_WORKERS) as executor:
            futures = {
                executor.submit(download_build, build, console): build
                for build in selected_builds
            }

            for future in as_completed(futures):
                build = futures[future]
                try:
                    version = future.result()
                    if version:
                        completed_versions.append(version)
                except Exception as e:
                    console.print(
                        f"Download of Blender {build.version} failed: {e}",
                        style="bold red",
                    )

        if completed_versions:
            console.print(
                f"\nCompleted downloading {len(completed_versions)} builds: {', '.join(completed_versions)}"
            )
        else:
            console.print(
                "\nNo builds were downloaded successfully", style="bold yellow"
            )

    except KeyboardInterrupt:
        console.print(
            "\nDownloads interrupted by user. Cleaning up...", style="bold yellow"
        )
        # We can't cancel the downloads directly, but we can inform the user
        console.print(
            "Note: Download processes may still be running in the background."
        )
        console.print("You may need to manually kill aria2c or wget processes.")

    except Exception as e:
        console.print(f"\nAn error occurred during downloads: {e}", style="bold red")


def _cleanup_existing_builds(
    download_dir: Path, version: str, console: Console
) -> bool:
    """Remove existing builds of a specific version.

    Args:
        download_dir: Path to the download directory
        version: Version string to match
        console: Console for output

    Returns:
        True if cleanup was successful or no builds needed cleanup
    """
    existing_builds = list(download_dir.glob(f"blender-{version}*"))

    if existing_builds:
        console.print(f"\nExisting builds found for {version}:")
        for build_dir in existing_builds:
            console.print(f"  - {build_dir}")

        # Remove existing builds
        for build_dir in existing_builds:
            try:
                console.print(f"Removing {build_dir}...")
                if build_dir.is_dir():
                    subprocess.run(["rm", "-rf", str(build_dir)], check=True)
                else:
                    build_dir.unlink()
            except (subprocess.CalledProcessError, OSError) as e:
                console.print(f"Failed to remove {build_dir}: {e}", style="bold red")
                return False

    return True


def _download_file(url: str, download_dir: Path, console: Console) -> bool:
    """Download a file using aria2c or wget.

    Args:
        url: URL to download
        download_dir: Directory to save the file
        console: Console for output

    Returns:
        True if download was successful
    """
    try:
        # Check if aria2c is available
        if shutil.which("aria2c"):
            console.print("Using aria2c for faster download...")
            subprocess.run(
                [
                    "aria2c",
                    "-s",
                    "16",
                    "-x",
                    "16",
                    "-k",
                    "1M",
                    "-d",
                    str(download_dir),
                    url,
                ],
                check=True,
            )
        else:
            console.print("aria2c not found, falling back to wget...")
            subprocess.run(["wget", "-P", str(download_dir), url], check=True)
        return True
    except subprocess.CalledProcessError as e:
        console.print(f"Download failed: {e}", style="bold red")
        return False


def _extract_archive(archive_path: Path, target_dir: Path, console: Console) -> bool:
    """Extract an archive to the target directory.

    Args:
        archive_path: Path to the archive file
        target_dir: Directory to extract to
        console: Console for output

    Returns:
        True if extraction was successful
    """
    try:
        console.print(f"Extracting {archive_path.name}...")
        subprocess.run(
            ["tar", "-xf", str(archive_path), "-C", str(target_dir)], check=True
        )
        return True
    except subprocess.CalledProcessError as e:
        console.print(f"Extraction failed: {e}", style="bold red")
        return False


def _create_version_info(extract_path: Path, build: BlenderBuild) -> None:
    """Create a version.json file in the extracted directory with build information.

    Args:
        extract_path: Path to the extracted build directory
        build: Build information to save
    """
    if not extract_path.exists():
        return

    # Create a dictionary with all build information
    build_info = {
        "version": build.version,
        "branch": build.branch,
        "risk_id": build.risk_id,
        "file_size": build.file_size,
        "file_mtime": build.file_mtime,
        "file_name": build.file_name,
        "platform": build.platform,
        "architecture": build.architecture,
        "build_time": build.build_time,
        "mtime_formatted": build.mtime_formatted,
        "download_date": datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
    }

    # Write the information to a JSON file
    version_file = extract_path / "version.json"
    with open(version_file, "w") as f:
        json.dump(build_info, f, indent=2)
