package core

import (
	"github.com/LeeFred3042U/kitcat/internal/storage"
	"github.com/LeeFred3042U/kitcat/internal/models"
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
