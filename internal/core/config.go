package core

import (
	"fmt"
	"os"
	"strings"
)

// SetConfig sets a key-value pair in .kitcat/config
func SetConfig(key, value string, global bool) error {
	// Simple append implementation for MVP
	// In a real implementation, this should parse and replace existing keys
	f, err := os.OpenFile(".kitcat/config", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = fmt.Fprintf(f, "%s=%s\n", key, value)
	return err
}

func GetConfig(key string) (string, bool, error) {
	data, err := os.ReadFile(".kitcat/config")
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}

	lines := strings.Split(string(data), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		parts := strings.SplitN(lines[i], "=", 2)
		if len(parts) == 2 && parts[0] == key {
			return parts[1], true, nil
		}
	}
	return "", false, nil
}

func PrintAllConfig() error {
	data, err := os.ReadFile(".kitcat/config")
	if err == nil {
		fmt.Print(string(data))
	}
	return nil
}
