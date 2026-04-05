//go:build integration

// Package integration_test contains end-to-end tests that build and exercise
// the devx binary as a subprocess. Run with:
//
//	go test ./tests/integration/... -tags integration -v -timeout 5m
package integration_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"testing"
)

// devxBin is the path to the compiled devx binary, set by TestMain.
var devxBin string

// TestMain builds the devx binary once and runs all integration tests against it.
func TestMain(m *testing.M) {
	bin, cleanup, err := buildBinary()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: could not build devx binary: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()
	devxBin = bin
	os.Exit(m.Run())
}

// buildBinary compiles cmd/devx to a temp directory and returns the binary
// path plus a cleanup func.
func buildBinary() (binPath string, cleanup func(), err error) {
	root, err := findProjectRoot()
	if err != nil {
		return "", nil, err
	}

	dir, err := os.MkdirTemp("", "devx-integration-*")
	if err != nil {
		return "", nil, err
	}

	bin := filepath.Join(dir, "devx")
	if goruntime.GOOS == "windows" {
		bin += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", bin, "./cmd/devx")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, fmt.Errorf("go build failed:\n%s\n%w", out, err)
	}

	return bin, func() { _ = os.RemoveAll(dir) }, nil
}

// findProjectRoot walks up from the current working directory until it finds
// a go.mod file, then returns that directory.
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found starting from %s", dir)
		}
		dir = parent
	}
}

// withManifest creates a temp directory containing a devx.yaml with the given
// content and returns the directory path. Cleanup is registered with t.Cleanup.
func withManifest(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "devx.yaml"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// run executes the devx binary with the provided args in workdir (pass "" to
// inherit the test process's working directory). It returns stdout, stderr, and
// the process exit code.
func run(t *testing.T, workdir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(devxBin, args...)
	if workdir != "" {
		cmd.Dir = workdir
	}
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return outBuf.String(), errBuf.String(), exitErr.ExitCode()
		}
		t.Fatalf("unexpected error running devx %v: %v", args, err)
	}
	return outBuf.String(), errBuf.String(), 0
}
