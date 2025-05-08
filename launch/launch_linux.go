//go:build linux
// +build linux

package launch

import (
	"fmt"
	"os/exec"
	"syscall"
)

// BlenderInNewTerminal launches Blender in a new terminal window (Linux-specific)
func BlenderInNewTerminal(blenderExe string) error {
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
		cmd := exec.Command(term.name, term.args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}
		err := cmd.Start()
		if err == nil {
			cmd.Process.Release()
			return nil
		}
	}

	return fmt.Errorf("failed to launch Blender: no terminal emulator worked")
}
