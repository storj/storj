#!/usr/bin/env bash

# Copyright (C) 2025 Storj Labs, Inc.
# See LICENSE for copying information.

# This script checks that the release binaries are not compiled
# from a dirty git repository.

set -euo pipefail

# Check if directory argument is provided
if [ $# -eq 0 ]; then
    echo "Usage: $0 <directory>"
    echo "Example: $0 ./release/v1.0.0"
    exit 1
fi

BASE_DIR="$1"

# Check if directory exists
if [ ! -d "$BASE_DIR" ]; then
    echo "Error: Directory '$BASE_DIR' does not exist"
    exit 1
fi

echo "Checking release binaries in: $BASE_DIR"
echo ""

failed_binaries=()
checked_count=0

# Walk through each subdirectory
for folder in "$BASE_DIR"/*; do
    if [ ! -d "$folder" ]; then
        continue
    fi

    folder_name=$(basename "$folder")
    echo "Checking folder: $folder_name"

    # Process each file in the folder
    for file in "$folder"/*; do
        if [ ! -f "$file" ]; then
            continue
        fi

        # Skip non-binary files (zip files, msi files, etc.)
        case "$file" in
            *.zip|*.msi|*.sha256|sha256sums)
                continue
                ;;
        esac

        binary_name=$(basename "$file")
        checked_count=$((checked_count + 1))

        echo -n "  Checking: $binary_name ... "

        # Run go version -m and check for vcs.modified
        if ! version_output=$(go version -m "$file" 2>&1); then
            echo "SKIP (not a Go binary)"
            checked_count=$((checked_count - 1))
            continue
        fi

        # Check if vcs.modified exists in the output
        if ! echo "$version_output" | grep -q "vcs.modified"; then
            echo "WARN (no vcs.modified field)"
            continue
        fi

        # Check if vcs.modified=false
        if echo "$version_output" | grep -q "vcs.modified=false"; then
            echo "OK"
        else
            echo "FAIL (vcs.modified=true)"
            failed_binaries+=("$folder_name/$binary_name")
        fi
    done
done

echo ""
echo "================================"
echo "Summary:"
echo "  Total binaries checked: $checked_count"

if [ ${#failed_binaries[@]} -eq 0 ]; then
    echo "  Result: ✓ All binaries have vcs.modified=false"
    exit 0
else
    echo "  Result: ✗ ${#failed_binaries[@]} binary(ies) have vcs.modified=true"
    echo ""
    echo "Failed binaries:"
    for binary in "${failed_binaries[@]}"; do
        echo "  - $binary"
    done
    exit 1
fi
