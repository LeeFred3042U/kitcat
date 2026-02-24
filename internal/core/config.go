package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// getConfigPath returns the active configuration file path.
func getConfigPath(global bool) string {
	if global {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, ".kitcatconfig")
		}
	}
	return ".kitcat/config"
}

// SetConfig updates or appends a key-value pair atomically.
func SetConfig(key, value string, global bool) error {
	path := getConfigPath(global)
	data, _ := os.ReadFile(path)
	
	lines := strings.Split(string(data), "\n")
	var out []string
	found := false

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == key {
			out = append(out, fmt.Sprintf("%s=%s", key, value))
			found = true
		} else {
			out = append(out, line)
		}
	}

	if !found {
		out = append(out, fmt.Sprintf("%s=%s", key, value))
	}

	content := strings.Join(out, "\n") + "\n"

	// If writing locally, use our transactional SafeWrite
	if !global && IsSafePath(path) {
		return SafeWrite(path, []byte(content), 0644)
	}
	
	// Global writes fallback to standard os write
	return os.WriteFile(path, []byte(content), 0644)
}

// GetConfig checks the local repo config first, and falls back to the global config.
func GetConfig(key string) (string, bool, error) {
	// Try Local
	if val, ok := readConfigKey(".kitcat/config", key); ok {
		return val, true, nil
	}

	// Try Global Fallback
	globalPath := getConfigPath(true)
	if val, ok := readConfigKey(globalPath, key); ok {
		return val, true, nil
	}

	return "", false, nil
}

// readConfigKey is a helper to extract a value from a specific file.
func readConfigKey(path, key string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}

	lines := strings.Split(string(data), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		parts := strings.SplitN(lines[i], "=", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == key {
			return strings.TrimSpace(parts[1]), true
		}
	}
	return "", false
}

// PrintAllConfig prints the raw config file contents for local or global.
func PrintAllConfig(global bool) error {
	path := getConfigPath(global)
	data, err := os.ReadFile(path)
	if err == nil {
		fmt.Print(string(data))
	}
	return nil
}