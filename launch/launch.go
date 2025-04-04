package launch

import (
	"fmt"
	"os/exec"
	"runtime"
)

// BlenderInNewTerminal launches Blender in a new terminal window
// blenderExe is the path to the Blender executable
// Returns error if launch fails
func BlenderInNewTerminal(blenderExe string) error {
	// Command and arguments vary by OS
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		// Try multiple common terminal emulators on Linux
		terminals := []struct {
			name string
			args []string
		}{
			{"x-terminal-emulator", []string{"-e", blenderExe}},
			{"gnome-terminal", []string{"--", blenderExe}},
			{"alacritty", []string{"-e", blenderExe}},
			{"xterm", []string{"-e", blenderExe}},
			{"konsole", []string{"-e", blenderExe}},
		}

		for _, term := range terminals {
			cmd = exec.Command(term.name, term.args...)
			err := cmd.Run()
			if err == nil {
				return nil // Successfully launched
			}
			// Continue to next terminal if this one failed
		}

		return fmt.Errorf("failed to launch Blender: no terminal emulator worked")

	case "darwin":
		// On macOS, use open command which uses the default Terminal app
		cmd = exec.Command("open", "-a", "Terminal", blenderExe)

	case "windows":
		// On Windows, launch Blender directly with console mode
		// This runs Blender with its console window visible without opening multiple terminals
		cmd = exec.Command(blenderExe, "-con")

	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	// Start the command
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("error launching Blender: %v", err)
	}

	return nil
}
