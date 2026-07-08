package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/1Panel-dev/1Panel/agent/app/model"
)

func TestValidateWindowsServiceNameRejectsTraversalAndMetacharacters(t *testing.T) {
	valid := []string{"demo-service", "base_auth", "app.v2", "Service01"}
	for _, name := range valid {
		if err := validateWindowsServiceName(name); err != nil {
			t.Fatalf("expected %q to be valid, got %v", name, err)
		}
	}

	invalid := []string{
		"",
		"..",
		`..\..\Windows\Temp\x`,
		"../../etc/passwd",
		"a/b",
		`a\b`,
		"svc;calc",
		"svc|calc",
		"svc$(calc)",
		"svc`calc`",
		"svc name",
		".hidden",
		"-leading",
	}
	for _, name := range invalid {
		if err := validateWindowsServiceName(name); err == nil {
			t.Fatalf("expected %q to be rejected", name)
		}
	}
}

func TestPathWithinBlocksEscape(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "svc", "a.xml")
	if !pathWithin(root, filepath.Dir(inside)) {
		t.Fatalf("expected %q to be within %q", inside, root)
	}
	if !pathWithin(root, root) {
		t.Fatalf("expected root to be within itself")
	}
	outside := filepath.Join(root, "..", "other")
	if pathWithin(root, outside) {
		t.Fatalf("expected %q to be reported outside %q", outside, root)
	}
	if pathWithin(root, "") {
		t.Fatalf("expected empty target to be outside")
	}
}

func TestSanitizeServiceNameStripsMetacharacters(t *testing.T) {
	cases := map[string]string{
		"base_auth":  "base-auth",
		"base.auth":  "base-auth",
		"svc;calc":   "svc-calc",
		"svc$(calc)": "svc-calc",
		"  a  b  ":   "a-b",
		"a//b\\c":    "a-b-c",
		";;;":        "",
	}
	for in, want := range cases {
		if got := sanitizeServiceName(in); got != want {
			t.Fatalf("sanitizeServiceName(%q)=%q, want %q", in, got, want)
		}
	}
}

func TestIsValidComposeServiceNameRejectsInjection(t *testing.T) {
	for _, ok := range []string{"base_auth", "web.api", "svc-1"} {
		if !isValidComposeServiceName(ok) {
			t.Fatalf("expected %q to be a valid compose service name", ok)
		}
	}
	for _, bad := range []string{"", "svc;calc", "svc up; calc", "svc$(id)", "svc`id`", "svc|nc"} {
		if isValidComposeServiceName(bad) {
			t.Fatalf("expected %q to be rejected as compose service name", bad)
		}
	}
}

func TestSafeConfigFileNameStripsDirectories(t *testing.T) {
	cases := map[string]string{
		"application.yml":        "application.yml",
		`..\..\Windows\evil.yml`: "evil.yml",
		"../../etc/cron.d/x":     "x",
		"sub/dir/app.yml":        "app.yml",
		"..":                     "",
		".":                      "",
	}
	for in, want := range cases {
		if got := safeConfigFileName(in); got != want {
			t.Fatalf("safeConfigFileName(%q)=%q, want %q", in, got, want)
		}
	}
}

func TestGeneratedPathStaysInsideWorkDirConfig(t *testing.T) {
	workDir := filepath.Join(t.TempDir(), "app")
	cfg := managedWindowsServiceConfig{FileName: `..\..\Windows\Temp\evil.yml`}
	got := cfg.generatedPath(workDir)
	want := filepath.Join(workDir, "config", "evil.yml")
	if got != want {
		t.Fatalf("generatedPath escaped: got %q, want %q", got, want)
	}
}

func TestBuildPowerShellScriptDropsInvokeExpression(t *testing.T) {
	installDir := t.TempDir()
	item := model.WindowsService{
		Name:        "demo-java",
		DisplayName: "Demo Java",
		ServiceType: "java",
		ExecPath:    filepath.Join(installDir, "java.exe"),
		WorkDir:     filepath.Join(installDir, "app"),
		JarPath:     filepath.Join(installDir, "app", "demo.jar"),
	}
	svc := &WindowsServiceService{}
	script := svc.buildPowerShellScript(&item)
	if strings.Contains(script, "Invoke-Expression") {
		t.Fatalf("generated script must not use Invoke-Expression:\n%s", script)
	}
}

func TestAtomicWriteFileReplacesContentAndPreservesPerm(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "application.yml")
	if err := os.WriteFile(path, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := atomicWriteFile(path, []byte("new content"), 0o640); err != nil {
		t.Fatalf("atomicWriteFile returned error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new content" {
		t.Fatalf("expected %q, got %q", "new content", string(data))
	}
	// No stray temp files should remain in the directory.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("expected only the target file to remain, got %v", names)
	}
}

func TestReadFileWithLimitRejectsOversizedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.bin")
	if err := os.WriteFile(path, make([]byte, 1024), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readFileWithLimit(path, 512); err == nil {
		t.Fatalf("expected oversized file to be rejected")
	}
	if _, err := readFileWithLimit(path, 4096); err != nil {
		t.Fatalf("expected file within limit to read, got %v", err)
	}
}
