#!/bin/bash
set -e

# Setup
PROJECT_ROOT=$(pwd)
COMMITS_LOG=".kitcat/commits.log"
mkdir -p .kitcat
rm -f "$COMMITS_LOG"

echo "Starting concurrent write test..."



cat > internal/storage/concurrent_test_runner_test.go <<EOF
package storage_test

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/LeeFred3042U/kitcat/internal/models"
	"github.com/LeeFred3042U/kitcat/internal/storage"
)

func TestConcurrentAppend(t *testing.T) {
	// Clean up previous runs regardless of where verify script ran
	_ = os.RemoveAll(".kitcat")
	
	commitsCount := 50
	var wg sync.WaitGroup
	wg.Add(commitsCount)

	for i := 0; i < commitsCount; i++ {
		go func(id int) {
			defer wg.Done()
			commit := models.Commit{
				ID:      fmt.Sprintf("commit-%d", id),
				Message: fmt.Sprintf("Message %d", id),
				AuthorName:  "Tester",
				Timestamp:   time.Now(),
			}
			if err := storage.AppendCommit(commit); err != nil {
				t.Errorf("Failed to append: %v", err)
			}
		}(i)
	}

	wg.Wait()

	// Verify
	commits, err := storage.ReadCommits()
	if err != nil {
		t.Fatalf("Failed to read commits: %v", err)
	}
	if len(commits) != commitsCount {
		t.Errorf("Expected %d commits, got %d", commitsCount, len(commits))
	}
}
EOF

echo "Running Go concurrent test..."
go test ./internal/storage -v -run TestConcurrentAppend
TEST_EXIT_CODE=$?

rm internal/storage/concurrent_test_runner_test.go

if [ $TEST_EXIT_CODE -eq 0 ]; then
    echo "Stress test PASSED"
else
    echo "Stress test FAILED"
    exit 1
fi
