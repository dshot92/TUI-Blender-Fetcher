import select
import sys
import termios
import tty
from typing import Optional


def getch(timeout=0.1) -> Optional[str]:
    """Read a single character from stdin without requiring the user to hit enter.

    Simple approach that works with basic terminal input.

    Args:
        timeout: Time to wait for input in seconds

    Returns:
        The character read, or None if no input is available before timeout
    """
    fd = sys.stdin.fileno()
    old_settings = termios.tcgetattr(fd)
    try:
        tty.setraw(fd)

        # Check if input is available
        rlist, _, _ = select.select([sys.stdin], [], [], timeout)
        if not rlist:
            return None

        # Read a single character
        char = sys.stdin.read(1)

        # Handle escape sequences (arrow keys)
        if char == "\x1b":  # Escape character
            # Check if there are more characters available (for arrow keys)
            rlist, _, _ = select.select([sys.stdin], [], [], 0.01)
            if rlist:
                next_char = sys.stdin.read(1)
                if next_char == "[":  # Arrow keys start with \x1b[
                    # Read the final character that identifies the arrow key
                    final_char = sys.stdin.read(1)
                    return f"\033[{final_char}"

        return char
    finally:
        # Restore terminal settings
        termios.tcsetattr(fd, termios.TCSADRAIN, old_settings)
