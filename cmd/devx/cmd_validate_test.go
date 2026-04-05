package main

import (
	"os"
	"path/filepath"
	"testing"
)

// writeManifest writes content to devx.yaml in a temp dir and returns
// the file path and a cleanup func that restores the original working directory.
func writeManifest(t *testing.T, content string) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	filePath := filepath.Join(dir, "devx.yaml")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return filePath, func() {}
}

func TestRunValidate_ValidManifest(t *testing.T) {
	path, cleanup := writeManifest(t, `version: 1
project:
  name: my-app
  defaultProfile: local
profiles:
  local:
    services:
      api:
        image: nginx:alpine
`)
	defer cleanup()

	if err := runValidate([]string{"--file", path}); err != nil {
		t.Fatalf("expected no error for valid manifest, got: %v", err)
	}
}

func TestRunValidate_WithTools(t *testing.T) {
	path, cleanup := writeManifest(t, `version: 1
project:
  name: my-app
  defaultProfile: local
profiles:
  local:
    services:
      api:
        image: nginx:alpine
tools:
  - name: go
    check: "go version"
    install:
      linux: "apt-get install golang"
`)
	defer cleanup()

	if err := runValidate([]string{"--file", path}); err != nil {
		t.Fatalf("expected no error for manifest with valid tools, got: %v", err)
	}
}

func TestRunValidate_WithSetup(t *testing.T) {
	path, cleanup := writeManifest(t, `version: 1
project:
  name: my-app
  defaultProfile: local
profiles:
  local:
    services:
      api:
        image: nginx:alpine
setup:
  - name: restore
    run: "npm install"
    runOnce: true
    platform: all
`)
	defer cleanup()

	if err := runValidate([]string{"--file", path}); err != nil {
		t.Fatalf("expected no error for manifest with valid setup, got: %v", err)
	}
}

func TestRunValidate_InvalidProfile(t *testing.T) {
	path, cleanup := writeManifest(t, `version: 1
project:
  name: my-app
  defaultProfile: local
profiles:
  local:
    services:
      api:
        image: nginx:alpine
        dependsOn:
          - nonexistent-dep
`)
	defer cleanup()

	if err := runValidate([]string{"--file", path}); err == nil {
		t.Fatal("expected error for manifest with invalid profile")
	}
}

func TestRunValidate_InvalidTools(t *testing.T) {
	path, cleanup := writeManifest(t, `version: 1
project:
  name: my-app
  defaultProfile: local
profiles:
  local:
    services:
      api:
        image: nginx:alpine
tools:
  - name: badtool
`)
	defer cleanup()

	if err := runValidate([]string{"--file", path}); err == nil {
		t.Fatal("expected error for tool missing check command")
	}
}

func TestRunValidate_InvalidSetup(t *testing.T) {
	path, cleanup := writeManifest(t, `version: 1
project:
  name: my-app
  defaultProfile: local
profiles:
  local:
    services:
      api:
        image: nginx:alpine
setup:
  - name: bad-step
    run: "echo ok"
    platform: freebsd
`)
	defer cleanup()

	if err := runValidate([]string{"--file", path}); err == nil {
		t.Fatal("expected error for step with invalid platform")
	}
}

func TestRunValidate_FileNotFound(t *testing.T) {
	err := runValidate([]string{"--file", "/nonexistent/path/devx.yaml"})
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestRunValidate_MalformedYAML(t *testing.T) {
	path, cleanup := writeManifest(t, `:::this is not valid yaml:::`)
	defer cleanup()

	if err := runValidate([]string{"--file", path}); err == nil {
		t.Fatal("expected error for malformed YAML")
	}
}

func TestRunValidate_DefaultFile(t *testing.T) {
	// Uses devx.yaml from the current directory.
	defer chdirTemp(t, validManifest)()

	if err := runValidate([]string{}); err != nil {
		t.Fatalf("expected no error using default file path, got: %v", err)
	}
}

// ── setupStatusIcon ────────────────────────────────────────────────────────

func TestSetupStatusIcon(t *testing.T) {
	cases := []struct {
		status string
		want   string
	}{
		{"ok", "✓"},
		{"installed", "✓"},
		{"skipped", "–"},
		{"not-applicable", "–"},
		{"failed", "✗"},
		{"missing", "✗"},
		{"unknown", "?"},
		{"", "?"},
	}
	for _, tc := range cases {
		got := setupStatusIcon(tc.status)
		if got != tc.want {
			t.Errorf("setupStatusIcon(%q) = %q, want %q", tc.status, got, tc.want)
		}
	}
}
