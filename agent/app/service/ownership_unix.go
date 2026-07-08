//go:build !windows

package service

import (
	"os"
	"syscall"
)

func fileOwnerIDs(info os.FileInfo) (uid, gid int, ok bool) {
	if info == nil {
		return -1, -1, false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return -1, -1, false
	}
	return int(stat.Uid), int(stat.Gid), true
}
