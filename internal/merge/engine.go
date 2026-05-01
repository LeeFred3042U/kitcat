package merge

import (
	"fmt"

	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// Conflict represents a merge conflict for a single path.
//
// When both branches modify a file differently (or one deletes while the
// other modifies), automatic resolution cannot be performed. In that case
// the merge planner records the blob hashes for all three stages:
//
//   - Base   : common ancestor version
//   - Ours   : current branch version
//   - Theirs : merging branch version
//
// These hashes allow higher layers (merge application or UX) to perform
// a text merge or present the conflict to the user.
type Conflict struct {
	BaseHash   string
	OursHash   string
	TheirsHash string
}

// CleanEntry holds the blob hash and file mode for a clean (conflict-free)
// merge update. Mode is preserved so that executable bits and symlinks
// (0o100755, 0o120000) are not silently reset to 0o100644 across merges.
type CleanEntry struct {
	Hash string
	Mode uint32
}

// MergePlan is a pure data representation of the outcome of a 3-way merge
// between three trees.
//
// The structure contains only the minimal information required to apply
// the merge later. No filesystem operations are performed during planning.
type MergePlan struct {
	// CleanUpdates contains paths that merged without conflict and should
	// be updated to the specified blob hash and file mode.
	CleanUpdates map[string]CleanEntry

	// Conflicts contains paths that require manual resolution.
	Conflicts map[string]Conflict

	// Deletions lists paths that should be removed from the working tree
	// and index because both sides deleted the file or the merge logic
	// determined the deletion is the correct outcome.
	Deletions []string
}

// MergeTrees performs a 3-way merge between three tree snapshots and
// produces a MergePlan describing the result.
//
// The function is intentionally pure: it performs no disk I/O and does
// not mutate repository state. Instead it compares blob hashes for each
// path across the base, ours, and theirs trees and determines the correct
// merge action.
//
// The algorithm evaluates each path independently using simple hash
// comparisons to determine whether the file:
//
//   - remained unchanged
//   - changed only in one branch
//   - changed identically in both branches
//   - or changed differently and therefore conflicts
func MergeTrees(base, ours, theirs map[string]storage.TreeEntry) *MergePlan {
	plan := &MergePlan{
		CleanUpdates: make(map[string]CleanEntry),
		Conflicts:    make(map[string]Conflict),
		Deletions:    make([]string, 0),
	}

	// Gather the union of all paths across the three trees.
	allPaths := make(map[string]bool)
	for p := range base {
		allPaths[p] = true
	}
	for p := range ours {
		allPaths[p] = true
	}
	for p := range theirs {
		allPaths[p] = true
	}

	// parseMode converts an octal-string mode (e.g. "100755") to uint32.
	parseMode := func(s string) uint32 {
		var m uint32
		fmt.Sscanf(s, "%o", &m)
		if m == 0 {
			m = 0o100644
		}
		return m
	}

	// Evaluate each path independently.
	for path := range allPaths {
		baseEntry, inBase := base[path]
		oursEntry, inOurs := ours[path]
		theirsEntry, inTheirs := theirs[path]

		var bHash, oHash, tHash string
		if inBase {
			bHash = baseEntry.Hash
		}
		if inOurs {
			oHash = oursEntry.Hash
		}
		if inTheirs {
			tHash = theirsEntry.Hash
		}

		// Case 1: Unchanged in both branches relative to base.
		if oHash == bHash && tHash == bHash {
			if inBase {
				plan.CleanUpdates[path] = CleanEntry{Hash: bHash, Mode: parseMode(baseEntry.Mode)}
			}
			continue
		}

		// Case 2: Both branches made the exact same change
		// (or both deleted the file).
		if oHash == tHash {
			if inOurs {
				plan.CleanUpdates[path] = CleanEntry{Hash: oHash, Mode: parseMode(oursEntry.Mode)}
			} else {
				plan.Deletions = append(plan.Deletions, path)
			}
			continue
		}

		// Case 3: Only OURS changed the file.
		if tHash == bHash {
			if inOurs {
				plan.CleanUpdates[path] = CleanEntry{Hash: oHash, Mode: parseMode(oursEntry.Mode)}
			} else {
				plan.Deletions = append(plan.Deletions, path)
			}
			continue
		}

		// Case 4: Only THEIRS changed the file.
		if oHash == bHash {
			if inTheirs {
				plan.CleanUpdates[path] = CleanEntry{Hash: tHash, Mode: parseMode(theirsEntry.Mode)}
			} else {
				plan.Deletions = append(plan.Deletions, path)
			}
			continue
		}

		// Case 5: Both sides changed the file differently
		// (or one deleted while the other modified).
		plan.Conflicts[path] = Conflict{
			BaseHash:   bHash,
			OursHash:   oHash,
			TheirsHash: tHash,
		}
	}

	return plan
}
