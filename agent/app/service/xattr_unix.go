//go:build !windows

package service

import (
	"encoding/base64"
	"errors"

	"golang.org/x/sys/unix"
)

func removeFileRemark(filePath string) error {
	return unix.Lremovexattr(filePath, fileRemarkXattr)
}

func setFileRemark(filePath string, value []byte) error {
	return unix.Lsetxattr(filePath, fileRemarkXattr, value, 0)
}

func getFileRemark(filePath string) (string, error) {
	size, err := unix.Lgetxattr(filePath, fileRemarkXattr, nil)
	if err != nil {
		if isXattrNotFound(err) {
			return "", nil
		}
		return "", err
	}
	if size == 0 {
		return "", nil
	}
	buf := make([]byte, size)
	n, err := unix.Lgetxattr(filePath, fileRemarkXattr, buf)
	if err != nil {
		return "", err
	}
	decoded, err := base64.StdEncoding.DecodeString(string(buf[:n]))
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func isXattrNotSupported(err error) bool {
	return errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EOPNOTSUPP)
}

func isXattrNotFound(err error) bool {
	return errors.Is(err, unix.ENODATA)
}
