#!/bin/bash

# Stress test for concurrent kitcat add operations
# This script verifies that the index remains valid under concurrent writes

set -e

echo "=== Kitcat Concurrent Write Stress Test ==="
echo ""

# Resolve kitcat executable path (check local built binary first)
if [ -f "./kitcat.exe" ]; then
    KITCAT_BIN="$(pwd)/kitcat.exe"
elif [ -f "./kitcat" ]; then
    KITCAT_BIN="$(pwd)/kitcat"
else
    KITCAT_BIN="kitcat"
fi

echo "Using kitcat binary: $KITCAT_BIN"

# Create a temporary test directory
TEST_DIR=$(mktemp -d)
echo "Test directory: $TEST_DIR"
cd "$TEST_DIR"

# Initialize a kitcat repository
echo "Initializing kitcat repository..."
"$KITCAT_BIN" init
if [ $? -ne 0 ]; then
    echo "ERROR: Failed to initialize kitcat repository"
    exit 1
fi

# Create test files
echo "Creating test files..."
for i in {1..20}; do
    echo "Test content $i" > "file_$i.txt"
done

# Function to add a file
add_file() {
    local file=$1
    "$KITCAT_BIN" add "$file" 2>&1 | grep -v "^$" || true
}

# Export function for parallel execution
# Export function and binary path for parallel execution
export KITCAT_BIN
export -f add_file

# Run concurrent add operations
echo ""
echo "Running 20 concurrent 'kitcat add' operations..."
echo "This tests the file locking and atomic write mechanisms..."
echo ""

# Use GNU parallel if available, otherwise fall back to background jobs
if command -v parallel &> /dev/null; then
    parallel -j 10 add_file ::: file_*.txt
else
    # Fallback: use background jobs
    for file in file_*.txt; do
        add_file "$file" &
    done
    wait
fi

echo ""
echo "All operations completed. Verifying index integrity..."
echo ""

# Verify the index file exists
if [ ! -f ".kitcat/index" ]; then
    echo "ERROR: Index file does not exist!"
    exit 1
fi

# Verify the index is valid JSON
# Verify the index is valid JSON
if ! python -c "import json, sys; json.load(open('.kitcat/index'))" 2>/dev/null; then
    echo "ERROR: Index file is not valid JSON!"
    echo "Index contents:"
    cat .kitcat/index
    exit 1
fi

# Count entries in the index
INDEX_COUNT=$(python -c "import json; print(len(json.load(open('.kitcat/index'))))")
echo "Index contains $INDEX_COUNT entries"

# Verify all 20 files are in the index
EXPECTED_COUNT=20
if [ "$INDEX_COUNT" -ne "$EXPECTED_COUNT" ]; then
    echo "WARNING: Expected $EXPECTED_COUNT entries, but found $INDEX_COUNT"
    echo "This might indicate lost updates due to race conditions"
    echo ""
    echo "Index contents:"
    jq . .kitcat/index
    exit 1
fi

# Verify each file is in the index
echo "Verifying all files are tracked..."
MISSING_FILES=0
# Verify each file is in the index
echo "Verifying all files are tracked..."
MISSING_FILES=0
for i in {1..20}; do
    if ! python -c "import json, sys; sys.exit(0 if 'file_$i.txt' in json.load(open('.kitcat/index')) else 1)" 2>/dev/null; then
        echo "ERROR: file_$i.txt is missing from the index!"
        MISSING_FILES=$((MISSING_FILES + 1))
    fi
done

if [ $MISSING_FILES -gt 0 ]; then
    echo ""
    echo "ERROR: $MISSING_FILES files are missing from the index!"
    exit 1
fi

# Clean up
cd ..
rm -rf "$TEST_DIR"

echo ""
echo "✓ SUCCESS: All tests passed!"
echo "  - Index is valid JSON"
echo "  - All 20 files are tracked"
echo "  - No corruption detected"
echo ""
echo "The atomic write implementation is working correctly under concurrent load."
