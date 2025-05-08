//go:build windows
// +build windows

package launch

import (
	"fmt"
	"os/exec"
)

// BlenderInNewTerminal launches Blender in a new terminal window (Windows-specific)
func BlenderInNewTerminal(blenderExe string) error {
	cmd := exec.Command("cmd", "/C", "start", "", blenderExe, "-con")
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to launch Blender in new terminal: %w", err)
	}
	return nil
}
