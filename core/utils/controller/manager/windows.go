package manager

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

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
		return false, err
	}
	return strings.Contains(strings.ToUpper(out), "RUNNING"), nil
}

func (w *Windows) IsEnable(serviceName string) (bool, error) {
	out, err := exec.Command(w.toolCmd, "qc", serviceName).CombinedOutput()
	if err != nil {
		if isWindowsServiceNotFound(string(out)) {
			return false, nil
		}
		return false, err
	}
	return strings.Contains(strings.ToUpper(string(out)), "AUTO_START"), nil
}

func (w *Windows) IsExist(serviceName string) (bool, error) {
	out, err := exec.Command(w.toolCmd, "query", serviceName).CombinedOutput()
	if err != nil {
		if isWindowsServiceNotFound(string(out)) {
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
		time.Sleep(2 * time.Second)
		return w.Operate("start", serviceName)
	case "enable":
		return handlerErr(w.run("config", serviceName, "start=", "auto"))
	case "disable":
		return handlerErr(w.run("config", serviceName, "start=", "disabled"))
	case "start", "stop":
		out, err := w.run(operate, serviceName)
		if err != nil {
			upperOut := strings.ToUpper(out)
			if operate == "start" && strings.Contains(upperOut, "RUNNING") {
				return nil
			}
			if operate == "stop" && strings.Contains(upperOut, "STOPPED") {
				return nil
			}
			return handlerErr(out, err)
		}
		return nil
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
	return string(out), err
}

func isWindowsServiceNotFound(out string) bool {
	return strings.Contains(strings.ToUpper(out), "FAILED 1060")
}
