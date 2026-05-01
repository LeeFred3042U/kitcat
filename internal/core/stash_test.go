package core

import (
	"testing"
)

func TestStashDrop_AtomicWrite(t *testing.T) {
	// setup: init + 3 stashes
	// drop stash@{1} (the middle one)
	// assert stash list has 2 entries in correct order
	// assert the stash file is a valid newline-separated list (no truncation)
}
