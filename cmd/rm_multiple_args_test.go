package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func buildKitcatBinary(t *testing.T, dir string) string {
	t.Helper()

	binName := "kitcat"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(dir, binName)

	buildCmd := exec.Command("go", "build", "-o", binPath, "main.go")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build kitcat binary: %v\nOutput: %s", err, output)
	}

	return binPath
}

func readIndex(t *testing.T, repoDir string) map[string]string {
	t.Helper()

	indexPath := filepath.Join(repoDir, ".kitcat", "index")
	content, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("Failed to read index file: %v", err)
	}
	idx := map[string]string{}
	if len(content) == 0 {
		return idx
	}
	if err := json.Unmarshal(content, &idx); err != nil {
		t.Fatalf("Failed to parse index JSON: %v\nContent: %s", err, content)
	}
	return idx
}

func TestRMMultipleFilesRemovesAll(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := buildKitcatBinary(t, tmpDir)

	initCmd := exec.Command(binPath, "init")
	initCmd.Dir = tmpDir
	if output, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("init failed: %v\nOutput: %s", err, output)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("one"), 0o644); err != nil {
		t.Fatalf("write file1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("two"), 0o644); err != nil {
		t.Fatalf("write file2: %v", err)
	}

	addCmd := exec.Command(binPath, "add", "file1.txt", "file2.txt")
	addCmd.Dir = tmpDir
	if output, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("add failed: %v\nOutput: %s", err, output)
	}

	rmCmd := exec.Command(binPath, "rm", "file1.txt", "file2.txt")
	rmCmd.Dir = tmpDir
	if output, err := rmCmd.CombinedOutput(); err != nil {
		t.Fatalf("rm failed: %v\nOutput: %s", err, output)
	}

	if _, err := os.Stat(filepath.Join(tmpDir, "file1.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected file1.txt removed from disk; stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "file2.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected file2.txt removed from disk; stat err=%v", err)
	}

	idx := readIndex(t, tmpDir)
	if _, ok := idx["file1.txt"]; ok {
		t.Fatalf("expected file1.txt removed from index")
	}
	if _, ok := idx["file2.txt"]; ok {
		t.Fatalf("expected file2.txt removed from index")
	}
}

func TestRMContinuesAfterErrorAndReturnsNonZero(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := buildKitcatBinary(t, tmpDir)

	initCmd := exec.Command(binPath, "init")
	initCmd.Dir = tmpDir
	if output, err := initCmd.CombinedOutput(); err != nil {
		t.Fatalf("init failed: %v\nOutput: %s", err, output)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("one"), 0o644); err != nil {
		t.Fatalf("write file1: %v", err)
	}

	addCmd := exec.Command(binPath, "add", "file1.txt")
	addCmd.Dir = tmpDir
	if output, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("add failed: %v\nOutput: %s", err, output)
	}

	rmCmd := exec.Command(binPath, "rm", "file1.txt", "does-not-exist.txt")
	rmCmd.Dir = tmpDir
	err := rmCmd.Run()
	if err == nil {
		t.Fatalf("expected non-zero exit code when one removal fails")
	}
	if ee, ok := err.(*exec.ExitError); ok {
		if ee.ExitCode() == 0 {
			t.Fatalf("expected non-zero exit code, got 0")
		}
	} else {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}

	// It should still have removed the first tracked file.
	if _, err := os.Stat(filepath.Join(tmpDir, "file1.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected file1.txt removed from disk even when later arg fails; stat err=%v", err)
	}
	idx := readIndex(t, tmpDir)
	if _, ok := idx["file1.txt"]; ok {
		t.Fatalf("expected file1.txt removed from index even when later arg fails")
	}
}
