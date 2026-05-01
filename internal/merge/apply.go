package merge

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/LeeFred3042U/kitcat/internal/plumbing"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// ApplyMergePlan executes a previously computed MergePlan by mutating the
// working directory and updating the repository index.
//
// The operation is performed inside storage.UpdateIndex so that all index
// mutations occur under an exclusive index lock. This ensures that the
// workspace changes and index updates remain consistent.
//
// The plan is applied in three phases:
//
//  1. Deletions      – remove files from the workspace and index.
//  2. Clean updates  – write merged files with no conflicts.
//  3. Conflicts      – generate conflict markers and mark the index entry
//     as a conflict state.
func ApplyMergePlan(plan *MergePlan) error {
	return storage.UpdateIndex(func(index map[string]plumbing.IndexEntry) error {
		// 1. Process Deletions
		for _, path := range plan.Deletions {
			os.Remove(path)     // Remove from workspace
			delete(index, path) // Remove from index
		}

		// 2. Process Clean Updates
		for path, entry := range plan.CleanUpdates {
			content, err := storage.ReadObject(entry.Hash)
			if err != nil {
				return fmt.Errorf("failed to read object %s for path %s: %w", entry.Hash, path, err)
			}

			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}

			perm := os.FileMode(0o644)
			if entry.Mode&0o111 != 0 {
				perm = 0o755
			}
			if err := os.WriteFile(path, content, perm); err != nil {
				return err
			}

			hb, _ := storage.HexToHash(entry.Hash)
			index[path] = plumbing.IndexEntry{
				Path:  path,
				Hash:  hb,
				Mode:  entry.Mode, // Preserve source mode (exec bit, symlink, etc.)
				Stage: 0,          // Stage 0 = Clean/Resolved
			}
		}

		// 3. Process Conflicts (Using the Text Engine)
		for path, conflict := range plan.Conflicts {
			baseText := safeRead(conflict.BaseHash)
			oursText := safeRead(conflict.OursHash)
			theirsText := safeRead(conflict.TheirsHash)

			// Execute the text-level 3-way merge to generate conflict markers.
			mergedText, _ := Merge3(baseText, oursText, theirsText)

			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}

			if err := os.WriteFile(path, []byte(mergedText), 0o644); err != nil {
				return err
			}

			// The index is currently modeled as map[string]IndexEntry, so
			// multiple stage entries (1,2,3) for the same path cannot be
			// represented simultaneously. As a workaround, the "ours"
			// version is stored with Stage=2 to signal a conflicted entry.
			hb, _ := storage.HexToHash(conflict.OursHash)
			index[path] = plumbing.IndexEntry{
				Path:  path,
				Hash:  hb,
				Mode:  0o100644,
				Stage: 2, // Git stage 2 = "Ours"
			}
		}

		return nil
	})
}

// safeRead reads the object identified by the provided hash and returns
// its contents as a string.
//
// If the hash is empty or the object cannot be read, the function returns
// an empty string. This behavior is useful during merge operations where
// one side of the merge may represent a file addition or deletion.
func safeRead(hash string) string {
	if hash == "" {
		return ""
	}
	content, err := storage.ReadObject(hash)
	if err != nil {
		return ""
	}
	return string(content)
}
