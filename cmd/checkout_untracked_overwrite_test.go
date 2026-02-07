package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func buildKitcatBinaryForTest(t *testing.T) string {
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

func runCmd(t *testing.T, dir string, name string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return string(out), ee.ExitCode()
	}
	return string(out), 1
}

func TestCheckoutBranchDoesNotOverwriteUntrackedFiles(t *testing.T) {
	binPath := buildKitcatBinaryForTest(t)
	repoDir := t.TempDir()

	// init
	if out, code := runCmd(t, repoDir, binPath, "init"); code != 0 {
		t.Fatalf("init failed (code=%d): %s", code, out)
	}

	// base commit on main
	if err := os.WriteFile(filepath.Join(repoDir, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, code := runCmd(t, repoDir, binPath, "add", "base.txt"); code != 0 {
		t.Fatalf("add failed (code=%d): %s", code, out)
	}
	if out, code := runCmd(t, repoDir, binPath, "commit", "-m", "base"); code != 0 {
		t.Fatalf("commit failed (code=%d): %s", code, out)
	}

	// create branch with tracked file "untracked.txt"
	if out, code := runCmd(t, repoDir, binPath, "checkout", "-b", "withfile"); code != 0 {
		t.Fatalf("checkout -b failed (code=%d): %s", code, out)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "untracked.txt"), []byte("from-branch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, code := runCmd(t, repoDir, binPath, "add", "untracked.txt"); code != 0 {
		t.Fatalf("add untracked.txt failed (code=%d): %s", code, out)
	}
	if out, code := runCmd(t, repoDir, binPath, "commit", "-m", "add file"); code != 0 {
		t.Fatalf("commit add file failed (code=%d): %s", code, out)
	}

	// checkout back to main (removes tracked untracked.txt)
	if out, code := runCmd(t, repoDir, binPath, "checkout", "main"); code != 0 {
		t.Fatalf("checkout main failed (code=%d): %s", code, out)
	}

	// Ensure the conflicting file is ignored so IsWorkDirDirty does not block the checkout (regression scenario).
	if f, err := os.OpenFile(filepath.Join(repoDir, ".kitignore"), os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		_, _ = f.WriteString("untracked.txt\n")
		_ = f.Close()
	}

	// create local untracked file that conflicts with target branch
	localContent := []byte("local-work\n")
	if err := os.WriteFile(filepath.Join(repoDir, "untracked.txt"), localContent, 0o644); err != nil {
		t.Fatal(err)
	}

	out, code := runCmd(t, repoDir, binPath, "checkout", "withfile")
	if code == 0 {
		t.Fatalf("expected checkout to fail, got code=0. Output: %s", out)
	}
	if !strings.Contains(out, "untracked file") {
		t.Fatalf("expected error mentioning untracked file overwrite. Output: %s", out)
	}

	// file should be preserved
	after, err := os.ReadFile(filepath.Join(repoDir, "untracked.txt"))
	if err != nil {
		t.Fatalf("expected untracked.txt to still exist, read failed: %v", err)
	}
	if string(after) != string(localContent) {
		t.Fatalf("expected untracked.txt preserved; got: %q", string(after))
	}

	// HEAD should still point to main (checkout aborted before HEAD change)
	headBytes, err := os.ReadFile(filepath.Join(repoDir, ".kitcat", "HEAD"))
	if err != nil {
		t.Fatalf("failed reading HEAD: %v", err)
	}
	if !strings.Contains(string(headBytes), "refs/heads/main") {
		t.Fatalf("expected HEAD to remain on main; got: %q", string(headBytes))
	}
}
