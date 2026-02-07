package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func buildKitcatBinary(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	binName := "kitcat"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(tmpDir, binName)

	buildCmd := exec.Command("go", "build", "-o", binPath, "main.go")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build kitcat binary: %v\nOutput: %s", err, output)
	}

	return binPath
}

func TestResetTooManyArgumentsExitCode(t *testing.T) {
	binPath := buildKitcatBinary(t)

	cmd := exec.Command(binPath, "reset", "--hard", "HEAD", "extra")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("Expected non-zero exit code, got 0. Output: %s", output)
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("Expected ExitError, got %T: %v", err, err)
	}

	if code := exitErr.ExitCode(); code != 2 {
		t.Fatalf("Expected exit code 2, got %d. Output: %s", code, output)
	}

	if !strings.Contains(string(output), "Error: too many arguments") {
		t.Fatalf("Expected error message 'Error: too many arguments'. Output: %s", output)
	}
}

func TestResetNoArgsShowsUsageExitCode(t *testing.T) {
	binPath := buildKitcatBinary(t)

	cmd := exec.Command(binPath, "reset")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("Expected non-zero exit code, got 0. Output: %s", output)
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("Expected ExitError, got %T: %v", err, err)
	}

	if code := exitErr.ExitCode(); code != 2 {
		t.Fatalf("Expected exit code 2, got %d. Output: %s", code, output)
	}

	if !strings.Contains(string(output), "Usage: kitkat reset") {
		t.Fatalf("Expected usage message. Output: %s", output)
	}
}
