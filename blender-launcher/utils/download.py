import json
import os
import subprocess
import threading
from datetime import datetime
from pathlib import Path
from typing import List, Optional, Dict, Tuple

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

    # Automatically clean up incomplete downloads without prompting
    if download_path.exists():
        # console.print(f"Found incomplete download file: {filename}")
        # console.print("Removing incomplete download file...")
        download_path.unlink()

    # Check for existing build DIRECTORIES only (not archive files)
    existing_builds = [
        build_dir
        for build_dir in download_dir.glob(f"blender-{build.version}*")
        if build_dir.is_dir()
    ]

    should_remove = False

    if existing_builds and not skip_confirmation:
        # console.print(
        #     f"\nExisting build directories found for Blender {build.version}:"
        # )
        # for build_dir in existing_builds:
        #     console.print(f"  - {build_dir}")

        # Ask for confirmation before downloading
        if not Confirm.ask(
            f"Remove existing Blender {build.version} build(s)?", default=True
        ):
            # console.print(f"Keeping existing Blender {build.version} build(s)")
            return None

        should_remove = True

    try:
        # console.print(f"\nStarting download of {filename}...")

        # Download the file
        if not _download_file(build.url, download_dir, console):
            return None

        # AFTER successful download, remove existing builds if user confirmed
        if should_remove or skip_confirmation:
            for build_dir in existing_builds:
                try:
                    # console.print(f"Removing {build_dir}...")
                    if build_dir.is_dir():
                        subprocess.run(["rm", "-rf", str(build_dir)], check=True)
                    else:
                        build_dir.unlink()
                except (subprocess.CalledProcessError, OSError) as e:
                    # console.print(
                    #     f"Failed to remove {build_dir}: {e}", style="bold red"
                    # )
                    if download_path.exists():
                        download_path.unlink()
                    return None

        # console.print(f"Extraction of {filename}...")
        # Extract the archive
        if not _extract_archive(download_path, download_dir, console):
            return None

        # Verify the extraction was successful and the directory exists
        if not extract_path.exists() or not extract_path.is_dir():
            # console.print(f"Extraction failed: Directory {extract_path} not found", style="bold red")
            return None

        # Create version information file only after extraction is verified successful
        _create_version_info(extract_path, build)

        # Clean up the archive file
        download_path.unlink()
        # console.print(f"Cleaned up archive file for {build.version}")

        # console.print(
        #     f"Download and extraction of Blender {build.version} completed successfully"
        # )

        return build.version

    except Exception as e:
        # console.print(
        #     f"Failed to download/extract {build.version}: {e}", style="bold red"
        # )
        return None


def download_multiple_builds(
    builds: List[BlenderBuild], log_file_paths: Optional[Dict[str, str]] = None
) -> bool:
    """Download multiple builds in parallel.

    Args:
        builds: List of builds to download
        log_file_paths: Optional dictionary mapping build version to pre-created log file path.
                          If provided, these paths will be used for logging.

    Returns:
        True if all downloads were successful, False otherwise
    """
    # Create console for output
    console = Console()

    # Ensure download directory exists
    download_dir = Path(AppConfig.DOWNLOAD_PATH)
    download_dir.mkdir(parents=True, exist_ok=True)

    # Automatically clean up any incomplete download files first
    for build in builds:
        download_path = download_dir / build.file_name
        if download_path.exists():
            # console.print(f"Found incomplete download file: {build.file_name}")
            # console.print("Removing incomplete download file...")
            download_path.unlink()

    # Ask confirmation for removing existing builds BEFORE downloading
    all_existing_builds = []
    versions_to_remove: Dict[str, List[Path]] = {}

    for build in builds:
        # Only consider directories, not archive files
        existing = [
            build_dir
            for build_dir in download_dir.glob(f"blender-{build.version}*")
            if build_dir.is_dir()
        ]
        if existing:
            versions_to_remove[build.version] = existing
            all_existing_builds.extend(existing)

    should_remove = False

    if all_existing_builds:
        # console.print("\nExisting build directories found:")
        # for build_dir in all_existing_builds:
        #     console.print(f"  - {build_dir}")

        # Get confirmation for updates
        if not Confirm.ask(
            "This will remove existing builds and download updates. Proceed?",
            default=True,
        ):
            # console.print("Download cancelled")
            return False

        should_remove = True

    # console.print(f"Files will be downloaded to: {download_dir}\n")

    # Dictionary to track each download (now stores temp file paths)
    temp_log_files: Dict[str, str] = {}
    completed_versions = []
    successful_downloads = []

    try:
        # Create a separate thread for each download
        threads = []
        for build in builds:
            # Get the log file path for this build from the provided dictionary
            log_file_path = (
                log_file_paths.get(build.version) if log_file_paths else None
            )

            if not log_file_path:
                # Handle case where log path is not provided (e.g., fallback or error)
                # For now, we might skip or raise an error if paths are expected
                # If log_file_paths is None, this indicates an internal issue or misuse.
                console.print(
                    f"Error: Log file path not provided for {build.version}. Skipping download.",
                    style="bold red",
                )
                continue  # Or raise ValueError("Log file paths must be provided")

            # Create thread for this download, passing the temp file path
            thread = threading.Thread(
                target=_download_file_with_log,
                args=(build, download_dir, log_file_path, console),  # Pass the path
            )
            threads.append((thread, build))

        # Start all download threads
        for thread, _ in threads:
            thread.start()

        # Wait for all threads to complete
        for thread, _ in threads:
            thread.join()

        # Check results and collect successful downloads
        for _, build in threads:
            log_file_path = (
                log_file_paths.get(build.version) if log_file_paths else None
            )
            if log_file_path:
                log_path = Path(log_file_path)  # Work with Path object
                if log_path.exists():
                    if _check_download_success(str(log_path)):  # Pass string path
                        download_path = download_dir / build.file_name
                        successful_downloads.append((build, download_path))
                    else:
                        # console.print(
                        #     f"Download of {build.version} failed (check log {log_file_path}).", style="bold red"
                        # ) # Optional: print log path on failure
                        pass
                else:
                    # console.print(
                    #     f"Log file {log_file_path} missing for {build.version}.", style="bold red"
                    # )
                    pass
            else:
                # This case might happen if temp file creation failed earlier
                # console.print(
                #     f"No log file path tracked for {build.version}.", style="bold red"
                # )
                pass

        # If we got successful downloads and should remove existing builds
        if successful_downloads and should_remove:
            # console.print("\nRemoving existing builds...")
            for version, paths in versions_to_remove.items():
                # console.print(f"Removing existing Blender {version} build(s)...")
                for build_dir in paths:
                    try:
                        # console.print(f"Removing {build_dir}...")
                        if build_dir.is_dir():
                            subprocess.run(["rm", "-rf", str(build_dir)], check=True)
                        else:
                            build_dir.unlink()
                    except (subprocess.CalledProcessError, OSError) as e:
                        # console.print(
                        #     f"Failed to remove {build_dir}: {e}", style="bold red"
                        # )
                        pass

        # Now extract all successful downloads
        for build, download_path in successful_downloads:
            try:
                filename = build.file_name
                extracted_dir_name = filename.replace(".tar.xz", "")
                extract_path = download_dir / extracted_dir_name

                # console.print(f"Extraction of {filename}...")
                if _extract_archive(download_path, download_dir, console):
                    # Verify the extraction was successful and the directory exists
                    if not extract_path.exists() or not extract_path.is_dir():
                        # console.print(f"Extraction failed: Directory {extract_path} not found", style="bold red")
                        pass
                    else:
                        # Create version information file only after extraction is verified successful
                        _create_version_info(extract_path, build)

                        # Clean up the archive file
                        download_path.unlink()
                        # console.print(f"Cleaned up archive file for {build.version}")

                        # console.print(
                        #     f"Extraction of Blender {build.version} completed successfully"
                        # )
                        completed_versions.append(build.version)
            except Exception as e:
                # console.print(
                #     f"Extraction of {build.version} failed: {e}", style="bold red"
                # )
                pass

        if completed_versions:
            # console.print(
            #     f"\nCompleted downloading {len(completed_versions)} builds: {', '.join(completed_versions)}"
            # )
            return True
        else:
            # console.print(
            #     "\nNo builds were downloaded successfully", style="bold yellow"
            # )
            return False

    except KeyboardInterrupt:
        # console.print(
        #     "\nDownloads interrupted by user. Cleaning up...", style="bold yellow"
        # )
        # Clean up temp files
        for log_path_str in temp_log_files.values():
            log_file = Path(log_path_str)
            if log_file.exists():
                try:
                    log_file.unlink()
                except OSError:
                    pass  # Ignore cleanup errors on interrupt
        # We can't cancel the downloads directly, but we can inform the user
        # console.print(
        #     "Note: Download processes may still be running in the background."
        # )
        # console.print("You may need to manually kill wget processes.")
        return False

    except Exception as e:
        # console.print(f"\nAn error occurred during downloads: {e}", style="bold red")
        # Clean up temp files
        for log_path_str in temp_log_files.values():
            log_file = Path(log_path_str)
            if log_file.exists():
                try:
                    log_file.unlink()
                except OSError:
                    pass  # Ignore cleanup errors on exception
        return False


def _download_file_with_log(
    build: BlenderBuild, download_dir: Path, log_file_path: str, console: Console
) -> bool:
    """Download a file using wget with output to a log file.

    Args:
        build: The build to download
        download_dir: Directory to save the file
        log_file_path: Path to log file for download progress
        console: Console for output

    Returns:
        True if download was successful
    """
    try:
        filename = build.file_name
        # console.print(f"[bold]Starting download of {filename}...[/bold]")

        # console.print("[bold]Using wget for download[/bold]")
        # Open the log file path in append mode for wget logging
        with open(log_file_path, "w") as f:  # Use the provided path
            # Set environment variables for consistent output format
            env = os.environ.copy()
            env["LC_ALL"] = "C"

            process = subprocess.run(
                [
                    "wget",
                    "--verbose",  # Use verbose to get more progress info
                    "--progress=bar:force:noscroll",  # Force progress bar
                    "--show-progress",  # Always show progress
                    "-P",
                    str(download_dir),  # Download directory
                    build.url,
                ],
                stdout=f,
                stderr=subprocess.STDOUT,
                check=True,
                env=env,  # Use our modified environment
            )

        return True
    except subprocess.CalledProcessError as e:
        # Append error to the existing log file
        try:
            with open(log_file_path, "a") as f:  # Use the provided path
                f.write(f"\nDownload failed: {e}\n")
        except IOError:
            pass  # Ignore if we can't write to log
        return False
    except Exception as e:  # Catch other potential errors like file opening issues
        # Append error to the existing log file
        try:
            with open(log_file_path, "a") as f:  # Use the provided path
                f.write(f"\nAn error occurred during download setup: {e}\n")
        except IOError:
            pass  # Ignore if we can't write to log
        return False


def _get_progress_and_speed_from_log(log_file_path: str) -> Optional[Tuple[float, str]]:
    """Extract download progress percentage and speed from log file.

    Args:
        log_file_path: Path to log file

    Returns:
        Tuple containing progress percentage (0-100) and speed in bytes/second
    """
    try:
        log_path = Path(log_file_path)
        # Ensure file exists before opening
        if not log_path.exists():
            return None
        with log_path.open("r") as f:
            content = f.read()

        import re

        # Look for wget progress format
        # Wget can output progress in a few different formats:
        # 1. Basic format: 43% [=======>              ] 12,345,678   1.23M/s eta 45s
        # 2. Alternative format: 43% [=======>              ] 12.3M/45.6M 1.23M/s eta 45s
        # 3. Verbose format:  2% [>                                                  ] 1,176,576   2.83MB/s    eta 2m 29s

        # Extract percentage first (simplest regex)
        percentage_pattern = r"(\d+)%"
        percentage_matches = re.findall(percentage_pattern, content)

        if percentage_matches:
            percentage = float(percentage_matches[-1])

            # Then look for different speed formats
            # Try multiple common formats with increasing flexibility
            speed_patterns = [
                r"(\d+[\.,]?\d*\s?[KMG]B/s)",  # 3.85MB/s format
                r"(\d+[\.,]?\d*\s?[KMG]i?B/s)",  # 3.85MiB/s format
                r"(\d+[\.,]?\d*\s?[kmg]/s)",  # 3.85M/s format (case insensitive)
                r"(\d[\d\.,]*\s*[KMG][Ii]?B/s)",  # More flexible pattern
                r"(\d[\d\.,]*\s*[KMG]/s)",  # Generic speed pattern
            ]

            # Search for each pattern
            for pattern in speed_patterns:
                speed_matches = re.findall(pattern, content, re.IGNORECASE)
                if speed_matches:
                    # Return the speed in its original format for wget
                    return (percentage, speed_matches[-1])

            # If we found percentage but no speed, at least return the percentage
            return (percentage, "")

        # If we can't find a percentage, check if download might be starting
        if "Starting download" in content or "Downloading" in content:
            return (0.0, "Starting...")

        return None
    except Exception as e:
        # If we encounter any error, return None
        return None


def _check_download_success(log_file_path: str) -> bool:
    """Check if download completed successfully based on log file.

    Args:
        log_file_path: Path to log file

    Returns:
        True if download was successful
    """
    try:
        log_path = Path(log_file_path)
        # Ensure file exists before opening
        if not log_path.exists():
            return False
        with log_path.open("r") as f:
            content = f.read()

        # Check for wget success (no error message and high percentage)
        if "ERROR" not in content and "failed" not in content.lower():
            # Check last percentage if available
            result = _get_progress_and_speed_from_log(log_file_path)
            if result:
                percentage, speed = result
                if percentage > 99:
                    return True

        return False
    except Exception:
        return False


def _download_file(url: str, download_dir: Path, console: Console) -> bool:
    """Download a file using wget.

    Args:
        url: URL to download
        download_dir: Directory to save the file
        console: Console for output

    Returns:
        True if download was successful
    """
    try:
        # console.print("[bold]Using wget for download[/bold]")
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
        # console.print(f"Download failed: {e}", style="bold red")
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
        # console.print(f"Extracting {archive_path.name}...")
        subprocess.run(
            ["tar", "-xf", str(archive_path), "-C", str(target_dir)], check=True
        )
        return True
    except subprocess.CalledProcessError as e:
        # console.print(f"Extraction failed: {e}", style="bold red")
        return False


def _create_version_info(extract_path: Path, build: BlenderBuild) -> None:
    """Create a version.json file in the extracted directory with build information.

    Args:
        extract_path: Path to the extracted build directory
        build: Build information to save
    """
    # Perform a more thorough check to ensure the extraction was successful
    if not extract_path.exists() or not extract_path.is_dir():
        return

    # Check if the blender executable exists, confirming it's a valid Blender installation
    blender_executable = extract_path / "blender"
    if not blender_executable.exists():
        return

    try:
        # Calculate directory size
        directory_size = _calculate_directory_size(extract_path)

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
            "directory_size": directory_size,  # Add the calculated directory size
            "hash": build.hash,  # Add hash information
        }

        # Write the information to a JSON file
        version_file = extract_path / "version.json"
        with open(version_file, "w") as f:
            json.dump(build_info, f, indent=2)
    except Exception:
        # If anything goes wrong during version.json creation, don't leave a partial file
        version_file = extract_path / "version.json"
        if version_file.exists():
            try:
                version_file.unlink()
            except:
                pass


def _calculate_directory_size(path: Path) -> int:
    """Calculate the total size of a directory.

    Args:
        path: Directory path

    Returns:
        Total size in bytes
    """
    total_size = 0
    # Use Path.rglob to recursively find all files
    for file_path in path.rglob("*"):
        # Only include files (not directories) and skip symlinks
        if file_path.is_file() and not file_path.is_symlink():
            total_size += file_path.stat().st_size

    return total_size
