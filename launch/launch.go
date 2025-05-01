package launch

import (
	"fmt"
	"os/exec"
	"runtime"
	"syscall"
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
			{"x-terminal-emulator", []string{"-e", "nohup", blenderExe, "&"}},
			{"gnome-terminal", []string{"--", "bash", "-c", "exec " + blenderExe}},
			{"alacritty", []string{"-e", "bash", "-c", "exec " + blenderExe}},
			{"xterm", []string{"-e", "bash", "-c", "exec " + blenderExe}},
			{"konsole", []string{"-e", "bash", "-c", "exec " + blenderExe}},
		}

		for _, term := range terminals {
			cmd = exec.Command(term.name, term.args...)
			// Set process group to detach from parent
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Setpgid: true,
			}
			err := cmd.Start()
			if err == nil {
				// Detach from the process so it's not killed when parent exits
				cmd.Process.Release()
				return nil // Successfully launched
			}
			// Continue to next terminal if this one failed
		}

		return fmt.Errorf("failed to launch Blender: no terminal emulator worked")

	case "darwin":
		// On macOS, use open command which uses the default Terminal app
		cmd = exec.Command("open", "-a", "Terminal", blenderExe)

	case "windows":
		// On Windows, launch Blender directly with console mode and detached
		// Use start command which creates a new cmd window
		cmd = exec.Command("cmd", "/C", "start", blenderExe, "-con")

	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	// Start the command
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("error launching Blender: %v", err)
	}

	// Detach from the process so it can run independently
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("error detaching from Blender process: %v", err)
	}

	return nil
}
