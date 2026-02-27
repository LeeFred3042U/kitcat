package core

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/LeeFred3042U/kitcat/internal/storage"
)

/*
Binary detection using NUL-byte heuristic
*/
func isBinary(data []byte) bool {
	return bytes.Contains(data, []byte{0})
}

/*
kitcat grep implementation
*/
func Grep(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: kitcat grep [--line-number] <pattern>")
	}

	showLineNumber := false
	i := 0

	if args[0] == "--line-number" {
		showLineNumber = true
		i++
	}

	if len(args[i:]) < 1 {
		return fmt.Errorf("usage: kitcat grep [--line-number] <pattern>")
	}

	pattern := args[i]

	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}

	// Load kitcat index (only tracked files)
	indexMap, err := storage.LoadIndex()
	if err != nil {
		return err
	}

	// Deterministic order: sort file paths
	paths := make([]string, 0, len(indexMap))
	for path := range indexMap {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		// Skip binary files
		if isBinary(data) {
			continue
		}

		// Only UTF-8 files are scanned
		if !utf8.Valid(data) {
			continue
		}

		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		lineNo := 1

		for scanner.Scan() {
			line := scanner.Text()
			if re.MatchString(line) {
				if showLineNumber {
					fmt.Printf("%s:%d:%s\n", path, lineNo, line)
				} else {
					fmt.Printf("%s:%s\n", path, line)
				}
			}
			lineNo++
		}
	}

	return nil
}
