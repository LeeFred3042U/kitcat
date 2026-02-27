package core

import (
	"path/filepath"
	"encoding/hex"
	"os/exec"
	"strings"
	"bufio"
	"bytes"
	"time"
	"fmt"
	"io"
	"os"

	"github.com/LeeFred3042U/kitcat/internal/models"
	"github.com/LeeFred3042U/kitcat/internal/plumbing"
	"github.com/LeeFred3042U/kitcat/internal/storage"
	"github.com/LeeFred3042U/kitcat/internal/repo"
)

// CaptureViaEditor opens the user's preferred terminal editor to capture text.
// It strips out comments (lines starting with #) before returning the text.
func CaptureViaEditor(filename, initialContent string) (string, error) {
	editor := os.Getenv("GIT_EDITOR")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi" // Fallback to vi, standard Unix behavior
	}

	msgFilePath := filepath.Join(repo.Dir, filename)
	if err := os.WriteFile(msgFilePath, []byte(initialContent), 0644); err != nil {
		return "", err
	}
	defer os.Remove(msgFilePath) // Always clean up

	cmd := exec.Command(editor, msgFilePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("editor aborted")
	}

	content, err := os.ReadFile(msgFilePath)
	if err != nil {
		return "", err
	}

	// Strip comments and trim whitespace
	var finalLines []string
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "#") {
			finalLines = append(finalLines, line)
		}
	}

	return strings.TrimSpace(strings.Join(finalLines, "\n")), nil
}

// FindRepoRoot searches for the .kitcat directory starting from the current
// working directory and walking up the tree. Returns the absolute path to
// the repository root or an error if not found.
// This function is pure and does not mutate the current working directory.
func FindRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return "", err
	}

	dir := absCwd
	for {
		if _, err := os.Stat(filepath.Join(dir, repo.Dir)); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not a kitcat repository (or any of the parent directories): %s", repo.Dir)
		}
		dir = parent
	}
}

// IsRepoInitialized checks if a repository exists and switches the Current
// Working Directory (CWD) to the repository root if found.
// Wraps FindRepoRoot for backward compatibility with main.go command flow.
func IsRepoInitialized() bool {
	root, err := FindRepoRoot()
	if err != nil {
		return false
	}
	if err := os.Chdir(root); err != nil {
		return false
	}
	return true
}

// GetHeadState returns the active branch name or raw commit hash if detached.
func GetHeadState() (string, error) {
	headContent, err := os.ReadFile(".kitcat/HEAD")
	if err != nil {
		return "", err
	}
	content := strings.TrimSpace(string(headContent))
	if strings.HasPrefix(content, "ref: refs/heads/") {
		return strings.TrimPrefix(content, "ref: refs/heads/"), nil
	}
	return content, nil
}

// IsDetachedHead returns true when HEAD contains a direct commit hash.
func IsDetachedHead() (bool, error) {
	headContent, err := os.ReadFile(".kitcat/HEAD")
	if err != nil {
		return false, err
	}
	return !strings.HasPrefix(string(headContent), "ref: "), nil
}

// VerifyCleanTree ensures index matches HEAD tree exactly.
// Used as a safety guard before destructive operations.
func VerifyCleanTree() error {
	index, err := storage.LoadIndex()
	if err != nil {
		return err
	}

	head, err := storage.GetLastCommit()
	if err != nil {
		if err == storage.ErrNoCommits && len(index) == 0 {
			return nil
		}
		return err
	}

	headTree, err := storage.ParseTree(head.TreeHash)
	if err != nil {
		return err
	}

	if len(index) != len(headTree) {
		return fmt.Errorf("working tree is dirty")
	}

	for path, entry := range index {
		headEntry, ok := headTree[path]
		if !ok {
			return fmt.Errorf("dirty: %s", path)
		}

		// Convert binary hash to hex for comparison with tree map.
		entryHashHex := hex.EncodeToString(entry.Hash[:])
		if entryHashHex != headEntry.Hash {
			return fmt.Errorf("dirty: %s", path)
		}
	}
	return nil
}

// RestoreIndexFromCommit replaces the index contents with entries from a commit tree.
func RestoreIndexFromCommit(commitID string) error {
	commit, err := storage.FindCommit(commitID)
	if err != nil {
		return err
	}

	tree, err := storage.ParseTree(commit.TreeHash)
	if err != nil {
		return err
	}

	return storage.UpdateIndex(func(index map[string]plumbing.IndexEntry) error {
		// Clear existing entries before rebuilding.
		for k := range index {
			delete(index, k)
		}

		for path, entry := range tree {
			hb, _ := storage.HexToHash(entry.Hash)

			var mode uint32
			if _, err := fmt.Sscanf(entry.Mode, "%o", &mode); err != nil {
				mode = 0100644
			}

			index[path] = plumbing.IndexEntry{
				Path: path,
				Hash: hb,
				Mode: mode,
			}
		}
		return nil
	})
}

// IsWorkDirDirty checks staged and unstaged differences between HEAD, index,
// and working directory. Returns true on first detected change.
func IsWorkDirDirty() (bool, error) {
	headTree := make(map[string]storage.TreeEntry)
	lastCommit, err := GetHeadCommit()
	if err == nil {
		tree, parseErr := storage.ParseTree(lastCommit.TreeHash)
		if parseErr != nil {
			return false, parseErr
		}
		headTree = tree
	}

	index, err := storage.LoadIndex()
	if err != nil {
		return false, err
	}

	// Compare index vs HEAD to detect staged changes.
	allPaths := make(map[string]bool)
	for path := range headTree {
		allPaths[path] = true
	}
	for path := range index {
		allPaths[path] = true
	}

	for path := range allPaths {
		headEntry, inHead := headTree[path]
		indexEntry, inIndex := index[path]

		var indexHash string
		if inIndex {
			indexHash = hex.EncodeToString(indexEntry.Hash[:])
		}

		if (inIndex && !inHead) || (!inIndex && inHead) || (inIndex && inHead && headEntry.Hash != indexHash) {
			return true, nil
		}
	}

	// Compare working tree vs index.
	err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		cleanPath := filepath.Clean(path)

		// Skip repo metadata directory.
		if info.IsDir() || strings.HasPrefix(cleanPath, repo.Dir+string(os.PathSeparator)) || cleanPath == repo.Dir {
			return nil
		}

		indexEntry, isTracked := index[cleanPath]
		if !isTracked {
			return fmt.Errorf("untracked")
		}

		currentHash, hashErr := storage.HashFile(cleanPath)
		if hashErr != nil {
			return hashErr
		}

		indexHashHex := hex.EncodeToString(indexEntry.Hash[:])
		if currentHash != indexHashHex {
			return fmt.Errorf("modified")
		}
		return nil
	})

	if err != nil {
		if err.Error() == "untracked" || err.Error() == "modified" {
			return true, nil
		}
		return false, err
	}

	return false, nil
}

// UpdateBranchPointer updates either the branch reference or HEAD itself.
// Uses SafeWrite to avoid partial writes during ref updates.
func UpdateBranchPointer(commitHash string) error {
	headData, err := os.ReadFile(repo.HeadPath)
	if err != nil {
		return fmt.Errorf("unable to read HEAD file: %w", err)
	}
	ref := strings.TrimSpace(string(headData))

	if strings.HasPrefix(ref, "ref: ") {
		refPath := strings.TrimPrefix(ref, "ref: ")
		branchFile := filepath.Join(".kitcat", refPath)
		if err := SafeWrite(branchFile, []byte(commitHash), 0644); err != nil {
			return fmt.Errorf("failed to update branch pointer: %w", err)
		}
		return nil
	}

	if err := SafeWrite(repo.HeadPath, []byte(commitHash), 0644); err != nil {
		return fmt.Errorf("failed to update HEAD: %w", err)
	}
	return nil
}

// readHead resolves HEAD to a commit hash, following symbolic refs.
func readHead() (string, error) {
	headData, err := os.ReadFile(repo.HeadPath)
	if err != nil {
		return "", err
	}
	ref := strings.TrimSpace(string(headData))

	if strings.HasPrefix(ref, "ref: ") {
		refPath := strings.TrimPrefix(ref, "ref: ")
		branchFile := filepath.Join(".kitcat", refPath)
		commitHash, err := os.ReadFile(branchFile)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(commitHash)), nil
	}
	return ref, nil
}

// IsSafePath rejects absolute paths and parent traversal segments to avoid
// writes outside repository boundaries.
func IsSafePath(path string) bool {
	cleanPath := filepath.Clean(path)
	if filepath.IsAbs(cleanPath) {
		return false
	}
	if strings.Contains(cleanPath, "..") {
		return false
	}
	return true
}

// SafeWrite performs atomic file updates by writing to a temp file and renaming.
func SafeWrite(filename string, data []byte, perm os.FileMode) error {
	dirPath := filepath.Dir(filename)
	f, err := os.CreateTemp(dirPath, "atomic-")
	if err != nil {
		return err
	}
	tmpName := f.Name()
	defer os.Remove(tmpName)

	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(perm); err != nil {
		f.Close()
		return err
	}
	f.Close()
	return os.Rename(tmpName, filename)
}

// GetHeadCommit returns the commit referenced by HEAD, which may differ
// from the last chronological commit after resets or history rewrites.
func GetHeadCommit() (models.Commit, error) {
	commitHash, err := readHead()
	if err != nil {
		return models.Commit{}, err
	}
	return storage.FindCommit(commitHash)
}

// copyRecursive copies a file or directory tree.
// Used when os.Rename fails across filesystem boundaries.
func copyRecursive(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		// Create destination directory before descending so structure is preserved.
		if err := os.MkdirAll(dst, info.Mode()); err != nil {
			return err
		}

		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}

		// Recursively copy children to maintain relative layout.
		for _, e := range entries {
			s := filepath.Join(src, e.Name())
			d := filepath.Join(dst, e.Name())
			if err := copyRecursive(s, d); err != nil {
				return err
			}
		}
		return nil
	}

	// File copy
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	// Ensure parent directory exists to avoid partial copy failures.
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	// Stream copy avoids loading entire file into memory.
	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	// Preserve original file mode for consistency with git mv fallback behaviour.
	return out.Chmod(info.Mode())
}

// checkoutIndexFromMap writes tree contents directly into the working
// directory. Existing files may be overwritten.
func checkoutIndexFromMap(tree map[string]storage.TreeEntry) error {
	for path, entry := range tree {
		content, err := storage.ReadObject(entry.Hash)
		if err != nil {
			return err
		}

		perm := os.FileMode(0644)
		var modeVal uint32
		if _, err := fmt.Sscanf(entry.Mode, "%o", &modeVal); err == nil {
			if (modeVal & 0111) != 0 {
				perm = 0755
			}
		}

		if err := os.WriteFile(path, content, perm); err != nil {
			return err
		}
	}
	return nil
}

// ReflogAppend writes a standard git-style entry to the reflog for a given reference.
func ReflogAppend(refname, oldHash, newHash, message string) error {
	name, _, _ := GetConfig("user.name")
	email, _, _ := GetConfig("user.email")
	if name == "" {
		name = "Unknown"
	}
	if email == "" {
		email = "unknown@example.com"
	}

	timestamp := time.Now().Unix()
	tzOffset := time.Now().Format("-0700")

	if oldHash == "" {
		oldHash = "0000000000000000000000000000000000000000"
	}

	logEntry := fmt.Sprintf("%s %s %s <%s> %d %s\t%s\n", oldHash, newHash, name, email, timestamp, tzOffset, message)

	logPath := filepath.Join(".kitcat", "logs", refname)
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(logEntry)
	return err
}

// UpdateWorkspaceAndIndex forces the working directory and index to match
// the tree snapshot of a given commit.
func UpdateWorkspaceAndIndex(commitHash string) error {
	commit, err := storage.FindCommit(commitHash)
	if err != nil {
		return err
	}
	
	// Delegate to the new architecture logic
	if err := CheckoutTree(commit.TreeHash); err != nil {
		return err
	}
	return ReadTree(commit.TreeHash)
}
