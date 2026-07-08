package service

// Linux managed-service (systemd) backend for the service-management feature.
//
// This file has no _linux build suffix on purpose: it must compile under
// GOOS=windows too (the platform is dispatched at runtime via platform.Current),
// so it must not import any linux-only package. Everything here is either a pure
// artifact generator (unit / start script / uninstall script) or a thin wrapper
// around cross-platform os/exec calls that only ever run when Current()==linux.

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/1Panel-dev/1Panel/agent/app/model"
)

// systemdSystemUnitDir is the fixed root for unit files we install. Service names
// are validated (validateWindowsServiceName) before being joined here, so a name
// can never traverse out of this directory. It is a package var (not a const) only
// so tests can point it at a t.TempDir() and exercise the real register/teardown
// file writes without root; production never reassigns it.
var systemdSystemUnitDir = "/etc/systemd/system"

// systemdVendorUnitDirs are the distro-owned unit roots consulted by the
// anti-hijack guard: a unit file present in any of them belongs to a system
// package (nginx, docker, ...) and must never be shadowed by a 1Panel unit in
// /etc/systemd/system, regardless of what `systemctl is-enabled` reports (it can
// fail via DBus while the file is plainly on disk). Package var for testability,
// same pattern as systemdSystemUnitDir; production never reassigns it.
var systemdVendorUnitDirs = []string{"/lib/systemd/system", "/usr/lib/systemd/system"}

const (
	// managedServiceUnitMarker is the ownership stamp carried in the first comment
	// line of every unit we generate. Teardown/overwrite paths only touch a unit
	// that carries this marker, so a system unit (ssh, docker, ...) is never removed.
	managedServiceUnitMarker = "Managed by 1Panel"
	managedServiceUnitHeader = "# Managed by 1Panel service management; do not edit manually."
)

func linuxSystemdUnitPath(name string) string {
	return filepath.Join(systemdSystemUnitDir, strings.TrimSpace(name)+".service")
}

// prepareLinuxArtifacts writes the per-service directory contents on Linux: env
// file, launch script, unit-file copy, and uninstall script. All text artifacts
// are LF with no BOM (systemd EnvironmentFile requirement; scripts follow suit).
func (w *WindowsServiceService) prepareLinuxArtifacts(item *model.WindowsService, serviceDir, envFilePath string, managedConfig *managedWindowsServiceConfig) error {
	if envFilePath == "" {
		envFilePath = filepath.Join(serviceDir, item.Name+".env")
	}
	item.EnvFilePath = envFilePath
	// The env file can hold plaintext secrets (DB passwords, etc.) and is read by
	// systemd as root, so it is 0600 — never world-readable under the 0755 service
	// tree. NOTE: if a future unit adds User=<non-root> to drop privileges, this must
	// become 0640 with the group set to the service user, or the process cannot read it.
	if err := atomicWriteFile(item.EnvFilePath, []byte(w.buildEnvFile(item, managedConfig)), 0o600); err != nil {
		return err
	}

	scriptPath := filepath.Join(serviceDir, item.Name+".sh")
	if err := atomicWriteFile(scriptPath, []byte(w.buildLinuxStartScript(item)), 0o755); err != nil {
		return err
	}

	item.ServicePath = filepath.Join(serviceDir, item.Name+".service")
	unitContent, err := buildSystemdUnitFile(item)
	if err != nil {
		return err
	}
	if err := atomicWriteFile(item.ServicePath, []byte(unitContent), 0o644); err != nil {
		return err
	}

	uninstallPath := filepath.Join(serviceDir, item.Name+"-uninstall.sh")
	uninstall := buildLinuxUninstallScript(item, serviceDir, linuxSystemdUnitPath(item.Name))
	return atomicWriteFile(uninstallPath, []byte(uninstall), 0o755)
}

// syncLinuxRegistration mirrors the Windows registration path: it tears down a
// prior registration, then (if requested) installs the unit under systemd,
// reloads the daemon, and enables/disables autostart, driving everything through
// the injected controller variables so the sequence is test-observable.
func (w *WindowsServiceService) syncLinuxRegistration(item *model.WindowsService, previousRegistered bool) error {
	if previousRegistered {
		w.unregisterLinuxService(*item)
	}

	if !item.RegisterService {
		item.Status = "NotRegistered"
		item.Message = windowsServiceNotRegisteredMessage
		return nil
	}

	if _, err := exec.LookPath("systemctl"); err != nil {
		item.Status = "UnHealthy"
		item.Message = "systemd is required for service registration on linux"
		return errors.New(item.Message)
	}

	unitPath := linuxSystemdUnitPath(item.Name)
	if err := ensureLinuxServiceRegisterable(item.Name, unitPath); err != nil {
		item.Status = "UnHealthy"
		item.Message = err.Error()
		return err
	}

	unitContent, err := buildSystemdUnitFile(item)
	if err != nil {
		item.Status = "UnHealthy"
		item.Message = err.Error()
		return err
	}
	if err := os.MkdirAll(buildWindowsServiceLogDir(), 0o755); err != nil {
		item.Status = "UnHealthy"
		item.Message = err.Error()
		return err
	}
	if err := atomicWriteFile(unitPath, []byte(unitContent), 0o644); err != nil {
		item.Status = "UnHealthy"
		item.Message = err.Error()
		return err
	}
	if err := controllerReload(); err != nil {
		item.Status = "UnHealthy"
		item.Message = err.Error()
		return err
	}

	action := "disable"
	if item.AutoStart {
		action = "enable"
	}
	if err := controllerHandle(action, managedControllerUnitName(item.Name)); err != nil {
		item.Status = "UnHealthy"
		item.Message = err.Error()
		return err
	}

	item.Status = "Stopped"
	item.Message = ""
	return nil
}

// unregisterLinuxService stops, disables, and removes a unit we own. It is a
// no-op unless the on-disk unit carries our ownership marker, so it can never
// tear down a system service that happens to share the name.
func (w *WindowsServiceService) unregisterLinuxService(item model.WindowsService) {
	name := strings.TrimSpace(item.Name)
	if name == "" {
		return
	}
	unitPath := linuxSystemdUnitPath(name)
	if !managedUnitOwnedByPanel(unitPath) {
		return
	}
	unitName := managedControllerUnitName(name)
	_ = controllerHandle("stop", unitName)
	_ = controllerHandle("disable", unitName)
	_ = os.Remove(unitPath)
	_ = controllerReload()
}

// ensureLinuxServiceRegisterable is the anti-hijack guard: it refuses to install
// over a unit we do not own, whether that unit was placed manually under
// /etc/systemd/system or ships as a system unit (e.g. ssh.service, docker.service)
// that systemd already knows about. A unit already stamped as ours is fine to
// overwrite (idempotent re-register / update).
func ensureLinuxServiceRegisterable(name, unitPath string) error {
	if managedUnitOwnedByPanel(unitPath) {
		return nil
	}
	if _, err := os.Stat(unitPath); err == nil {
		return fmt.Errorf("a service named %q already exists and is not managed by 1Panel; refusing to overwrite it", name)
	}
	// Filesystem backstop that needs neither systemctl nor DBus: a unit file under a
	// vendor dir is a packaged system service (installed-but-disabled nginx/docker/...)
	// even when `systemctl is-enabled` errors, so refuse to shadow it.
	unitFileName := strings.TrimSpace(name) + ".service"
	for _, vendorDir := range systemdVendorUnitDirs {
		if _, err := os.Stat(filepath.Join(vendorDir, unitFileName)); err == nil {
			return fmt.Errorf("a system service named %q already exists; refusing to overwrite it", name)
		}
	}
	// exist==true is authoritative REGARDLESS of the accompanying error: systemd's
	// IsExist reports (true, exit-1) for an installed-but-disabled unit, and skipping
	// the guard on that error would let 1Panel shadow a real system service. Only a
	// clean (false, ...) result — including (false, "No such file") for a genuinely
	// fresh name — may proceed; do not fail closed on the error alone, or every
	// legitimate create would be rejected.
	if exist, _ := controllerCheckExist(managedControllerUnitName(name)); exist {
		return fmt.Errorf("a system service named %q already exists; refusing to overwrite it", name)
	}
	return nil
}

// managedUnitOwnedByPanel reports whether the unit at path exists and carries our
// ownership marker in its header.
func managedUnitOwnedByPanel(unitPath string) bool {
	file, err := os.Open(unitPath)
	if err != nil {
		return false
	}
	defer file.Close()
	buf := make([]byte, 512)
	n, _ := file.Read(buf)
	return strings.Contains(string(buf[:n]), managedServiceUnitMarker)
}

func buildSystemdUnitFile(item *model.WindowsService) (string, error) {
	serviceDir := buildWindowsServiceDir(item.Name)
	return buildSystemdUnit(item, serviceDir, item.EnvFilePath, buildWindowsServiceLogDir())
}

// buildSystemdUnit renders the unit file. It is pure: given an item and the three
// directories, it returns the exact file content (or an error if a value cannot be
// safely embedded). Every value written into the unit is passed through
// sanitizeUnitValue first, so a newline (directive injection) is rejected and a
// systemd specifier ('%') is escaped.
func buildSystemdUnit(item *model.WindowsService, serviceDir, envFilePath, logDir string) (string, error) {
	displayName := strings.TrimSpace(item.DisplayName)
	if displayName == "" {
		displayName = item.Name
	}
	description, err := sanitizeUnitValue(displayName)
	if err != nil {
		return "", err
	}

	workDir := item.WorkDir
	if workDir == "" {
		workDir = filepath.Dir(item.ExecPath)
	}
	workDirValue, err := sanitizeUnitValue(workDir)
	if err != nil {
		return "", err
	}

	scriptValue, err := sanitizeUnitValue(filepath.Join(serviceDir, item.Name+".sh"))
	if err != nil {
		return "", err
	}
	envValue, err := sanitizeUnitValue(envFilePath)
	if err != nil {
		return "", err
	}
	outLogValue, err := sanitizeUnitValue(filepath.Join(logDir, item.Name+".out.log"))
	if err != nil {
		return "", err
	}
	errLogValue, err := sanitizeUnitValue(filepath.Join(logDir, item.Name+".err.log"))
	if err != nil {
		return "", err
	}

	after := "network.target"
	if isDeliveryPackageComposeItem(*item) {
		after = "network.target docker.service"
	}

	// WorkingDirectory / EnvironmentFile / StandardOutput take the literal rest of
	// the line (no quote processing), so they are written unquoted. ExecStart is
	// tokenized like a shell command line, so the script path is double-quoted to
	// survive spaces.
	return fmt.Sprintf(`%s
[Unit]
Description=%s
After=%s
Wants=network.target

[Service]
Type=simple
EnvironmentFile=-%s
WorkingDirectory=%s
ExecStart=/bin/bash "%s"
Restart=always
RestartSec=5
TimeoutStopSec=15
StandardOutput=append:%s
StandardError=append:%s

[Install]
WantedBy=multi-user.target
`, managedServiceUnitHeader, description, after, envValue, workDirValue, scriptValue, outLogValue, errLogValue), nil
}

// buildLinuxStartScript renders the launch script systemd's ExecStart invokes.
// Environment variables come from the unit's EnvironmentFile, so the script only
// changes into the working directory, optionally extends the loader path, and
// exec's the real process (java -jar ... or docker compose ... up <svc>).
func (w *WindowsServiceService) buildLinuxStartScript(item *model.WindowsService) string {
	workDir := item.WorkDir
	if workDir == "" {
		workDir = filepath.Dir(item.ExecPath)
	}
	var b strings.Builder
	b.WriteString("#!/bin/bash\n")
	b.WriteString(managedServiceUnitHeader + "\n")
	b.WriteString("cd " + bashDoubleQuote(workDir) + "\n")
	if strings.TrimSpace(item.DLLDir) != "" {
		// Keep $LD_LIBRARY_PATH unescaped so the existing value is preserved.
		b.WriteString(`export LD_LIBRARY_PATH="` + escapeShellDoubleQuoted(item.DLLDir) + `:$LD_LIBRARY_PATH"` + "\n")
	}
	b.WriteString("exec " + w.buildLinuxCommandLine(item) + "\n")
	return b.String()
}

// buildLinuxCommandLine mirrors buildPowerShellScript's command construction so
// both platforms launch a service identically: compose services run through the
// compose CLI; java/delivery package jars split JVM args before -jar and app args after,
// reusing detachManagedSpringConfigArgument. User Args are appended verbatim, the
// same as the Windows path (no new injection surface).
func (w *WindowsServiceService) buildLinuxCommandLine(item *model.WindowsService) string {
	args := strings.TrimSpace(item.Args)
	if isDeliveryPackageComposeItem(*item) {
		composeCmd := strings.TrimSpace(item.ExecPath)
		serviceName := strings.TrimSpace(item.Args)
		line := composeCmd + " -f " + bashDoubleQuote(item.ConfigPath) + " up"
		if serviceName != "" {
			line += " " + serviceName
		}
		return line
	}

	line := bashDoubleQuote(item.ExecPath)
	if (item.ServiceType == "java" || item.ServiceType == "package") && item.JarPath != "" && !strings.Contains(args, "-jar") {
		appArgs := ""
		args, appArgs = detachManagedSpringConfigArgument(args, item.ConfigPath)
		if args != "" {
			line += " " + args
		}
		line += " -jar " + bashDoubleQuote(item.JarPath)
		if appArgs != "" {
			// detachManagedSpringConfigArgument only returns a non-empty appArgs when it
			// matched the exact --spring.config.location="<configPath>" token, so rebuild
			// it from ConfigPath with proper shell escaping rather than re-emitting the
			// raw (unescaped) token — otherwise a ConfigPath containing $, ` or " would
			// break the double-quoted string or trigger shell expansion at launch.
			line += " --spring.config.location=" + bashDoubleQuote(item.ConfigPath)
		}
	} else if args != "" {
		line += " " + args
	}
	return line
}

// buildLinuxUninstallScript renders the self-contained teardown script. It stops
// and disables the unit, removes the unit file, reloads systemd, and finally
// removes its own service directory, mirroring the Windows uninstall behavior.
func buildLinuxUninstallScript(item *model.WindowsService, serviceDir, unitPath string) string {
	unitName := strings.TrimSpace(item.Name) + ".service"
	var b strings.Builder
	b.WriteString("#!/bin/bash\n")
	b.WriteString(managedServiceUnitHeader + "\n")
	b.WriteString("systemctl stop " + bashDoubleQuote(unitName) + " 2>/dev/null || true\n")
	b.WriteString("systemctl disable " + bashDoubleQuote(unitName) + " 2>/dev/null || true\n")
	b.WriteString("rm -f " + bashDoubleQuote(unitPath) + "\n")
	b.WriteString("systemctl daemon-reload 2>/dev/null || true\n")
	b.WriteString("rm -rf " + bashDoubleQuote(serviceDir) + "\n")
	return b.String()
}

// sanitizeUnitValue makes a caller-influenced string safe to embed as a systemd
// unit value: a newline would let an attacker inject additional directives, so it
// is rejected outright; a literal '%' is escaped to '%%' so systemd does not
// expand it as a specifier.
func sanitizeUnitValue(value string) (string, error) {
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("value %q contains an illegal newline", value)
	}
	return strings.ReplaceAll(value, "%", "%%"), nil
}

// escapeShellDoubleQuoted escapes the four characters that remain special inside a
// bash double-quoted string: backslash, backtick, double quote, and dollar sign.
// strings.NewReplacer performs a single non-overlapping pass, so a backslash it
// inserts is never re-escaped.
func escapeShellDoubleQuoted(value string) string {
	return strings.NewReplacer(
		`\`, `\\`,
		"`", "\\`",
		`"`, `\"`,
		`$`, `\$`,
	).Replace(value)
}

func bashDoubleQuote(value string) string {
	return `"` + escapeShellDoubleQuoted(value) + `"`
}
