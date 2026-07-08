//go:build windows

package service

import (
	"os/exec"
	"strconv"
)

func configureLongRunningCommand(cmd *exec.Cmd) {
}

func killLongRunningCommand(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	// Terminate the whole process tree. A bare Process.Kill only ends the
	// directly-spawned process, leaving any grandchildren (interpreters,
	// forked workers) orphaned and holding ports/handles/memory. taskkill /T
	// walks and kills the tree; fall back to Process.Kill if taskkill fails.
	if err := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid)).Run(); err != nil {
		_ = cmd.Process.Kill()
	}
}
