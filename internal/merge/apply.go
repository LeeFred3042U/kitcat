package merge

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/LeeFred3042U/kitcat/internal/plumbing"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// ApplyMergePlan executes a calculated merge plan.
// It mutates the workspace files and updates the index transactionally.
func ApplyMergePlan(plan *MergePlan) error {
	return storage.UpdateIndex(func(index map[string]plumbing.IndexEntry) error {

		// 1. Process Deletions
		for _, path := range plan.Deletions {
			os.Remove(path)     // Remove from workspace
			delete(index, path) // Remove from index
		}

		// 2. Process Clean Updates
		for path, hash := range plan.CleanUpdates {
			content, err := storage.ReadObject(hash)
			if err != nil {
				return fmt.Errorf("failed to read object %s for path %s: %w", hash, path, err)
			}

			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}

			if err := os.WriteFile(path, content, 0644); err != nil {
				return err
			}

			hb, _ := storage.HexToHash(hash)
			index[path] = plumbing.IndexEntry{
				Path:  path,
				Hash:  hb,
				Mode:  0100644, // Default standard file mode
				Stage: 0,       // Stage 0 = Clean/Resolved
			}
		}

		// 3. Process Conflicts (Using the Text Engine)
		for path, conflict := range plan.Conflicts {
			baseText := safeRead(conflict.BaseHash)
			oursText := safeRead(conflict.OursHash)
			theirsText := safeRead(conflict.TheirsHash)

			// Execute the text-level 3-way merge to generate <<<<<<< markers
			mergedText, _ := Merge3(baseText, oursText, theirsText)

			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}

			if err := os.WriteFile(path, []byte(mergedText), 0644); err != nil {
				return err
			}

			// Because kitcat's index is currently a `map[string]IndexEntry`, we can't 
			// store Stages 1, 2, and 3 under the same path key simultaneously.
			// As a robust workaround, we store the 'Ours' hash and mark it as Stage 2.
			// This signals to `status` and `commit` that the file is in a conflict state.
			hb, _ := storage.HexToHash(conflict.OursHash)
			index[path] = plumbing.IndexEntry{
				Path:  path,
				Hash:  hb,
				Mode:  0100644,
				Stage: 2, // Git Stage 2 = "Ours" (Conflicted)
			}
		}

		return nil
	})
}

// safeRead retrieves an object's content as a string, returning an empty 
// string if the hash is empty (e.g., if a file was newly added, not modified).
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
