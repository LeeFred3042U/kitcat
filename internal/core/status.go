package core

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/plumbing"
	"github.com/LeeFred3042U/kitcat/internal/storage"
	"github.com/LeeFred3042U/kitcat/internal/constant"
)

// Status compares HEAD, index, and working directory state.
func Status() (string, error) {
	// Resolve branch name early so detached HEAD or missing refs
	// do not interfere with later index/tree inspection.
	branch := getBranchName()
	index, err := storage.LoadIndex()
	if err != nil {
		return "", fmt.Errorf("failed to load index: %w", err)
	}

	// Build HEAD tree snapshot if a commit exists; absence is treated
	// as an empty tree to allow status to work in fresh repositories.
	headTree := make(map[string]storage.TreeEntry)
	if headCommit, err := storage.GetLastCommit(); err == nil {
		headTree, _ = storage.ParseTree(headCommit.TreeHash)
	}

	var unmerged []string
	var staged []string
	var notStaged []string
	var untracked []string

	// Collect potential rename candidates while scanning index to avoid
	// additional passes over large trees later.
	var addedInIndex []string
	var deletedInIndex []string

	// Analyze index entries against HEAD to derive staged state and conflicts.
	processedPaths := make(map[string]bool)

	for path, entry := range index {
		processedPaths[path] = true

		// Non-zero stage indicates a merge conflict; collapse multiple
		// stage entries into a single logical unmerged record.
		if entry.Stage > 0 {
			status := "unmerged"
			switch entry.Stage {
			case 2:
				status = "both modified"
			case 3:
				status = "both modified"
			}

			isDup := false
			for _, u := range unmerged {
				if strings.Contains(u, path) {
					isDup = true
					break
				}
			}
			if !isDup {
				unmerged = append(unmerged, fmt.Sprintf("%s:   %s", status, path))
			}
			continue
		}

		// Compare HEAD tree metadata against index snapshot to determine
		// whether the change is content-based or permission-mode based.
		entryHashHex := fmt.Sprintf("%x", entry.Hash)
		entryModeOctal := fmt.Sprintf("%06o", entry.Mode)

		if headEntry, inHead := headTree[path]; inHead {
			if entryHashHex != headEntry.Hash {
				staged = append(staged, fmt.Sprintf("modified:   %s", path))
			} else if entryModeOctal != headEntry.Mode {
				staged = append(staged, fmt.Sprintf("modified:   %s (mode)", path))
			}
		} else {
			addedInIndex = append(addedInIndex, path)
		}
	}

	// Files present in HEAD but absent from index are staged deletions.
	for path := range headTree {
		if _, inIndex := index[path]; !inIndex {
			deletedInIndex = append(deletedInIndex, path)
		}
	}

	// Rename detection is performed after collecting additions/deletions
	// so expensive similarity checks are limited to relevant candidates.
	staged = append(staged, detectRenames(addedInIndex, deletedInIndex, index, headTree)...)

	// Load ignore rules once; failures are non-fatal so status remains usable.
	ignorePatterns, _ := LoadIgnorePatterns()
	proxyIndex := make(map[string]string) // for Ignore logic

	// Walk working directory and compare filesystem state against index cache.
	filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		// Skip repository metadata and unsafe paths to avoid leaking outside repo root.
		if err != nil || path == "." || strings.HasPrefix(path, constant.RepoDir) {
			return nil
		}
		if info.IsDir() {
			return nil
		}

		cleanPath := filepath.Clean(path)
		if !IsSafePath(cleanPath) {
			return nil
		}

		if entry, tracked := index[cleanPath]; tracked {
			// Skip conflicted entries; they are already reported from index state.
			if entry.Stage > 0 {
				return nil
			}

			// Detect executable bit drift independently from content hash.
			fileMode := info.Mode()
			isExec := (fileMode & 0111) != 0
			entryExec := (entry.Mode & 0111) != 0
			modeChanged := isExec != entryExec

			// REMOVED: isModified stat cache check.
			// Always hash the file to guarantee modification detection.
			// Relying strictly on Size and MTime fails in fast integration tests
			// where two edits happen in the exact same millisecond.
			hash, err := storage.HashFile(cleanPath)
			if err == nil {
				if hash != fmt.Sprintf("%x", entry.Hash) {
					notStaged = append(notStaged, fmt.Sprintf("modified:   %s", cleanPath))
				} else if modeChanged {
					notStaged = append(notStaged, fmt.Sprintf("modified:   %s (mode)", cleanPath))
				}
			}
		} else {
			// Untracked files are filtered through ignore patterns to avoid noise.
			if !ShouldIgnore(cleanPath, ignorePatterns, proxyIndex) {
				untracked = append(untracked, cleanPath)
			}
		}
		return nil
	})

	// Files missing from disk but present in index are reported as unstaged deletions.
	for path, entry := range index {
		if entry.Stage > 0 {
			continue
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			notStaged = append(notStaged, fmt.Sprintf("deleted:    %s", path))
		}
	}

	// Produce final formatted output once all state has been derived.
	return formatStatus(branch, unmerged, staged, notStaged, untracked), nil
}

// detectRenames matches additions and deletions by Hash (100%) or Content Similarity (>50%)
func detectRenames(added, deleted []string, index map[string]plumbing.IndexEntry, headTree map[string]storage.TreeEntry) []string {
	var results []string
	usedDeleted := make(map[string]bool)

	// Exact hash match provides a fast O(n²) but cheap rename detection
	// before attempting slower content similarity comparisons.
	for i := len(added) - 1; i >= 0; i-- {
		addPath := added[i]
		addHash := fmt.Sprintf("%x", index[addPath].Hash)

		for _, delPath := range deleted {
			if usedDeleted[delPath] {
				continue
			}
			if headTree[delPath].Hash == addHash {
				results = append(results, fmt.Sprintf("renamed:    %s -> %s", delPath, addPath))
				usedDeleted[delPath] = true
				added = append(added[:i], added[i+1:]...)
				break
			}
		}
	}

	// Similarity pass loads blob contents and is intentionally deferred
	// to reduce decompression and memory overhead.
	for _, addPath := range added {
		bestMatch := ""
		bestScore := 0.0

		// Load new content once to avoid repeated object reads.
		newContent, _ := storage.ReadObject(fmt.Sprintf("%x", index[addPath].Hash))

		for _, delPath := range deleted {
			if usedDeleted[delPath] {
				continue
			}

			oldContent, _ := storage.ReadObject(headTree[delPath].Hash)

			score := calculateSimilarity(oldContent, newContent)
			if score > 0.5 && score > bestScore {
				bestScore = score
				bestMatch = delPath
			}
		}

		if bestMatch != "" {
			results = append(results, fmt.Sprintf("renamed:    %s -> %s", bestMatch, addPath))
			usedDeleted[bestMatch] = true
		} else {
			results = append(results, fmt.Sprintf("new file:   %s", addPath))
		}
	}

	// Any remaining unmatched deletions are treated as true removals.
	for _, delPath := range deleted {
		if !usedDeleted[delPath] {
			results = append(results, fmt.Sprintf("deleted:    %s", delPath))
		}
	}
	return results
}

// calculateSimilarity returns a 0.0-1.0 score based on line overlap (Jaccard Index)
func calculateSimilarity(a, b []byte) float64 {
	// Line-based sets trade accuracy for speed and simplicity;
	// suitable for heuristic rename detection rather than diffing.
	linesA := strings.Split(string(a), "\n")
	linesB := strings.Split(string(b), "\n")

	setA := make(map[string]bool)
	for _, l := range linesA {
		if len(l) > 0 {
			setA[l] = true
		}
	}

	intersection := 0
	union := len(setA)

	for _, l := range linesB {
		if len(l) == 0 {
			continue
		}
		if setA[l] {
			intersection++
		} else {
			union++
		}
	}

	// Empty content on both sides implies identical similarity.
	if union == 0 {
		return 1.0
	}
	return float64(intersection) / float64(union)
}

func getBranchName() string {
	// Default to detached HEAD to avoid implying branch semantics
	// when HEAD contains a raw commit hash.
	branch := "HEAD (detached)"
	headContent, err := os.ReadFile(constant.HeadPath)
	if err == nil {
		content := strings.TrimSpace(string(headContent))
		if strings.HasPrefix(content, "ref: refs/heads/") {
			branch = strings.TrimPrefix(content, "ref: refs/heads/")
		}
	}
	return branch
}

func formatStatus(branch string, unmerged, staged, notStaged, untracked []string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("On branch %s\n", branch))

	printSection := func(title, color string, items []string) {
		if len(items) > 0 {
			// Sorting ensures deterministic output regardless of map iteration order.
			sort.Strings(items)
			sb.WriteString(title + ":\n")
			for _, s := range items {
				sb.WriteString(fmt.Sprintf("  \033[%sm%s\033[0m\n", color, s))
			}
			sb.WriteString("\n")
		}
	}

	printSection("Unmerged paths", "31", unmerged)
	printSection("Changes to be committed", "32", staged)
	printSection("Changes not staged for commit", "31", notStaged)
	printSection("Untracked files", "31", untracked)

	// Explicit clean message avoids ambiguous empty output.
	if len(unmerged)+len(staged)+len(notStaged)+len(untracked) == 0 {
		sb.WriteString("nothing to commit, working tree clean")
	}
	return strings.TrimSpace(sb.String())
}
