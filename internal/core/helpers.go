package core

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/LeeFred3042U/kitcat/internal/models"
	"github.com/LeeFred3042U/kitcat/internal/plumbing"
	"github.com/LeeFred3042U/kitcat/internal/repo"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

var (
	errUntracked = errors.New("untracked")
	errModified  = errors.New("modified")
)

func selectBestMergeBase(bases []string) (string, error) {
	if len(bases) == 0 {
		return "", fmt.Errorf("no merge base found")
	}
	if len(bases) == 1 {
		return bases[0], nil
	}

	// TODO: replace with generation-based scoring later
	// For now: deterministic pick (lexicographically smallest)
	best := bases[0]
	for _, b := range bases[1:] {
		if b < best {
			best = b
		}
	}
	return best, nil
}

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
	if err := os.WriteFile(msgFilePath, []byte(initialContent), 0o644); err != nil {
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
// working directory and walking up the tree.
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

// IsDetachedHead returns true when HEAD contains a direct commit hash.
func IsDetachedHead() (bool, error) {
	headContent, err := os.ReadFile(repo.HeadPath)
	if err != nil {
		return false, err
	}
	return !strings.HasPrefix(string(headContent), "ref: "), nil
}

// VerifyCleanTree ensures index matches HEAD tree exactly.
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
		for k := range index {
			delete(index, k)
		}

		for path, entry := range tree {
			hb, _ := storage.HexToHash(entry.Hash)

			var mode uint32
			if _, err := fmt.Sscanf(entry.Mode, "%o", &mode); err != nil {
				mode = 0o100644
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

// IsWorkDirDirty checks staged and unstaged differences.
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

	err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		cleanPath := filepath.Clean(path)

		if info.IsDir() || strings.HasPrefix(cleanPath, repo.Dir+string(os.PathSeparator)) || cleanPath == repo.Dir {
			return nil
		}

		indexEntry, isTracked := index[cleanPath]
		if !isTracked {
			// Untracked files are not considered dirty (matches Git behaviour).
			// Only files already recorded in the index can make the working
			// directory "dirty". Operations that would overwrite an untracked
			// file must perform their own per-path collision check (as Checkout
			// already does via its untracked collision check in step 2.5).
			return nil
		}

		// Use os.Lstat directly rather than relying on Walk's info, to guarantee
		// symlink detection is not affected by filesystem or Walk implementation
		// differences across platforms.
		lfi, lstatErr := os.Lstat(cleanPath)
		if lstatErr != nil {
			return lstatErr
		}

		var currentHash string
		if lfi.Mode()&os.ModeSymlink != 0 {
			// For symlinks, hash the link target PATH string (what the blob stores),
			// not the target file's content (what os.ReadFile would return by
			// following the link). HashFile uses os.ReadFile which follows symlinks
			// and would always report a mismatch for correctly staged symlinks.
			target, readlinkErr := os.Readlink(cleanPath)
			if readlinkErr != nil {
				return readlinkErr
			}
			var hashErr error
			currentHash, hashErr = plumbing.HashAndWriteObject([]byte(target), "blob")
			if hashErr != nil {
				return hashErr
			}
		} else {
			var hashErr error
			currentHash, hashErr = storage.HashFile(cleanPath)
			if hashErr != nil {
				return hashErr
			}
		}

		indexHashHex := hex.EncodeToString(indexEntry.Hash[:])
		if currentHash != indexHashHex {
			return errModified
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, errUntracked) || errors.Is(err, errModified) {
			return true, nil
		}
		return false, err
	}

	return false, nil
}

// GetHeadState returns the active branch name or raw commit hash if detached.
func GetHeadState() (string, error) {
	headContent, err := os.ReadFile(repo.HeadPath)
	if err != nil {
		return "", err
	}
	content := strings.TrimSpace(string(headContent))
	if trimmed, ok := strings.CutPrefix(content, "ref: refs/heads/"); ok {
		return trimmed, nil
	}
	return content, nil
}

// UpdateBranchPointer updates either the branch reference or HEAD itself.
func UpdateBranchPointer(commitHash string) error {
	headData, err := os.ReadFile(repo.HeadPath)
	if err != nil {
		return fmt.Errorf("unable to read HEAD file: %w", err)
	}
	ref := strings.TrimSpace(string(headData))

	if refPath, ok := strings.CutPrefix(ref, "ref: "); ok {
		branchFile := filepath.Join(repo.Dir, refPath)
		if err := SafeWrite(branchFile, []byte(commitHash), 0o644); err != nil {
			return fmt.Errorf("failed to update branch pointer: %w", err)
		}
		return nil
	}

	if err := SafeWrite(repo.HeadPath, []byte(commitHash), 0o644); err != nil {
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

	if refPath, ok := strings.CutPrefix(ref, "ref: "); ok {
		branchFile := filepath.Join(repo.Dir, refPath)
		commitHash, err := os.ReadFile(branchFile)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(commitHash)), nil
	}
	return ref, nil
}

// IsSafePath rejects absolute paths and parent traversal segments.
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

// SafeWrite performs atomic file updates with a Windows-safe retry loop.
func SafeWrite(filename string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(filename)

	// Create temp file in same directory
	f, err := os.CreateTemp(dir, "atomic-")
	if err != nil {
		return err
	}
	tmpName := f.Name()

	cleanup := func(e error) error {
		f.Close()
		_ = os.Remove(tmpName)
		return e
	}

	// Write
	if _, err := f.Write(data); err != nil {
		return cleanup(err)
	}

	// Ensure file contents hit disk
	if err := f.Sync(); err != nil {
		return cleanup(err)
	}

	if err := f.Chmod(perm); err != nil {
		return cleanup(err)
	}

	if err := f.Close(); err != nil {
		return cleanup(err)
	}

	// Windows-safe retry loop
	const maxRetries = 5
	delay := 10 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		err = os.Rename(tmpName, filename)
		if err == nil {
			break
		}

		// Only retry on known transient Windows errors
		if !isRetryable(err) {
			_ = os.Remove(tmpName)
			return err
		}

		if i < maxRetries-1 {
			time.Sleep(delay)
			delay *= 2
		}
	}

	if err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rename %s -> %s failed: %w", tmpName, filename, err)
	}

	// Unix durability: fsync directory
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}

	return nil
}

func isRetryable(err error) bool {
	// Windows tends to return EBUSY/EACCES if antivirus
	// or another process is holding the file briefly.
	return errors.Is(err, syscall.EBUSY) ||
		errors.Is(err, syscall.EACCES)
}

// GetHeadCommit returns the commit referenced by HEAD.
func GetHeadCommit() (models.Commit, error) {
	commitHash, err := readHead()
	if err != nil {
		return models.Commit{}, err
	}
	return storage.FindCommit(commitHash)
}

// copyRecursive copies a file or directory tree.
func copyRecursive(src, dst string) error {
	info, err := os.Lstat(src) // Lstat so symlinks are not followed
	if err != nil {
		return err
	}

	// Reproduce symlinks as symlinks, not as copies of their targets.
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(target, dst)
	}

	if info.IsDir() {
		if err := os.MkdirAll(dst, info.Mode()); err != nil {
			return err
		}

		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}

		for _, e := range entries {
			s := filepath.Join(src, e.Name())
			d := filepath.Join(dst, e.Name())
			if err := copyRecursive(s, d); err != nil {
				return err
			}
		}
		return nil
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return out.Chmod(info.Mode())
}

// ReflogAppend atomically appends a git-style reflog entry to the given
// reference log. The previous content is read, the new entry is appended
// in memory, and the result is written via SafeWrite (temp-file + rename)
// to prevent two concurrent writers from interleaving partial lines.
func ReflogAppend(refname, oldHash, newHash, message string) error {
	name, _, _ := GetConfig("user.name")
	email, _, _ := GetConfig("user.email")
	if name == "" {
		name = "Unknown"
	}
	if email == "" {
		email = "unknown@example.com"
	}

	now := time.Now()
	timestamp := now.Unix()
	tzOffset := now.Format("-0700")

	if oldHash == "" {
		oldHash = "0000000000000000000000000000000000000000"
	}

	newEntry := fmt.Sprintf("%s %s %s <%s> %d %s\t%s\n", oldHash, newHash, name, email, timestamp, tzOffset, message)

	logPath := filepath.Join(repo.Dir, "logs", refname)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}

	// Read existing log content (ignore error if file doesn't exist yet).
	existing, _ := os.ReadFile(logPath)

	// Append in memory then write atomically to avoid concurrent interleaving.
	return SafeWrite(logPath, append(existing, []byte(newEntry)...), 0o644)
}

// UpdateWorkspaceAndIndex forces the working directory and index to match
// the tree snapshot of a given commit.
func UpdateWorkspaceAndIndex(commitHash string) error {
	commit, err := storage.FindCommit(commitHash)
	if err != nil {
		return err
	}

	if err := CheckoutTree(commit.TreeHash); err != nil {
		return err
	}
	return ReadTree(commit.TreeHash)
}
