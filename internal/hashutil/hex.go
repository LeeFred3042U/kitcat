package hashutil

import (
	"encoding/hex"
	"fmt"
)

// DecodeHex decodes a 40-character lowercase hex SHA-1 string into a [20]byte.
func DecodeHex(s string) ([20]byte, error) {
	var out [20]byte
	if len(s) != 40 {
		return out, fmt.Errorf("invalid SHA-1 hex length %d (want 40)", len(s))
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return out, err
	}
	copy(out[:], b)
	return out, nil
}
