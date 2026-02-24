package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/LeeFred3042U/kitcat/internal/constant"
)

func handleLink(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: %s link <new-name>\n", constant.AppName)
		os.Exit(exitUsage)
	}

	newName := args[0]

	// 1. RESTRICTION: Hardcoded blacklist of shell builtins and dangerous commands
	restricted := []string{
		"bash", "zsh", "sh", "cd", "ls", "rm", "mv", "cp", "git", 
		"sudo", "alias", "echo", "pwd", "cat", "mkdir", "clear",
	}
	for _, r := range restricted {
		if newName == r {
			die("cannot link to '%s': reserved system command", newName)
		}
	}

	// 2. RESTRICTION: Dynamic system check
	// This checks if the user's OS already has this command installed!
	if existingPath, err := exec.LookPath(newName); err == nil {
		die("cannot link to '%s': command already exists on your system at %s", newName, existingPath)
	}

	// Get the path of the currently running kitcat binary
	exePath, err := os.Executable()
	if err != nil {
		die("failed to find executable path: %v", err)
	}

	// Create the link in the exact same directory as the kitcat binary
	targetPath := filepath.Join(filepath.Dir(exePath), newName)

	// Try to create a symbolic link first
	if err := os.Symlink(exePath, targetPath); err != nil {
		// Fallback to a hard link (Windows often restricts symlinks without Admin rights)
		if err := os.Link(exePath, targetPath); err != nil {
			die("failed to create link: %v\nTry running your terminal as Administrator.", err)
		}
	}

	fmt.Printf("Successfully linked '%s' to %s!\n", newName, constant.AppName)
	fmt.Printf("You can now use '%s' instead of '%s'.\n", newName, constant.AppName)
}
