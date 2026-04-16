package core

import (
	"os"
	"testing"
	"time"
)

// TestLoadIgnorePatterns_MtimeInvalidation verifies that LoadIgnorePatterns
// invalidates its in-memory cache when the .kitignore file's modification time
// changes.
//
// Workflow:
//  1. Chdir into a temp directory and call ClearIgnoreCache to start from a clean
//     state (prevents cross-test cache pollution).
//  2. Write ".kitignore" containing the pattern "*.log".
//  3. Call LoadIgnorePatterns — this is the first call, so it reads from disk and
//     populates the cache with the "*.log" pattern.
//  4. Overwrite ".kitignore" with the new pattern "*.tmp", then use os.Chtimes to
//     set the file's mtime to now+2s, which is strictly later than the cached mtime.
//  5. Call LoadIgnorePatterns again.
//  6. Assert the returned patterns contain "*.tmp" and do NOT contain "*.log",
//     proving the cache was invalidated and the file was re-read from disk.
func TestLoadIgnorePatterns_MtimeInvalidation(t *testing.T) {
	tmpDir := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd) //nolint:errcheck
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Ensure no stale cache from a previous test run in this process.
	ClearIgnoreCache()

	// Populate cache with *.log.
	if err := os.WriteFile(".kitignore", []byte("*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	patterns, err := LoadIgnorePatterns()
	if err != nil {
		t.Fatalf("first LoadIgnorePatterns: %v", err)
	}
	found := false
	for _, p := range patterns {
		if p.Pattern == "*.log" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected *.log in initial patterns, got: %v", patterns)
	}

	// Replace content and advance mtime to force cache miss.
	if err := os.WriteFile(".kitignore", []byte("*.tmp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(".kitignore", future, future); err != nil {
		t.Fatal(err)
	}

	patterns, err = LoadIgnorePatterns()
	if err != nil {
		t.Fatalf("second LoadIgnorePatterns: %v", err)
	}

	hasLog, hasTmp := false, false
	for _, p := range patterns {
		if p.Pattern == "*.log" {
			hasLog = true
		}
		if p.Pattern == "*.tmp" {
			hasTmp = true
		}
	}
	if hasLog {
		t.Error("stale *.log pattern present after mtime invalidation")
	}
	if !hasTmp {
		t.Error("expected *.tmp after mtime invalidation, not found")
	}
}

// TestClearIgnoreCache verifies that calling ClearIgnoreCache forces
// LoadIgnorePatterns to re-read .kitignore on the next invocation, even when the
// file's mtime has not changed (same-clock-tick write).
func TestClearIgnoreCache(t *testing.T) {
	tmpDir := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd) //nolint:errcheck
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	ClearIgnoreCache()

	// Seed cache with *.log.
	if err := os.WriteFile(".kitignore", []byte("*.log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadIgnorePatterns(); err != nil {
		t.Fatalf("seeding cache: %v", err)
	}

	// Record original mtime before overwriting.
	info, err := os.Stat(".kitignore")
	if err != nil {
		t.Fatal(err)
	}
	originalMtime := info.ModTime()

	// Overwrite with *.tmp but restore the same mtime to prevent natural cache miss.
	if err := os.WriteFile(".kitignore", []byte("*.tmp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(".kitignore", originalMtime, originalMtime); err != nil {
		t.Fatal(err)
	}

	// Without clearing the cache, LoadIgnorePatterns would return the stale *.log.
	// Now clear and reload.
	ClearIgnoreCache()

	fresh, err := LoadIgnorePatterns()
	if err != nil {
		t.Fatalf("LoadIgnorePatterns after ClearIgnoreCache: %v", err)
	}

	hasTmp := false
	for _, p := range fresh {
		if p.Pattern == "*.tmp" {
			hasTmp = true
		}
	}
	if !hasTmp {
		t.Fatalf("expected *.tmp after ClearIgnoreCache + reload, got: %v", fresh)
	}
}
