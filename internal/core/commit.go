package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/plumbing"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// Commit creates a new commit object from the current index state.
func Commit(message string) (string, error) {
	// 1. MERGE STATE CHECK
	isMerge := false
	parents := []string{}
	if mergeHeadBytes, err := os.ReadFile(".kitcat/MERGE_HEAD"); err == nil {
		mergeHead := strings.TrimSpace(string(mergeHeadBytes))
		if mergeHead != "" {
			isMerge = true
		}
	}

	// 2. BULLETPROOF PREFLIGHT: Block unresolved conflicts
	index, err := storage.LoadIndex()
	if err != nil {
		return "", fmt.Errorf("failed to load index: %w", err)
	}
	
	for path, entry := range index {
		// Primary check: if the index serialization ever supports stages, block it here.
		if entry.Stage != 0 { 
			return "", fmt.Errorf("cannot commit because you have unmerged files.\nFix conflicts in '%s', run 'kitcat add', and commit again", path)
		}

		// Fallback check: Physically scan the file for conflict markers if a merge is active.
		// This guarantees that a user cannot commit unresolved <<<<<<< lines.
		if isMerge {
			if content, err := os.ReadFile(path); err == nil {
				strContent := string(content)
				if strings.Contains(strContent, "<<<<<<< HEAD") && strings.Contains(strContent, "=======") {
					return "", fmt.Errorf("cannot commit because you have unmerged files.\nFix conflicts in '%s', run 'kitcat add', and commit again", path)
				}
			}
		}
	}

	// 3. PARENT CHAIN SETUP
	var oldHeadHash string
	headCommit, err := storage.GetLastCommit()
	if err == nil {
		parents = append(parents, headCommit.ID)
		oldHeadHash = headCommit.ID
	}

	// If it's a valid, resolved merge, inject the second parent.
	if isMerge {
		mergeHeadBytes, _ := os.ReadFile(".kitcat/MERGE_HEAD")
		parents = append(parents, strings.TrimSpace(string(mergeHeadBytes)))
		
		if message == "" {
			if msgBytes, err := os.ReadFile(".kitcat/MERGE_MSG"); err == nil {
				message = strings.TrimSpace(string(msgBytes))
			} else {
				message = "Merge commit"
			}
		}
	} else if message == "" {
		// --- THE EDITOR INTEGRATION ---
		initialContent := "\n# Please enter the commit message for your changes. Lines starting\n# with '#' will be ignored, and an empty message aborts the commit.\n"
		
		capturedMsg, err := CaptureViaEditor("COMMIT_EDITMSG", initialContent)
		if err != nil {
			return "", err
		}
		if capturedMsg == "" {
			return "", fmt.Errorf("aborting commit due to empty commit message")
		}
		message = capturedMsg
	}

	name, _, _ := GetConfig("user.name")
	email, _, _ := GetConfig("user.email")
	if name == "" { name = "Unknown" }
	if email == "" { email = "unknown@example.com" }
	authorStr := fmt.Sprintf("%s <%s>", name, email)

	treeHash, err := plumbing.WriteTree(storage.IndexPath)
	if err != nil {
		return "", fmt.Errorf("failed to write tree: %w", err)
	}

	opts := plumbing.CommitOptions{
		Tree:      treeHash,
		Parents:   parents,
		Author:    authorStr,
		Committer: authorStr,
		Message:   message,
	}

	commitHash, err := plumbing.CommitTree(opts)
	if err != nil {
		return "", err
	}

	if err := updateHead(commitHash); err != nil {
		return "", err
	}

	// 4. CLEANUP MERGE STATE
	if isMerge {
		os.Remove(".kitcat/MERGE_HEAD")
		os.Remove(".kitcat/MERGE_MSG")
	}

	// Reflog updates
	headData, _ := os.ReadFile(".kitcat/HEAD")
	ref := strings.TrimSpace(string(headData))
	if strings.HasPrefix(ref, "ref: ") {
		refPath := strings.TrimPrefix(ref, "ref: ")
		ReflogAppend(refPath, oldHeadHash, commitHash, "commit: "+message)
	}
	ReflogAppend("HEAD", oldHeadHash, commitHash, "commit: "+message)

	return commitHash, nil
}

// CommitAll stages tracked changes before creating a commit.
func CommitAll(message string) (string, error) {
	if err := AddAll(); err != nil {
		return "", err
	}
	return Commit(message)
}

// AmendCommit replaces the current HEAD commit with a new commit object.
func AmendCommit(message string) (string, error) {
	// Block amending during an active merge (Git invariant)
	if _, err := os.Stat(".kitcat/MERGE_HEAD"); err == nil {
		return "", fmt.Errorf("fatal: You are in the middle of a merge -- cannot amend.")
	}

	head, err := storage.GetLastCommit()
	if err != nil {
		return "", fmt.Errorf("nothing to amend")
	}
	oldHeadHash := head.ID

	if message == "" {
		message = head.Message
	}

	treeHash, err := plumbing.WriteTree(storage.IndexPath)
	if err != nil {
		return "", err
	}

	parents := []string{}
	if head.Parent != "" {
		parents = append(parents, head.Parent)
	}

	name, _, _ := GetConfig("user.name")
	email, _, _ := GetConfig("user.email")
	authorStr := fmt.Sprintf("%s <%s>", name, email)

	opts := plumbing.CommitOptions{
		Tree:      treeHash,
		Parents:   parents,
		Author:    authorStr,
		Committer: authorStr,
		Message:   message,
	}

	commitHash, err := plumbing.CommitTree(opts)
	if err != nil {
		return "", err
	}
	
	if err := updateHead(commitHash); err != nil {
		return "", err
	}

	headData, _ := os.ReadFile(".kitcat/HEAD")
	ref := strings.TrimSpace(string(headData))
	if refPath, ok := strings.CutPrefix(ref, "ref: "); ok {
		ReflogAppend(refPath, oldHeadHash, commitHash, "commit (amend): "+message)
	}
	ReflogAppend("HEAD", oldHeadHash, commitHash, "commit (amend): "+message)

	return commitHash, nil
}

// updateHead updates the branch reference pointed to by HEAD.
// If HEAD is detached or malformed, defaults to refs/heads/main.
func updateHead(commitHash string) error {
	headData, _ := os.ReadFile(".kitcat/HEAD")
	refPath := strings.TrimSpace(strings.TrimPrefix(string(headData), "ref: "))
	if refPath == "" {
		refPath = "refs/heads/main"
	}

	if err := os.MkdirAll(filepath.Dir(".kitcat/"+refPath), 0755); err != nil {
		return err
	}
	
	return SafeWrite(".kitcat/"+refPath, []byte(commitHash), 0644)
}