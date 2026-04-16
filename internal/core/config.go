package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/repo"
)

// getConfigPath resolves the path to the configuration file.
//
// The operation determines scope:
//   - If global is true, it uses ~/.kitcatconfig
//   - If global is false, it uses the local repository config
func getConfigPath(global bool) string {
	if global {
		home, err := os.UserHomeDir()
		if err != nil {
			return ".kitcatconfig"
		}
		return filepath.Join(home, ".kitcatconfig")
	}
	return filepath.Join(repo.Dir, "config")
}

// splitKey parses a dot-separated Git config key into its components.
//
// For example:
//   - "user.name" -> section: "user", subsection: "", key: "name"
//   - "remote.origin.url" -> section: "remote", subsection: "origin", key: "url"
func splitKey(fullKey string) (section, subsection, key string, err error) {
	parts := strings.Split(fullKey, ".")
	if len(parts) < 2 {
		return "", "", "", fmt.Errorf("error: key does not contain a section: %s", fullKey)
	}
	section = parts[0]
	key = parts[len(parts)-1]
	if len(parts) > 2 {
		subsection = strings.Join(parts[1:len(parts)-1], ".")
	}
	return section, subsection, key, nil
}

// parseSectionHeader extracts section names from an INI header.
//
// For example:
//   - "[core]" -> section: "core", subsection: ""
//   - "[remote \"origin\"]" -> section: "remote", subsection: "origin"
func parseSectionHeader(line string) (section, subsection string, isSection bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, "]") {
		return "", "", false
	}
	content := line[1 : len(line)-1]
	parts := strings.SplitN(content, " ", 2)
	section = parts[0]
	if len(parts) > 1 {
		subsection = strings.Trim(parts[1], "\"")
	}
	return section, subsection, true
}

// SetConfig updates or appends a key-value pair in an INI format.
//
// The operation modifies the file safely:
//   - Matches existing sections and subsections
//   - Preserves comments and unrelated sections
//   - Appends new sections to the end of the file if missing
func SetConfig(fullKey, value string, global bool) error {
	section, subsection, key, err := splitKey(fullKey)
	if err != nil {
		return err
	}

	path := getConfigPath(global)
	data, _ := os.ReadFile(path)
	lines := strings.Split(string(data), "\n")

	// If the file is completely empty or missing, prevent a leading blank line.
	if len(lines) == 1 && lines[0] == "" {
		lines = []string{}
	}

	var out []string
	inTargetSection := false
	keyFound := false
	sectionFound := false

	for _, line := range lines {
		trimLine := strings.TrimSpace(line)

		// Detect Section Headers
		if strings.HasPrefix(trimLine, "[") && strings.HasSuffix(trimLine, "]") {
			currSec, currSub, ok := parseSectionHeader(trimLine)
			if ok {
				// If we are leaving the target section and haven't found the key, inject it here.
				if inTargetSection && !keyFound {
					out = append(out, fmt.Sprintf("\t%s = %s", key, value))
					keyFound = true
				}
				inTargetSection = (currSec == section && currSub == subsection)
				if inTargetSection {
					sectionFound = true
				}
			}
		}

		// Detect Keys within the active section
		if inTargetSection && !keyFound && !strings.HasPrefix(trimLine, "[") && !strings.HasPrefix(trimLine, "#") && trimLine != "" {
			parts := strings.SplitN(trimLine, "=", 2)
			if len(parts) == 2 {
				k := strings.TrimSpace(parts[0])
				if k == key {
					// Replace the existing line
					out = append(out, fmt.Sprintf("\t%s = %s", key, value))
					keyFound = true
					continue
				}
			}
		}
		out = append(out, line)
	}

	// Handle End-of-File injections
	if inTargetSection && !keyFound {
		out = append(out, fmt.Sprintf("\t%s = %s", key, value))
	} else if !sectionFound {
		if len(out) > 0 && out[len(out)-1] != "" {
			out = append(out, "")
		}
		if subsection == "" {
			out = append(out, fmt.Sprintf("[%s]", section))
		} else {
			out = append(out, fmt.Sprintf("[%s \"%s\"]", section, subsection))
		}
		out = append(out, fmt.Sprintf("\t%s = %s", key, value))
	}

	// Ensure the file ends with a newline to match Git's standard
	if len(out) > 0 && out[len(out)-1] != "" {
		out = append(out, "")
	}

	return SafeWrite(path, []byte(strings.Join(out, "\n")), 0o644)
}






// GetConfig retrieves a value from the INI configuration file.
//
// The operation parses sections iteratively:
//   - Skips unrelated sections and comments
//   - Returns the value and a boolean indicating if it was found
func GetConfig(fullKey string) (string, bool, error) {
	section, subsection, key, err := splitKey(fullKey)
	if err != nil {
		return "", false, err
	}

	// Priority 1: Check Local Config
	if val, found := readKeyFromPath(getConfigPath(false), section, subsection, key); found {
		return val, true, nil
	}
	// Priority 2: Check Global Config
	if val, found := readKeyFromPath(getConfigPath(true), section, subsection, key); found {
		return val, true, nil
	}

	return "", false, nil
}

// readKeyFromPath is an internal helper that parses a specific config file.
func readKeyFromPath(path, targetSection, targetSubsection, targetKey string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}

	var currentSection, currentSubsection string
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		trimLine := strings.TrimSpace(line)
		if trimLine == "" || strings.HasPrefix(trimLine, "#") || strings.HasPrefix(trimLine, ";") {
			continue
		}

		if strings.HasPrefix(trimLine, "[") && strings.HasSuffix(trimLine, "]") {
			currentSection, currentSubsection, _ = parseSectionHeader(trimLine)
			continue
		}

		if currentSection == targetSection && currentSubsection == targetSubsection {
			parts := strings.SplitN(trimLine, "=", 2)
			if len(parts) == 2 {
				k := strings.TrimSpace(parts[0])
				v := strings.TrimSpace(parts[1])
				if k == targetKey {
					return v, true
				}
			}
		}
	}
	return "", false
}

// PrintAllConfig outputs all configured key-value pairs in dot-notation.
//
// The operation formats INI back into Git CLI output:
//   - "core.bare=false"
//   - "remote.origin.url=https://..."
func PrintAllConfig(global bool) error {
	path := getConfigPath(global)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil // File not existing is not fatal
	}

	var currentSection, currentSubsection string
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		trimLine := strings.TrimSpace(line)
		if trimLine == "" || strings.HasPrefix(trimLine, "#") || strings.HasPrefix(trimLine, ";") {
			continue
		}

		if strings.HasPrefix(trimLine, "[") && strings.HasSuffix(trimLine, "]") {
			currentSection, currentSubsection, _ = parseSectionHeader(trimLine)
			continue
		}

		parts := strings.SplitN(trimLine, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			if currentSubsection == "" {
				fmt.Printf("%s.%s=%s\n", currentSection, k, v)
			} else {
				fmt.Printf("%s.%s.%s=%s\n", currentSection, currentSubsection, k, v)
			}
		}
	}
	return nil
}
