package manager

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// windowsServiceNotFoundRe matches sc.exe "service does not exist" (error 1060).
// Matching the numeric code (with digit boundaries) instead of the English word
// "FAILED" keeps detection working on localized Windows, where the message reads
// e.g. "失败 1060" rather than "FAILED 1060".
var windowsServiceNotFoundRe = regexp.MustCompile(`(^|[^0-9])1060([^0-9]|$)`)

type Windows struct {
	toolCmd string
}

func NewWindows() *Windows {
	return &Windows{toolCmd: "sc"}
}

func (w *Windows) Name() string {
	return "windows"
}

func (w *Windows) IsActive(serviceName string) (bool, error) {
	out, err := w.query(serviceName)
	if err != nil {
		// A non-existent service reports error 1060; treat it as "not active"
		// to stay consistent with IsExist/IsEnable instead of surfacing an error.
		if isWindowsServiceNotFound(out) {
			return false, nil
		}
		return false, err
	}
	return strings.Contains(strings.ToUpper(out), "RUNNING"), nil
}

func (w *Windows) IsEnable(serviceName string) (bool, error) {
	out, err := w.run("qc", serviceName)
	if err != nil {
		if isWindowsServiceNotFound(out) {
			return false, nil
		}
		return false, err
	}
	return strings.Contains(strings.ToUpper(out), "AUTO_START"), nil
}

func (w *Windows) IsExist(serviceName string) (bool, error) {
	out, err := w.run("query", serviceName)
	if err != nil {
		if isWindowsServiceNotFound(out) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (w *Windows) Status(serviceName string) (string, error) {
	return w.query(serviceName)
}

func (w *Windows) Operate(operate, serviceName string) error {
	switch operate {
	case "restart":
		if err := w.Operate("stop", serviceName); err != nil {
			return err
		}
		return w.Operate("start", serviceName)
	case "enable":
		return handlerErr(w.run("config", serviceName, "start=", "auto"))
	case "disable":
		return handlerErr(w.run("config", serviceName, "start=", "demand"))
	case "start", "stop":
		out, err := w.run(operate, serviceName)
		if err != nil {
			upperOut := strings.ToUpper(out)
			if operate == "start" && strings.Contains(upperOut, "RUNNING") {
				return nil
			}
			if operate == "stop" && (strings.Contains(upperOut, "STOPPED") || isWindowsServiceAlreadyStopped(out)) {
				return nil
			}
			return handlerErr(out, err)
		}
		return w.waitForExpectedState(serviceName, operate)
	default:
		return fmt.Errorf("unsupported operate %s for windows service", operate)
	}
}

func (w *Windows) Reload() error {
	return nil
}

func (w *Windows) query(serviceName string) (string, error) {
	return w.run("query", serviceName)
}

func (w *Windows) run(args ...string) (string, error) {
	out, err := exec.Command(w.toolCmd, args...).CombinedOutput()
	return decodeWindowsOutput(out), err
}

// DecodeWindowsOutput converts console output of Windows command-line tools
// (sc.exe, WinSW.exe, ...) to UTF-8 for callers outside this package.
func DecodeWindowsOutput(out []byte) string {
	return decodeWindowsOutput(out)
}

// decodeWindowsOutput converts sc.exe output to UTF-8. On localized Windows the
// console output is in the OEM code page (CP936/GBK for Simplified Chinese), so
// raw bytes are not valid UTF-8 and would render as mojibake once passed to the
// API/frontend. ASCII and already-UTF-8 output is valid UTF-8 and passes through
// untouched, so this is a no-op on English/other systems.
func decodeWindowsOutput(out []byte) string {
	if utf8.Valid(out) {
		return string(out)
	}
	if decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(out); err == nil {
		return string(decoded)
	}
	return string(out)
}

func (w *Windows) waitForExpectedState(serviceName, operate string) error {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		out, err := w.query(serviceName)
		if err == nil {
			upperOut := strings.ToUpper(out)
			if operate == "start" && strings.Contains(upperOut, "RUNNING") {
				return nil
			}
			if operate == "stop" && strings.Contains(upperOut, "STOPPED") {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s to %s", serviceName, operate)
}

func isWindowsServiceNotFound(out string) bool {
	return windowsServiceNotFoundRe.MatchString(out)
}

// isWindowsServiceAlreadyStopped detects `sc stop` failing because the service
// is not running (error 1062). Without this, restart (stop-then-start) aborts on
// an already-stopped service and never brings it back up.
func isWindowsServiceAlreadyStopped(out string) bool {
	upper := strings.ToUpper(out)
	return strings.Contains(upper, "1062") || strings.Contains(upper, "HAS NOT BEEN STARTED")
}
