//go:build windows

package service

import "errors"

var errWindowsXattrUnsupported = errors.New("xattr not supported on windows")
var errWindowsXattrNotFound = errors.New("xattr not found on windows")

func removeFileRemark(filePath string) error {
	return errWindowsXattrUnsupported
}

func setFileRemark(filePath string, value []byte) error {
	return errWindowsXattrUnsupported
}

func getFileRemark(filePath string) (string, error) {
	return "", errWindowsXattrUnsupported
}

func isXattrNotSupported(err error) bool {
	return errors.Is(err, errWindowsXattrUnsupported)
}

func isXattrNotFound(err error) bool {
	return errors.Is(err, errWindowsXattrNotFound)
}
