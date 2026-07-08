//go:build !windows

package files

import (
	"os"
	"strconv"
	"syscall"
)

type fileOwnership struct {
	User  string
	Group string
	UID   string
	GID   string
}

func resolveFileOwnership(info os.FileInfo) fileOwnership {
	if info == nil {
		return fileOwnership{}
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return fileOwnership{}
	}
	return fileOwnership{
		User:  GetUsername(stat.Uid),
		Group: GetGroup(stat.Gid),
		UID:   strconv.FormatUint(uint64(stat.Uid), 10),
		GID:   strconv.FormatUint(uint64(stat.Gid), 10),
	}
}

func GetFileOwnerIDs(info os.FileInfo) (uid, gid int, ok bool) {
	if info == nil {
		return -1, -1, false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return -1, -1, false
	}
	return int(stat.Uid), int(stat.Gid), true
}
