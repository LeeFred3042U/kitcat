package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// Status compares the state of the working directory, index, and last commit,
// then prints a summary of the changes
func Status() error {
	// Print the current branch status at the top
	headState, err := GetHeadState()
	if err != nil {
		headState = "no commits yet"
	}
	fmt.Printf("On branch %s\n", headState)

	// Load the tree from the commit that HEAD points to
	// Note: We use GetHeadCommit() instead of storage.GetLastCommit() because
	// after a reset, HEAD might point to an earlier commit than the last in the log
	headTree := make(map[string]string)
	headCommit, err := GetHeadCommit()
	if err == nil {
		tree, parseErr := storage.ParseTree(headCommit.TreeHash)
		if parseErr != nil {
			return parseErr
		}
		headTree = tree
	} else if err != storage.ErrNoCommits {
		return err
	}

	// Load the current staging area
	index, err := storage.LoadIndex()
	if err != nil {
		return err
	}

	// Load ignore patterns
	ignorePatterns, err := LoadIgnorePatterns()
	if err != nil {
		return err
	}

	// Prepare slices to hold the categorized changes
	stagedChanges := []string{}
	unstagedChanges := []string{}
	untrackedFiles := []string{}

	// Create a set of all file paths from both HEAD and the index for a complete comparison
	allPaths := make(map[string]bool)
	for path := range headTree {
		allPaths[path] = true
	}
	for path := range index {
		allPaths[path] = true
	}

	// Categorize Staged Changes (Index vs. HEAD)
	for path := range allPaths {
		headHash, inHead := headTree[path]
		indexHash, inIndex := index[path]

		if inIndex && !inHead {
			stagedChanges = append(stagedChanges, fmt.Sprintf("new file:  %s", path))
		} else if !inIndex && inHead {
			stagedChanges = append(stagedChanges, fmt.Sprintf("deleted:   %s", path))
		} else if inIndex && inHead && headHash != indexHash {
			stagedChanges = append(stagedChanges, fmt.Sprintf("modified:  %s", path))
		}
	}

	// Track which files we've seen in the working directory
	visitedPaths := make(map[string]bool)

	// Categorize Unstaged & Untracked Changes (Working Directory vs. Index)
	err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		cleanPath := filepath.Clean(path)

		// Skip the .kitcat directory and other directories
		if info.IsDir() || strings.HasPrefix(cleanPath, RepoDir+string(os.PathSeparator)) ||
			cleanPath == RepoDir {
			return nil
		}

		visitedPaths[cleanPath] = true

		indexHash, isTracked := index[cleanPath]

		// If the file is not in the index, it's untracked
		if !isTracked {
			// Check if file should be ignored
			if ShouldIgnore(cleanPath, ignorePatterns, index) {
				return nil // Skip ignored files
			}
			untrackedFiles = append(untrackedFiles, cleanPath)
			return nil
		}

		// If the file is tracked, hash it and compare with the index to see if it's been modified
		currentHash, hashErr := storage.HashFile(cleanPath)
		if hashErr != nil {
			return hashErr
		}
		if currentHash != indexHash {
			unstagedChanges = append(unstagedChanges, fmt.Sprintf("modified:  %s", cleanPath))
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Check for files that are in the index but not in the working directory (deleted files)
	for path := range index {
		if !visitedPaths[path] {
			// Verify it's actually missing
			if _, err := os.Stat(path); os.IsNotExist(err) {
				unstagedChanges = append(unstagedChanges, fmt.Sprintf("deleted:   %s", path))
			}
		}
	}

	// Print Final Summary - Only show sections that have content
	if len(stagedChanges) > 0 {
		fmt.Println("\nChanges to be committed:")
		for _, change := range stagedChanges {
			fmt.Printf("\t%s\n", change)
		}
	}

	if len(unstagedChanges) > 0 {
		fmt.Println("\nChanges not staged for commit:")
		for _, change := range unstagedChanges {
			fmt.Printf("\t%s\n", change)
		}
	}

	if len(untrackedFiles) > 0 {
		fmt.Println("\nUntracked files:")
		for _, file := range untrackedFiles {
			fmt.Printf("\t%s\n", file)
		}
	}

	// If all sections are empty, show a clean message
	if len(stagedChanges) == 0 && len(unstagedChanges) == 0 && len(untrackedFiles) == 0 {
		fmt.Println("nothing to commit, working tree clean")
	}

	return nil
}
