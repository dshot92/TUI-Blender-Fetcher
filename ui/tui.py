import signal
import subprocess
from dataclasses import dataclass, field
from pathlib import Path
from typing import Dict, List, Optional, Set, Tuple, Union, Any, Callable

from rich.console import Console
from rich.table import Table
from rich.text import Text
from rich.live import Live
from rich.panel import Panel

from ..api.builder_api import fetch_builds
from ..config.app_config import AppConfig, Colors
from ..models.build_info import BlenderBuild, LocalBuildInfo
from ..utils.download import download_multiple_builds
from ..utils.input import (
    get_keypress,
    prompt_input,
    prompt_integer,
    prompt_confirm,
    KEY_UP,
    KEY_DOWN,
    KEY_LEFT,
    KEY_RIGHT,
    KEY_ENTER,
    KEY_ENTER_ALT,
    KEY_SPACE,
    KEY_ESC,
)
from ..utils.local_builds import get_local_builds


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
        table = Table(show_header=True, expand=True)

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

            # Determine version style based on local builds
            if build.version in self.state.local_builds:
                local_info = self.state.local_builds[build.version]
                style = (
                    "yellow"
                    if local_info.time == build.build_time or not local_info.time
                    else "green"
                )
                version_col = Text(f"■ Blender {build.version}", style=style)
            else:
                version_col = Text(f"  Blender {build.version}")

            row_style = "reverse bold" if i == self.state.cursor_position else ""

            table.add_row(
                str(build_num),
                selected,
                version_col,
                build.branch,
                type_text,
                f"{build.size_mb:.1f}MB",
                build.mtime_formatted,
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
            ("Download Directory:", AppConfig.DOWNLOAD_PATH),
            ("Version Cutoff:", AppConfig.VERSION_CUTOFF),
            ("Max Workers:", str(AppConfig.MAX_WORKERS)),
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

            # Don't use Live context since it's conflicting with our custom input handling
            while running:
                try:
                    # Get key input with our custom key handling
                    key = get_keypress(timeout=0.1)

                    # If no key was pressed, check if we need a refresh
                    if key is None:
                        if self.state.needs_refresh:
                            self.display_tui()
                            self.state.needs_refresh = False
                        continue

                    # Handle the key based on the current page
                    handler = self.page_handlers.get(self.state.current_page)
                    if handler:
                        running = handler(key)
                        # Always refresh display after handling key
                        self.display_tui()

                except Exception as inner_e:
                    self.console.print(
                        f"Error handling input: {inner_e}", style="bold red"
                    )
                    self.console.print(f"Type: {type(inner_e)}")
                    self.display_tui()

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
            KEY_ENTER_ALT: self._edit_current_setting,  # Handle both Enter key types
            "k": lambda: self._move_settings_cursor(-1),
            "j": lambda: self._move_settings_cursor(1),
            "s": self._switch_to_builds_page,
            "q": lambda: False,  # Return False to exit
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
            # Move down (3 settings total)
            self.state.settings_cursor = min(self.state.settings_cursor + 1, 2)

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
            False to exit, True to continue
        """
        result = self.launch_blender()
        # If launch was successful, exit the application
        return False if result else True

    def _handle_download(self) -> bool:
        """Download selected builds.

        Returns:
            True to continue running
        """
        if not self.state.has_fetched or not self.state.builds:
            self.console.print("No online builds to download", style="yellow")
            return True

        builds_to_download = []

        if self.state.selected_builds:
            # Get the builds that are currently selected
            sorted_builds = self.sort_builds(self.state.builds)
            for idx in self.state.selected_builds:
                if 0 <= idx < len(sorted_builds):
                    builds_to_download.append(sorted_builds[idx])
        else:
            # If no builds are selected, download the one under the cursor
            sorted_builds = self.sort_builds(self.state.builds)
            if 0 <= self.state.cursor_position < len(sorted_builds):
                builds_to_download.append(sorted_builds[self.state.cursor_position])

        if not builds_to_download:
            self.console.print("No builds selected for download", style="yellow")
            return True

        # Confirm downloads with the user
        versions = ", ".join(b.version for b in builds_to_download)
        if not prompt_confirm(f"Download Blender {versions}?", default=True):
            self.console.print("Download cancelled", style="yellow")
            return True

        # Clear screen before starting download to avoid UI conflicts
        self.clear_screen()
        self.console.print(
            f"[bold green]Starting download of {len(builds_to_download)} build(s)...[/bold green]"
        )

        # Perform download without status spinner
        success = download_multiple_builds(builds_to_download)
        if success:
            self.console.print("Download completed successfully", style="green")
            # Refresh local builds list
            self.state.local_builds = get_local_builds()
        else:
            self.console.print("Download failed", style="bold red")

        self.display_tui()
        return True

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

    def _edit_current_setting(self) -> None:
        """Edit the currently selected setting."""
        try:
            self.clear_screen()

            if self.state.settings_cursor == 0:  # Download path
                self._edit_download_path()
            elif self.state.settings_cursor == 1:  # Version cutoff
                self._edit_version_cutoff()
            elif self.state.settings_cursor == 2:  # Max workers
                self._edit_max_workers()

            self.display_tui()
        except ValueError as e:
            self.console.print(f"Error updating settings: {e}", style="bold red")
            self.display_tui()

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

    def _edit_max_workers(self) -> None:
        """Edit the max workers setting."""
        current = AppConfig.MAX_WORKERS

        new_workers = prompt_integer(
            f"Enter maximum download workers [cyan]({current})[/cyan]", default=current
        )

        if new_workers and new_workers != current:
            try:
                AppConfig.MAX_WORKERS = new_workers
                AppConfig.save_config()
                self.state.needs_refresh = True
            except Exception as e:
                self.console.print(
                    f"Failed to update max workers: {e}", style="bold red"
                )
