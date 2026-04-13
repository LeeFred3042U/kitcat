package merge

import (
	"strings"

	"github.com/LeeFred3042U/kitcat/internal/diff"
)

// Edit represents a modification applied to a contiguous region of the
// base text during merge processing.
//
// BaseStart and BaseEnd define the affected range in the base text
// (half-open interval). Lines contains the replacement lines introduced
// by the edit.
type Edit struct {
	BaseStart int
	BaseEnd   int
	Lines     []string
}

// Merge3 performs a three-way text merge between base, ours, and theirs.
//
// The function operates on line-oriented text and attempts to combine
// modifications from both branches relative to the base version. When
// both branches modify the same region differently, standard conflict
// markers are inserted into the result.
//
// The returned boolean indicates whether a conflict occurred during
// the merge.
func Merge3(base, ours, theirs string) (string, bool) {
	// Fast paths for identical files.
	if ours == theirs {
		return ours, false
	}
	if base == ours {
		return theirs, false
	}
	if base == theirs {
		return ours, false
	}

	baseLines := strings.Split(base, "\n")
	oursLines := strings.Split(ours, "\n")
	theirsLines := strings.Split(theirs, "\n")

	// Compute line-based diffs against the base.
	oursDiffs := diff.DiffLines(baseLines, oursLines)
	theirsDiffs := diff.DiffLines(baseLines, theirsLines)

	oursEdits := diffToEdits(oursDiffs)
	theirsEdits := diffToEdits(theirsDiffs)

	var result []string
	baseIdx := 0
	conflict := false

	oIdx, tIdx := 0, 0

	// Walk through the base file and interleave/compare edits from both sides.
	for oIdx < len(oursEdits) || tIdx < len(theirsEdits) {
		var oEdit, tEdit *Edit
		if oIdx < len(oursEdits) {
			oEdit = &oursEdits[oIdx]
		}
		if tIdx < len(theirsEdits) {
			tEdit = &theirsEdits[tIdx]
		}

		// Determine the next base position where an edit begins.
		var nextBase int
		if oEdit != nil && tEdit != nil {
			nextBase = min(oEdit.BaseStart, tEdit.BaseStart)
		} else if oEdit != nil {
			nextBase = oEdit.BaseStart
		} else {
			nextBase = tEdit.BaseStart
		}

		// Copy unchanged base lines up to the next edit boundary.
		for baseIdx < nextBase {
			result = append(result, baseLines[baseIdx])
			baseIdx++
		}

		// Group overlapping edits from both sides into a single region.
		regionStart := nextBase
		regionEnd := nextBase
		var oGroup, tGroup []Edit

		for {
			expanded := false

			if oIdx < len(oursEdits) && oursEdits[oIdx].BaseStart <= regionEnd {
				oGroup = append(oGroup, oursEdits[oIdx])
				if oursEdits[oIdx].BaseEnd > regionEnd {
					regionEnd = oursEdits[oIdx].BaseEnd
				}
				oIdx++
				expanded = true
			}

			if tIdx < len(theirsEdits) && theirsEdits[tIdx].BaseStart <= regionEnd {
				tGroup = append(tGroup, theirsEdits[tIdx])
				if theirsEdits[tIdx].BaseEnd > regionEnd {
					regionEnd = theirsEdits[tIdx].BaseEnd
				}
				tIdx++
				expanded = true
			}

			if !expanded {
				break
			}
		}

		// Apply edits within the region.
		if len(oGroup) > 0 && len(tGroup) == 0 {
			for _, e := range oGroup {
				result = append(result, e.Lines...)
			}
		} else if len(tGroup) > 0 && len(oGroup) == 0 {
			for _, e := range tGroup {
				result = append(result, e.Lines...)
			}
		} else {
			// Both sides modified the same region.
			oStr := buildRegion(baseLines, regionStart, regionEnd, oGroup)
			tStr := buildRegion(baseLines, regionStart, regionEnd, tGroup)

			if slicesEqual(oStr, tStr) {
				// Identical edits — no conflict.
				result = append(result, oStr...)
			} else {
				// Diverging edits — emit conflict markers.
				conflict = true
				result = append(result, "<<<<<<< HEAD")
				result = append(result, oStr...)
				result = append(result, "=======")
				result = append(result, tStr...)
				result = append(result, ">>>>>>> MERGE_HEAD")
			}
		}

		baseIdx = regionEnd
	}

	// Append any remaining base lines after the final edit.
	for baseIdx < len(baseLines) {
		result = append(result, baseLines[baseIdx])
		baseIdx++
	}

	return strings.Join(result, "\n"), conflict
}

// diffToEdits converts a sequence of Myers diff operations into a list
// of Edit structures describing modifications relative to the base text.
func diffToEdits(diffs []diff.Diff[string]) []Edit {
	var edits []Edit
	baseIdx := 0
	var currentEdit *Edit

	for _, d := range diffs {
		switch d.Operation {
		case diff.EQUAL:
			if currentEdit != nil {
				edits = append(edits, *currentEdit)
				currentEdit = nil
			}
			baseIdx += len(d.Text)

		case diff.DELETE:
			if currentEdit == nil {
				currentEdit = &Edit{BaseStart: baseIdx, BaseEnd: baseIdx + len(d.Text)}
			} else {
				currentEdit.BaseEnd += len(d.Text)
			}
			baseIdx += len(d.Text)

		case diff.INSERT:
			if currentEdit == nil {
				currentEdit = &Edit{
					BaseStart: baseIdx,
					BaseEnd:   baseIdx,
					Lines:     append([]string{}, d.Text...),
				}
			} else {
				currentEdit.Lines = append(currentEdit.Lines, d.Text...)
			}
		}
	}

	if currentEdit != nil {
		edits = append(edits, *currentEdit)
	}

	return edits
}

// buildRegion reconstructs the merged representation of a specific base
// region after applying a sequence of edits.
func buildRegion(baseLines []string, start, end int, edits []Edit) []string {
	var res []string
	bIdx := start

	for _, e := range edits {
		for bIdx < e.BaseStart {
			res = append(res, baseLines[bIdx])
			bIdx++
		}

		res = append(res, e.Lines...)
		bIdx = e.BaseEnd
	}

	for bIdx < end {
		res = append(res, baseLines[bIdx])
		bIdx++
	}

	return res
}

// slicesEqual reports whether two string slices contain identical elements
// in the same order.
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

// min returns the smaller of the two provided integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
