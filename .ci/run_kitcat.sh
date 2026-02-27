#!/usr/bin/bash
set -e

# go version
go version

echo "Testing kitcat..."

# Build kitcat binary
go build -o kitcat ./cmd/main.go

# Initialize empty repository for testing
mkdir -p test-repo
cd test-repo
../kitcat init

# Configure kitcat (config for this run)
../kitcat config --global user.name "testci"
../kitcat config --global user.email "testci@example.com"

echo "Kitcat setup completed "
