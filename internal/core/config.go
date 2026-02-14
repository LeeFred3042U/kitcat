package core

import (
	"fmt"
	"os"
	"strings"
)

// SetConfig appends a key-value pair to .kitcat/config.
// Existing keys are not replaced; later entries override earlier ones
// during lookup. The 'global' flag is currently unused.
func SetConfig(key, value string, global bool) error {
	// Simple append-only implementation; no deduplication or validation.
	f, err := os.OpenFile(".kitcat/config", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = fmt.Fprintf(f, "%s=%s\n", key, value)
	return err
}

// GetConfig returns the most recent value for a key by scanning
// the config file from bottom to top.
func GetConfig(key string) (string, bool, error) {
	data, err := os.ReadFile(".kitcat/config")
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}

	lines := strings.Split(string(data), "\n")

	// Reverse scan ensures newest entry wins without rewriting the file.
	for i := len(lines) - 1; i >= 0; i-- {
		parts := strings.SplitN(lines[i], "=", 2)
		if len(parts) == 2 && parts[0] == key {
			return parts[1], true, nil
		}
	}
	return "", false, nil
}

// PrintAllConfig prints the raw config file contents.
// Missing file is silently ignored.
func PrintAllConfig() error {
	data, err := os.ReadFile(".kitcat/config")
	if err == nil {
		fmt.Print(string(data))
	}
	return nil
}
