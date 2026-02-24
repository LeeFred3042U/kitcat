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
	// Using Sprintf to handle generic slice printing.
	builder.WriteString(fmt.Sprintf("%v", d.Text))
	return builder.String()
}

// MyersDiff is the main struct for performing the diff operation.
// It holds the two sequences to be compared.
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

// DiffLines is an optimized helper for diffing slice of strings (lines).
// It maps strings to unique integer IDs (hashing) before running the Myers algorithm.
// This significantly speeds up comparisons for long lines or large files.
// Implements the optimization TODO: mapping lines/blocks to hashes.
func DiffLines(lines1, lines2 []string) []Diff[string] {
	// 1. Intern strings to IDs (Map string -> int)
	table := make(map[string]int)
	var reverseTable []string

	intern := func(s string) int {
		if id, ok := table[s]; ok {
			return id
		}
		id := len(reverseTable)
		table[s] = id
		reverseTable = append(reverseTable, s)
		return id
	}

	// Convert []string -> []int
	ids1 := make([]int, len(lines1))
	for i, s := range lines1 {
		ids1[i] = intern(s)
	}
	ids2 := make([]int, len(lines2))
	for i, s := range lines2 {
		ids2[i] = intern(s)
	}

	// 2. Run Myers Diff on Integers (Fast)
	md := NewMyersDiff(ids1, ids2)
	intDiffs := md.Diffs()

	// 3. Reconstruct []Diff[string] from []Diff[int]
	res := make([]Diff[string], len(intDiffs))
	for i, d := range intDiffs {
		res[i].Operation = d.Operation
		res[i].Text = make([]string, len(d.Text))
		for j, id := range d.Text {
			res[i].Text[j] = reverseTable[id]
		}
	}

	return res
}

// diffMain is the core function that orchestrates the diffing process.
// It trims common prefixes and suffixes before computing the diff on the
// remaining parts, which is a key optimization.
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

	// Compute the diff on the middle block.
	diffs := md.diffCompute(text1, text2)

	// Restore the prefix and suffix by prepending and appending
	// an EQUAL diff operation for the common parts.
	if len(commonPrefix) > 0 {
		diffs = append([]Diff[T]{{EQUAL, commonPrefix}}, diffs...)
	}
	if len(commonSuffix) > 0 {
		diffs = append(diffs, Diff[T]{EQUAL, commonSuffix})
	}

	return diffs
}

// diffCompute handles the main diff computation after stripping common parts.
// It contains fast-path checks for empty sequences.
func (md *MyersDiff[T]) diffCompute(text1, text2 []T) []Diff[T] {
	// If one of the texts is empty, the diff is a simple insertion or deletion.
	if len(text1) == 0 {
		if len(text2) == 0 {
			return []Diff[T]{}
		}
		return []Diff[T]{{INSERT, text2}}
	}
	if len(text2) == 0 {
		return []Diff[T]{{DELETE, text1}}
	}

	// The core of the algorithm for non-trivial cases.
	return md.diffBisect(text1, text2)
}

// diffBisect finds the 'middle snake' of a diff and recursively constructs the diff.
// This is an optimization of the Myers algorithm that reduces the problem space.
// See Myers' 1986 paper: "An O(ND) Difference Algorithm and Its Variations."
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
	// If the total number of characters is odd, the front path will collide with the reverse path.
	front := (delta%2 != 0)

	// The main loop of the bisection algorithm. It extends paths from both
	// the start and the end of the sequences, looking for an overlap.
	for d := 0; d < maxD; d++ {
		// Walk the front path one step.
		for k1 := -d; k1 <= d; k1 += 2 {
			k1Offset := vOffset + k1
			x1 := 0
			if k1 == -d || (k1 != d && v1[k1Offset-1] < v1[k1Offset+1]) {
				x1 = v1[k1Offset+1]
			} else {
				x1 = v1[k1Offset-1] + 1
			}
			y1 := x1 - k1
			// Continue along diagonals (matches).
			for x1 < text1Length && y1 < text2Length && text1[x1] == text2[y1] {
				x1++
				y1++
			}
			v1[k1Offset] = x1

			// Check for overlap with the reverse path.
			if front {
				k2Offset := vOffset + delta - k1
				if k2Offset >= 0 && k2Offset < vLength && v2[k2Offset] != -1 {
					x2 := text1Length - v2[k2Offset]
					if x1 >= x2 {
						// Overlap detected, split the problem.
						return md.diffBisectSplit(text1, text2, x1, y1)
					}
				}
			}
		}

		// Walk the reverse path one step.
		for k2 := -d; k2 <= d; k2 += 2 {
			k2Offset := vOffset + k2
			x2 := 0
			if k2 == -d || (k2 != d && v2[k2Offset-1] < v2[k2Offset+1]) {
				x2 = v2[k2Offset+1]
			} else {
				x2 = v2[k2Offset-1] + 1
			}
			y2 := x2 - k2
			// Continue along diagonals (matches) in reverse.
			for x2 < text1Length && y2 < text2Length && text1[text1Length-x2-1] == text2[text2Length-y2-1] {
				x2++
				y2++
			}
			v2[k2Offset] = x2

			// Check for overlap with the front path.
			if !front {
				k1Offset := vOffset + delta - k2
				if k1Offset >= 0 && k1Offset < vLength && v1[k1Offset] != -1 {
					x1 := v1[k1Offset]
					if x1 >= text1Length-x2 {
						// Overlap detected, split the problem.
						return md.diffBisectSplit(text1, text2, x1, vOffset+x1-k1Offset)
					}
				}
			}
		}
	}

	// Fallback for cases where no commonality is found.
	return []Diff[T]{{DELETE, text1}, {INSERT, text2}}
}

// diffBisectSplit is called when an overlap is found in diffBisect.
// It splits the problem at the overlap point and recursively calls diffMain
// on the two sub-problems.
func (md *MyersDiff[T]) diffBisectSplit(text1, text2 []T, x, y int) []Diff[T] {
	text1a := text1[:x]
	text2a := text2[:y]
	text1b := text1[x:]
	text2b := text2[y:]

	// Compute both diffs serially and concatenate them.
	diffs := md.diffMain(text1a, text2a)
	diffs = append(diffs, md.diffMain(text1b, text2b)...)
	return diffs
}

// diffCommonPrefix determines the common prefix of two sequences.
// It returns the number of elements common to the start of each sequence.
func (md *MyersDiff[T]) diffCommonPrefix(text1, text2 []T) int {
	n := min(len(text1), len(text2))
	for i := 0; i < n; i++ {
		if text1[i] != text2[i] {
			return i
		}
	}
	return n
}

// diffCommonSuffix determines the common suffix of two sequences.
// It returns the number of elements common to the end of each sequence.
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

// min is a helper function to find the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
