package core

import (
	"fmt"

	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// ShowObject reads a stored object by hash and prints its raw payload
// to stdout. Intended for inspection/debugging rather than structured parsing.
func ShowObject(hash string) error {
	data, err := storage.ReadObject(hash)
	if err != nil {
		return err
	}

	// Output is printed verbatim; no object-type awareness or formatting is applied.
	fmt.Println(string(data))
	return nil
}
