//go:build windows

package service

// checkUpgradeSpace is a no-op on Windows: syscall.Statfs is unavailable there
// and the pre-upgrade free-space guard is best-effort, so we skip it rather than
// pull in a Windows-only disk API. The unix build carries the real check.
func checkUpgradeSpace() error {
	return nil
}
