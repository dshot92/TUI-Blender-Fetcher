//go:build windows
// +build windows

package local

import "os/exec"

func detachProcess(cmd *exec.Cmd) {
	// On Windows, we don't need to do anything special
} 