//go:build !windows

package server

import (
	"fmt"
	"os"
	"syscall"
)

const (
	masterSocketDirPerm      = 0o700
	masterSocketFilePerm     = 0o600
	masterSocketDirPermMask  = 0o077
	masterSocketFilePermMask = 0o077
)

func prepareMasterSocketDir(dir string) error {
	if err := os.MkdirAll(dir, masterSocketDirPerm); err != nil {
		return fmt.Errorf("create master socket dir %s failed: %w", dir, err)
	}
	if err := os.Chmod(dir, masterSocketDirPerm); err != nil {
		return fmt.Errorf("chmod master socket dir %s failed: %w", dir, err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("stat master socket dir %s failed: %w", dir, err)
	}
	if info.Mode().Perm()&masterSocketDirPermMask != 0 {
		return fmt.Errorf("master socket dir %s permission %#o is too permissive", dir, info.Mode().Perm())
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		if int(stat.Uid) != os.Geteuid() {
			return fmt.Errorf(
				"master socket dir %s owner uid %d does not match current process uid %d",
				dir, stat.Uid, os.Geteuid(),
			)
		}
	}
	return nil
}

func secureMasterSocket(sockPath string) error {
	if err := os.Chmod(sockPath, masterSocketFilePerm); err != nil {
		return fmt.Errorf("chmod master socket %s failed: %w", sockPath, err)
	}
	info, err := os.Stat(sockPath)
	if err != nil {
		return fmt.Errorf("stat master socket %s failed: %w", sockPath, err)
	}
	if info.Mode().Perm()&masterSocketFilePermMask != 0 {
		return fmt.Errorf("master socket %s permission %#o is too permissive", sockPath, info.Mode().Perm())
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	if int(stat.Uid) != os.Geteuid() {
		return fmt.Errorf(
			"master socket %s owner uid %d does not match current process uid %d",
			sockPath, stat.Uid, os.Geteuid(),
		)
	}
	return nil
}
