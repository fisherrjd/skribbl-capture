#!/bin/bash

# Build script for skribbl-capture
# Builds binaries for macOS and Windows

set -e  # Exit on error

echo "Building skribbl-capture..."

# Clean previous builds
rm -rf dist
mkdir -p dist

# Build for macOS (Intel)
# echo "Building for macOS (Intel)..."
# GOOS=darwin GOARCH=amd64 go build -o dist/skribbl-capture-macos-intel

# Build for macOS (Apple Silicon)
echo "Building for macOS (Apple Silicon)..."
GOOS=darwin GOARCH=arm64 go build -o dist/skribbl-capture-macos-arm64

# Build for Windows (64-bit)
echo "Building for Windows (64-bit)..."
GOOS=windows GOARCH=amd64 go build -o dist/skribbl-capture-windows-amd64.exe

# # Build for Windows (32-bit)
# echo "Building for Windows (32-bit)..."
# GOOS=windows GOARCH=386 go build -o dist/skribbl-capture-windows-386.exe

# # Build for Linux (64-bit) - bonus!
# echo "Building for Linux (64-bit)..."
# GOOS=linux GOARCH=amd64 go build -o dist/skribbl-capture-linux-amd64

echo ""
echo "✓ Build complete! Binaries are in ./dist/"
ls -lh dist/
