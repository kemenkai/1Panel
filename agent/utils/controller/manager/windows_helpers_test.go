package manager

import "testing"

func TestIsWindowsServiceNotFound(t *testing.T) {
	if !isWindowsServiceNotFound("[SC] EnumQueryServicesStatus:OpenService FAILED 1060:") {
		t.Fatalf("expected 1060 output to be detected as not-found")
	}
	if isWindowsServiceNotFound("SERVICE_NAME: demo\n        STATE: 4 RUNNING") {
		t.Fatalf("did not expect a running service to be reported as not-found")
	}
}

func TestIsWindowsServiceAlreadyStopped(t *testing.T) {
	stopped := []string{
		"[SC] ControlService FAILED 1062:",
		"The service has not been started.",
		"THE SERVICE HAS NOT BEEN STARTED.",
	}
	for _, out := range stopped {
		if !isWindowsServiceAlreadyStopped(out) {
			t.Fatalf("expected %q to be treated as already-stopped", out)
		}
	}
	running := []string{
		"SERVICE_NAME: demo\n        STATE: 4 RUNNING",
		"[SC] ControlService FAILED 5: Access is denied.",
	}
	for _, out := range running {
		if isWindowsServiceAlreadyStopped(out) {
			t.Fatalf("did not expect %q to be treated as already-stopped", out)
		}
	}
}
