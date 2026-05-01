package core

import (
	"github.com/LeeFred3042U/kitcat/internal/models"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

func collectCommitsForRebase(head, base string) ([]models.Commit, error) {
	visited := make(map[string]bool)
	var result []models.Commit

	var dfs func(string) error
	dfs = func(curr string) error {
		if curr == "" || curr == base || visited[curr] {
			return nil
		}
		visited[curr] = true

		c, err := storage.FindCommit(curr)
		if err != nil {
			return err
		}

		// Iterating with range over c.Parents is safe when the slice is
		// empty (root commit). An earlier implementation accessed c.Parents[0]
		// unconditionally which panicked on root commits that were not equal
		// to mergeBase (M6). The range loop terminates naturally at root.
		for _, p := range c.Parents {
			if err := dfs(p); err != nil {
				return err
			}
		}

		result = append(result, c) // post-order → correct replay order
		return nil
	}

	if err := dfs(head); err != nil {
		return nil, err
	}

	return result, nil
}
