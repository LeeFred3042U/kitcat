package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// Status compares HEAD, index, and working directory state to generate a
// human-readable repository status summary similar to `git status`.
func Status() (string, error) {
	index, err := storage.LoadIndex()
	if err != nil {
		return "", fmt.Errorf("failed to load index: %w", err)
	}

	// HEAD tree is optional; absence implies initial commit state.
	headTree := make(map[string]string)
	if headCommit, err := storage.GetLastCommit(); err == nil {
		headTree, _ = storage.ParseTree(headCommit.TreeHash)
	}

	var staged []string

	// Detect staged changes by comparing index entries against HEAD tree snapshot.
	for path, entry := range index {
		entryHashHex := fmt.Sprintf("%x", entry.Hash)
		if headHash, inHead := headTree[path]; inHead {
			if entryHashHex != headHash {
				staged = append(staged, fmt.Sprintf("modified:   %s", path))
			}
		} else {
			staged = append(staged, fmt.Sprintf("new file:   %s", path))
		}
	}

	// Files present in HEAD but removed from index are staged deletions.
	for path := range headTree {
		if _, inIndex := index[path]; !inIndex {
			staged = append(staged, fmt.Sprintf("deleted:    %s", path))
		}
	}

	var notStaged []string
	var untracked []string

	ignorePatterns, _ := LoadIgnorePatterns()

	// Proxy map is used only for ignore logic to mark tracked paths quickly.
	proxyIndex := make(map[string]string, len(index))
	for k := range index {
		proxyIndex[k] = ""
	}

	// Walk working directory to detect unstaged modifications and untracked files.
	walkErr := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil { return nil }

		// Skip repository metadata directory entirely to avoid false positives.
		if path == "." || path == RepoDir || strings.HasPrefix(path, RepoDir+string(os.PathSeparator)) {
			if info.IsDir() && path != "." { return filepath.SkipDir }
			return nil
		}
		if info.IsDir() { return nil }

		cleanPath := filepath.Clean(path)

		// Reject unsafe paths to prevent traversal outside repo boundaries.
		if !IsSafePath(cleanPath) { return nil }

		if entry, tracked := index[cleanPath]; tracked {
			// Use cached stat metadata as a fast-path to avoid hashing every file.
			if entry.Size != uint32(info.Size()) || entry.MTimeSec != uint32(info.ModTime().Unix()) {
				hash, err := storage.HashFile(cleanPath)
				if err == nil {
					entryHashHex := fmt.Sprintf("%x", entry.Hash)
					if hash != entryHashHex {
						notStaged = append(notStaged, fmt.Sprintf("modified:   %s", cleanPath))
					}
				}
			}
		} else {
			// Ignore patterns apply only to untracked files.
			if !ShouldIgnore(cleanPath, ignorePatterns, proxyIndex) {
				untracked = append(untracked, cleanPath)
			}
		}
		return nil
	})

	if walkErr != nil {
		return "", walkErr
	}

	// Detect tracked files deleted from working directory.
	for path := range index {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			notStaged = append(notStaged, fmt.Sprintf("deleted:    %s", path))
		}
	}

	// Sort output to keep CLI display deterministic regardless of filesystem order.
	sort.Strings(staged)
	sort.Strings(notStaged)
	sort.Strings(untracked)

	var sb strings.Builder
	if len(staged) == 0 && len(notStaged) == 0 && len(untracked) == 0 {
		return "nothing to commit, working tree clean", nil
	}

	if len(staged) > 0 {
		sb.WriteString("Changes to be committed:\n")
		for _, s := range staged {
			sb.WriteString(fmt.Sprintf("  \033[32m%s\033[0m\n", s))
		}
		sb.WriteString("\n")
	}

	if len(notStaged) > 0 {
		sb.WriteString("Changes not staged for commit:\n")
		for _, s := range notStaged {
			sb.WriteString(fmt.Sprintf("  \033[31m%s\033[0m\n", s))
		}
		sb.WriteString("\n")
	}

	if len(untracked) > 0 {
		sb.WriteString("Untracked files:\n")
		for _, s := range untracked {
			sb.WriteString(fmt.Sprintf("  \033[31m%s\033[0m\n", s))
		}
		sb.WriteString("\n")
	}

	return strings.TrimSpace(sb.String()), nil
}
