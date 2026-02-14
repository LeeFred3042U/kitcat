// Package diff provides an implementation of the Myers diff algorithm.
// It is designed to find the shortest edit script (a sequence of insertions
// and deletions) to transform one sequence into another. This implementation
// is generic and can work with slices of any comparable type.
package diff

import (
	"fmt"
	"strings"
)

// Operation defines the type of diff operation.
type Operation int8

const (
	// EQUAL indicates that the text is the same in both sequences.
	EQUAL Operation = 0
	// INSERT indicates that the text was inserted in the new sequence.
	INSERT Operation = 1
	// DELETE indicates that the text was deleted from the old sequence.
	DELETE Operation = 2
)

// op2chr converts an Operation to its character representation for display.
func op2chr(op Operation) rune {
	switch op {
	case DELETE:
		return '-'
	case INSERT:
		return '+'
	case EQUAL:
		return '='
	default:
		return '?'
	}
}

// Diff represents a single diff operation, containing the type of operation
// and the sequence of elements it affects.
type Diff[T comparable] struct {
	Operation Operation
	Text      []T
}

// String returns a human-readable string representation of the Diff.
func (d Diff[T]) String() string {
	var builder strings.Builder
	builder.WriteRune(op2chr(d.Operation))
	builder.WriteRune('\t')
	builder.WriteString(fmt.Sprintf("%v", d.Text))
	return builder.String()
}

// MyersDiff holds the two sequences to be compared.
// Instances are immutable once created.
type MyersDiff[T comparable] struct {
	text1 []T
	text2 []T
}

// NewMyersDiff creates a new MyersDiff instance with the provided sequences.
func NewMyersDiff[T comparable](text1, text2 []T) *MyersDiff[T] {
	return &MyersDiff[T]{
		text1: text1,
		text2: text2,
	}
}

// Diffs computes and returns the differences between the two texts.
// This is the primary public method to get the diff result.
func (md *MyersDiff[T]) Diffs() []Diff[T] {
	return md.diffMain(md.text1, md.text2)
}

// diffMain orchestrates the diffing process.
// Common prefixes and suffixes are trimmed first to shrink the search space.
func (md *MyersDiff[T]) diffMain(text1, text2 []T) []Diff[T] {
	// Trim common prefix.
	commonLength := md.diffCommonPrefix(text1, text2)
	commonPrefix := text1[:commonLength]
	text1 = text1[commonLength:]
	text2 = text2[commonLength:]

	// Trim common suffix.
	commonLength = md.diffCommonSuffix(text1, text2)
	commonSuffix := text1[len(text1)-commonLength:]
	text1 = text1[:len(text1)-commonLength]
	text2 = text2[:len(text2)-commonLength]

	// Compute diff on reduced middle block.
	diffs := md.diffCompute(text1, text2)

	// Restore stripped equalities to maintain original structure.
	if len(commonPrefix) > 0 {
		diffs = append([]Diff[T]{{EQUAL, commonPrefix}}, diffs...)
	}
	if len(commonSuffix) > 0 {
		diffs = append(diffs, Diff[T]{EQUAL, commonSuffix})
	}

	return diffs
}

// diffCompute handles fast-path cases before invoking the full Myers algorithm.
func (md *MyersDiff[T]) diffCompute(text1, text2 []T) []Diff[T] {
	if len(text1) == 0 {
		if len(text2) == 0 {
			return []Diff[T]{}
		}
		return []Diff[T]{{INSERT, text2}}
	}
	if len(text2) == 0 {
		return []Diff[T]{{DELETE, text1}}
	}

	return md.diffBisect(text1, text2)
}

// diffBisect finds the "middle snake" using Myers’ O(ND) algorithm.
// It expands paths from both ends until overlap is detected.
func (md *MyersDiff[T]) diffBisect(text1, text2 []T) []Diff[T] {
	text1Length, text2Length := len(text1), len(text2)
	maxD := (text1Length + text2Length + 1) / 2
	vOffset := maxD
	vLength := 2*maxD + 1

	v1 := make([]int, vLength)
	v2 := make([]int, vLength)
	for i := range v1 {
		v1[i] = -1
		v2[i] = -1
	}
	v1[vOffset+1] = 0
	v2[vOffset+1] = 0

	delta := text1Length - text2Length
	front := (delta%2 != 0)

	for d := 0; d < maxD; d++ {
		// Forward search phase.
		for k1 := -d; k1 <= d; k1 += 2 {
			k1Offset := vOffset + k1
			x1 := 0
			if k1 == -d || (k1 != d && v1[k1Offset-1] < v1[k1Offset+1]) {
				x1 = v1[k1Offset+1]
			} else {
				x1 = v1[k1Offset-1] + 1
			}
			y1 := x1 - k1

			// Extend along diagonal while elements match.
			for x1 < text1Length && y1 < text2Length && text1[x1] == text2[y1] {
				x1++
				y1++
			}
			v1[k1Offset] = x1

			// Detect overlap with reverse search.
			if front {
				k2Offset := vOffset + delta - k1
				if k2Offset >= 0 && k2Offset < vLength && v2[k2Offset] != -1 {
					x2 := text1Length - v2[k2Offset]
					if x1 >= x2 {
						return md.diffBisectSplit(text1, text2, x1, y1)
					}
				}
			}
		}

		// Reverse search phase.
		for k2 := -d; k2 <= d; k2 += 2 {
			k2Offset := vOffset + k2
			x2 := 0
			if k2 == -d || (k2 != d && v2[k2Offset-1] < v2[k2Offset+1]) {
				x2 = v2[k2Offset+1]
			} else {
				x2 = v2[k2Offset-1] + 1
			}
			y2 := x2 - k2

			for x2 < text1Length && y2 < text2Length && text1[text1Length-x2-1] == text2[text2Length-y2-1] {
				x2++
				y2++
			}
			v2[k2Offset] = x2

			if !front {
				k1Offset := vOffset + delta - k2
				if k1Offset >= 0 && k1Offset < vLength && v1[k1Offset] != -1 {
					x1 := v1[k1Offset]
					if x1 >= text1Length-x2 {
						return md.diffBisectSplit(text1, text2, x1, vOffset+x1-k1Offset)
					}
				}
			}
		}
	}

	// No overlap detected: entire block differs.
	return []Diff[T]{{DELETE, text1}, {INSERT, text2}}
}

// diffBisectSplit divides the problem at the overlap point and recurses.
func (md *MyersDiff[T]) diffBisectSplit(text1, text2 []T, x, y int) []Diff[T] {
	text1a := text1[:x]
	text2a := text2[:y]
	text1b := text1[x:]
	text2b := text2[y:]

	diffs := md.diffMain(text1a, text2a)
	diffs = append(diffs, md.diffMain(text1b, text2b)...)
	return diffs
}

// diffCommonPrefix returns the length of the shared prefix.
func (md *MyersDiff[T]) diffCommonPrefix(text1, text2 []T) int {
	n := min(len(text1), len(text2))
	for i := 0; i < n; i++ {
		if text1[i] != text2[i] {
			return i
		}
	}
	return n
}

// diffCommonSuffix returns the length of the shared suffix.
func (md *MyersDiff[T]) diffCommonSuffix(text1, text2 []T) int {
	len1, len2 := len(text1), len(text2)
	n := min(len1, len2)
	for i := 1; i <= n; i++ {
		if text1[len1-i] != text2[len2-i] {
			return i - 1
		}
	}
	return n
}

// min returns the smaller integer.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TODO: Optimize diff by mapping lines/blocks to hashes (like Git does).
// Use a map[string]int for line -> ID mapping before running Myers
// For now, we run pure Myers on raw runes for simplicitygit
