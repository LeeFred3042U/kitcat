package core

import (
	"github.com/LeeFred3042U/kitcat/internal/models"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

// Log returns the commit history starting from HEAD.
func Log() ([]models.Commit, error) {
	return storage.ReadCommits()
}
