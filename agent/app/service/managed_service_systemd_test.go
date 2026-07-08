package service

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/1Panel-dev/1Panel/agent/app/dto/request"
	"github.com/1Panel-dev/1Panel/agent/app/model"
	"github.com/1Panel-dev/1Panel/agent/global"
	"github.com/sirupsen/logrus"
)

// ensureTestLogger gives global.LOG a usable value for code paths that log; the
// production logger is initialized at startup but the test binary has none.
func ensureTestLogger(t *testing.T) {
	t.Helper()
	if global.LOG == nil {
		global.LOG = logrus.New()
	}
}

// requireLinuxRuntime skips a test whose behavior is gated on platform.Current()
// (==runtime.GOOS): the Linux managed-service branch only executes when the test
// binary itself runs on Linux. The file still compiles under GOOS=windows.
func requireLinuxRuntime(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skipf("linux managed-service behavior; runtime is %s", runtime.GOOS)
	}
}

func newSystemdTestItem(installDir string) *model.WindowsService {
	appDir := filepath.Join(installDir, "1panel", "services", "demo-java", "app")
	return &model.WindowsService{
		Name:        "demo-java",
		DisplayName: "Demo Java",
		ServiceType: "java",
		ExecPath:    filepath.Join(installDir, "zulu21-test", "bin", "java"),
		WorkDir:     appDir,
		JarPath:     filepath.Join(appDir, "demo.jar"),
		EnvFilePath: filepath.Join(installDir, "1panel", "services", "demo-java", "demo-java.env"),
	}
}

// --- unit content -----------------------------------------------------------

func TestBuildSystemdUnitRendersManagedDirectives(t *testing.T) {
	serviceDir := filepath.Join(t.TempDir(), "demo-java")
	logDir := filepath.Join(t.TempDir(), "log")
	envPath := filepath.Join(serviceDir, "demo-java.env")
	item := &model.WindowsService{
		Name:        "demo-java",
		DisplayName: "Demo Java",
		ServiceType: "java",
		WorkDir:     filepath.Join(serviceDir, "app"),
	}

	unit, err := buildSystemdUnit(item, serviceDir, envPath, logDir)
	if err != nil {
		t.Fatalf("buildSystemdUnit returned error: %v", err)
	}

	wants := []string{
		managedServiceUnitHeader,
		"Description=Demo Java",
		"After=network.target",
		"Wants=network.target",
		"Type=simple",
		"EnvironmentFile=-" + envPath,
		"WorkingDirectory=" + item.WorkDir,
		`ExecStart=/bin/bash "` + filepath.Join(serviceDir, "demo-java.sh") + `"`,
		"Restart=always",
		"RestartSec=5",
		"TimeoutStopSec=15",
		"StandardOutput=append:" + filepath.Join(logDir, "demo-java.out.log"),
		"StandardError=append:" + filepath.Join(logDir, "demo-java.err.log"),
		"WantedBy=multi-user.target",
	}
	for _, want := range wants {
		if !strings.Contains(unit, want) {
			t.Fatalf("expected unit to contain %q, got:\n%s", want, unit)
		}
	}
	// The ownership marker must be the first line so managedUnitOwnedByPanel matches.
	if !strings.HasPrefix(unit, managedServiceUnitHeader) {
		t.Fatalf("expected unit to start with ownership header, got:\n%s", unit)
	}
	if !strings.Contains(managedServiceUnitHeader, managedServiceUnitMarker) {
		t.Fatalf("ownership header %q must embed marker %q", managedServiceUnitHeader, managedServiceUnitMarker)
	}
}

func TestBuildSystemdUnitAddsDockerDependencyForComposeItem(t *testing.T) {
	serviceDir := filepath.Join(t.TempDir(), "base-auth")
	logDir := filepath.Join(t.TempDir(), "log")
	item := &model.WindowsService{
		Name:        "base-auth",
		DisplayName: "Base Auth",
		ServiceType: "package",
		ExecPath:    "docker compose",
		ConfigPath:  filepath.Join(serviceDir, "docker-compose.yml"),
		WorkDir:     serviceDir,
		Args:        "base-auth",
	}

	unit, err := buildSystemdUnit(item, serviceDir, "", logDir)
	if err != nil {
		t.Fatalf("buildSystemdUnit returned error: %v", err)
	}
	if !strings.Contains(unit, "After=network.target docker.service") {
		t.Fatalf("expected compose unit to order after docker.service, got:\n%s", unit)
	}
}

// --- escape / injection rejection -------------------------------------------

func TestSanitizeUnitValueRejectsNewlinesAndEscapesSpecifiers(t *testing.T) {
	for _, bad := range []string{"line\ninjected", "carriage\rreturn", "both\r\n"} {
		if _, err := sanitizeUnitValue(bad); err == nil {
			t.Fatalf("expected %q to be rejected", bad)
		}
	}
	got, err := sanitizeUnitValue("100% done %n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "100%% done %%n" {
		t.Fatalf("expected specifier escaping, got %q", got)
	}
}

func TestBuildSystemdUnitRejectsNewlineInjection(t *testing.T) {
	serviceDir := filepath.Join(t.TempDir(), "demo")
	item := &model.WindowsService{
		Name:        "demo",
		DisplayName: "evil\nExecStartPre=/bin/rm -rf /",
		ServiceType: "java",
		WorkDir:     serviceDir,
	}
	if _, err := buildSystemdUnit(item, serviceDir, "", serviceDir); err == nil {
		t.Fatal("expected a newline in DisplayName to be rejected as directive injection")
	}

	item.DisplayName = "Demo"
	item.WorkDir = "/tmp/evil\nExecStartPre=/bin/rm -rf /"
	if _, err := buildSystemdUnit(item, serviceDir, "", serviceDir); err == nil {
		t.Fatal("expected a newline in WorkDir to be rejected as directive injection")
	}
}

func TestBuildSystemdUnitEscapesSpecifierInDescription(t *testing.T) {
	serviceDir := filepath.Join(t.TempDir(), "demo")
	item := &model.WindowsService{
		Name:        "demo",
		DisplayName: "Batch 100% ready",
		ServiceType: "java",
		WorkDir:     serviceDir,
	}
	unit, err := buildSystemdUnit(item, serviceDir, "", serviceDir)
	if err != nil {
		t.Fatalf("buildSystemdUnit returned error: %v", err)
	}
	if !strings.Contains(unit, "Description=Batch 100%% ready") {
		t.Fatalf("expected '%%' to be escaped in Description, got:\n%s", unit)
	}
}

// --- unit paths stay inside the provided dirs -------------------------------

func TestBuildSystemdUnitKeepsPathsInsideProvidedDirs(t *testing.T) {
	serviceDir := filepath.Join(t.TempDir(), "demo-java")
	logDir := filepath.Join(t.TempDir(), "log")
	item := &model.WindowsService{
		Name:        "demo-java",
		DisplayName: "Demo Java",
		ServiceType: "java",
		WorkDir:     filepath.Join(serviceDir, "app"),
	}
	unit, err := buildSystemdUnit(item, serviceDir, "", logDir)
	if err != nil {
		t.Fatalf("buildSystemdUnit returned error: %v", err)
	}

	scriptPath := filepath.Join(serviceDir, "demo-java.sh")
	assertPathUnder(t, scriptPath, serviceDir)
	if !strings.Contains(unit, `ExecStart=/bin/bash "`+scriptPath+`"`) {
		t.Fatalf("expected ExecStart to launch the in-dir script, got:\n%s", unit)
	}
	assertPathUnder(t, filepath.Join(logDir, "demo-java.out.log"), logDir)
	assertPathUnder(t, filepath.Join(logDir, "demo-java.err.log"), logDir)
}

// --- anti-hijack / duplicate-name rejection ---------------------------------

func TestEnsureLinuxServiceRegisterableRejectsForeignUnit(t *testing.T) {
	restoreExist := controllerCheckExist
	controllerCheckExist = func(string) (bool, error) { return false, nil }
	defer func() { controllerCheckExist = restoreExist }()

	unitPath := filepath.Join(t.TempDir(), "demo.service")
	if err := os.WriteFile(unitPath, []byte("[Unit]\nDescription=someone else\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ensureLinuxServiceRegisterable("demo", unitPath); err == nil {
		t.Fatal("expected registration to be refused over an unowned on-disk unit")
	}
}

func TestEnsureLinuxServiceRegisterableRejectsExistingSystemUnit(t *testing.T) {
	restoreExist := controllerCheckExist
	controllerCheckExist = func(string) (bool, error) { return true, nil }
	defer func() { controllerCheckExist = restoreExist }()

	// No file on disk, but systemd already knows a unit by this name (e.g. ssh,
	// docker shipped under /lib) -> refuse rather than shadow/hijack it.
	unitPath := filepath.Join(t.TempDir(), "ssh.service")
	if err := ensureLinuxServiceRegisterable("ssh", unitPath); err == nil {
		t.Fatal("expected registration to be refused when a system unit already exists")
	}
}

func TestEnsureLinuxServiceRegisterableAllowsOwnedUnit(t *testing.T) {
	restoreExist := controllerCheckExist
	controllerCheckExist = func(string) (bool, error) { return true, nil }
	defer func() { controllerCheckExist = restoreExist }()

	unitPath := filepath.Join(t.TempDir(), "demo.service")
	if err := os.WriteFile(unitPath, []byte(managedServiceUnitHeader+"\n[Unit]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A unit we own is fine to overwrite (idempotent re-register / update), even
	// when systemd reports it as already existing.
	if err := ensureLinuxServiceRegisterable("demo", unitPath); err != nil {
		t.Fatalf("expected owned unit to be registerable, got %v", err)
	}
}

func TestEnsureLinuxServiceRegisterableAllowsCleanName(t *testing.T) {
	swapVendorUnitDirs(t, t.TempDir())
	restoreExist := controllerCheckExist
	controllerCheckExist = func(string) (bool, error) { return false, nil }
	defer func() { controllerCheckExist = restoreExist }()

	unitPath := filepath.Join(t.TempDir(), "fresh.service")
	if err := ensureLinuxServiceRegisterable("fresh", unitPath); err != nil {
		t.Fatalf("expected a clean name to be registerable, got %v", err)
	}
}

// --- registration systemctl sequence ----------------------------------------

// swapSystemdUnitDir points systemdSystemUnitDir at a temp dir for the duration of
// the test so the real register/teardown file writes can be exercised without root
// (they no longer touch /etc/systemd/system). Restored on cleanup.
func swapSystemdUnitDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	restore := systemdSystemUnitDir
	systemdSystemUnitDir = dir
	t.Cleanup(func() { systemdSystemUnitDir = restore })
	return dir
}

func TestSyncLinuxRegistrationDrivesSystemctlSequence(t *testing.T) {
	requireLinuxRuntime(t)
	if _, err := exec.LookPath("systemctl"); err != nil {
		t.Skip("systemctl not available")
	}
	swapSystemdUnitDir(t)

	installDir := t.TempDir()
	restoreInstallDir := global.CONF.Base.InstallDir
	global.CONF.Base.InstallDir = installDir
	defer func() { global.CONF.Base.InstallDir = restoreInstallDir }()

	item := newSystemdTestItem(installDir)
	item.Name = "onepanel-systemd-seqtest"
	item.RegisterService = true
	item.AutoStart = true
	unitPath := linuxSystemdUnitPath(item.Name)

	var calls []string
	restoreReload := controllerReload
	restoreHandle := controllerHandle
	restoreExist := controllerCheckExist
	controllerReload = func() error { calls = append(calls, "reload"); return nil }
	controllerHandle = func(op, name string) error {
		calls = append(calls, op+":"+name)
		return nil
	}
	controllerCheckExist = func(string) (bool, error) { return false, nil }
	defer func() {
		controllerReload = restoreReload
		controllerHandle = restoreHandle
		controllerCheckExist = restoreExist
	}()

	if err := (&WindowsServiceService{}).syncLinuxRegistration(item, false); err != nil {
		t.Fatalf("syncLinuxRegistration returned error: %v", err)
	}

	// R1 regression guard: systemctl (via the injected controller) is really driven;
	// the unit is written, the daemon reloaded, then autostart enabled. The controller
	// receives the EXACT systemd unit name (managedControllerUnitName appends .service).
	wantEnable := "enable:" + managedControllerUnitName(item.Name)
	if len(calls) != 2 || calls[0] != "reload" || calls[1] != wantEnable {
		t.Fatalf("expected [reload %s], got %v", wantEnable, calls)
	}
	data, err := os.ReadFile(unitPath)
	if err != nil {
		t.Fatalf("expected unit file to be installed: %v", err)
	}
	if !managedUnitOwnedByPanel(unitPath) {
		t.Fatalf("installed unit must carry the ownership marker, got:\n%s", string(data))
	}
	if item.Status != "Stopped" || item.Message != "" {
		t.Fatalf("expected Stopped status after register, got status=%q message=%q", item.Status, item.Message)
	}
}

// F1: a managed service name with uppercase letters must register successfully and
// drive the controller with the exact, case-preserved unit name (MyService.service),
// not the lowercased "myservice.service" LoadServiceName would have produced.
func TestSyncLinuxRegistrationUsesExactSystemdUnitNameForMixedCase(t *testing.T) {
	requireLinuxRuntime(t)
	if _, err := exec.LookPath("systemctl"); err != nil {
		t.Skip("systemctl not available")
	}
	swapSystemdUnitDir(t)

	installDir := t.TempDir()
	restoreInstallDir := global.CONF.Base.InstallDir
	global.CONF.Base.InstallDir = installDir
	defer func() { global.CONF.Base.InstallDir = restoreInstallDir }()

	item := newSystemdTestItem(installDir)
	item.Name = "MyService"
	item.RegisterService = true
	item.AutoStart = true
	unitPath := linuxSystemdUnitPath(item.Name)

	var handleNames []string
	restoreReload := controllerReload
	restoreHandle := controllerHandle
	restoreExist := controllerCheckExist
	controllerReload = func() error { return nil }
	controllerHandle = func(op, name string) error { handleNames = append(handleNames, name); return nil }
	controllerCheckExist = func(string) (bool, error) { return false, nil }
	defer func() {
		controllerReload = restoreReload
		controllerHandle = restoreHandle
		controllerCheckExist = restoreExist
	}()

	if err := (&WindowsServiceService{}).syncLinuxRegistration(item, false); err != nil {
		t.Fatalf("expected mixed-case service to register without error, got: %v", err)
	}
	if len(handleNames) != 1 || handleNames[0] != "MyService.service" {
		t.Fatalf("expected controller to receive exact unit name MyService.service, got %v", handleNames)
	}
	// The unit file must be installed at the case-preserved path and the item must not
	// be marked UnHealthy (which would trigger a create rollback).
	if filepath.Base(unitPath) != "MyService.service" {
		t.Fatalf("expected case-preserved unit filename, got %s", filepath.Base(unitPath))
	}
	if _, err := os.Stat(unitPath); err != nil {
		t.Fatalf("expected MyService.service unit to be installed: %v", err)
	}
	if item.Status != "Stopped" {
		t.Fatalf("expected Stopped status (no rollback) for mixed-case name, got %q (%q)", item.Status, item.Message)
	}
}

func TestSyncLinuxRegistrationDisablesWhenAutoStartOff(t *testing.T) {
	requireLinuxRuntime(t)
	if _, err := exec.LookPath("systemctl"); err != nil {
		t.Skip("systemctl not available")
	}
	swapSystemdUnitDir(t)

	installDir := t.TempDir()
	restoreInstallDir := global.CONF.Base.InstallDir
	global.CONF.Base.InstallDir = installDir
	defer func() { global.CONF.Base.InstallDir = restoreInstallDir }()

	item := newSystemdTestItem(installDir)
	item.Name = "onepanel-systemd-disabletest"
	item.RegisterService = true
	item.AutoStart = false

	var calls []string
	restoreReload := controllerReload
	restoreHandle := controllerHandle
	restoreExist := controllerCheckExist
	controllerReload = func() error { calls = append(calls, "reload"); return nil }
	controllerHandle = func(op, name string) error { calls = append(calls, op+":"+name); return nil }
	controllerCheckExist = func(string) (bool, error) { return false, nil }
	defer func() {
		controllerReload = restoreReload
		controllerHandle = restoreHandle
		controllerCheckExist = restoreExist
	}()

	if err := (&WindowsServiceService{}).syncLinuxRegistration(item, false); err != nil {
		t.Fatalf("syncLinuxRegistration returned error: %v", err)
	}
	wantDisable := "disable:" + managedControllerUnitName(item.Name)
	if len(calls) != 2 || calls[1] != wantDisable {
		t.Fatalf("expected %s when AutoStart off, got %v", wantDisable, calls)
	}
}

// --- delete / teardown sequence ---------------------------------------------

func TestUnregisterLinuxServiceSkipsForeignUnit(t *testing.T) {
	var calls []string
	restoreHandle := controllerHandle
	restoreReload := controllerReload
	controllerHandle = func(op, name string) error { calls = append(calls, op+":"+name); return nil }
	controllerReload = func() error { calls = append(calls, "reload"); return nil }
	defer func() {
		controllerHandle = restoreHandle
		controllerReload = restoreReload
	}()

	// No unit on disk at the fixed path -> managedUnitOwnedByPanel is false, so the
	// teardown must be a strict no-op (never stop/disable a service we do not own).
	(&WindowsServiceService{}).unregisterLinuxService(model.WindowsService{Name: "onepanel-absent-teardown-test"})
	if len(calls) != 0 {
		t.Fatalf("expected no controller calls for an unowned/absent unit, got %v", calls)
	}
}

func TestUnregisterLinuxServiceTearsDownOwnedUnit(t *testing.T) {
	requireLinuxRuntime(t)
	swapSystemdUnitDir(t)

	name := "onepanel-systemd-teardown-test"
	unitPath := linuxSystemdUnitPath(name)
	if err := os.WriteFile(unitPath, []byte(managedServiceUnitHeader+"\n[Unit]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var calls []string
	restoreHandle := controllerHandle
	restoreReload := controllerReload
	controllerHandle = func(op, n string) error { calls = append(calls, op+":"+n); return nil }
	controllerReload = func() error { calls = append(calls, "reload"); return nil }
	defer func() {
		controllerHandle = restoreHandle
		controllerReload = restoreReload
	}()

	(&WindowsServiceService{}).unregisterLinuxService(model.WindowsService{Name: name})

	unitName := managedControllerUnitName(name)
	expected := []string{"stop:" + unitName, "disable:" + unitName, "reload"}
	if strings.Join(calls, ",") != strings.Join(expected, ",") {
		t.Fatalf("expected teardown sequence %v, got %v", expected, calls)
	}
	if _, err := os.Stat(unitPath); !os.IsNotExist(err) {
		t.Fatalf("expected owned unit file to be removed, stat err: %v", err)
	}
}

// --- uninstall.sh content ----------------------------------------------------

func TestBuildLinuxUninstallScriptContent(t *testing.T) {
	requireLinuxRuntime(t)
	serviceDir := filepath.Join(t.TempDir(), "demo-java")
	unitPath := "/etc/systemd/system/demo-java.service"
	item := &model.WindowsService{Name: "demo-java", DisplayName: "Demo Java"}

	script := buildLinuxUninstallScript(item, serviceDir, unitPath)
	wants := []string{
		"#!/bin/bash",
		managedServiceUnitHeader,
		`systemctl stop "demo-java.service"`,
		`systemctl disable "demo-java.service"`,
		`rm -f "` + unitPath + `"`,
		"systemctl daemon-reload",
		`rm -rf "` + serviceDir + `"`,
	}
	for _, want := range wants {
		if !strings.Contains(script, want) {
			t.Fatalf("expected uninstall script to contain %q, got:\n%s", want, script)
		}
	}
}

// --- start-script command line ----------------------------------------------

func TestBuildLinuxStartScriptRendersJavaExec(t *testing.T) {
	requireLinuxRuntime(t)
	installDir := t.TempDir()
	appDir := filepath.Join(installDir, "app")
	item := &model.WindowsService{
		Name:        "demo-java",
		ServiceType: "java",
		ExecPath:    filepath.Join(installDir, "zulu21-test", "bin", "java"),
		WorkDir:     appDir,
		JarPath:     filepath.Join(appDir, "demo.jar"),
		Args:        "-Xms512m -Xmx512m",
	}

	script := (&WindowsServiceService{}).buildLinuxStartScript(item)
	if !strings.HasPrefix(script, "#!/bin/bash\n") {
		t.Fatalf("expected bash shebang, got:\n%s", script)
	}
	if !strings.Contains(script, `cd "`+appDir+`"`) {
		t.Fatalf("expected cd into workDir, got:\n%s", script)
	}
	expectedExec := `exec "` + item.ExecPath + `" -Xms512m -Xmx512m -jar "` + item.JarPath + `"`
	if !strings.Contains(script, expectedExec) {
		t.Fatalf("expected exec line %q, got:\n%s", expectedExec, script)
	}
}

func TestBuildLinuxStartScriptRendersComposeExec(t *testing.T) {
	requireLinuxRuntime(t)
	installDir := t.TempDir()
	composePath := filepath.Join(installDir, "docker-compose.yml")
	item := &model.WindowsService{
		Name:        "base-auth",
		ServiceType: "package",
		ExecPath:    "docker compose",
		WorkDir:     installDir,
		ConfigPath:  composePath,
		Args:        "base-auth",
	}
	script := (&WindowsServiceService{}).buildLinuxStartScript(item)
	expected := `exec docker compose -f "` + composePath + `" up base-auth`
	if !strings.Contains(script, expected) {
		t.Fatalf("expected compose exec line %q, got:\n%s", expected, script)
	}
}

func TestBuildLinuxStartScriptExportsLibraryPath(t *testing.T) {
	requireLinuxRuntime(t)
	installDir := t.TempDir()
	item := &model.WindowsService{
		Name:        "demo-java",
		ServiceType: "java",
		ExecPath:    filepath.Join(installDir, "bin", "java"),
		WorkDir:     filepath.Join(installDir, "app"),
		JarPath:     filepath.Join(installDir, "app", "demo.jar"),
		DLLDir:      filepath.Join(installDir, "app", "lib"),
	}
	script := (&WindowsServiceService{}).buildLinuxStartScript(item)
	expected := `export LD_LIBRARY_PATH="` + item.DLLDir + `:$LD_LIBRARY_PATH"`
	if !strings.Contains(script, expected) {
		t.Fatalf("expected LD_LIBRARY_PATH export %q, got:\n%s", expected, script)
	}
}

// --- env file: LF, no BOM ----------------------------------------------------

func TestPrepareLinuxArtifactsWritesEnvFileWithLFAndNoBOM(t *testing.T) {
	requireLinuxRuntime(t)
	installDir := t.TempDir()
	restoreInstallDir := global.CONF.Base.InstallDir
	global.CONF.Base.InstallDir = installDir
	defer func() { global.CONF.Base.InstallDir = restoreInstallDir }()

	item := newSystemdTestItem(installDir)
	if err := (&WindowsServiceService{}).prepareArtifacts(item, "", nil); err != nil {
		t.Fatalf("prepareArtifacts returned error: %v", err)
	}

	data, err := os.ReadFile(item.EnvFilePath)
	if err != nil {
		t.Fatalf("failed to read env file: %v", err)
	}
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		t.Fatalf("env file must not start with a UTF-8 BOM: % x", data[:3])
	}
	if strings.Contains(string(data), "\r") {
		t.Fatalf("env file must use LF line endings, got CR in:\n%q", string(data))
	}
	if !strings.Contains(string(data), "JAR_PATH="+item.JarPath) {
		t.Fatalf("expected env file to bind JAR_PATH, got:\n%s", string(data))
	}
}

// --- List returns entries on Linux ------------------------------------------

func TestListReturnsManagedServicesOnLinux(t *testing.T) {
	requireLinuxRuntime(t)
	restoreRepo := windowsServiceRepo
	windowsServiceRepo = testWindowsServiceRepo{
		items: []model.WindowsService{{
			BaseModel:       model.BaseModel{ID: 1},
			Name:            "demo-java",
			DisplayName:     "Demo Java",
			ServiceType:     "java",
			RegisterService: false,
		}},
	}
	defer func() { windowsServiceRepo = restoreRepo }()

	infos, err := (&WindowsServiceService{}).List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(infos) != 1 || infos[0].Name != "demo-java" {
		t.Fatalf("expected one managed service on linux, got %#v", infos)
	}
}

// --- log directory path ------------------------------------------------------

func TestBuildWindowsServiceLogDirIsPlatformScoped(t *testing.T) {
	installDir := t.TempDir()
	restoreInstallDir := global.CONF.Base.InstallDir
	global.CONF.Base.InstallDir = installDir
	defer func() { global.CONF.Base.InstallDir = restoreInstallDir }()

	segment := "windows-services"
	if runtime.GOOS != "windows" {
		segment = "services"
	}
	expected := filepath.Join(installDir, "1panel", "log", segment)
	if got := buildWindowsServiceLogDir(); got != expected {
		t.Fatalf("expected log dir %s, got %s", expected, got)
	}
	candidates := buildWindowsServiceLogCandidates("demo-java")
	if len(candidates) == 0 || filepath.Dir(candidates[0]) != expected {
		t.Fatalf("expected log candidates under %s, got %#v", expected, candidates)
	}
}

// --- UploadJar Linux baseline ------------------------------------------------

func TestUploadJarLinuxStoresUnderManagedRoot(t *testing.T) {
	requireLinuxRuntime(t)
	installDir := t.TempDir()
	restoreInstallDir := global.CONF.Base.InstallDir
	global.CONF.Base.InstallDir = installDir
	defer func() { global.CONF.Base.InstallDir = restoreInstallDir }()

	fakeBundledJava(t, installDir)
	res, err := (&WindowsServiceService{}).UploadJar("demo-java", "java", "demo.jar", strings.NewReader("jar-bytes"))
	if err != nil {
		t.Fatalf("UploadJar returned error: %v", err)
	}
	expectedJar := filepath.Join(installDir, "1panel", "services", "demo-java", "app", "demo.jar")
	if res.JarPath != expectedJar {
		t.Fatalf("expected jar under linux managed root %s, got %s", expectedJar, res.JarPath)
	}
	data, err := os.ReadFile(expectedJar)
	if err != nil || string(data) != "jar-bytes" {
		t.Fatalf("expected stored jar bytes, got data=%q err=%v", string(data), err)
	}
}

// --- java resolution: LookPath fallback -------------------------------------

func TestResolveBundledJavaExecutableFallsBackToLookPath(t *testing.T) {
	requireLinuxRuntime(t)
	installDir := t.TempDir()
	restoreInstallDir := global.CONF.Base.InstallDir
	global.CONF.Base.InstallDir = installDir
	defer func() { global.CONF.Base.InstallDir = restoreInstallDir }()

	// No bundled JRE under installDir; a fake java on PATH must be picked up.
	binDir := t.TempDir()
	fakeJava := filepath.Join(binDir, "java")
	if err := os.WriteFile(fakeJava, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)

	got, err := resolveBundledJavaExecutable()
	if err != nil {
		t.Fatalf("resolveBundledJavaExecutable returned error: %v", err)
	}
	if got != fakeJava {
		t.Fatalf("expected PATH java %s, got %s", fakeJava, got)
	}
}

func TestResolveBundledJavaExecutableErrorsWithInstallHint(t *testing.T) {
	requireLinuxRuntime(t)
	installDir := t.TempDir()
	restoreInstallDir := global.CONF.Base.InstallDir
	global.CONF.Base.InstallDir = installDir
	defer func() { global.CONF.Base.InstallDir = restoreInstallDir }()

	t.Setenv("PATH", t.TempDir()) // empty dir, no java

	_, err := resolveBundledJavaExecutable()
	if err == nil || !strings.Contains(err.Error(), "install a JRE/JDK") {
		t.Fatalf("expected an install-hint error, got %v", err)
	}
}

// --- WinSW is skipped on Linux ----------------------------------------------

func TestManagedServiceSkipsWinSWOnLinux(t *testing.T) {
	requireLinuxRuntime(t)
	installDir := t.TempDir()
	restoreInstallDir := global.CONF.Base.InstallDir
	global.CONF.Base.InstallDir = installDir
	defer func() { global.CONF.Base.InstallDir = restoreInstallDir }()

	if got := defaultManagedServiceWinSWPath(); got != "" {
		t.Fatalf("expected empty WinSW default on linux, got %q", got)
	}

	fakeBundledJava(t, installDir)
	item := model.WindowsService{
		Name:        "demo-java",
		DisplayName: "Demo Java",
		ServiceType: "java",
		JarPath:     filepath.Join(installDir, "1panel", "services", "demo-java", "app", "demo.jar"),
	}
	if err := (&WindowsServiceService{}).normalizeManagedFields(&item); err != nil {
		t.Fatalf("normalizeManagedFields returned error: %v", err)
	}
	if item.WinSWPath != "" {
		t.Fatalf("expected WinSWPath to stay empty on linux, got %q", item.WinSWPath)
	}
}

// --- systemd-analyze verify (optional) --------------------------------------

func TestGeneratedSystemdUnitPassesSystemdAnalyzeVerify(t *testing.T) {
	requireLinuxRuntime(t)
	analyze, err := exec.LookPath("systemd-analyze")
	if err != nil {
		t.Skip("systemd-analyze not available")
	}

	base := t.TempDir()
	serviceDir := filepath.Join(base, "demo-java")
	logDir := filepath.Join(base, "log")
	if err := os.MkdirAll(serviceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// systemd-analyze verify resolves ExecStart's binary, so the launch script must
	// exist on disk; /bin/bash is a real interpreter.
	scriptPath := filepath.Join(serviceDir, "demo-java.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/bash\nexec sleep 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	item := &model.WindowsService{
		Name:        "demo-java",
		DisplayName: "Demo Java",
		ServiceType: "java",
		WorkDir:     serviceDir,
	}
	unit, err := buildSystemdUnit(item, serviceDir, filepath.Join(serviceDir, "demo-java.env"), logDir)
	if err != nil {
		t.Fatalf("buildSystemdUnit returned error: %v", err)
	}
	unitPath := filepath.Join(base, "demo-java.service")
	if err := os.WriteFile(unitPath, []byte(unit), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command(analyze, "verify", unitPath).CombinedOutput()
	if err != nil {
		t.Fatalf("systemd-analyze verify failed: %v\n%s", err, string(out))
	}
}

// --- F1: name normalization / .service suffix rejection ---------------------

func TestValidateWindowsServiceNameRejectsDotServiceSuffixOnLinux(t *testing.T) {
	requireLinuxRuntime(t)
	for _, name := range []string{"myapp.service", "MyApp.SERVICE", "foo.Service"} {
		if err := validateWindowsServiceName(name); err == nil {
			t.Fatalf("expected %q to be rejected on linux (would double-suffix the unit)", name)
		}
	}
	// A normal name, even mixed case, is still accepted (F1: case must be preserved).
	if err := validateWindowsServiceName("MyService"); err != nil {
		t.Fatalf("expected MyService to be accepted, got %v", err)
	}
}

// --- F3: status mapping honors systemd's (true, err) existence contract ------

func TestRefreshRuntimeStatusTreatsDisabledUnitErrorAsPresent(t *testing.T) {
	requireLinuxRuntime(t)
	restoreExist := controllerCheckExist
	restoreActive := controllerCheckActive
	// systemd's IsExist returns (true, exit-1 error) for a disabled-but-present unit;
	// that error must NOT be surfaced as UnHealthy.
	controllerCheckExist = func(string) (bool, error) { return true, errors.New("exit status 1") }
	controllerCheckActive = func(string) (bool, error) { return false, nil }
	defer func() {
		controllerCheckExist = restoreExist
		controllerCheckActive = restoreActive
	}()

	item := &model.WindowsService{Name: "demo-java", RegisterService: true}
	(&WindowsServiceService{}).refreshRuntimeStatus(item)
	if item.Status != "Stopped" {
		t.Fatalf("expected disabled-but-present unit to map to Stopped, got %q (%q)", item.Status, item.Message)
	}
}

func TestRefreshRuntimeStatusMapsAbsentUnitToWaiting(t *testing.T) {
	requireLinuxRuntime(t)
	ensureTestLogger(t)
	restoreExist := controllerCheckExist
	restoreActive := controllerCheckActive
	activeCalled := false
	controllerCheckExist = func(string) (bool, error) {
		return false, errors.New("Failed to get unit file state: No such file or directory")
	}
	controllerCheckActive = func(string) (bool, error) { activeCalled = true; return false, nil }
	defer func() {
		controllerCheckExist = restoreExist
		controllerCheckActive = restoreActive
	}()

	item := &model.WindowsService{Name: "demo-java", RegisterService: true}
	(&WindowsServiceService{}).refreshRuntimeStatus(item)
	if item.Status != "Waiting" || item.Message != "service not installed" {
		t.Fatalf("expected absent unit to map to Waiting/service not installed, got %q (%q)", item.Status, item.Message)
	}
	if activeCalled {
		t.Fatal("expected CheckActive not to be called when the unit does not exist")
	}
}

// --- F2: secret-bearing files are not world-readable on Linux ----------------

func TestPrepareLinuxArtifactsWritesEnvFileRestricted(t *testing.T) {
	requireLinuxRuntime(t)
	installDir := t.TempDir()
	restoreInstallDir := global.CONF.Base.InstallDir
	global.CONF.Base.InstallDir = installDir
	defer func() { global.CONF.Base.InstallDir = restoreInstallDir }()

	item := newSystemdTestItem(installDir)
	if err := (&WindowsServiceService{}).prepareArtifacts(item, "", nil); err != nil {
		t.Fatalf("prepareArtifacts returned error: %v", err)
	}
	info, err := os.Stat(item.EnvFilePath)
	if err != nil {
		t.Fatalf("failed to stat env file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("expected env file mode 0600 (may contain secrets), got %#o", perm)
	}
}

func TestWriteManagedWindowsServiceConfigUsesRestrictedModeOnLinux(t *testing.T) {
	requireLinuxRuntime(t)
	configPath := filepath.Join(t.TempDir(), "app", "application.yml")
	if err := writeManagedWindowsServiceConfig(configPath, "server:\n  port: 8080\n"); err != nil {
		t.Fatalf("writeManagedWindowsServiceConfig returned error: %v", err)
	}
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("failed to stat config file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("expected config file mode 0600 (may contain secrets), got %#o", perm)
	}
}

// --- F6: spring config path is shell-escaped in the launch script ------------

func TestBuildLinuxStartScriptEscapesSpringConfigPath(t *testing.T) {
	configPath := `/opt/app/$(id).yml`
	item := &model.WindowsService{
		Name:        "demo-java",
		ServiceType: "java",
		ExecPath:    "/opt/java/bin/java",
		WorkDir:     "/opt/app",
		JarPath:     "/opt/app/demo.jar",
		ConfigPath:  configPath,
		Args:        `--spring.config.location="` + configPath + `"`,
	}
	script := (&WindowsServiceService{}).buildLinuxStartScript(item)
	want := `--spring.config.location="/opt/app/\$(id).yml"`
	if !strings.Contains(script, want) {
		t.Fatalf("expected escaped spring config path %q in:\n%s", want, script)
	}
	if strings.Contains(script, `location="/opt/app/$(id).yml"`) {
		t.Fatalf("spring config path was emitted unescaped (command injection) in:\n%s", script)
	}
}

// --- F7: compose service names in Args are validated on the direct path ------

func TestNormalizeManagedFieldsRejectsInjectionInComposeArgs(t *testing.T) {
	requireLinuxRuntime(t)
	newItem := func(args string) *model.WindowsService {
		return &model.WindowsService{
			Name:        "compose-svc",
			DisplayName: "Compose Svc",
			ServiceType: "package",
			ExecPath:    "docker compose",
			ConfigPath:  "/opt/app/docker-compose.yml",
			Args:        args,
		}
	}
	if err := (&WindowsServiceService{}).normalizeManagedFields(newItem("up; touch /tmp/pwned")); err == nil {
		t.Fatal("expected an injection-laden compose service name to be rejected")
	}
	// A legal single service name and empty Args (=> all services) both pass.
	if err := (&WindowsServiceService{}).normalizeManagedFields(newItem("base-auth")); err != nil {
		t.Fatalf("expected legal compose service name to pass, got %v", err)
	}
	if err := (&WindowsServiceService{}).normalizeManagedFields(newItem("")); err != nil {
		t.Fatalf("expected empty compose args to pass, got %v", err)
	}
}

// --- F8: operate=status is read-only and never drives the controller ---------

func TestOperateStatusIsReadOnlyAndDoesNotDriveController(t *testing.T) {
	requireLinuxRuntime(t)
	restoreRepo := windowsServiceRepo
	windowsServiceRepo = testWindowsServiceRepo{item: model.WindowsService{
		BaseModel:       model.BaseModel{ID: 1},
		Name:            "demo-java",
		ServiceType:     "java",
		RegisterService: true,
	}}
	defer func() { windowsServiceRepo = restoreRepo }()

	handleCalled := 0
	restoreHandle := controllerHandle
	restoreExist := controllerCheckExist
	restoreActive := controllerCheckActive
	controllerHandle = func(string, string) error { handleCalled++; return nil }
	controllerCheckExist = func(string) (bool, error) { return true, nil }
	controllerCheckActive = func(string) (bool, error) { return true, nil }
	defer func() {
		controllerHandle = restoreHandle
		controllerCheckExist = restoreExist
		controllerCheckActive = restoreActive
	}()

	info, err := (&WindowsServiceService{}).Operate(request.WindowsServiceOperate{ID: 1, Operate: "status"})
	if err != nil {
		t.Fatalf("Operate(status) returned error: %v", err)
	}
	if handleCalled != 0 {
		t.Fatalf("expected status to be read-only (zero controllerHandle calls), got %d", handleCalled)
	}
	if info == nil || info.Status != "Healthy" {
		t.Fatalf("expected refreshed Healthy status, got %#v", info)
	}
}

// --- F5: renaming a registered service tears down the old unit + dir ----------

func TestUpdateRenameTearsDownOldRegistrationAndDir(t *testing.T) {
	requireLinuxRuntime(t)
	if _, err := exec.LookPath("systemctl"); err != nil {
		t.Skip("systemctl not available")
	}
	swapSystemdUnitDir(t)

	installDir := t.TempDir()
	restoreInstallDir := global.CONF.Base.InstallDir
	global.CONF.Base.InstallDir = installDir
	defer func() { global.CONF.Base.InstallDir = restoreInstallDir }()
	fakeBundledJava(t, installDir)

	oldName := "old-svc"
	oldServiceDir := buildWindowsServiceDir(oldName)
	if err := os.MkdirAll(oldServiceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldServiceDir, "sentinel.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldUnitPath := linuxSystemdUnitPath(oldName)
	if err := os.WriteFile(oldUnitPath, []byte(managedServiceUnitHeader+"\n[Unit]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	restoreRepo := windowsServiceRepo
	windowsServiceRepo = testWindowsServiceRepo{item: model.WindowsService{
		BaseModel:       model.BaseModel{ID: 1},
		Name:            oldName,
		DisplayName:     "Old Svc",
		ServiceType:     "java",
		JarPath:         filepath.Join(oldServiceDir, "app", "demo.jar"),
		ServicePath:     filepath.Join(oldServiceDir, oldName+".service"),
		RegisterService: true,
	}}
	defer func() { windowsServiceRepo = restoreRepo }()

	var calls []string
	restoreHandle := controllerHandle
	restoreReload := controllerReload
	restoreExist := controllerCheckExist
	controllerHandle = func(op, name string) error { calls = append(calls, op+":"+name); return nil }
	controllerReload = func() error { calls = append(calls, "reload"); return nil }
	controllerCheckExist = func(string) (bool, error) { return false, nil }
	defer func() {
		controllerHandle = restoreHandle
		controllerReload = restoreReload
		controllerCheckExist = restoreExist
	}()

	newName := "new-svc"
	newJar := filepath.Join(buildWindowsServiceDir(newName), "app", "demo.jar")
	if err := os.MkdirAll(filepath.Dir(newJar), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newJar, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	register := true
	req := request.WindowsServiceUpdate{ID: 1}
	req.Name = newName
	req.DisplayName = "New Svc"
	req.ServiceType = "java"
	req.JarPath = newJar
	req.WorkDir = filepath.Dir(newJar)
	req.RegisterService = &register

	if err := (&WindowsServiceService{}).Update(req); err != nil {
		t.Fatalf("Update(rename) returned error: %v", err)
	}

	oldUnitName := managedControllerUnitName(oldName)
	joined := strings.Join(calls, ",")
	if !strings.Contains(joined, "stop:"+oldUnitName) || !strings.Contains(joined, "disable:"+oldUnitName) {
		t.Fatalf("expected old registration to be stopped+disabled under its old name, got %v", calls)
	}
	if _, err := os.Stat(oldUnitPath); !os.IsNotExist(err) {
		t.Fatalf("expected old unit file removed, stat err: %v", err)
	}
	if _, err := os.Stat(oldServiceDir); !os.IsNotExist(err) {
		t.Fatalf("expected old service dir removed, stat err: %v", err)
	}
	if _, err := os.Stat(linuxSystemdUnitPath(newName)); err != nil {
		t.Fatalf("expected new unit installed under new name: %v", err)
	}
}

// --- R1: anti-hijack guard covers disabled vendor units and DBus failures ----

// swapVendorUnitDirs points systemdVendorUnitDirs at the given dirs for the test,
// so guard behavior does not depend on the host's real /lib/systemd/system.
func swapVendorUnitDirs(t *testing.T, dirs ...string) {
	t.Helper()
	restore := systemdVendorUnitDirs
	systemdVendorUnitDirs = dirs
	t.Cleanup(func() { systemdVendorUnitDirs = restore })
}

func TestEnsureLinuxServiceRegisterableRejectsDisabledSystemUnit(t *testing.T) {
	swapVendorUnitDirs(t, t.TempDir())
	restoreExist := controllerCheckExist
	// systemd reports an installed-but-disabled vendor unit as (true, exit-1 error);
	// existence is authoritative and the guard must reject regardless of the error.
	controllerCheckExist = func(string) (bool, error) { return true, errors.New("exit status 1") }
	defer func() { controllerCheckExist = restoreExist }()

	unitPath := filepath.Join(t.TempDir(), "nginx.service")
	if err := ensureLinuxServiceRegisterable("nginx", unitPath); err == nil {
		t.Fatal("expected a disabled-but-present system unit to be refused (shadowing)")
	}
}

func TestEnsureLinuxServiceRegisterableRejectsVendorUnitFileOnDisk(t *testing.T) {
	vendorDir := t.TempDir()
	swapVendorUnitDirs(t, vendorDir)
	restoreExist := controllerCheckExist
	// Even if the controller cannot see the unit at all (DBus down -> (false, err)),
	// the on-disk vendor unit file alone must block registration.
	controllerCheckExist = func(string) (bool, error) { return false, errors.New("dbus connection refused") }
	defer func() { controllerCheckExist = restoreExist }()

	if err := os.WriteFile(filepath.Join(vendorDir, "docker.service"), []byte("[Unit]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	unitPath := filepath.Join(t.TempDir(), "docker.service")
	if err := ensureLinuxServiceRegisterable("docker", unitPath); err == nil {
		t.Fatal("expected a vendor-dir unit file to be refused without needing systemctl")
	}
}

func TestEnsureLinuxServiceRegisterableAllowsFreshNameDespiteExistError(t *testing.T) {
	swapVendorUnitDirs(t, t.TempDir())
	restoreExist := controllerCheckExist
	// A genuinely fresh name yields (false, "No such file...") from systemd; the
	// error alone must not fail-close or every legitimate create would be rejected.
	controllerCheckExist = func(string) (bool, error) {
		return false, errors.New("Failed to get unit file state: No such file or directory")
	}
	defer func() { controllerCheckExist = restoreExist }()

	unitPath := filepath.Join(t.TempDir(), "fresh-app.service")
	if err := ensureLinuxServiceRegisterable("fresh-app", unitPath); err != nil {
		t.Fatalf("expected a fresh name to be registerable despite the IsExist error, got %v", err)
	}
}

// --- R2: rename never destroys the old service before the new one exists -----

func TestUpdateRenameKeepsOldServiceWhenPrepareFails(t *testing.T) {
	requireLinuxRuntime(t)
	if _, err := exec.LookPath("systemctl"); err != nil {
		t.Skip("systemctl not available")
	}
	swapSystemdUnitDir(t)

	installDir := t.TempDir()
	restoreInstallDir := global.CONF.Base.InstallDir
	global.CONF.Base.InstallDir = installDir
	defer func() { global.CONF.Base.InstallDir = restoreInstallDir }()
	fakeBundledJava(t, installDir)

	oldName := "old-keep-svc"
	oldServiceDir := buildWindowsServiceDir(oldName)
	if err := os.MkdirAll(oldServiceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(oldServiceDir, "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldUnitPath := linuxSystemdUnitPath(oldName)
	if err := os.WriteFile(oldUnitPath, []byte(managedServiceUnitHeader+"\n[Unit]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	restoreRepo := windowsServiceRepo
	windowsServiceRepo = testWindowsServiceRepo{item: model.WindowsService{
		BaseModel:       model.BaseModel{ID: 1},
		Name:            oldName,
		DisplayName:     "Old Keep Svc",
		ServiceType:     "java",
		JarPath:         filepath.Join(oldServiceDir, "app", "demo.jar"),
		ServicePath:     filepath.Join(oldServiceDir, oldName+".service"),
		RegisterService: true,
	}}
	defer func() { windowsServiceRepo = restoreRepo }()

	var calls []string
	restoreHandle := controllerHandle
	restoreReload := controllerReload
	restoreExist := controllerCheckExist
	controllerHandle = func(op, name string) error { calls = append(calls, op+":"+name); return nil }
	controllerReload = func() error { return nil }
	controllerCheckExist = func(string) (bool, error) { return false, nil }
	defer func() {
		controllerHandle = restoreHandle
		controllerReload = restoreReload
		controllerCheckExist = restoreExist
	}()

	register := true
	req := request.WindowsServiceUpdate{ID: 1}
	req.Name = "new-broken-svc"
	req.DisplayName = "New Broken Svc"
	req.ServiceType = "java"
	req.JarPath = filepath.Join(installDir, "demo.jar")
	// A newline inside WorkDir is rejected by sanitizeUnitValue during
	// prepareArtifacts -> buildSystemdUnitFile, i.e. the new service fails to
	// prepare AFTER normalization passed — exactly the destroy-then-fail window.
	req.WorkDir = filepath.Join(installDir, "app") + "\nExecStartPre=/bin/true"
	req.RegisterService = &register

	if err := (&WindowsServiceService{}).Update(req); err == nil {
		t.Fatal("expected Update to fail when the new service cannot be prepared")
	}

	// The old registration and directory must be completely untouched.
	if _, err := os.Stat(oldUnitPath); err != nil {
		t.Fatalf("old unit file must survive a failed rename: %v", err)
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("old service dir must survive a failed rename: %v", err)
	}
	oldUnitName := managedControllerUnitName(oldName)
	for _, call := range calls {
		if strings.HasSuffix(call, ":"+oldUnitName) {
			t.Fatalf("old registration must not be driven during a failed rename, got %v", calls)
		}
	}
}

// --- R3: compose Args whitespace is normalized to single spaces --------------

func TestNormalizeManagedFieldsNormalizesComposeArgsWhitespace(t *testing.T) {
	requireLinuxRuntime(t)
	item := &model.WindowsService{
		Name:        "compose-multi",
		DisplayName: "Compose Multi",
		ServiceType: "package",
		ExecPath:    "docker compose",
		ConfigPath:  "/opt/app/docker-compose.yml",
		Args:        "svc1\nsvc2\tsvc3",
	}
	if err := (&WindowsServiceService{}).normalizeManagedFields(item); err != nil {
		t.Fatalf("normalizeManagedFields returned error: %v", err)
	}
	if item.Args != "svc1 svc2 svc3" {
		t.Fatalf("expected Args normalized to single spaces, got %q", item.Args)
	}
	script := (&WindowsServiceService{}).buildLinuxStartScript(item)
	if !strings.Contains(script, `up svc1 svc2 svc3`) {
		t.Fatalf("expected single-line compose exec, got:\n%s", script)
	}
	for _, line := range strings.Split(script, "\n") {
		if strings.HasPrefix(line, "exec ") {
			continue
		}
		if strings.Contains(line, "svc2") || strings.Contains(line, "svc3") {
			t.Fatalf("compose service names leaked onto their own script line:\n%s", script)
		}
	}
}

// --- R4: enable/disable persist the real runtime state, not a fake Healthy ---

func TestOperateEnablePersistsRealRuntimeStatus(t *testing.T) {
	requireLinuxRuntime(t)
	restoreRepo := windowsServiceRepo
	windowsServiceRepo = testWindowsServiceRepo{item: model.WindowsService{
		BaseModel:       model.BaseModel{ID: 1},
		Name:            "demo-java",
		ServiceType:     "java",
		RegisterService: true,
	}}
	defer func() { windowsServiceRepo = restoreRepo }()

	var handleOps []string
	restoreHandle := controllerHandle
	restoreExist := controllerCheckExist
	restoreActive := controllerCheckActive
	controllerHandle = func(op, name string) error { handleOps = append(handleOps, op); return nil }
	controllerCheckExist = func(string) (bool, error) { return true, nil }
	// The unit was enabled but the process is NOT running.
	controllerCheckActive = func(string) (bool, error) { return false, nil }
	defer func() {
		controllerHandle = restoreHandle
		controllerCheckExist = restoreExist
		controllerCheckActive = restoreActive
	}()

	info, err := (&WindowsServiceService{}).Operate(request.WindowsServiceOperate{ID: 1, Operate: "enable"})
	if err != nil {
		t.Fatalf("Operate(enable) returned error: %v", err)
	}
	if len(handleOps) != 1 || handleOps[0] != "enable" {
		t.Fatalf("expected exactly one enable controller call, got %v", handleOps)
	}
	if !info.AutoStart {
		t.Fatal("expected AutoStart to be flipped on")
	}
	if info.Status != "Stopped" {
		t.Fatalf("enable must not fake Healthy for a non-running process; expected Stopped, got %q", info.Status)
	}
}
