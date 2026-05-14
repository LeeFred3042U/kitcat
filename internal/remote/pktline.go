package remote

import (
	"fmt"
	"io"
	"strconv"
)

// pktLineFlush is the flush packet that terminates a pkt-line section.
var pktLineFlush = []byte("0000")

// encodePktLine encodes a single string as a Git pkt-line.
// Format: 4 hex digits of total length (including the 4-byte prefix) + payload.
// A newline is appended to the payload if not already present.
func encodePktLine(s string) []byte {
	if len(s) == 0 {
		return pktLineFlush
	}
	// pkt-line length includes the 4-byte length prefix itself.
	length := len(s) + 4
	return fmt.Appendf(nil, "%04x%s", length, s)
}

// decodePktLine reads one pkt-line from r.
// Returns ("", nil) for a flush packet (0000).
// Returns (line, nil) on success.
func decodePktLine(r io.Reader) (string, error) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return "", fmt.Errorf("reading pkt-line length: %w", err)
	}

	length, err := strconv.ParseInt(string(lenBuf), 16, 32)
	if err != nil {
		return "", fmt.Errorf("parsing pkt-line length %q: %w", lenBuf, err)
	}

	// Flush packet — signals end of a section.
	if length == 0 {
		return "", nil
	}

	if length < 4 {
		return "", fmt.Errorf("invalid pkt-line length %d", length)
	}

	payload := make([]byte, length-4)
	if _, err := io.ReadFull(r, payload); err != nil {
		return "", fmt.Errorf("reading pkt-line payload: %w", err)
	}

	// Strip trailing newline — callers work with plain strings.
	line := string(payload)
	if len(line) > 0 && line[len(line)-1] == '\n' {
		line = line[:len(line)-1]
	}
	return line, nil
}

// readPktLines reads all pkt-lines until a flush packet and returns them.
func readPktLines(r io.Reader) ([]string, error) {
	var lines []string
	for {
		line, err := decodePktLine(r)
		if err != nil {
			return nil, err
		}
		// Empty string signals flush packet — end of section.
		if line == "" {
			break
		}
		lines = append(lines, line)
	}
	return lines, nil
}
