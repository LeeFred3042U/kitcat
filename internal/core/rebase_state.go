package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// RebaseState tracks ongoing rebase
type RebaseState struct {
	HeadName    string   // "refs/heads/main" or empty if detached
	Onto        string   // Commit ID base
	OrigHead    string   // Commit ID where we started (for abort)
	TodoSteps   []string // List of commands
	CurrentStep int      // Index in TodoSteps (0-based)
	Message     string   // For squash/reword message accumulation
}

func EnsureRebaseDir() error {
	path := filepath.Join(RepoDir, "rebase-merge")
	return os.MkdirAll(path, 0755)
}

func SaveRebaseState(state RebaseState) error {
	if err := EnsureRebaseDir(); err != nil {
		return err
	}
	base := filepath.Join(RepoDir, "rebase-merge")

	// Helper to write files and check errors immediately
	write := func(filename, content string) error {
		return os.WriteFile(filepath.Join(base, filename), []byte(content), 0644)
	}

	if err := write("head-name", state.HeadName); err != nil { return err }
	if err := write("onto", state.Onto); err != nil { return err }
	if err := write("orig-head", state.OrigHead); err != nil { return err }
	if err := write("git-rebase-todo", strings.Join(state.TodoSteps, "\n")); err != nil { return err }
	if err := write("msgnum", fmt.Sprintf("%d", state.CurrentStep+1)); err != nil { return err }
	if err := write("message", state.Message); err != nil { return err }

	return nil
}

func LoadRebaseState() (*RebaseState, error) {
	base := filepath.Join(RepoDir, "rebase-merge")
	if _, err := os.Stat(base); os.IsNotExist(err) {
		return nil, fmt.Errorf("no rebase in progress")
	}

	// Helper to read files, returning empty string on error/missing
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

	step, _ := strconv.Atoi(msgNumData)
	// step is 1-based in file, 0-based in struct
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

func IsRebaseInProgress() bool {
	_, err := os.Stat(filepath.Join(RepoDir, "rebase-merge"))
	return err == nil
}

func ClearRebaseState() error {
	return os.RemoveAll(filepath.Join(RepoDir, "rebase-merge"))
}

func ReadNextTodo() (string, *RebaseState, error) {
	state, err := LoadRebaseState()
	if err != nil {
		return "", nil, err
	}
	// Check bounds
	if state.CurrentStep < 0 || state.CurrentStep >= len(state.TodoSteps) {
		return "", state, nil
	}
	return state.TodoSteps[state.CurrentStep], state, nil
}

func AdvanceRebaseStep(state *RebaseState) error {
	state.CurrentStep++
	return SaveRebaseState(*state)
}
