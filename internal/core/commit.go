package core

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/app"
	"github.com/LeeFred3042U/kitcat/internal/plumbing"
	"github.com/LeeFred3042U/kitcat/internal/repo"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// Commit creates a new commit object from the current index state.
func Commit(message string) (string, error) {
	isMerge := false
	parents := []string{}
	mergeHeadPath := filepath.Join(repo.Dir, "MERGE_HEAD")
	mergeMsgPath := filepath.Join(repo.Dir, "MERGE_MSG")

	if mergeHeadBytes, err := os.ReadFile(mergeHeadPath); err == nil {
		mergeHead := strings.TrimSpace(string(mergeHeadBytes))
		if mergeHead != "" {
			isMerge = true
		}
	}

	index, err := storage.LoadIndex()
	if err != nil {
		return "", fmt.Errorf("failed to load index: %w", err)
	}

	for path, entry := range index {
		if entry.Stage != 0 {
			return "", fmt.Errorf("cannot commit because you have unmerged files.\nFix conflicts in '%s', run '%s add', and commit again", path, app.Name)
		}

		// Fallback check: Physically scan the file for conflict markers if a merge is active.
		// This guarantees that a user cannot commit unresolved <<<<<<< lines.
		// Binary files are skipped (null-byte heuristic) to avoid false positives
		// and unnecessary memory allocation for large non-text blobs.
		if isMerge {
			if content, err := os.ReadFile(path); err == nil {
				// Treat any file containing a null byte as binary and skip it.
				if bytes.IndexByte(content, 0) != -1 {
					continue
				}
				strContent := string(content)
				if strings.Contains(strContent, "<<<<<<< HEAD") && strings.Contains(strContent, "=======") {
					return "", fmt.Errorf("cannot commit because you have unmerged files.\nFix conflicts in '%s', run '%s add', and commit again", path, app.Name)
				}
			}
		}
	}

	var oldHeadHash string
	headCommit, err := storage.GetLastCommit()
	if err == nil {
		parents = append(parents, headCommit.ID)
		oldHeadHash = headCommit.ID
	}

	// If it's a valid, resolved merge, inject the second parent.
	if isMerge {
		mergeHeadBytes, _ := os.ReadFile(mergeHeadPath)
		parents = append(parents, strings.TrimSpace(string(mergeHeadBytes)))

		if message == "" {
			if msgBytes, err := os.ReadFile(mergeMsgPath); err == nil {
				message = strings.TrimSpace(string(msgBytes))
			} else {
				message = "Merge commit"
			}
		}
	} else if message == "" {
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
	if name == "" {
		name = "Unknown"
	}
	if email == "" {
		email = "unknown@example.com"
	}
	authorStr := fmt.Sprintf("%s <%s>", name, email)

	treeHash, err := plumbing.WriteTree(repo.IndexPath)
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
		os.Remove(mergeHeadPath)
		os.Remove(mergeMsgPath)
	}
	// Reflog updates
	headData, _ := os.ReadFile(repo.HeadPath)
	ref := strings.TrimSpace(string(headData))
	if refPath, ok := strings.CutPrefix(ref, "ref: "); ok {
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
	if _, err := os.Stat(filepath.Join(repo.Dir, "MERGE_HEAD")); err == nil {
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

	treeHash, err := plumbing.WriteTree(repo.IndexPath)
	if err != nil {
		return "", err
	}

	parents := head.Parents
	if len(parents) > 0 {
		parents = append(parents, head.Parents...)
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

	headData, _ := os.ReadFile(repo.HeadPath)
	ref := strings.TrimSpace(string(headData))
	if refPath, ok := strings.CutPrefix(ref, "ref: "); ok {
		ReflogAppend(refPath, oldHeadHash, commitHash, "commit (amend): "+message)
	}
	ReflogAppend("HEAD", oldHeadHash, commitHash, "commit (amend): "+message)

	return commitHash, nil
}

// updateHead updates the branch reference pointed to by HEAD, or HEAD itself
// when in detached-HEAD state.
//
// When HEAD contains "ref: refs/heads/<branch>", the new commit hash is
// written to the branch file as normal. When HEAD is detached (contains a
// raw hash rather than a symbolic ref), the new commit hash is written
// directly to HEAD — the old code fell through to a fabricated refPath
// and silently created a stale file at .kitcat/<40-char-hash>.
func updateHead(commitHash string) error {
	headData, _ := os.ReadFile(repo.HeadPath)
	ref := strings.TrimSpace(string(headData))

	if refPath, ok := strings.CutPrefix(ref, "ref: "); ok {
		// Symbolic ref → update the branch file.
		fullRefPath := filepath.Join(repo.Dir, refPath)
		if err := os.MkdirAll(filepath.Dir(fullRefPath), 0o755); err != nil {
			return err
		}
		return SafeWrite(fullRefPath, []byte(commitHash), 0o644)
	}

	// Detached HEAD → write the new commit hash directly to HEAD.
	return SafeWrite(repo.HeadPath, []byte(commitHash), 0o644)
}
