//go:build windows

package cmd

import (
	"os/exec"
	"strconv"
)

func configureManagedCommand(cmd *exec.Cmd) {
}

func terminateManagedCommand(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	// Kill the entire process tree; Process.Kill alone orphans child processes
	// spawned by the managed command. See app/service/process_windows.go.
	if err := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid)).Run(); err != nil {
		_ = cmd.Process.Kill()
	}
}
