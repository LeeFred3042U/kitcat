// internal/storage/commitheap.go

package storage

import (
	"container/heap"

	"github.com/LeeFred3042U/kitcat/internal/models"
)

// commitHeap is a max-heap of Commit ordered by Timestamp (newest first).
type commitHeap []models.Commit

func (h commitHeap) Len() int { return len(h) }

func (h commitHeap) Less(i, j int) bool { return h[i].Timestamp.After(h[j].Timestamp) }

func (h commitHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

func (h *commitHeap) Push(x any) {
	*h = append(*h, x.(models.Commit))
}

func (h *commitHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

func ReadCommits() ([]models.Commit, error) {
	head, err := GetLastCommit()
	if err != nil {
		if err == ErrNoCommits {
			return nil, nil
		}
		return nil, err
	}

	seen := make(map[string]bool)
	h := &commitHeap{head}
	heap.Init(h)

	var result []models.Commit

	for h.Len() > 0 {
		curr := heap.Pop(h).(models.Commit)

		if seen[curr.ID] {
			continue
		}
		seen[curr.ID] = true
		result = append(result, curr)

		for _, parentID := range curr.Parents {
			if seen[parentID] {
				continue
			}
			parent, err := FindCommit(parentID)
			if err != nil {
				continue
			}
			heap.Push(h, parent)
		}
	}

	return result, nil
}
