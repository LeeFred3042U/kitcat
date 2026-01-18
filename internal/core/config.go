package core

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// getConfigPath returns the absolute path to the global kitcat config file
func getConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".kitcatconfig"), nil
}

// getLocalConfigPath returns the absolute path to the local repository config file
func getLocalConfigPath() (string, error) {
	localConfigPath := filepath.Join(RepoDir, ".kitcat", "config")
	return localConfigPath, nil
}

// readConfigFromPath loads a config file from a specific path into a map
func readConfigFromPath(path string) (map[string]string, error) {
	config := make(map[string]string)
	file, err := os.Open(path)
	// It's okay if the file doesn't exist yet, just return an empty map
	if os.IsNotExist(err) {
		return config, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// The file format is simple: key=value
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			config[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return config, scanner.Err()
}

// readConfig loads the global config file into a map
func readConfig() (map[string]string, error) {
	path, err := getConfigPath()
	if err != nil {
		return nil, err
	}
	return readConfigFromPath(path)
}

// SetConfig sets a key-value pair in the config file (local or global)
func SetConfig(key, value string, global bool) error {
	var path string

	if global {
		// Write to global config
		globalPath, err := getConfigPath()
		if err != nil {
			return err
		}
		path = globalPath
	} else {
		// Write to local config
		localConfigPath, err := getLocalConfigPath()
		if err != nil {
			return err
		}
		path = localConfigPath
		// Ensure directory exists before writing
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
	}

	config, err := readConfigFromPath(path)
	if err != nil {
		return fmt.Errorf("could not read existing config: %w", err)
	}

	// Update the map with the new key-value pair
	config[key] = value

	// Write the entire updated map back to the file
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	for k, v := range config {
		if _, err := fmt.Fprintf(file, "%s = %s\n", k, v); err != nil {
			return err
		}
	}

	return nil
}

// readKey reads a specific key from a config file at the given path
// Returns (value, true, nil) if found
// Returns ("", false, nil) if not found or file doesn't exist
// Returns error only on real I/O failure
func readKey(path, key string) (string, bool, error) {
	config, err := readConfigFromPath(path)
	if err != nil {
		// Missing file is not an error
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	value, ok := config[key]
	return value, ok, nil
}

// GetConfig reads a key from the config file (local first, then global)
func GetConfig(key string) (string, bool, error) {
	// 1. Try local config first
	localPath, err := getLocalConfigPath()
	if err != nil {
		return "", false, err
	}
	if value, found, err := readKey(localPath, key); err != nil {
		return "", false, err
	} else if found {
		return value, true, nil
	}

	// 2. Fallback to global config
	globalPath, err := getConfigPath()
	if err != nil {
		return "", false, err
	}
	return readKey(globalPath, key)
}

// PrintAllConfig prints all key-value pairs in the config file
func PrintAllConfig() error {
	config, err := readConfig()
	if err != nil {
		return err
	}
	for k, v := range config {
		fmt.Printf("%s = %s\n", k, v)
	}
	return nil
}
