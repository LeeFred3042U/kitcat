package core_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LeeFred3042U/kitcat/internal/core"
	"github.com/LeeFred3042U/kitcat/internal/storage"
	"io"
)

func setupTestRepo(t *testing.T) (string, func()) {
	t.Helper()

	// Save current working directory
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// Create temp directory
	tmpDir := t.TempDir()

	// Change to temp directory
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Initialize repository
	if err := core.InitRepo(); err != nil {
		t.Fatal(err)
	}

	// Set up git config for commits
	if err := core.SetConfig("user.name", "Test User", false); err != nil {
		t.Fatal(err)
	}
	if err := core.SetConfig("user.email", "test@example.com", false); err != nil {
		t.Fatal(err)
	}

	// Cleanup function
	cleanup := func() {
		_ = os.Chdir(cwd)
	}

	return tmpDir, cleanup
}

func TestStash_BasicWorkflow(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create and commit initial file
	testFile := "test.txt"
	if err := os.WriteFile(testFile, []byte("initial content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := core.AddFile(testFile); err != nil {
		t.Fatal(err)
	}
	if _, _, err := core.Commit("initial commit"); err != nil {
		t.Fatal(err)
	}

	// Modify the file
	if err := os.WriteFile(testFile, []byte("modified content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := core.AddFile(testFile); err != nil {
		t.Fatal(err)
	}

	// Stash the changes
	if err := core.Stash(); err != nil {
		t.Fatalf("Stash failed: %v", err)
	}

	// Verify working directory is clean
	isDirty, err := core.IsWorkDirDirty()
	if err != nil {
		t.Fatal(err)
	}
	if isDirty {
		t.Error("Working directory should be clean after stash")
	}

	// Verify stash log exists and contains the commit
	stashLogPath := filepath.Join(tmpDir, ".kitcat", "stash.log")
	stashHashBytes, err := os.ReadFile(stashLogPath)
	if err != nil {
		t.Fatalf("Stash log should exist: %v", err)
	}

	stashHash := strings.TrimSpace(string(stashHashBytes))
	if stashHash == "" {
		t.Error("Stash hash should not be empty")
	}

	// Verify stash commit exists in commits.log
	stashCommit, err := storage.FindCommit(stashHash)
	if err != nil {
		t.Fatalf("Stash commit should exist: %v", err)
	}

	// Verify WIP message format
	if !strings.HasPrefix(stashCommit.Message, "WIP on ") {
		t.Errorf("Stash commit message should start with 'WIP on', got: %s", stashCommit.Message)
	}
}

func TestStash_CleanWorkingDirectory(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create and commit initial file
	testFile := "test.txt"
	if err := os.WriteFile(testFile, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := core.AddFile(testFile); err != nil {
		t.Fatal(err)
	}
	if _, _, err := core.Commit("initial commit"); err != nil {
		t.Fatal(err)
	}

	// Try to stash with clean directory
	err := core.Stash()
	if err == nil {
		t.Fatal("Stash should fail with clean working directory")
	}
	if !strings.Contains(err.Error(), "nothing to stash") {
		t.Errorf("Expected 'nothing to stash' error, got: %v", err)
	}
}

func TestStash_StagedAndUnstagedChanges(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create and commit initial files
	file1 := "file1.txt"
	file2 := "file2.txt"
	if err := os.WriteFile(file1, []byte("content1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("content2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := core.AddFile(file1); err != nil {
		t.Fatal(err)
	}
	if err := core.AddFile(file2); err != nil {
		t.Fatal(err)
	}
	if _, _, err := core.Commit("initial commit"); err != nil {
		t.Fatal(err)
	}

	// Create staged changes
	if err := os.WriteFile(file1, []byte("staged content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := core.AddFile(file1); err != nil {
		t.Fatal(err)
	}

	// Create unstaged changes
	if err := os.WriteFile(file2, []byte("unstaged content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Stash all changes
	if err := core.Stash(); err != nil {
		t.Fatalf("Stash failed: %v", err)
	}

	// Verify working directory matches HEAD
	content1, err := os.ReadFile(file1)
	if err != nil {
		t.Fatal(err)
	}
	if string(content1) != "content1" {
		t.Errorf("file1 should be reset to HEAD content, got: %s", string(content1))
	}

	content2, err := os.ReadFile(file2)
	if err != nil {
		t.Fatal(err)
	}
	if string(content2) != "content2" {
		t.Errorf("file2 should be reset to HEAD content, got: %s", string(content2))
	}

	// Verify stash log exists
	stashLogPath := filepath.Join(tmpDir, ".kitcat", "stash.log")
	if _, err := os.Stat(stashLogPath); os.IsNotExist(err) {
		t.Error("Stash log should exist")
	}
}

func TestStashPop_Success(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create and commit initial file
	testFile := "test.txt"
	originalContent := "original content"
	modifiedContent := "modified content"

	if err := os.WriteFile(testFile, []byte(originalContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := core.AddFile(testFile); err != nil {
		t.Fatal(err)
	}
	if _, _, err := core.Commit("initial commit"); err != nil {
		t.Fatal(err)
	}

	// Modify and stash
	if err := os.WriteFile(testFile, []byte(modifiedContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := core.AddFile(testFile); err != nil {
		t.Fatal(err)
	}
	if err := core.Stash(); err != nil {
		t.Fatal(err)
	}

	// Verify file is reset
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != originalContent {
		t.Errorf("File should be reset after stash, got: %s", string(content))
	}

	// Pop the stash
	if err := core.StashPop(); err != nil {
		t.Fatalf("StashPop failed: %v", err)
	}

	// Verify changes are restored
	content, err = os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != modifiedContent {
		t.Errorf("File should be restored after pop, got: %s, want: %s", string(content), modifiedContent)
	}

	// Verify stash log is empty or deleted
	stashLogPath := filepath.Join(tmpDir, ".kitcat", "stash.log")
	content, err = os.ReadFile(stashLogPath)
	if err == nil && len(strings.TrimSpace(string(content))) > 0 {
		t.Error("Stash log should be empty after pop")
	}
}

func TestStashPop_NoStash(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create and commit initial file
	testFile := "test.txt"
	if err := os.WriteFile(testFile, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := core.AddFile(testFile); err != nil {
		t.Fatal(err)
	}
	if _, _, err := core.Commit("initial commit"); err != nil {
		t.Fatal(err)
	}

	// Try to pop without stash
	err := core.StashPop()
	if err == nil {
		t.Fatal("StashPop should fail when no stash exists")
	}
	if !strings.Contains(err.Error(), "no stash entries found") {
		t.Errorf("Expected 'no stash entries found' error, got: %v", err)
	}
}

func TestStashPop_DirtyWorkingDirectory(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create and commit initial file
	testFile := "test.txt"
	if err := os.WriteFile(testFile, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := core.AddFile(testFile); err != nil {
		t.Fatal(err)
	}
	if _, _, err := core.Commit("initial commit"); err != nil {
		t.Fatal(err)
	}

	// Modify and stash
	if err := os.WriteFile(testFile, []byte("stashed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := core.AddFile(testFile); err != nil {
		t.Fatal(err)
	}
	if err := core.Stash(); err != nil {
		t.Fatal(err)
	}

	// Make new changes
	if err := os.WriteFile(testFile, []byte("new changes"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Try to pop with dirty directory
	err := core.StashPop()
	if err == nil {
		t.Fatal("StashPop should fail with dirty working directory")
	}
	if !strings.Contains(err.Error(), "would be overwritten") {
		t.Errorf("Expected 'would be overwritten' error, got: %v", err)
	}
}

func TestStash_WIPCommitMessage(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create and commit initial file
	testFile := "test.txt"
	commitMessage := "test commit message"
	if err := os.WriteFile(testFile, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := core.AddFile(testFile); err != nil {
		t.Fatal(err)
	}
	if _, _, err := core.Commit(commitMessage); err != nil {
		t.Fatal(err)
	}

	// Modify and stash
	if err := os.WriteFile(testFile, []byte("modified"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := core.AddFile(testFile); err != nil {
		t.Fatal(err)
	}
	if err := core.Stash(); err != nil {
		t.Fatal(err)
	}

	// Read stash commit from log
	stashLogPath := filepath.Join(tmpDir, ".kitcat", "stash.log")
	stashHashBytes, err := os.ReadFile(stashLogPath)
	if err != nil {
		t.Fatal(err)
	}

	stashHash := strings.TrimSpace(string(stashHashBytes))
	stashCommit, err := storage.FindCommit(stashHash)
	if err != nil {
		t.Fatal(err)
	}

	// Verify message format: "WIP on <branch>: <commit_message>"
	expectedPrefix := "WIP on main: " + commitMessage
	if stashCommit.Message != expectedPrefix {
		t.Errorf("Expected stash message '%s', got: '%s'", expectedPrefix, stashCommit.Message)
	}
}

func TestStash_PreservesIndex(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create and commit initial files
	file1 := "file1.txt"
	file2 := "file2.txt"
	if err := os.WriteFile(file1, []byte("content1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("content2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := core.AddFile(file1); err != nil {
		t.Fatal(err)
	}
	if err := core.AddFile(file2); err != nil {
		t.Fatal(err)
	}
	if _, _, err := core.Commit("initial commit"); err != nil {
		t.Fatal(err)
	}

	// Stage only file1
	if err := os.WriteFile(file1, []byte("modified1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := core.AddFile(file1); err != nil {
		t.Fatal(err)
	}

	// Stash
	if err := core.Stash(); err != nil {
		t.Fatal(err)
	}

	// Read stash commit and verify tree
	stashLogPath := filepath.Join(tmpDir, ".kitcat", "stash.log")
	stashHashBytes, err := os.ReadFile(stashLogPath)
	if err != nil {
		t.Fatal(err)
	}

	stashHash := strings.TrimSpace(string(stashHashBytes))
	stashCommit, err := storage.FindCommit(stashHash)
	if err != nil {
		t.Fatal(err)
	}

	// Parse stash tree
	stashTree, err := storage.ParseTree(stashCommit.TreeHash)
	if err != nil {
		t.Fatal(err)
	}

	// Verify both files are in the stash tree
	if _, ok := stashTree[file1]; !ok {
		t.Error("file1 should be in stash tree")
	}
	if _, ok := stashTree[file2]; !ok {
		t.Error("file2 should be in stash tree")
	}
}

func TestStash_MultipleFiles(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create and commit multiple files
	files := []string{"file1.txt", "file2.txt", "file3.txt"}
	for _, file := range files {
		if err := os.WriteFile(file, []byte("original"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := core.AddFile(file); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := core.Commit("initial commit"); err != nil {
		t.Fatal(err)
	}

	// Modify all files
	for _, file := range files {
		if err := os.WriteFile(file, []byte("modified"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := core.AddFile(file); err != nil {
			t.Fatal(err)
		}
	}

	// Stash
	if err := core.Stash(); err != nil {
		t.Fatalf("Stash failed: %v", err)
	}

	// Verify all files are reset
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != "original" {
			t.Errorf("File %s should be reset, got: %s", file, string(content))
		}
	}

	// Pop and verify all files are restored
	if err := core.StashPop(); err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != "modified" {
			t.Errorf("File %s should be restored, got: %s", file, string(content))
		}
	}
}

func TestStash_NoCommits(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create a file without committing
	testFile := "test.txt"
	if err := os.WriteFile(testFile, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := core.AddFile(testFile); err != nil {
		t.Fatal(err)
	}

	// Try to stash
	err := core.Stash()
	if err == nil {
		t.Fatal("Stash should fail when there are no commits")
	}
	if !strings.Contains(err.Error(), "no commits yet") {
		t.Errorf("Expected 'no commits yet' error, got: %v", err)
	}
}

func TestStash_UnstagedChanges(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	// 1. Setup: Initialize and commit file.txt v1
	testFile := "file.txt"
	if err := os.WriteFile(testFile, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := core.AddFile(testFile); err != nil {
		t.Fatal(err)
	}
	if _, _, err := core.Commit("initial commit"); err != nil {
		t.Fatal(err)
	}

	// 2. Modify: Change content to v2 (unstaged)
	if err := os.WriteFile(testFile, []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 3. Push: Run stash
	if err := core.Stash(); err != nil {
		t.Fatalf("Stash failed: %v", err)
	}

	// 4. Assert: Working directory is clean (content reverts to v1)
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "v1" {
		t.Errorf("File should be reset to v1 after stash, got: %s", string(content))
	}

	// 5. Pop: Run stash pop
	if err := core.StashPop(); err != nil {
		t.Fatalf("StashPop failed: %v", err)
	}

	// 6. Assert: Working directory contains v2
	content, err = os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "v2" {
		t.Errorf("File should be restored to v2 after pop, got: %s", string(content))
	}

	// 7. Verification: Ensure stash log is empty/updated
	stashLogPath := filepath.Join(tmpDir, ".kitcat", "stash.log")
	content, err = os.ReadFile(stashLogPath)
	if err == nil && len(strings.TrimSpace(string(content))) > 0 {
		t.Error("Stash log should be empty after pop")
	}
}

func TestStash_List(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	// Create and commit initial file
	testFile := "test.txt"
	if err := os.WriteFile(testFile, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := core.AddFile(testFile); err != nil {
		t.Fatal(err)
	}
	if _, _, err := core.Commit("commit 1"); err != nil {
		t.Fatal(err)
	}

	// Stash 1 (Oldest)
	if err := os.WriteFile(testFile, []byte("wip1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := core.StashPush("message 1"); err != nil {
		t.Fatal(err)
	}

	// Stash 2 (Newest)
	if err := os.WriteFile(testFile, []byte("wip2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := core.StashPush("message 2"); err != nil {
		t.Fatal(err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := core.StashList()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("StashList failed: %v", err)
	}

	out, _ := io.ReadAll(r)
	output := string(out)

	// Verify output contains messages and correct indices
	// Expected:
	// stash@{0}: WIP on ...: message 2
	// stash@{1}: WIP on ...: message 1

	if !strings.Contains(output, "stash@{0}:") {
		t.Error("Output should contain stash@{0}")
	}
	if !strings.Contains(output, "message 2") {
		t.Error("Output should contain message 2")
	}
	if !strings.Contains(output, "stash@{1}:") {
		t.Error("Output should contain stash@{1}")
	}
	if !strings.Contains(output, "message 1") {
		t.Error("Output should contain message 1")
	}

	// Verify order
	idx1 := strings.Index(output, "message 1")
	idx2 := strings.Index(output, "message 2")
	if idx2 == -1 || idx1 == -1 {
		t.Fatal("Messages not found in output")
	}
	if idx2 > idx1 {
		t.Error("message 2 (newer) should appear before message 1 (older)")
	}
}
