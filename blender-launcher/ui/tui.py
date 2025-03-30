import signal
import subprocess
from dataclasses import dataclass, field
from pathlib import Path
from typing import Dict, List, Optional, Set, Tuple, Union, Any, Callable
import sys
import time

from rich.console import Console
from rich.table import Table
from rich.text import Text

from ..api.builder_api import fetch_builds
from ..config.app_config import AppConfig
from ..models.build_info import BlenderBuild, LocalBuildInfo
from ..utils.input import (
    get_keypress,
    prompt_input,
    KEY_UP,
    KEY_DOWN,
    KEY_LEFT,
    KEY_RIGHT,
    KEY_ENTER,
    KEY_ENTER_ALT,
    KEY_SPACE,
)
from ..utils.local_builds import get_local_builds, delete_local_build


@dataclass
class UIState:
    """Holds the state of the TUI."""

    builds: List[BlenderBuild] = field(default_factory=list)
    selected_builds: Set[int] = field(default_factory=set)
    current_page: str = "builds"  # "builds" or "settings"
    cursor_position: int = 0  # Current cursor position for navigation
    settings_cursor: int = 0  # Cursor position in settings page
    needs_refresh: bool = False  # Flag to indicate if a refresh is needed
    has_fetched: bool = False  # Flag to track if we've fetched builds from online
    local_builds: Dict[str, LocalBuildInfo] = field(default_factory=dict)
    # Sort configuration
    sort_column: int = (
        2  # Default sort on Version column (index 2 after removing cursor column)
    )
    sort_reverse: bool = True  # Sort descending by default
    # Dictionary to store build numbers that persist through sorting
    build_numbers: Dict[str, int] = field(default_factory=dict)
    # Download progress tracking
    download_progress: Dict[str, float] = field(default_factory=dict)
    download_speed: Dict[str, str] = field(default_factory=dict)


class BlenderTUI:
    """Text User Interface for Blender build management."""

    def __init__(self):
        self.console = Console()
        self.state = UIState()

        # Key handlers for different pages
        self.page_handlers = {
            "builds": self._handle_builds_page_input,
            "settings": self._handle_settings_page_input,
        }

        # Set up a signal handler for window resize
        signal.signal(signal.SIGWINCH, self._handle_resize)

        # Load initial local builds
        self.state.local_builds = get_local_builds()
        self._initialize_build_numbers()

    def _initialize_build_numbers(self) -> None:
        """Initialize build numbers based on available local builds."""
        if not self.state.build_numbers and self.state.local_builds:
            sorted_local_versions = sorted(
                self.state.local_builds.keys(),
                key=lambda v: tuple(map(int, v.split("."))),
                reverse=True,
            )
            for i, version in enumerate(sorted_local_versions):
                self.state.build_numbers[version] = i + 1

    def _handle_resize(self, *args) -> None:
        """Handle terminal resize events by setting a flag to refresh the UI."""
        self.state.needs_refresh = True

    def clear_screen(self) -> None:
        """Clear the terminal screen."""
        self.console.clear()

    def print_navigation_bar(self) -> None:
        """Print a navigation bar with keybindings at the bottom of the screen."""
        if self.state.current_page == "builds":
            # Split the commands into two rows for better readability
            self.console.print("─" * 80)  # Simple separator

            # Use explicit Text objects to control styling and prevent auto-detection
            self.console.print(
                "[bold]Space[/bold]:Select  [bold]D[/bold]:Download  [bold]Enter[/bold]:Launch  [bold]F[/bold]:Fetch Online Builds",
                highlight=False,
            )
            if not self.state.has_fetched and self.state.local_builds:
                # Only show X key for deleting when viewing local builds
                self.console.print(
                    "[bold]X[/bold]:Delete  [bold]R[/bold]:Reverse Sort  [bold]S[/bold]:Settings  [bold]Q[/bold]:Quit",
                    highlight=False,
                )
            else:
                self.console.print(
                    "[bold]R[/bold]:Reverse Sort  [bold]S[/bold]:Settings  [bold]Q[/bold]:Quit",
                    highlight=False,
                )
        else:  # settings page
            self.console.print("─" * 80)  # Simple separator
            self.console.print(
                "[bold]Enter[/bold]:Edit  [bold]S[/bold]:Back to builds  [bold]Q[/bold]:Quit",
                highlight=False,
            )

    def print_build_table(self) -> None:
        """Print a table of available Blender builds."""
        self.state.local_builds = get_local_builds()
        self._initialize_build_numbers()

        # Column names for sorting indication
        column_names = [
            "#",
            "",
            "Version",
            "Branch",
            "Type",
            "Size",
            "Build Date",
        ]

        table = self._create_table(column_names)

        # Show either online builds or local builds
        if self.state.has_fetched and self.state.builds:
            self._add_online_builds_to_table(table)
        elif not self.state.has_fetched:
            self._add_local_builds_to_table(table)

        self.console.print(table)

        # Print legend below the table ONLY when online builds are shown
        if self.state.has_fetched and self.state.builds:
            self.console.print(
                Text("■ Current Local Version", style="yellow"),
                Text("■ Update Available", style="green"),
                sep="   ",
            )

    def _create_table(self, column_names: List[str]) -> Table:
        """Create and configure the table with proper columns.

        Args:
            column_names: List of column header names

        Returns:
            Configured table object
        """
        # Use a standard box style from Rich
        from rich import box

        table = Table(show_header=True, expand=True, box=box.SIMPLE_HEAVY)

        for i, col_name in enumerate(column_names):
            # Add sort indicator to column header
            if i == self.state.sort_column:
                col_name = f"{col_name} {'↑' if not self.state.sort_reverse else '↓'}"

            # Add columns in order with left alignment
            if i == 0:  # Number column
                table.add_column(
                    col_name, justify="center", width=2
                )  # Will fit build numbers
            elif i == 1:  # Selected column
                table.add_column(
                    col_name, justify="center", width=2
                )  # Will fit [X] or [ ]
            elif i == 2:  # Version column
                table.add_column(col_name, justify="left")  # Variable width
            elif i == 3:  # Branch column
                table.add_column(col_name, justify="center")  # Will fit branch names
            elif i == 4:  # Type column
                table.add_column(col_name, justify="center")  # Will fit risk types
            elif i == 5:  # Size column
                table.add_column(col_name, justify="center")  # Will fit size values
            elif i == 6:  # Build Date column
                table.add_column(col_name, justify="center")  # Will fit dates

        return table

    def _add_online_builds_to_table(self, table: Table) -> None:
        """Add online builds to the table.

        Args:
            table: Table to add rows to
        """
        sorted_builds = self.sort_builds(self.state.builds)

        for i, build in enumerate(sorted_builds):
            selected = "[X]" if i in self.state.selected_builds else "[ ]"
            build_num = self.state.build_numbers.get(build.version, i + 1)
            type_text = self._get_style_for_risk_id(build.risk_id)

            # Check if this build is being downloaded
            is_downloading = build.version in self.state.download_progress
            download_progress = self.state.download_progress.get(build.version, 0)
            download_speed = self.state.download_speed.get(build.version, "")

            # Determine version style based on local builds or download status
            if is_downloading:
                # For downloading builds, show progress in the size column and speed in the date column
                size_text = f"{download_progress:.0f}%"
                date_text = download_speed

                # Set type column to show "downloading"
                type_text = Text("downloading", style="green bold")

                # Set row style for downloading build without red background
                row_style = "reverse bold" if i == self.state.cursor_position else ""

                # Set version with indicator
                version_col = Text(f"↓ Blender {build.version}", style="green bold")

            elif build.version in self.state.local_builds:
                local_info = self.state.local_builds[build.version]
                style = (
                    "yellow"
                    if local_info.time == build.build_time or not local_info.time
                    else "green"
                )
                version_col = Text(f"■ Blender {build.version}", style=style)
                row_style = "reverse bold" if i == self.state.cursor_position else ""
                size_text = f"{build.size_mb:.1f}MB"
                date_text = build.mtime_formatted
            else:
                version_col = Text(f"  Blender {build.version}")
                row_style = "reverse bold" if i == self.state.cursor_position else ""
                size_text = f"{build.size_mb:.1f}MB"
                date_text = build.mtime_formatted

            table.add_row(
                str(build_num),
                selected,
                version_col,
                build.branch,
                type_text,
                size_text,
                date_text,
                style=row_style,
            )

    def _add_local_builds_to_table(self, table: Table) -> None:
        """Add local builds to the table.

        Args:
            table: Table to add rows to
        """
        if not self.state.local_builds:
            self.console.print(
                "No local builds found. Press F to fetch online builds.",
                style="bold yellow",
            )
            return

        # Convert local builds dictionary to a sortable list and sort it
        local_build_list = self._get_sorted_local_build_list()

        for i, (version, build_info) in enumerate(local_build_list):
            selected = "[X]" if i in self.state.selected_builds else "[ ]"
            build_num = self.state.build_numbers.get(version, i + 1)
            row_style = "reverse bold" if i == self.state.cursor_position else ""
            type_text = (
                self._get_style_for_risk_id(build_info.risk_id)
                if build_info.risk_id
                else ""
            )

            # Format build date nicely if available
            build_date = self._format_build_date(build_info)

            table.add_row(
                str(build_num),
                selected,
                Text(f"Blender {version}"),
                build_info.branch or "",
                type_text,
                "",  # Size (unavailable for locals)
                build_date,
                style=row_style,
            )

    def _get_sorted_local_build_list(self) -> List[Tuple[str, LocalBuildInfo]]:
        """Get a sorted list of local builds based on current sort settings.

        Returns:
            List of (version, LocalBuildInfo) tuples
        """
        local_build_list = list(self.state.local_builds.items())

        # Define sort key functions for different columns
        sort_keys = {
            2: lambda x: tuple(map(int, x[0].split("."))),  # Version
            3: lambda x: x[1].branch or "",  # Branch
            4: lambda x: x[1].risk_id or "",  # Type
            6: lambda x: x[1].time or "",  # Build Date
        }

        # Use the appropriate sort key if defined, otherwise default to version
        sort_key = sort_keys.get(self.state.sort_column, sort_keys[2])

        # Sort the list
        local_build_list.sort(key=sort_key, reverse=self.state.sort_reverse)

        return local_build_list

    def _get_style_for_risk_id(self, risk_id: Optional[str]) -> Text:
        """Get styled text for risk ID.

        Args:
            risk_id: The risk ID string or None

        Returns:
            Styled Text object
        """
        if not risk_id:
            return Text("")

        risk_styles = {
            "stable": "blue",
            "alpha": "magenta",
            "candidate": "cyan",
        }

        style = risk_styles.get(risk_id, "")
        return Text(risk_id, style=style)

    def _format_build_date(self, build_info: LocalBuildInfo) -> str:
        """Format build date from build info.

        Args:
            build_info: LocalBuildInfo object

        Returns:
            Formatted date string
        """
        if build_info.build_date:
            return build_info.build_date

        if build_info.time and len(build_info.time) == 13:
            try:
                from datetime import datetime

                return datetime.strptime(build_info.time, "%Y%m%d_%H%M").strftime(
                    "%Y-%m-%d %H:%M"
                )
            except ValueError:
                pass

        return build_info.time or "Unknown"

    def print_settings_table(self) -> None:
        """Print a table for settings configuration."""
        # Simpler approach to avoid Rich formatting errors
        self.console.print("[bold]Settings[/bold]")
        self.console.print("─" * 80)

        settings = [
            ("Download Directory:", str(AppConfig.DOWNLOAD_PATH)),
            ("Version Cutoff:", AppConfig.VERSION_CUTOFF),
        ]

        for i, (setting, value) in enumerate(settings):
            # Simple highlighted row for the cursor position
            if i == self.state.settings_cursor:
                self.console.print(
                    f"[reverse bold]> {setting:<20} {value}[/reverse bold]"
                )
            else:
                self.console.print(f"  {setting:<20} {value}")

        # Add some spacing at the bottom
        self.console.print()

    def display_tui(self) -> None:
        """Display the TUI using Rich's components."""
        # Only clear the screen if we're not in a download - this helps reduce flashing
        # during frequent updates of download progress
        if not self.state.download_progress:
            self.clear_screen()

        if self.state.current_page == "builds":
            if self.state.has_fetched and not self.state.builds:
                self.console.print(
                    "No builds found. Try adjusting version cutoff in Settings.",
                    style="bold yellow",
                )
            else:
                self.print_build_table()
        else:  # settings page
            self.clear_screen()  # Always clear for settings page
            self.print_settings_table()

        # Move navigation bar to bottom
        self.print_navigation_bar()

    def _fetch_online_builds(self) -> bool:
        """Fetch builds from the Blender server.

        Returns:
            True to continue running
        """
        # Clear screen to avoid UI interference
        self.clear_screen()
        self.console.print("[bold green]Fetching Blender builds...[/bold green]")

        try:
            self.state.builds = fetch_builds()
            self.state.has_fetched = True
            self.state.cursor_position = 0  # Reset cursor position
            self.console.print("Successfully fetched builds", style="green")
        except Exception as e:
            self.console.print(f"Error fetching builds: {e}", style="bold red")
            return True  # Continue running even if fetch fails

        # After successful fetch, ensure we refresh the display
        self.display_tui()
        return True

    def launch_blender(self) -> Optional[bool]:
        """Launch the selected Blender version.

        Returns:
            True if Blender was launched successfully, False to exit, None to continue
        """
        if not self.state.local_builds:
            self.console.print("No local builds found to launch.", style="bold red")
            return None

        # Determine which build to launch based on cursor position
        if self.state.has_fetched:
            # When showing online builds
            sorted_builds = self.sort_builds(self.state.builds)
            if self.state.cursor_position >= len(sorted_builds):
                self.console.print("Invalid selection.", style="bold red")
                return None

            selected_build = sorted_builds[self.state.cursor_position]
            version = selected_build.version

            # Check if this version exists locally
            if version not in self.state.local_builds:
                self.console.print(
                    f"Blender {version} is not installed locally.", style="bold red"
                )
                return None
        else:
            # When showing local builds only
            # Get the version from the sorted list
            local_build_list = self._get_sorted_local_build_list()

            if self.state.cursor_position >= len(local_build_list):
                self.console.print("Invalid selection.", style="bold red")
                return None

            # Get the version at the cursor position from sorted local builds
            version = local_build_list[self.state.cursor_position][0]

        # Find the directory for this version
        build_dir = self._find_build_directory(version)
        if not build_dir:
            self.console.print(
                f"Could not find installation directory for Blender {version}.",
                style="bold red",
            )
            return None

        # Path to the blender executable
        blender_executable = build_dir / "blender"

        if not blender_executable.exists():
            self.console.print(
                f"Blender executable not found at {blender_executable}.",
                style="bold red",
            )
            return None

        # Launch Blender in the background
        try:
            self.console.print(f"Launching Blender {version}...", style="bold green")
            subprocess.Popen([str(blender_executable)], start_new_session=True)
            # Return True to indicate successful launch and exit the application
            return True
        except Exception as e:
            self.console.print(f"Failed to launch Blender: {e}", style="bold red")
            return None

    def _find_build_directory(self, version: str) -> Optional[Path]:
        """Find the build directory for a specific version.

        Args:
            version: Blender version string

        Returns:
            Path to the build directory if found, None otherwise
        """
        download_dir = Path(AppConfig.DOWNLOAD_PATH)

        # Look for the exact directory that has this version
        for dir_path in download_dir.glob(f"blender-{version}*"):
            if dir_path.is_dir():
                return dir_path

        return None

    def sort_builds(self, builds: List[BlenderBuild]) -> List[BlenderBuild]:
        """Sort builds based on current sort configuration.

        Args:
            builds: List of builds to sort

        Returns:
            Sorted list of builds
        """
        if not builds:
            return []

        # Make a copy to not modify the original list
        sorted_builds = builds.copy()

        # Define sort keys for different columns
        sort_keys = {
            # Numbers are just the position, so no need for column 0
            1: lambda b: self.state.cursor_position
            in self.state.selected_builds,  # Selected
            2: lambda b: tuple(map(int, b.version.split("."))),  # Version
            3: lambda b: b.branch,  # Branch
            4: lambda b: b.risk_id,  # Type
            5: lambda b: b.size_mb,  # Size
            6: lambda b: b.file_mtime,  # Build Date
        }

        # Use the appropriate sort key if defined, otherwise don't sort
        if self.state.sort_column in sort_keys:
            sorted_builds.sort(
                key=sort_keys[self.state.sort_column], reverse=self.state.sort_reverse
            )

        return sorted_builds

    def run(self) -> None:
        """Run the TUI application."""
        try:
            # Initial display
            self.display_tui()

            # Main input loop
            running = True
            last_refresh_time = time.time()
            last_full_refresh = time.time()

            # Track UI state to minimize refreshes
            is_download_in_progress = False

            while running:
                try:
                    # Get key input with our custom key handling
                    # Use a shorter timeout when downloads are in progress to keep the UI responsive
                    # but not so short that it causes excessive refreshing
                    timeout = 0.1 if self.state.download_progress else 0.5
                    key = get_keypress(timeout=timeout)

                    current_time = time.time()

                    # Check for download state changes
                    download_active = bool(self.state.download_progress)
                    if download_active != is_download_in_progress:
                        is_download_in_progress = download_active
                        # Only do a full refresh when download state changes
                        self.clear_screen()
                        self.display_tui()
                        last_refresh_time = current_time
                        last_full_refresh = current_time
                        continue

                    # If no key was pressed, check if we need a refresh for download progress
                    if key is None:
                        if self.state.needs_refresh and self.state.download_progress:
                            # For downloads, limit refresh rate to prevent flashing
                            time_since_last_refresh = current_time - last_refresh_time
                            if (
                                time_since_last_refresh >= 0.5
                            ):  # Max 2 refreshes per second for downloads
                                # For downloads, don't clear screen, just reprint the table
                                self.update_build_table_only()
                                self.state.needs_refresh = False
                                last_refresh_time = current_time
                        elif self.state.needs_refresh:
                            # For non-download refreshes, limit to 3 per second
                            time_since_last_refresh = current_time - last_refresh_time
                            if time_since_last_refresh >= 0.3:
                                # Do a full refresh every 3 seconds max
                                time_since_full_refresh = (
                                    current_time - last_full_refresh
                                )
                                if time_since_full_refresh >= 3.0:
                                    self.clear_screen()
                                    self.display_tui()
                                    last_full_refresh = current_time
                                else:
                                    # Just reprint the table without clearing
                                    self.update_build_table_only()

                                self.state.needs_refresh = False
                                last_refresh_time = current_time
                        continue

                    # Handle the key based on the current page
                    handler = self.page_handlers.get(self.state.current_page)
                    if handler:
                        running = handler(key)
                        # Always refresh display after handling key
                        self.clear_screen()
                        self.display_tui()
                        last_refresh_time = time.time()
                        last_full_refresh = time.time()

                except Exception as inner_e:
                    self.console.print(
                        f"Error handling input: {inner_e}", style="bold red"
                    )
                    self.console.print(f"Type: {type(inner_e)}")
                    self.display_tui()
                    last_refresh_time = time.time()

        except Exception as e:
            self.console.print(f"An error occurred: {e}", style="bold red")

    def _handle_builds_page_input(self, key: str) -> bool:
        """Handle input for the builds page.

        Args:
            key: The pressed key

        Returns:
            False to exit the application, True to continue
        """
        # Map keys to handlers with string constants
        key_handlers = {
            KEY_UP: self._move_cursor_up,
            KEY_DOWN: self._move_cursor_down,
            KEY_LEFT: self._move_column_left,
            KEY_RIGHT: self._move_column_right,
            KEY_ENTER: self._launch_blender,
            KEY_ENTER_ALT: self._launch_blender,  # Handle both Enter key types
            KEY_SPACE: self._toggle_selection,
            "k": self._move_cursor_up,
            "j": self._move_cursor_down,
            "h": self._move_column_left,
            "l": self._move_column_right,
            "r": self._toggle_sort_reverse,
            "f": self._fetch_online_builds,
            "s": self._switch_to_settings,
            "q": lambda: False,  # Return False to exit
            "d": self._handle_download,
            "x": self._delete_selected_build,
            "\x03": lambda: False,  # Ctrl-C (ASCII value 3) to exit
        }

        # Check if key is in our handlers
        if key in key_handlers:
            return key_handlers[key]()

        # Handle letter keys (case-insensitive)
        if isinstance(key, str) and len(key) == 1 and key.isalpha():
            key_lower = key.lower()
            if key_lower in key_handlers:
                return key_handlers[key_lower]()

        # Handle number keys for selection
        if isinstance(key, str) and key.isdigit():
            self._handle_number_selection(int(key))

        return True  # Continue running by default

    def _handle_settings_page_input(self, key: str) -> bool:
        """Handle input for the settings page.

        Args:
            key: The pressed key

        Returns:
            False to exit the application, True to continue
        """
        # Map keys to handlers with string constants
        key_handlers = {
            KEY_UP: lambda: self._move_settings_cursor(-1),
            KEY_DOWN: lambda: self._move_settings_cursor(1),
            KEY_ENTER: self._edit_current_setting,
            KEY_ENTER_ALT: self._edit_current_setting,
            "k": lambda: self._move_settings_cursor(-1),
            "j": lambda: self._move_settings_cursor(1),
            "s": self._switch_to_builds_page,
            "q": lambda: False,  # Return False to exit
            "\x03": lambda: False,  # Ctrl-C (ASCII value 3) to exit
        }

        # Check if key is in our handlers
        if key in key_handlers:
            return key_handlers[key]()

        # Handle letter keys (case-insensitive)
        if isinstance(key, str) and len(key) == 1 and key.isalpha():
            key_lower = key.lower()
            if key_lower in key_handlers:
                return key_handlers[key_lower]()

        return True  # Continue running by default

    def _move_settings_cursor(self, direction: int) -> bool:
        """Move settings cursor up or down.

        Args:
            direction: -1 for up, 1 for down

        Returns:
            True to continue running
        """
        if direction < 0:
            # Move up
            self.state.settings_cursor = max(0, self.state.settings_cursor - 1)
        else:
            # Move down (only 2 settings now)
            self.state.settings_cursor = min(self.state.settings_cursor + 1, 1)

        self.display_tui()
        return True

    def _switch_to_builds_page(self) -> bool:
        """Switch back to builds page.

        Returns:
            True to continue running
        """
        self.state.current_page = "builds"
        self.display_tui()
        return True

    def _move_cursor_up(self) -> bool:
        """Move cursor up.

        Returns:
            True to continue running
        """
        self.state.cursor_position = max(0, self.state.cursor_position - 1)
        self.display_tui()
        return True

    def _move_cursor_down(self) -> bool:
        """Move cursor down.

        Returns:
            True to continue running
        """
        max_index = (
            len(self.state.builds) - 1
            if self.state.has_fetched
            else len(self.state.local_builds) - 1
        )
        self.state.cursor_position = min(self.state.cursor_position + 1, max_index)
        self.display_tui()
        return True

    def _move_column_left(self) -> bool:
        """Select previous column.

        Returns:
            True to continue running
        """
        if self.state.sort_column > 0:
            self.state.sort_column -= 1
            self.display_tui()
        return True

    def _move_column_right(self) -> bool:
        """Select next column.

        Returns:
            True to continue running
        """
        if self.state.sort_column < 6:
            self.state.sort_column += 1
            self.display_tui()
        return True

    def _toggle_sort_reverse(self) -> bool:
        """Reverse sort order.

        Returns:
            True to continue running
        """
        self.state.sort_reverse = not self.state.sort_reverse
        self.display_tui()
        return True

    def _toggle_selection(self) -> bool:
        """Toggle selection state of the current item.

        Returns:
            True to continue running
        """
        if not self._has_visible_builds():
            return True

        if self.state.cursor_position in self.state.selected_builds:
            self.state.selected_builds.remove(self.state.cursor_position)
        else:
            self.state.selected_builds.add(self.state.cursor_position)
        self.display_tui()
        return True

    def _has_visible_builds(self) -> bool:
        """Check if there are any builds visible in the current view."""
        return (self.state.has_fetched and self.state.builds) or (
            not self.state.has_fetched and self.state.local_builds
        )

    def _switch_to_settings(self) -> bool:
        """Switch to settings page.

        Returns:
            True to continue running
        """
        self.state.current_page = "settings"
        self.display_tui()
        return True

    def _launch_blender(self) -> bool:
        """Launch the selected Blender version.

        Returns:
            False to exit (only on builds page), True to continue
        """
        # Only attempt to launch Blender if we're on the builds page
        if self.state.current_page == "builds":
            result = self.launch_blender()
            # If launch was successful, exit the application
            return False if result else True

        # On settings page, just continue running
        return True

    def _handle_download(self) -> bool:
        """Download selected builds and show progress in the table.

        Returns:
            True to continue running
        """
        if not self.state.has_fetched or not self.state.builds:
            self.console.print("No online builds to download", style="yellow")
            return True

        builds_to_download = []
        download_indices = []

        if self.state.selected_builds:
            # Get the builds that are currently selected
            sorted_builds = self.sort_builds(self.state.builds)
            for idx in self.state.selected_builds:
                if 0 <= idx < len(sorted_builds):
                    builds_to_download.append(sorted_builds[idx])
                    download_indices.append(idx)
        else:
            # If no builds are selected, download the one under the cursor
            sorted_builds = self.sort_builds(self.state.builds)
            if 0 <= self.state.cursor_position < len(sorted_builds):
                builds_to_download.append(sorted_builds[self.state.cursor_position])
                download_indices.append(self.state.cursor_position)

        if not builds_to_download:
            self.console.print("No builds selected for download", style="yellow")
            return True

        # Initialize download tracking
        self.state.download_progress = {}
        self.state.download_speed = {}
        for build in builds_to_download:
            self.state.download_progress[build.version] = 0
            self.state.download_speed[build.version] = "Starting..."

        # Clear the screen to start fresh
        self.clear_screen()

        # Mark builds as being downloaded so they show in the table
        # Then use the standard display_tui to render the initial state
        self.display_tui()

        # Start download in background thread
        import threading

        download_thread = threading.Thread(
            target=self._download_with_progress_tracking,
            args=(builds_to_download,),
            daemon=True,
        )
        download_thread.start()

        # Return to main loop, which will show progress in the table
        # through normal refresh cycles
        return True

    def _download_with_progress_tracking(self, builds: List[BlenderBuild]) -> None:
        """Download builds and track progress for display in the main table.

        Args:
            builds: The builds to download
        """
        import time
        import threading
        import os
        from ..utils.download import (
            download_multiple_builds,
            _get_progress_and_speed_from_log,
            get_log_file_path,
        )

        # Start the downloads
        download_thread = threading.Thread(
            target=download_multiple_builds, args=(builds,), daemon=True
        )
        download_thread.start()

        # Get log file paths
        temp_log_files = {}
        for build in builds:
            temp_log_files[build.version] = get_log_file_path(build)

        # Track previous progress values
        previous_progress = {}
        for build in builds:
            previous_progress[build.version] = -1

        # Track the last time we requested a UI refresh
        last_refresh_request = time.time()

        try:
            # Give download_multiple_builds a chance to create log files
            time.sleep(1)

            running = True
            completed_builds = set()

            while running and download_thread.is_alive():
                any_progress_updated = False
                significant_change = False
                current_time = time.time()

                for build in builds:
                    if build.version in completed_builds:
                        continue

                    log_file = temp_log_files[build.version]
                    if os.path.exists(log_file):
                        result = _get_progress_and_speed_from_log(log_file)
                        if result:
                            percentage, speed = result

                            # Check if the progress has changed significantly
                            # For smoother UI, consider smaller changes significant as progress increases
                            progress_delta = abs(
                                percentage - previous_progress[build.version]
                            )
                            if (
                                progress_delta >= 5
                                or previous_progress[build.version] == -1
                                or (
                                    percentage == 100
                                    and previous_progress[build.version] < 100
                                )
                            ):
                                significant_change = True
                                previous_progress[build.version] = percentage

                            # Update progress tracking
                            self.state.download_progress[build.version] = percentage
                            self.state.download_speed[build.version] = speed
                            any_progress_updated = True

                            # Mark as completed if download is done
                            if percentage >= 100:
                                completed_builds.add(build.version)
                                self.state.download_speed[build.version] = (
                                    "Extracting..."
                                )

                # Tell main loop to update the UI if there's a significant change
                # and sufficient time has passed since last refresh
                if significant_change and (current_time - last_refresh_request >= 0.5):
                    self.state.needs_refresh = True
                    last_refresh_request = current_time

                # Check if we should exit the loop
                if not any_progress_updated and not download_thread.is_alive():
                    running = False

                # Don't check too frequently to avoid excess CPU usage
                time.sleep(0.5)

            # Final update
            for build in builds:
                self.state.download_progress[build.version] = 100
                self.state.download_speed[build.version] = "Complete"

            # Force refresh to show final status
            self.state.needs_refresh = True

            # Wait a bit for the user to see completion
            time.sleep(1)

            # Update local builds with any new downloads
            self.state.local_builds = get_local_builds()

            # Clean up and force final refresh
            self.state.download_progress = {}
            self.state.download_speed = {}
            self.state.needs_refresh = True

        except Exception as e:
            # Show error
            for build in builds:
                self.state.download_progress[build.version] = 0
                self.state.download_speed[build.version] = f"Error: {e}"

            # Force refresh to show error status
            self.state.needs_refresh = True
            time.sleep(2)

            # Clean up
            self.state.download_progress = {}
            self.state.download_speed = {}
            self.state.needs_refresh = True

    def _handle_number_selection(self, num: int) -> None:
        """Handle selection by number key.

        Args:
            num: The number key pressed
        """
        # Only move cursor if we have a valid build list
        if not self._has_visible_builds():
            return

        # Get the list we're currently viewing
        current_builds = self._get_current_build_list()

        # Find the build with the matching number in the CURRENT display order
        for i, item in enumerate(current_builds):
            # Get version from the item based on type
            if self.state.has_fetched:
                version = item.version
            else:
                version = item[0]  # First item in tuple is version

            build_num = self.state.build_numbers.get(version)
            if build_num == num:
                self.state.cursor_position = i
                # Toggle selection state
                if i in self.state.selected_builds:
                    self.state.selected_builds.remove(i)
                else:
                    self.state.selected_builds.add(i)
                break

        self.display_tui()

    def _get_current_build_list(
        self,
    ) -> Union[List[BlenderBuild], List[Tuple[str, LocalBuildInfo]]]:
        """Get the currently visible build list based on view state.

        Returns:
            Either a list of BlenderBuild objects or a list of (version, LocalBuildInfo) tuples
        """
        if self.state.has_fetched:
            # When looking at online builds, we need the sorted list
            return self.sort_builds(self.state.builds)
        else:
            # When looking at local builds, get the sorted list
            return self._get_sorted_local_build_list()

    def _edit_current_setting(self) -> bool:
        """Edit the currently selected setting.

        Returns:
            True to continue running
        """
        try:
            self.clear_screen()

            if self.state.settings_cursor == 0:  # Download path
                self._edit_download_path()
            elif self.state.settings_cursor == 1:  # Version cutoff
                self._edit_version_cutoff()

            self.display_tui()
            return True  # Return True to prevent application exit
        except ValueError as e:
            self.console.print(f"Error updating settings: {e}", style="bold red")
            self.display_tui()
            return True  # Return True to prevent application exit

    def _edit_download_path(self) -> None:
        """Edit the download path setting."""
        current = AppConfig.DOWNLOAD_PATH

        new_path = prompt_input(
            f"Enter new download path [cyan]({current})[/cyan]", default=str(current)
        )

        if new_path and new_path != str(current):
            try:
                # Update the class attribute directly
                AppConfig.DOWNLOAD_PATH = Path(new_path)
                AppConfig.save_config()  # Assuming there's a class method to save
                self.state.needs_refresh = True
            except Exception as e:
                self.console.print(
                    f"Failed to update download path: {e}", style="bold red"
                )

    def _edit_version_cutoff(self) -> None:
        """Edit the version cutoff setting."""
        current = AppConfig.VERSION_CUTOFF

        new_cutoff = prompt_input(
            f"Enter minimum Blender version to display [cyan]({current})[/cyan]",
            default=current,
        )

        if new_cutoff and new_cutoff != current:
            try:
                AppConfig.VERSION_CUTOFF = new_cutoff
                AppConfig.save_config()
                self.state.needs_refresh = True
            except Exception as e:
                self.console.print(
                    f"Failed to update version cutoff: {e}", style="bold red"
                )

    def _delete_selected_build(self) -> bool:
        """Delete the selected local build(s).

        If multiple builds are selected via toggle, delete them all.
        Only works when viewing local builds (not online builds).

        Returns:
            True to continue running
        """
        if self.state.has_fetched or not self.state.local_builds:
            return True

        local_build_list = self._get_sorted_local_build_list()

        if self.state.selected_builds:
            # Delete all selected builds
            selected_indices = sorted(self.state.selected_builds)
            versions_to_delete = []
            for idx in selected_indices:
                if 0 <= idx < len(local_build_list):
                    version, _ = local_build_list[idx]
                    versions_to_delete.append(version)
            if not versions_to_delete:
                return True

            self.clear_screen()
            prompt = (
                "Delete Blender builds: " + ", ".join(versions_to_delete) + "? (y/n)"
            )
            self.console.print(prompt, style="bold red")

            while True:
                key = get_keypress(timeout=None)
                if key is not None:
                    break

            if key.lower() != "y":
                return True

            overall_success = True
            for version in versions_to_delete:
                success = delete_local_build(version)
                if not success:
                    overall_success = False

            self.clear_screen()
            if overall_success:
                self.console.print(
                    "Deleted Blender builds: " + ", ".join(versions_to_delete),
                    style="bold green",
                )
            else:
                self.console.print(
                    "Failed to delete one or more builds", style="bold red"
                )

            self.console.print("Press any key to continue...")
            while True:
                key = get_keypress(timeout=None)
                if key is not None:
                    break

            self.state.selected_builds.clear()
            self.state.local_builds = get_local_builds()
            max_index = len(self.state.local_builds) - 1
            if max_index < 0:
                max_index = 0
            self.state.cursor_position = min(self.state.cursor_position, max_index)
            return True
        else:
            # No multiple selection, delete the build at the current cursor
            if self.state.cursor_position < 0 or self.state.cursor_position >= len(
                local_build_list
            ):
                return True
            version, _ = local_build_list[self.state.cursor_position]
            self.clear_screen()
            self.console.print(f"Delete Blender {version}? (y/n)", style="bold red")
            while True:
                key = get_keypress(timeout=None)
                if key is not None:
                    break
            if key.lower() != "y":
                return True
            success = delete_local_build(version)
            self.clear_screen()
            if success:
                if self.state.cursor_position in self.state.selected_builds:
                    self.state.selected_builds.remove(self.state.cursor_position)
                self.state.local_builds = get_local_builds()
                max_index = len(self.state.local_builds) - 1
                if max_index < 0:
                    max_index = 0
                self.state.cursor_position = min(self.state.cursor_position, max_index)
                self.console.print(f"Deleted Blender {version}", style="bold green")
            else:
                self.console.print(
                    f"Failed to delete Blender {version}", style="bold red"
                )
            self.console.print("Press any key to continue...")
            while True:
                key = get_keypress(timeout=None)
                if key is not None:
                    break
            return True

    def update_build_table_only(self) -> None:
        """Update just the build table without clearing the screen.
        Used during downloads to minimize flicker.
        """
        # Move cursor to top of console
        self.console.clear()

        # Print the table
        self.print_build_table()

        # Print the nav bar
        self.print_navigation_bar()
