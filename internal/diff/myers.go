// Package diff provides an implementation of the Myers diff algorithm.
// It computes the shortest edit script (SES) that transforms one sequence
// into another using insertions and deletions. The implementation is generic
// and operates on slices of any comparable type.
package diff

import (
	"fmt"
	"strings"
)

// Operation identifies the type of diff operation produced by the algorithm.
//
// Each operation describes how a segment of the sequence changed relative
// to the original input.
type Operation int8

const (
	// EQUAL indicates that the elements are identical in both sequences.
	EQUAL Operation = 0

	// INSERT indicates elements present only in the new sequence.
	INSERT Operation = 1

	// DELETE indicates elements removed from the original sequence.
	DELETE Operation = 2
)

// op2chr converts an Operation into a single-character representation.
// This is primarily used when producing human-readable diff output.
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

// Diff represents a single diff operation produced by the algorithm.
//
// Operation describes the type of change, while Text contains the
// sequence elements affected by the operation.
type Diff[T comparable] struct {
	Operation Operation
	Text      []T
}

// String returns a human-readable representation of the diff operation.
// It prefixes the operation symbol followed by the affected sequence.
func (d Diff[T]) String() string {
	var builder strings.Builder
	builder.WriteRune(op2chr(d.Operation))
	builder.WriteRune('\t')
	// fmt.Sprintf is used to print generic slices.
	builder.WriteString(fmt.Sprintf("%v", d.Text))
	return builder.String()
}

// MyersDiff encapsulates the data required to perform a Myers diff
// between two sequences.
//
// The algorithm computes the shortest edit script (SES) that transforms
// text1 into text2.
type MyersDiff[T comparable] struct {
	text1 []T
	text2 []T
}

// NewMyersDiff constructs a new MyersDiff instance using the provided
// sequences as the original and modified inputs.
func NewMyersDiff[T comparable](text1, text2 []T) *MyersDiff[T] {
	return &MyersDiff[T]{
		text1: text1,
		text2: text2,
	}
}

// Diffs executes the Myers diff algorithm and returns the sequence of
// operations required to transform text1 into text2.
func (md *MyersDiff[T]) Diffs() []Diff[T] {
	return md.diffMain(md.text1, md.text2)
}

// DiffLines is a convenience helper for diffing slices of strings.
//
// To improve performance for large inputs, it interns strings into
// integer identifiers before running the Myers algorithm. The diff is
// then reconstructed back into string form after computation.
func DiffLines(lines1, lines2 []string) []Diff[string] {
	// Intern strings to integer IDs.
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

	// Run Myers diff on integer sequences.
	md := NewMyersDiff(ids1, ids2)
	intDiffs := md.Diffs()

	// Reconstruct string diffs from integer IDs.
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

// diffMain coordinates the diffing process.
//
// It trims common prefixes and suffixes before running the main algorithm
// on the remaining middle section. This optimization reduces the search
// space and improves performance.
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

	// Compute diff for the remaining middle block.
	diffs := md.diffCompute(text1, text2)

	// Restore trimmed prefix and suffix as EQUAL operations.
	if len(commonPrefix) > 0 {
		diffs = append([]Diff[T]{{EQUAL, commonPrefix}}, diffs...)
	}
	if len(commonSuffix) > 0 {
		diffs = append(diffs, Diff[T]{EQUAL, commonSuffix})
	}

	return diffs
}

// diffCompute performs the main diff computation after prefix/suffix
// trimming. It handles trivial cases before delegating to diffBisect.
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

// diffBisect implements Myers' divide-and-conquer algorithm to find the
// middle "snake" (longest diagonal match). The problem is split around
// this midpoint and solved recursively.
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

		// Forward search.
		for k1 := -d; k1 <= d; k1 += 2 {
			k1Offset := vOffset + k1

			var x1 int
			if k1 == -d || (k1 != d && v1[k1Offset-1] < v1[k1Offset+1]) {
				x1 = v1[k1Offset+1]
			} else {
				x1 = v1[k1Offset-1] + 1
			}

			y1 := x1 - k1

			for x1 < text1Length && y1 < text2Length && text1[x1] == text2[y1] {
				x1++
				y1++
			}

			v1[k1Offset] = x1

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

		// Reverse search.
		for k2 := -d; k2 <= d; k2 += 2 {
			k2Offset := vOffset + k2

			var x2 int
			if k2 == -d || (k2 != d && v2[k2Offset-1] < v2[k2Offset+1]) {
				x2 = v2[k2Offset+1]
			} else {
				x2 = v2[k2Offset-1] + 1
			}

			y2 := x2 - k2

			for x2 < text1Length && y2 < text2Length &&
				text1[text1Length-x2-1] == text2[text2Length-y2-1] {
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

	return []Diff[T]{{DELETE, text1}, {INSERT, text2}}
}

// diffBisectSplit divides the diff problem at the detected midpoint
// and recursively processes the two halves.
func (md *MyersDiff[T]) diffBisectSplit(text1, text2 []T, x, y int) []Diff[T] {
	text1a := text1[:x]
	text2a := text2[:y]
	text1b := text1[x:]
	text2b := text2[y:]

	diffs := md.diffMain(text1a, text2a)
	diffs = append(diffs, md.diffMain(text1b, text2b)...)
	return diffs
}

// diffCommonPrefix returns the length of the shared prefix between
// two sequences.
func (md *MyersDiff[T]) diffCommonPrefix(text1, text2 []T) int {
	n := min(len(text1), len(text2))
	for i := 0; i < n; i++ {
		if text1[i] != text2[i] {
			return i
		}
	}
	return n
}

// diffCommonSuffix returns the length of the shared suffix between
// two sequences.
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

// min returns the smaller of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
