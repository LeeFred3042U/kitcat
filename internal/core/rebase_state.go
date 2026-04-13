package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/repo"
)

// RebaseState holds the on-disk representation of a rebase in progress.
// Fields map directly to files under <repo>/.kitcat/rebase-merge.
type RebaseState struct {
	HeadName    string   // "refs/heads/main" or empty if detached
	Onto        string   // Commit ID base
	OrigHead    string   // Commit ID where we started (for abort)
	TodoSteps   []string // List of commands (one per line)
	CurrentStep int      // Index in TodoSteps (0-based)
	Message     string   // For squash/reword message accumulation
}

// EnsureRebaseDir creates the rebase-merge directory if it does not exist.
func EnsureRebaseDir() error {
	path := filepath.Join(repo.Dir, "rebase-merge")
	return os.MkdirAll(path, 0o755)
}

// SaveRebaseState persists the RebaseState into files under .kitcat/rebase-merge.
// Writes are simple text files; failures return immediately.
func SaveRebaseState(state RebaseState) error {
	if err := EnsureRebaseDir(); err != nil {
		return err
	}
	base := filepath.Join(repo.Dir, "rebase-merge")

	// Helper to write files and check errors immediately.
	write := func(filename, content string) error {
		return os.WriteFile(filepath.Join(base, filename), []byte(content), 0o644)
	}

	if err := write("head-name", state.HeadName); err != nil {
		return err
	}
	if err := write("onto", state.Onto); err != nil {
		return err
	}
	if err := write("orig-head", state.OrigHead); err != nil {
		return err
	}

	// The todo file stores one command per line.
	if err := write("git-rebase-todo", strings.Join(state.TodoSteps, "\n")); err != nil {
		return err
	}

	// msgnum is stored 1-based on-disk; CurrentStep is 0-based in memory.
	if err := write("msgnum", fmt.Sprintf("%d", state.CurrentStep+1)); err != nil {
		return err
	}

	// message accumulates commit messages for squash/reword operations.
	if err := write("message", state.Message); err != nil {
		return err
	}

	return nil
}

// LoadRebaseState reconstructs RebaseState from files under .kitcat/rebase-merge.
// Missing directory yields an error indicating no rebase is in progress.
func LoadRebaseState() (*RebaseState, error) {
	base := filepath.Join(repo.Dir, "rebase-merge")
	if _, err := os.Stat(base); os.IsNotExist(err) {
		return nil, fmt.Errorf("no rebase in progress")
	}

	// Helper to read files; returns empty string on any read error to keep loading robust.
	read := func(filename string) string {
		content, _ := os.ReadFile(filepath.Join(base, filename))
		return strings.TrimSpace(string(content))
	}

	headName := read("head-name")
	onto := read("onto")
	origHead := read("orig-head")
	todoData := read("git-rebase-todo")
	msgNumData := read("msgnum")
	message := read("message")

	// msgnum stored 1-based; convert to 0-based index.
	step, _ := strconv.Atoi(msgNumData)
	if step > 0 {
		step--
	}

	return &RebaseState{
		HeadName:    headName,
		Onto:        onto,
		OrigHead:    origHead,
		TodoSteps:   strings.Split(todoData, "\n"),
		CurrentStep: step,
		Message:     message,
	}, nil
}

// IsRebaseInProgress returns true if the rebase-merge directory exists.
func IsRebaseInProgress() bool {
	_, err := os.Stat(filepath.Join(repo.Dir, "rebase-merge"))
	return err == nil
}

// ClearRebaseState removes the rebase-merge directory and all its files.
func ClearRebaseState() error {
	return os.RemoveAll(filepath.Join(repo.Dir, "rebase-merge"))
}

// ReadNextTodo returns the next todo command and the loaded state.
// If the current step is out of bounds, returns empty command with the state.
func ReadNextTodo() (string, *RebaseState, error) {
	state, err := LoadRebaseState()
	if err != nil {
		return "", nil, err
	}
	// Bounds check: return nil command if current step is invalid.
	if state.CurrentStep < 0 || state.CurrentStep >= len(state.TodoSteps) {
		return "", state, nil
	}
	return state.TodoSteps[state.CurrentStep], state, nil
}

// AdvanceRebaseStep increments the CurrentStep and persists the updated state.
// Caller must provide a valid, mutable state pointer.
func AdvanceRebaseStep(state *RebaseState) error {
	state.CurrentStep++
	return SaveRebaseState(*state)
}
