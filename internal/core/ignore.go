package core

import (
	"bufio"
	"fmt"
	"os"
	"time"
	"path/filepath"
	"strings"
	"sync"
)

// IgnorePattern represents a single parsed line from .kitignore.
// The struct preserves original text while storing normalized matching data.
type IgnorePattern struct {
	Original    string // Original pattern line from file
	Pattern     string // Normalized pattern used for matching
	IsDirectory bool   // True if pattern ends with '/'
	LineNumber  int    // Source line number for warnings/debugging
}

// Global cache for ignore patterns. RWMutex ensures concurrent readers
// during filesystem scans without repeated disk parsing.
var (
    ignoreCache     []IgnorePattern
    ignoreCacheMu   sync.RWMutex
    ignoreCacheMtime time.Time
)

func LoadIgnorePatterns() ([]IgnorePattern, error) {
    info, err := os.Stat(".kitignore")

    // Normalize "file missing" case
    var mtime time.Time
    fileExists := true

    if err != nil {
        if os.IsNotExist(err) {
            fileExists = false
            mtime = time.Time{} // zero time
        } else {
            return nil, fmt.Errorf("error stating .kitignore: %w", err)
        }
    } else {
        mtime = info.ModTime()
    }

    // Fast path cache hit
    ignoreCacheMu.RLock()
    if mtime.Equal(ignoreCacheMtime) {
        patterns := ignoreCache
        ignoreCacheMu.RUnlock()
        return patterns, nil
    }
    ignoreCacheMu.RUnlock()

    // Slow path reload
    ignoreCacheMu.Lock()
    defer ignoreCacheMu.Unlock()

    // Double-check under write lock
    if mtime.Equal(ignoreCacheMtime) {
        return ignoreCache, nil
    }

    var patterns []IgnorePattern

    if fileExists {
        file, err := os.Open(".kitignore")
        if err != nil {
            return nil, fmt.Errorf("error reading .kitignore: %w", err)
        }
        defer file.Close()

        scanner := bufio.NewScanner(file)
        lineNumber := 0

        for scanner.Scan() {
            lineNumber++
            line := strings.TrimSpace(scanner.Text())

            if line == "" || strings.HasPrefix(line, "#") {
                continue
            }

            isDirectory := strings.HasSuffix(line, "/")
            pattern := line
            if isDirectory {
                pattern = strings.TrimSuffix(pattern, "/")
            }

            if !isValidPattern(pattern) {
                fmt.Fprintf(os.Stderr,
                    "warning: .kitignore line %d: invalid pattern '%s' (skipping)\n",
                    lineNumber, line)
                continue
            }

            patterns = append(patterns, IgnorePattern{
                Original:    line,
                Pattern:     pattern,
                IsDirectory: isDirectory,
                LineNumber:  lineNumber,
            })
        }

        if err := scanner.Err(); err != nil {
            return nil, fmt.Errorf("error reading .kitignore: %w", err)
        }
    } else {
        // Missing file → empty cache
        patterns = []IgnorePattern{}
    }

    // Update cache
    ignoreCache = patterns
    ignoreCacheMtime = mtime

    return patterns, nil
}

// ShouldIgnore returns true if a path matches any ignore pattern.
// Tracked files are explicitly exempt from ignore rules.
func ShouldIgnore(path string, patterns []IgnorePattern, trackedFiles map[string]string) bool {
	// Tracked files must never be ignored.
	if _, isTracked := trackedFiles[path]; isTracked {
		return false
	}

	for _, pattern := range patterns {
		if matchesPattern(path, pattern) {
			return true
		}
	}

	return false
}

// matchesPattern evaluates a single IgnorePattern against a path.
// Supports directory-only rules, glob matching, and simplified "**" recursion.
func matchesPattern(path string, pattern IgnorePattern) bool {
	// Normalize separators to forward slashes for cross-platform matching.
	path = filepath.ToSlash(path)
	patternStr := filepath.ToSlash(pattern.Pattern)

	// Directory patterns match the directory itself and all descendants.
	if pattern.IsDirectory {
		if path == patternStr {
			return true
		}
		if strings.HasPrefix(path, patternStr+"/") {
			return true
		}
		return false
	}

	// Recursive patterns delegate to specialized matcher.
	if strings.Contains(patternStr, "**") {
		return matchesRecursivePattern(path, patternStr)
	}

	// First match only the basename for simple patterns.
	matched, err := filepath.Match(patternStr, filepath.Base(path))
	if err != nil {
		return false
	}
	if matched {
		return true
	}

	// Also test against full path for patterns containing directories.
	matched, err = filepath.Match(patternStr, path)
	if err != nil {
		return false
	}

	return matched
}

// matchesRecursivePattern implements simplified handling of "**".
// It allows matching across arbitrary directory depth but does not fully
// implement Git's pathspec semantics.
func matchesRecursivePattern(path, pattern string) bool {
	parts := strings.Split(pattern, "**")

	// Pattern "**" matches everything.
	if len(parts) == 2 && parts[0] == "" && parts[1] == "" {
		return true
	}

	if len(parts) == 2 {
		prefix := strings.TrimSuffix(parts[0], "/")
		suffix := strings.TrimPrefix(parts[1], "/")

		if prefix != "" && !strings.HasPrefix(path, prefix) {
			return false
		}

		if suffix != "" {
			pathParts := strings.Split(path, "/")
			for i := range pathParts {
				subPath := strings.Join(pathParts[i:], "/")
				matched, err := filepath.Match(suffix, subPath)
				if err == nil && matched {
					return true
				}
				matched, err = filepath.Match(suffix, pathParts[i])
				if err == nil && matched {
					return true
				}
			}
			return false
		}

		return true
	}

	// Fallback for complex patterns not fully supported.
	return false
}

// isValidPattern validates glob syntax before adding to cache.
func isValidPattern(pattern string) bool {
	if pattern == "" {
		return false
	}

	// filepath.Match performs syntax validation implicitly.
	_, err := filepath.Match(pattern, "test")
	if err != nil {
		return false
	}

	return true
}

// ClearIgnoreCache forces the next LoadIgnorePatterns call to re-parse
// from disk, bypassing the mtime check. Call this after programmatically
// writing a new .kitignore within the same process, or in tests that
// create ignore files and need immediate consistency.
func ClearIgnoreCache() {
    ignoreCacheMu.Lock()
    defer ignoreCacheMu.Unlock()
    ignoreCacheMtime = time.Time{} // zero invalidates any real mtime
}
