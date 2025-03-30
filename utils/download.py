import json
import shutil
import subprocess
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime
from pathlib import Path
from typing import List, Optional, Dict

from rich.console import Console
from rich.prompt import Confirm

from ..config.app_config import AppConfig
from ..models.build_info import BlenderBuild


def download_build(
    build: BlenderBuild, console: Console, skip_confirmation: bool = False
) -> Optional[str]:
    """Download and extract a specific build.

    Args:
        build: The build to download
        console: Console for output
        skip_confirmation: Whether to skip confirmation for removing existing builds

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

    # Check for existing builds BEFORE download
    existing_builds = list(download_dir.glob(f"blender-{build.version}*"))
    should_remove = False

    if existing_builds and not skip_confirmation:
        console.print(f"\nExisting builds found for {build.version}:")
        for build_dir in existing_builds:
            console.print(f"  - {build_dir}")

        # Ask for confirmation before downloading
        if not Confirm.ask(
            f"Remove existing Blender {build.version} build(s)?", default=True
        ):
            console.print(f"Keeping existing Blender {build.version} build(s)")
            return None

        should_remove = True

    try:
        console.print(f"\nStarting download of {filename}...")

        # Download the file
        if not _download_file(build.url, download_dir, console):
            return None

        # AFTER successful download, remove existing builds if user confirmed
        if should_remove or skip_confirmation:
            for build_dir in existing_builds:
                try:
                    console.print(f"Removing {build_dir}...")
                    if build_dir.is_dir():
                        subprocess.run(["rm", "-rf", str(build_dir)], check=True)
                    else:
                        build_dir.unlink()
                except (subprocess.CalledProcessError, OSError) as e:
                    console.print(
                        f"Failed to remove {build_dir}: {e}", style="bold red"
                    )
                    if download_path.exists():
                        download_path.unlink()
                    return None

        console.print(f"Extraction of {filename}...")
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


def download_multiple_builds(builds: List[BlenderBuild]) -> bool:
    """Download multiple builds in parallel.

    Args:
        builds: List of builds to download

    Returns:
        True if all downloads were successful, False otherwise
    """
    # Create console for output
    console = Console()

    # Ensure download directory exists
    download_dir = Path(AppConfig.DOWNLOAD_PATH)
    download_dir.mkdir(parents=True, exist_ok=True)

    # Ask confirmation for removing existing builds BEFORE downloading
    all_existing_builds = []
    versions_to_remove: Dict[str, List[Path]] = {}

    for build in builds:
        existing = list(download_dir.glob(f"blender-{build.version}*"))
        if existing:
            versions_to_remove[build.version] = existing
            all_existing_builds.extend(existing)

    should_remove = False

    if all_existing_builds:
        console.print("\nExisting builds found:")
        for build_dir in all_existing_builds:
            console.print(f"  - {build_dir}")

        # Get confirmation for updates
        if not Confirm.ask(
            "This will remove existing builds and download updates. Proceed?",
            default=True,
        ):
            console.print("Download cancelled")
            return False

        should_remove = True

    console.print(
        f"\nStarting parallel download of {len(builds)} builds with {AppConfig.MAX_WORKERS} workers...\n"
    )
    console.print(f"Files will be downloaded to: {download_dir}\n")

    # Use ThreadPoolExecutor to download and extract in parallel
    completed_versions = []
    download_paths = {}

    try:
        # First download all builds
        with ThreadPoolExecutor(max_workers=AppConfig.MAX_WORKERS) as executor:
            futures = {}

            for build in builds:
                # We need to create a modified version of download_build that doesn't remove builds
                def download_file_only(build=build):
                    try:
                        download_dir = Path(AppConfig.DOWNLOAD_PATH)
                        filename = build.file_name
                        download_path = download_dir / filename

                        console.print(f"\nStarting download of {filename}...")
                        if _download_file(build.url, download_dir, console):
                            return build, download_path
                        return None
                    except Exception as e:
                        console.print(
                            f"Download of {build.version} failed: {e}", style="bold red"
                        )
                        return None

                futures[executor.submit(download_file_only)] = build

            successful_downloads = []
            for future in as_completed(futures):
                try:
                    result = future.result()
                    if result:
                        build, path = result
                        successful_downloads.append((build, path))
                except Exception as e:
                    console.print(f"Download failed: {e}", style="bold red")

        # If we got successful downloads and should remove existing builds
        if successful_downloads and should_remove:
            console.print("\nRemoving existing builds...")
            for version, paths in versions_to_remove.items():
                console.print(f"Removing existing Blender {version} build(s)...")
                for build_dir in paths:
                    try:
                        console.print(f"Removing {build_dir}...")
                        if build_dir.is_dir():
                            subprocess.run(["rm", "-rf", str(build_dir)], check=True)
                        else:
                            build_dir.unlink()
                    except (subprocess.CalledProcessError, OSError) as e:
                        console.print(
                            f"Failed to remove {build_dir}: {e}", style="bold red"
                        )

        # Now extract all successful downloads
        for build, download_path in successful_downloads:
            try:
                filename = build.file_name
                extracted_dir_name = filename.replace(".tar.xz", "")
                extract_path = download_dir / extracted_dir_name

                console.print(f"Extraction of {filename}...")
                if _extract_archive(download_path, download_dir, console):
                    # Create version information file
                    _create_version_info(extract_path, build)

                    # Clean up the archive file
                    download_path.unlink()
                    console.print(f"Cleaned up archive file for {build.version}")

                    console.print(
                        f"Extraction of Blender {build.version} completed successfully"
                    )
                    completed_versions.append(build.version)
            except Exception as e:
                console.print(
                    f"Extraction of {build.version} failed: {e}", style="bold red"
                )

        if completed_versions:
            console.print(
                f"\nCompleted downloading {len(completed_versions)} builds: {', '.join(completed_versions)}"
            )
            return True
        else:
            console.print(
                "\nNo builds were downloaded successfully", style="bold yellow"
            )
            return False

    except KeyboardInterrupt:
        console.print(
            "\nDownloads interrupted by user. Cleaning up...", style="bold yellow"
        )
        # We can't cancel the downloads directly, but we can inform the user
        console.print(
            "Note: Download processes may still be running in the background."
        )
        console.print("You may need to manually kill aria2c or wget processes.")
        return False

    except Exception as e:
        console.print(f"\nAn error occurred during downloads: {e}", style="bold red")
        return False


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
            console.print(
                "[bold]Using aria2c for faster download with 16 connections[/bold]"
            )
            # Show download progress with speed information - run in foreground mode
            process = subprocess.run(
                [
                    "aria2c",
                    "-s",
                    "16",  # 16 connections
                    "-x",
                    "16",  # 16 connections per server
                    "-k",
                    "1M",  # Chunk size
                    "--console-log-level=notice",  # Show important messages
                    "--summary-interval=1",  # Update summary every second
                    "--file-allocation=none",  # Speeds up start time
                    "--auto-file-renaming=false",  # Don't rename files
                    "-d",
                    str(download_dir),  # Download directory
                    url,
                ],
                check=True,
            )
        else:
            console.print("[bold]aria2c not found, falling back to wget[/bold]")
            # Use wget with progress bar in foreground
            process = subprocess.run(
                [
                    "wget",
                    "--no-verbose",  # Not completely quiet
                    "--progress=bar:force:noscroll",  # Force progress bar
                    "--show-progress",  # Always show progress
                    "-P",
                    str(download_dir),  # Download directory
                    url,
                ],
                check=True,
            )
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
