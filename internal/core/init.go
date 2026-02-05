package core

import (
	"fmt"
	"os"
	"path/filepath"
)

const colorYellow = "\033[33m"

// isPathExist checks if a path exist or not
func isPathExist(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// InitRepo sets up the .kitcat directory structure.
func InitRepo() error {
	// Idempotent: do not error if repo exists, just ensure structure is correct

	// Create all necessary subdirectories using the public constants.
	dirs := []string{
		RepoDir,
		ObjectsDir,
		HeadsDir,
		TagsDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil && !os.IsExist(err) {
			return err
		}
	}

	// Create empty files only if they do not exist.
	files := []string{IndexPath, CommitsPath}
	for _, file := range files {
		if !isPathExist(file) {
			f, err := os.Create(file)
			if err != nil {
				return err
			}
			f.Close()
		}
	}

	// Create the HEAD file to point to the default branch (main) only if it does not exist.
	headContent := []byte("ref: refs/heads/main")
	if !isPathExist(HeadPath) {
		if err := os.WriteFile(HeadPath, headContent, 0o644); err != nil {
			return err
		}
		fmt.Printf("%sUsing 'main' as the name for the default branch.%s\n\n", colorYellow, colorReset)
		fmt.Printf("%sBranches can be renamed via this command:%s\n", colorYellow, colorReset)
		fmt.Printf("%s\tkitcat branch -m <branch_name>%s\n\n", colorYellow, colorReset)
		fmt.Printf("%sList all the branches via this command:%s\n", colorYellow, colorReset)
		fmt.Printf("%s\tkitcat branch -l%s\n", colorYellow, colorReset)
	}
	// Generating empty main branch file only if it does not exist.
	mainBranchPath := filepath.Join(HeadsDir, "main")
	if !isPathExist(mainBranchPath) {
		if err := os.WriteFile(mainBranchPath, []byte(""), 0o644); err != nil {
			return err
		}
	}

	// Create default .kitignore to prevent self-tracking only if it does not exist.
	ignoreContent := []byte(".DS_Store\nkitcat\nkitcat.exe\n*.lock\n.kitignore\n")
	if !isPathExist(".kitignore") {
		if err := os.WriteFile(".kitignore", ignoreContent, 0o644); err != nil {
			return err
		}
	}

	if absPath, err := filepath.Abs(RepoDir); err != nil {
		return err
	} else {
		fmt.Printf("%s\nInitialized empty kitcat repository in %s\n\n%s", colorYellow, absPath, colorReset)
	}
	return nil
}
