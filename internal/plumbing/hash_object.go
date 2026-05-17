package plumbing

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
)

// HashAndWriteObject constructs a Git-compatible object, computes its
// content-addressed SHA-1 identifier, and persists the object into the
// repository object database.
//
// The object layout follows the canonical Git format:
//
//	"<type> <size>\0<payload>"
//
// The header is included in the hash calculation, ensuring compatibility
// with Git’s object model and guaranteeing identical hashes for identical
// objects.
//
// Objects are stored using a fan-out directory structure:
//
//	.kitcat/objects/<first-two-hex>/<remaining-hash>
//
// This layout prevents excessive numbers of files from accumulating in a
// single directory and mirrors Git’s object storage strategy.
//
// If the object already exists in the object database, the write is skipped
// because objects are immutable. The function still returns the computed
// object hash.
func HashAndWriteObject(content []byte, objType string) (string, error) {
	// Object header is part of the hash input; omitting it would produce
	// incompatible object IDs compared to Git.
	header := fmt.Sprintf("%s %d\x00", objType, len(content))
	store := append([]byte(header), content...)

	// Hash is computed over the full header+payload buffer.
	sum := sha1.Sum(store)
	hash := fmt.Sprintf("%x", sum)

	// Fan-out directory structure avoids too many files in a single directory.
	dir := filepath.Join(".kitcat/objects", hash[:2])
	path := filepath.Join(dir, hash[2:])

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	// Objects are immutable; skip write if object already exists.
	if _, err := os.Stat(path); os.IsNotExist(err) {
		var buf bytes.Buffer
		w := zlib.NewWriter(&buf)

		// Objects are stored compressed to reduce disk usage
		// and match Git's storage format.
		if _, err := w.Write(store); err != nil {
			return "", err
		}
		w.Close()

		// Write atomically via temp-file + rename. A direct os.WriteFile leaves a
		// partial (corrupt) object if the process crashes mid-write; the existence
		// check above would then skip re-writing it permanently.
		//
		// NOTE: storage.atomicio.WriteFileFile implements the same pattern but importing
		// the storage package from plumbing would create an import cycle
		// (storage → plumbing → storage). The equivalent logic is therefore
		// duplicated here. If this package is ever restructured into a separate
		// ioutil layer, replace this block with a call to that helper instead.
		tmp, err := os.CreateTemp(dir, "obj-*.tmp")
		if err != nil {
			return "", err
		}
		tmpPath := tmp.Name()

		_, writeErr := tmp.Write(buf.Bytes())
		syncErr := tmp.Sync()
		closeErr := tmp.Close()

		if writeErr != nil || syncErr != nil || closeErr != nil {
			os.Remove(tmpPath)
			if writeErr != nil {
				return "", writeErr
			}
			if syncErr != nil {
				return "", syncErr
			}
			return "", closeErr
		}

		if err := os.Rename(tmpPath, path); err != nil {
			os.Remove(tmpPath)
			return "", err
		}
	}

	return hash, nil
}
