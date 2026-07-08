//go:build !windows

package service

import (
	"fmt"
	"syscall"

	"github.com/1Panel-dev/1Panel/core/global"
)

// checkUpgradeSpace refuses to start an upgrade when the install partition has
// less than minUpgradeFreeSpace available (upstream behavior). syscall.Statfs is
// unix-only, so the check lives in a build-tagged file; see the windows variant
// for the no-op fallback that keeps core cross-compiling for the Windows agent.
func checkUpgradeSpace() error {
	dir := global.CONF.Base.InstallDir
	var stat syscall.Statfs_t
	if err := syscall.Statfs(dir, &stat); err != nil {
		return err
	}
	avail := stat.Bavail * uint64(stat.Bsize)
	if avail < minUpgradeFreeSpace {
		return fmt.Errorf("available space of %s is %d MB, less than required 500MB", dir, avail>>20)
	}
	return nil
}
