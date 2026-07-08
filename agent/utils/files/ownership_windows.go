//go:build windows

package files

import "os"

type fileOwnership struct {
	User  string
	Group string
	UID   string
	GID   string
}

func resolveFileOwnership(info os.FileInfo) fileOwnership {
	return fileOwnership{}
}

func GetFileOwnerIDs(info os.FileInfo) (uid, gid int, ok bool) {
	return -1, -1, false
}
