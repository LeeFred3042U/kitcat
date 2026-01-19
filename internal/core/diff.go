package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/diff"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// ANSI color codes for formatting terminal output
const (
	colorReset = "\033[0m"
	colorRed   = "\033[31m"
	colorGreen = "\033[32m"
	colorBlue  = "\033[1;34m"
)

func isDiffBinary(content []byte) bool {
	limit := min(len(content), 512)

	for i := range limit {
		if content[i] == 0x00 {
			return true
		}
	}
	return false
}

// displayDiff formats and prints the structured diff output from the Myers algorithm.
// It iterates through each change (insertion, deletion, or equal) and applies the appropriate color
func displayDiff(diffs []diff.Diff[string]) {
	// Loop over each diff "chunk" provided by the algorithm
	for _, d := range diffs {
		lines := d.Text
		switch d.Operation {
		// If the operation was an INSERT, print each line in green with a '+' prefix
		case diff.INSERT:
			for _, line := range lines {
				fmt.Printf("%s+ %s%s\n", colorGreen, line, colorReset)
			}
		// If the operation was a DELETE, print each line in red with a '-' prefix
		case diff.DELETE:
			for _, line := range lines {
				fmt.Printf("%s- %s%s\n", colorRed, line, colorReset)
			}
		// If the lines are EQUAL, print them with the default color and two spaces for context
		case diff.EQUAL:
			for _, line := range lines {
				fmt.Printf("  %s\n", line)
			}
		}
	}
}

// Diff calculates and displays the differences between the last commit and the current staging area (index)
// It identifies which files have been added, deleted, or modified.
func Diff(staged bool) error {
	// Retrieve the metadata for the most recent commit.
	lastCommit, err := storage.GetLastCommit()
	if err != nil {
		// If there are no commits yet, there's nothing to compare against.
		if err == storage.ErrNoCommits {
			fmt.Println("No commits yet. Nothing to diff against.")
			return nil
		}
		return err
	}

	// Load the current staging area into a map. This represents what will be in the *next* commit
	index, err := storage.LoadIndex()
	if err != nil {
		return err
	}

	if staged {

		// From the commit, get the tree object which represents the state of the repository at that time
		// This is a map of `filePath -> contentHash`
		tree, err := storage.ParseTree(lastCommit.TreeHash)
		if err != nil {
			return err
		}

		// First Loop: Iterate through files in the index to find additions and modifications
		for path, indexHash := range index {
			treeHash, ok := tree[path]
			// If a file is in the index but not in the old tree, it's a new file.
			if !ok {
				fmt.Printf("%sAdded file: %s%s\n", colorBlue, path, colorReset)

				// Show content of added file (all lines are additions)
				content, err := storage.ReadObject(indexHash)
				if err != nil {
					return err
				}

				contentStr := strings.TrimRight(string(content), "\n")
				fileLines := strings.Split(contentStr, "\n")
				emptyLines := []string{}

				myers := diff.NewMyersDiff(emptyLines, fileLines)
				diffs := myers.Diffs()
				displayDiff(diffs)
				continue
			}

			// If the file exists in both, but the content hash is different, it has been modified
			if indexHash != treeHash {
				fmt.Printf("%sModified file: %s%s\n", colorBlue, path, colorReset)

				// Read the old and new content from the object store.
				oldContent, err := storage.ReadObject(treeHash)
				if err != nil {
					return err
				}
				newContent, err := storage.ReadObject(indexHash)
				if err != nil {
					return err
				}

				// If the Content is binary
				if isDiffBinary(oldContent) || isDiffBinary(newContent) {
					fmt.Println("Binary files differ")
					continue
				}

				// Split file content into lines to prepare for the diff algorithm
				oldLines := strings.Split(string(oldContent), "\n")
				newLines := strings.Split(string(newContent), "\n")

				// Using the Myers algorithm to compute the differences
				myers := diff.NewMyersDiff(oldLines, newLines)
				diffs := myers.Diffs()

				// Display the computed differences with color
				displayDiff(diffs)
			}
		}

		// Next Loop: Iterate through files in the old tree to find deletions.
		for path := range tree {
			// If a file was in the old tree but is no longer in the index, it has been deleted.
			if _, ok := index[path]; !ok {
				fmt.Printf("%sDeleted file: %s%s\n", colorBlue, path, colorReset)
			}
		}
	} else {
		// Case B: unstaged diff (Index vs Working Directory)
		// Equivalent to `git diff` (not `--cached`)

		for path, indexHash := range index {
			// Read current working directory file
			fileContent, err := os.ReadFile(path)
			if err != nil {
				// File deleted from working directory (but still staged)
				fmt.Printf("%sDeleted (unstaged): %s%s\n", colorRed, path, colorReset)
				continue
			}

			// Read staged content from index
			indexContent, err := storage.ReadObject(indexHash)
			if err != nil {
				return fmt.Errorf("failed to read index object %s: %w", indexHash, err)
			}

			// Compare: working directory vs index (staged)
			if string(fileContent) != string(indexContent) {
				fmt.Printf("%sChanged (unstaged): %s%s\n", colorBlue, path, colorReset)

				// If the diff is binary
				if isDiffBinary(fileContent) || isDiffBinary(indexContent) {
					fmt.Println("Binary files differ")
				}

				// Index content = "old" (what's staged)
				// Working dir content = "new" (current changes)
				indexLines := strings.Split(string(indexContent), "\n")
				contentStr := strings.TrimRight(string(fileContent), "\n")
				workDirLines := strings.Split(contentStr, "\n")

				// Myers diff: staged (old) → working dir (new)
				myers := diff.NewMyersDiff(indexLines, workDirLines)
				diffs := myers.Diffs()
				displayDiff(diffs)
			}
		}

		// Untracked files: exist in working directory but not staged (recursive walk)
		var untracked []string
		err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}

			name := filepath.Base(path)

			// Skip .kitcat directory
			if name == ".kitcat" {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			// Skip directories (continue walking into subdirs)
			if d.IsDir() {
				return nil
			}

			// Only process regular files
			if !d.Type().IsRegular() {
				return nil
			}

			// Get relative path for comparison with index
			relPath, err := filepath.Rel(".", path)
			if err != nil {
				return err
			}

			// File exists on disk but not in index = untracked/new
			if _, ok := index[relPath]; !ok {
				untracked = append(untracked, relPath)
			}
			return nil
		})
		if err != nil {
			return err
		}

		// Show untracked file content (all lines are additions)
		for _, path := range untracked {
			content, err := os.ReadFile(path)
			if err != nil {
				continue
			}

			// If the content is binary
			if isDiffBinary(content) {
				fmt.Println("Binary files differ")
				continue
			}

			fmt.Printf("%s%sUntracked:%s %s\n", colorGreen, colorBlue, colorReset, path)

			// Trim trailing newlines and split into lines
			contentStr := strings.TrimRight(string(content), "\n")
			fileLines := strings.Split(contentStr, "\n")
			emptyLines := []string{}

			// Myers diff: empty (old) → file content (new)
			// All lines will show as green additions
			myers := diff.NewMyersDiff(emptyLines, fileLines)
			diffs := myers.Diffs()

			// Display the computed differences with color
			displayDiff(diffs)
		}
	}

	return nil
}
