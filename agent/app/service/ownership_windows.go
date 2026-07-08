//go:build windows

package service

import "os"

func fileOwnerIDs(info os.FileInfo) (uid, gid int, ok bool) {
	return -1, -1, false
}
