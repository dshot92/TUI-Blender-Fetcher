import sys
from typing import Optional, Any, Dict, List, Union
from rich.prompt import Prompt, IntPrompt, Confirm
from rich.console import Console
from rich.text import Text
from rich.live import Live

# Define key constants as strings instead of using rich.key.Key
KEY_UP = "KEY_UP"
KEY_DOWN = "KEY_DOWN"
KEY_LEFT = "KEY_LEFT"
KEY_RIGHT = "KEY_RIGHT"
KEY_ENTER = "\r"  # Enter key
KEY_ENTER_ALT = "\n"  # Alternative Enter key representation
KEY_SPACE = " "
KEY_ESC = "\x1b"
KEY_TAB = "\t"

console = Console()


def get_keypress(timeout: float = 0.1) -> Optional[str]:
    """Read a single keypress with timeout.

    This is a more modern replacement for getch() using Rich's capabilities.

    Args:
        timeout: Time to wait for input in seconds

    Returns:
        The key identifier or None if no input is available
    """
    import os
    import select
    import termios
    import tty

    # Make stdin non-blocking for timeout functionality
    fd = sys.stdin.fileno()

    # Save the terminal attributes
    old_settings = termios.tcgetattr(fd)
    try:
        # Set the terminal to raw mode
        tty.setraw(fd)

        # Check if input is available
        rlist, _, _ = select.select([sys.stdin], [], [], timeout)
        if rlist:
            # Read a single character
            key = os.read(fd, 1).decode("utf-8")

            # Handle escape sequences
            if key == "\x1b":
                # Check if there are more characters in the buffer
                rlist, _, _ = select.select([sys.stdin], [], [], 0.01)
                if rlist:
                    next_char = os.read(fd, 1).decode("utf-8")
                    if next_char == "[":
                        # Arrow keys
                        code = os.read(fd, 1).decode("utf-8")
                        if code == "A":
                            return KEY_UP
                        elif code == "B":
                            return KEY_DOWN
                        elif code == "C":
                            return KEY_RIGHT
                        elif code == "D":
                            return KEY_LEFT
                return KEY_ESC

            # Handle Enter, Space, etc.
            if key == "\r" or key == "\n":
                return KEY_ENTER
            elif key == " ":
                return KEY_SPACE
            elif key == "\t":
                return KEY_TAB

            return key
        else:
            return None
    finally:
        # Restore terminal settings
        termios.tcsetattr(fd, termios.TCSADRAIN, old_settings)


def prompt_input(prompt_text: str, default: str = "") -> str:
    """Get text input from the user with a styled prompt.

    Args:
        prompt_text: The prompt to display
        default: Default value

    Returns:
        The user input text
    """
    result = ""
    try:
        # Clear the current line and show prompt without the default Rich prompt text
        console.print(f"{prompt_text}: ", end="", highlight=False)
        # Use sys.stdin directly to avoid Rich's special handling
        result = sys.stdin.readline().strip()
        return result if result else default
    except (KeyboardInterrupt, EOFError):
        return default


def prompt_integer(prompt_text: str, default: int = 0) -> int:
    """Get integer input from the user with validation.

    Args:
        prompt_text: The prompt to display
        default: Default value

    Returns:
        The validated integer input
    """
    while True:
        try:
            console.print(f"{prompt_text} [{default}]: ", end="", highlight=False)
            result = sys.stdin.readline().strip()
            if not result and default is not None:
                return default
            return int(result)
        except ValueError:
            console.print("Please enter a valid integer.", style="bold red")
        except (KeyboardInterrupt, EOFError):
            return default


def prompt_confirm(prompt_text: str, default: bool = False) -> bool:
    """Get a yes/no confirmation from the user.

    Args:
        prompt_text: The prompt to display
        default: Default value

    Returns:
        True for yes, False for no
    """
    default_text = "Y/n" if default else "y/N"
    while True:
        try:
            console.print(f"{prompt_text} [{default_text}]: ", end="", highlight=False)
            result = sys.stdin.readline().strip().lower()
            if not result:
                return default
            if result in ("y", "yes"):
                return True
            if result in ("n", "no"):
                return False
            console.print("Please enter 'y' or 'n'.", style="bold red")
        except (KeyboardInterrupt, EOFError):
            return default
