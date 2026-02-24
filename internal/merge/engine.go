package merge

import (
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// Conflict represents a path where automatic resolution failed.
// It holds the blob hashes for the three stages so the UX layer can resolve them.
type Conflict struct {
	BaseHash   string
	OursHash   string
	TheirsHash string
}

// MergePlan is the pure data representation of a merge outcome.
type MergePlan struct {
	CleanUpdates map[string]string   // Paths that merged cleanly -> resulting blob hash
	Conflicts    map[string]Conflict // Paths requiring manual resolution
	Deletions    []string            // Paths to be safely removed
}

// MergeTrees orchestrates the 3-way tree traversal.
// It is a PURE function: it performs no disk IO and mutates no state.
func MergeTrees(base, ours, theirs map[string]storage.TreeEntry) *MergePlan {
	plan := &MergePlan{
		CleanUpdates: make(map[string]string),
		Conflicts:    make(map[string]Conflict),
		Deletions:    make([]string, 0),
	}

	// 1. Gather the union of all paths across the three trees
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

	// 2. Evaluate each path independently
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

		// Case 1: Unchanged in both branches (or missing in all, technically impossible here)
		if oHash == bHash && tHash == bHash {
			if inBase {
				plan.CleanUpdates[path] = bHash
			}
			continue
		}

		// Case 2: Both branches made the EXACT SAME change (or both deleted it)
		if oHash == tHash {
			if inOurs {
				plan.CleanUpdates[path] = oHash // Both modified/added identically
			} else {
				plan.Deletions = append(plan.Deletions, path) // Both deleted identically
			}
			continue
		}

		// Case 3: Only OURS changed it (Theirs left it as Base)
		if tHash == bHash {
			if inOurs {
				plan.CleanUpdates[path] = oHash // Modified or Added by Ours
			} else {
				plan.Deletions = append(plan.Deletions, path) // Deleted by Ours
			}
			continue
		}

		// Case 4: Only THEIRS changed it (Ours left it as Base)
		if oHash == bHash {
			if inTheirs {
				plan.CleanUpdates[path] = tHash // Modified or Added by Theirs
			} else {
				plan.Deletions = append(plan.Deletions, path) // Deleted by Theirs
			}
			continue
		}

		// Case 5: CONFLICT! Both changed it differently, or one modified and one deleted.
		plan.Conflicts[path] = Conflict{
			BaseHash:   bHash,
			OursHash:   oHash,
			TheirsHash: tHash,
		}
	}

	return plan
}
