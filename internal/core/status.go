package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/storage"
)

func Status() (string, error) {
	index, err := storage.LoadIndex()
	if err != nil {
		return "", fmt.Errorf("failed to load index: %w", err)
	}

	headTree := make(map[string]string)
	if headCommit, err := storage.GetLastCommit(); err == nil {
		headTree, _ = storage.ParseTree(headCommit.TreeHash)
	}

	var staged []string
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

	for path := range headTree {
		if _, inIndex := index[path]; !inIndex {
			staged = append(staged, fmt.Sprintf("deleted:    %s", path))
		}
	}

	var notStaged []string
	var untracked []string

	ignorePatterns, _ := LoadIgnorePatterns()
	proxyIndex := make(map[string]string, len(index))
	for k := range index {
		proxyIndex[k] = ""
	}

	walkErr := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil { return nil }
		if path == "." || path == RepoDir || strings.HasPrefix(path, RepoDir+string(os.PathSeparator)) {
			if info.IsDir() && path != "." { return filepath.SkipDir }
			return nil
		}
		if info.IsDir() { return nil }

		cleanPath := filepath.Clean(path)
		if !IsSafePath(cleanPath) { return nil }

		if entry, tracked := index[cleanPath]; tracked {
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
			if !ShouldIgnore(cleanPath, ignorePatterns, proxyIndex) {
				untracked = append(untracked, cleanPath)
			}
		}
		return nil
	})

	if walkErr != nil {
		return "", walkErr
	}

	for path := range index {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			notStaged = append(notStaged, fmt.Sprintf("deleted:    %s", path))
		}
	}

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
