package manager

import (
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// Localized (Simplified Chinese) Windows reports a missing service with
// "失败 1060" instead of "FAILED 1060". Detection must key off the numeric
// error code so status stays correct on non-English systems.
func TestIsWindowsServiceNotFound_Localized(t *testing.T) {
	notFound := []string{
		"[SC] EnumQueryServicesStatus:OpenService FAILED 1060:",
		"[SC] EnumQueryServicesStatus:OpenService 失败 1060:",
		"指定的服务未安装 (1060)",
	}
	for _, out := range notFound {
		if !isWindowsServiceNotFound(out) {
			t.Fatalf("expected %q to be detected as not-found", out)
		}
	}

	present := []string{
		"SERVICE_NAME: demo\n        STATE: 4 RUNNING",
		"service-11060 is running", // 11060 must not be mistaken for 1060
		"",
	}
	for _, out := range present {
		if isWindowsServiceNotFound(out) {
			t.Fatalf("did not expect %q to be reported as not-found", out)
		}
	}
}

// sc.exe output on Simplified Chinese Windows is CP936/GBK. decodeWindowsOutput
// must turn those bytes into valid UTF-8, while leaving ASCII / already-UTF-8
// output untouched so English and other systems are unaffected.
func TestDecodeWindowsOutput(t *testing.T) {
	gbk, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte("失败 1060 服务未安装"))
	if err != nil {
		t.Fatalf("failed to build GBK fixture: %v", err)
	}
	if got := decodeWindowsOutput(gbk); got != "失败 1060 服务未安装" {
		t.Fatalf("GBK output not decoded to UTF-8, got %q", got)
	}

	// already valid UTF-8 must pass through unchanged
	utf8In := "已停止 STOPPED"
	if got := decodeWindowsOutput([]byte(utf8In)); got != utf8In {
		t.Fatalf("valid UTF-8 output was altered, got %q", got)
	}

	// pure ASCII must pass through unchanged
	ascii := "SERVICE_NAME: demo STATE: 1 STOPPED"
	if got := decodeWindowsOutput([]byte(ascii)); got != ascii {
		t.Fatalf("ASCII output was altered, got %q", got)
	}
}
