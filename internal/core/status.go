package core

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/plumbing"
	"github.com/LeeFred3042U/kitcat/internal/repo"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// Status computes repository state by comparing HEAD, index,
// and working directory. Output is deterministic.
func Status() (string, error) {
	branch := getBranchName()

	index, err := storage.LoadIndex()
	if err != nil {
		return "", fmt.Errorf("load index: %w", err)
	}

	// HEAD tree resolution.
	// Absence of a commit is treated as empty tree.
	headTree := make(map[string]storage.TreeEntry)
	if headCommit, err := storage.GetLastCommit(); err == nil {
		headTree, _ = storage.ParseTree(headCommit.TreeHash)
	}

	var (
		unmerged  []string
		staged    []string
		notStaged []string
		untracked []string
	)

	unmergedSet := make(map[string]struct{})
	addedInIndex := make([]string, 0)
	deletedInIndex := make([]string, 0)

	// ------------------------------------------------------------------
	// Index vs HEAD
	// ------------------------------------------------------------------

	for path, entry := range index {
		// Conflict entries (stage > 0).
		if entry.Stage > 0 {
			if _, seen := unmergedSet[path]; !seen {
				unmergedSet[path] = struct{}{}
				unmerged = append(unmerged,
					fmt.Sprintf("both modified:   %s", path))
			}
			continue
		}

		entryHash := hex.EncodeToString(entry.Hash[:])
		entryMode := fmt.Sprintf("%06o", entry.Mode)

		if headEntry, inHead := headTree[path]; inHead {
			if entryHash != headEntry.Hash {
				staged = append(staged,
					fmt.Sprintf("modified:   %s", path))
			} else if entryMode != headEntry.Mode {
				staged = append(staged,
					fmt.Sprintf("modified:   %s (mode)", path))
			}
		} else {
			addedInIndex = append(addedInIndex, path)
		}
	}

	for path := range headTree {
		if _, inIndex := index[path]; !inIndex {
			deletedInIndex = append(deletedInIndex, path)
		}
	}

	staged = append(staged,
		detectRenames(addedInIndex, deletedInIndex, index, headTree)...)

	// ------------------------------------------------------------------
	// Working directory scan
	// ------------------------------------------------------------------

	ignorePatterns, _ := LoadIgnorePatterns()
	proxyIndex := make(map[string]string)

	filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == "." || strings.HasPrefix(path, repo.Dir) {
			return nil
		}
		if d.IsDir() {
			return nil
		}

		cleanPath := filepath.Clean(path)
		if !IsSafePath(cleanPath) {
			return nil
		}

		entry, tracked := index[cleanPath]
		if tracked {
			if entry.Stage > 0 {
				return nil
			}

			info, err := d.Info()
			if err != nil {
				return nil
			}

			// Fast size check before hashing.
			if uint64(info.Size()) != uint64(entry.Size) {
				notStaged = append(notStaged,
					fmt.Sprintf("modified:   %s", cleanPath))
				return nil
			}

			// Executable bit comparison.
			isExec := (info.Mode() & 0111) != 0
			entryExec := (entry.Mode & 0111) != 0
			modeChanged := isExec != entryExec

			hash, err := storage.HashFile(cleanPath)
			if err == nil {
				entryHash := hex.EncodeToString(entry.Hash[:])
				if hash != entryHash {
					notStaged = append(notStaged,
						fmt.Sprintf("modified:   %s", cleanPath))
				} else if modeChanged {
					notStaged = append(notStaged,
						fmt.Sprintf("modified:   %s (mode)", cleanPath))
				}
			}
		} else {
			if !ShouldIgnore(cleanPath, ignorePatterns, proxyIndex) {
				untracked = append(untracked, cleanPath)
			}
		}

		return nil
	})

	// Detect deleted files (present in index, absent on disk).
	for path, entry := range index {
		if entry.Stage > 0 {
			continue
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			notStaged = append(notStaged,
				fmt.Sprintf("deleted:    %s", path))
		}
	}

	return formatStatus(branch, unmerged, staged, notStaged, untracked), nil
}

// detectRenames resolves rename operations using two-phase matching.
// Phase 1: exact hash match (O(n)).
// Phase 2: similarity-based heuristic (>50% Jaccard).
func detectRenames(
	added, deleted []string,
	index map[string]plumbing.IndexEntry,
	headTree map[string]storage.TreeEntry,
) []string {

	var results []string
	usedDeleted := make(map[string]struct{})

	// Exact match phase.
	hashToDeleted := make(map[string]string)
	for _, del := range deleted {
		hashToDeleted[headTree[del].Hash] = del
	}

	remainingAdded := make([]string, 0, len(added))

	for _, add := range added {
		entry := index[add]
		addHash := hex.EncodeToString(entry.Hash[:])

		if del, ok := hashToDeleted[addHash]; ok {
			results = append(results,
				fmt.Sprintf("renamed:    %s -> %s", del, add))
			usedDeleted[del] = struct{}{}
		} else {
			remainingAdded = append(remainingAdded, add)
		}
	}

	// Similarity phase.
	for _, add := range remainingAdded {
		entry := index[add]
		newContent, _ := storage.ReadObject(
			hex.EncodeToString(entry.Hash[:]),
		)

		bestMatch := ""
		bestScore := 0.0

		for _, del := range deleted {
			if _, used := usedDeleted[del]; used {
				continue
			}

			oldContent, _ := storage.ReadObject(headTree[del].Hash)
			score := calculateSimilarity(oldContent, newContent)

			if score > 0.5 && score > bestScore {
				bestScore = score
				bestMatch = del
			}
		}

		if bestMatch != "" {
			results = append(results,
				fmt.Sprintf("renamed:    %s -> %s", bestMatch, add))
			usedDeleted[bestMatch] = struct{}{}
		} else {
			results = append(results,
				fmt.Sprintf("new file:   %s", add))
		}
	}

	for _, del := range deleted {
		if _, used := usedDeleted[del]; !used {
			results = append(results,
				fmt.Sprintf("deleted:    %s", del))
		}
	}

	return results
}

// calculateSimilarity returns Jaccard line similarity in range [0,1].
func calculateSimilarity(a, b []byte) float64 {
	linesA := bytes.Split(a, []byte("\n"))
	linesB := bytes.Split(b, []byte("\n"))

	setA := make(map[string]struct{})
	for _, l := range linesA {
		if len(l) > 0 {
			setA[string(l)] = struct{}{}
		}
	}

	intersection := 0
	union := len(setA)

	for _, l := range linesB {
		if len(l) == 0 {
			continue
		}
		str := string(l)
		if _, ok := setA[str]; ok {
			intersection++
		} else {
			union++
		}
	}

	if union == 0 {
		return 1.0
	}
	return float64(intersection) / float64(union)
}

// getBranchName resolves current branch.
// Falls back to detached state if HEAD contains a raw commit.
func getBranchName() string {
	branch := "HEAD (detached)"

	headContent, err := os.ReadFile(repo.HeadPath)
	if err != nil {
		return branch
	}

	content := strings.TrimSpace(string(headContent))
	if trimmed, ok := strings.CutPrefix(content, "ref: refs/heads/"); ok {
		return trimmed
	}

	return branch
}

// formatStatus renders deterministic status output.
func formatStatus(branch string, unmerged, staged, notStaged, untracked []string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("On branch %s\n", branch))

	printSection := func(title, color string, items []string) {
		if len(items) == 0 {
			return
		}
		sort.Strings(items)
		sb.WriteString(title + ":\n")
		for _, s := range items {
			sb.WriteString(fmt.Sprintf("  \033[%sm%s\033[0m\n", color, s))
		}
		sb.WriteString("\n")
	}

	printSection("Unmerged paths", "31", unmerged)
	printSection("Changes to be committed", "32", staged)
	printSection("Changes not staged for commit", "31", notStaged)
	printSection("Untracked files", "31", untracked)

	if len(unmerged)+len(staged)+len(notStaged)+len(untracked) == 0 {
		sb.WriteString("nothing to commit, working tree clean")
	}

	return strings.TrimSpace(sb.String())
}
