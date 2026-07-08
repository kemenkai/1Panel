//go:build windows

package server

// On Windows the local agent uses an HTTP loopback listener instead of a unix
// socket (see platform.UseLocalAgentSocket), so these hardening helpers are
// no-ops. They exist only to satisfy the cross-platform call sites in server.go.

func prepareMasterSocketDir(dir string) error {
	return nil
}

func secureMasterSocket(sockPath string) error {
	return nil
}
