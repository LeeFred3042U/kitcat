package core

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/diff"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// ANSI color codes
const (
	colorReset = "\033[0m"
	colorRed   = "\033[31m"
	colorGreen = "\033[32m"
	colorBlue  = "\033[1;34m"
)

// Diff calculates and displays differences.
// staged=true:  Diff HEAD vs Index (git diff --staged)
// staged=false: Diff Index vs Workdir (git diff)
func Diff(staged bool) error {
	// 1. Load Index
	index, err := storage.LoadIndex()
	if err != nil {
		return err
	}

	// 2. Load HEAD Commit Tree (if exists)
	var headTree map[string]storage.TreeEntry // UPDATED TYPE
	lastCommit, err := storage.GetLastCommit()
	if err == nil {
		headTree, err = storage.ParseTree(lastCommit.TreeHash)
		if err != nil {
			return err
		}
	} else if err != storage.ErrNoCommits {
		return err
	} else {
		// No commits yet (empty HEAD)
		headTree = make(map[string]storage.TreeEntry) // UPDATED TYPE
	}

	if staged {
		// --- Staged Diff: Compare HEAD (old) vs Index (new) ---

		for path, entry := range index {
			indexHashHex := hex.EncodeToString(entry.Hash[:])
			headEntry, inHead := headTree[path]

			if !inHead {
				// Added in Index
				fmt.Printf("%sAdded: %s%s\n", colorGreen, path, colorReset)
				newContent, _ := storage.ReadObject(indexHashHex)
				printLineDiff("", string(newContent))
				continue
			}

			// Extract Hash from the TreeEntry struct
			if indexHashHex != headEntry.Hash {
				// Modified in Index
				fmt.Printf("%sModified: %s%s\n", colorBlue, path, colorReset)
				oldContent, _ := storage.ReadObject(headEntry.Hash)
				newContent, _ := storage.ReadObject(indexHashHex)
				printLineDiff(string(oldContent), string(newContent))
			}
		}

		// Deleted in Index (present in HEAD, missing in Index)
		for path, headEntry := range headTree {
			if _, inIndex := index[path]; !inIndex {
				fmt.Printf("%sDeleted: %s%s\n", colorRed, path, colorReset)
				oldContent, _ := storage.ReadObject(headEntry.Hash)
				printLineDiff(string(oldContent), "")
			}
		}

	} else {
		// --- Unstaged Diff: Compare Index (old) vs Workdir (new) ---

		for path, entry := range index {
			// Read file from Working Directory
			contentBytes, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				// Tracked in index, missing on disk -> Deleted
				fmt.Printf("%sDeleted: %s%s\n", colorRed, path, colorReset)
				indexHashHex := hex.EncodeToString(entry.Hash[:])
				indexContentBytes, _ := storage.ReadObject(indexHashHex)
				printLineDiff(string(indexContentBytes), "")
				continue
			}
			if err != nil {
				return fmt.Errorf("failed to read %s: %w", path, err)
			}
			workContent := string(contentBytes)

			// Read file from Index
			indexHashHex := hex.EncodeToString(entry.Hash[:])
			indexBytes, err := storage.ReadObject(indexHashHex)
			if err != nil {
				return fmt.Errorf("failed to read object %s: %w", indexHashHex, err)
			}
			indexContent := string(indexBytes)

			// Compare content
			if workContent != indexContent {
				fmt.Printf("%sModified: %s%s\n", colorBlue, path, colorReset)
				printLineDiff(indexContent, workContent)
			}
		}
	}

	return nil
}

// printLineDiff calculates and prints the diff between two strings
func printLineDiff(old, new string) {
	var oldLines, newLines []string
	if old != "" {
		oldLines = strings.Split(strings.TrimRight(old, "\n"), "\n")
	}
	if new != "" {
		newLines = strings.Split(strings.TrimRight(new, "\n"), "\n")
	}

	// Use the optimized DiffLines function which handles hashing
	diffs := diff.DiffLines(oldLines, newLines)

	for _, c := range diffs {
		lines := c.Text
		switch c.Operation {
		case diff.INSERT:
			for _, l := range lines {
				fmt.Printf("%s+ %s%s\n", colorGreen, l, colorReset)
			}
		case diff.DELETE:
			for _, l := range lines {
				fmt.Printf("%s- %s%s\n", colorRed, l, colorReset)
			}
		case diff.EQUAL:
			for _, l := range lines {
				fmt.Printf("  %s\n", l)
			}
		}
	}
}
