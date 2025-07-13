#!/bin/bash
# Install dependencies and build for Linux

set -e

echo "Installing Go dependencies..."
go mod tidy

echo "Building webpcon..."
go build -o webpcon main.go

echo
echo "Done! You can run ./webpcon now."