#!/usr/bin/env python3
"""
Main entry point for the Blender Fetcher application.
Uses Rich for a more interactive terminal UI.
"""

from .ui.tui import BlenderTUI


def main():
    """Run the Blender Fetcher application."""
    tui = BlenderTUI()
    tui.run()
