package core

import (
	"github.com/LeeFred3042U/kitcat/internal/models"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// Log returns the linear commit history starting from HEAD.
// Traversal and parsing are handled by storage.ReadCommits.
func Log() ([]models.Commit, error) {
	return storage.ReadCommits()
}
