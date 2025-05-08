//go:build darwin
// +build darwin

package launch

import (
	"fmt"
	"os/exec"
)

// BlenderInNewTerminal launches Blender in a new terminal window (macOS-specific)
func BlenderInNewTerminal(blenderExe string) error {
	cmd := exec.Command("open", "-a", "Terminal", blenderExe)
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to launch Blender in new terminal: %w", err)
	}
	return nil
}
