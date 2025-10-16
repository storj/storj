#!/usr/bin/env bash

# Copyright (C) 2025 Storj Labs, Inc.
# See LICENSE for copying information.

set -euo pipefail

if [ $# -ne 1 ]; then
    echo "Usage: $0 <folder>"
    echo "Signs all windows executables in folder with storj-sign"
    exit 1
fi

FOLDER="$1"

if [ ! -d "$FOLDER" ]; then
    echo "Error: Directory '$FOLDER' does not exist"
    exit 1
fi

# Validate FOLDER is an absolute path or convert it
FOLDER="$(cd "$FOLDER" && pwd)" || {
    echo "Error: Cannot resolve absolute path for '$1'"
    exit 1
}

# Sign all .exe files in the folder
for exefile in "$FOLDER"/*.exe; do
    # Skip if no exe files exist (glob didn't match)
    if [ ! -f "$exefile" ]; then
        echo "Did not find any .exe files in $FOLDER"
        continue
    fi

    # Validate file is within FOLDER (prevent traversal attacks via symlinks)
    exefile_real="$(realpath "$exefile")" || continue
    if [[ "$exefile_real" != "$FOLDER"/* ]]; then
        echo "Warning: Skipping file outside FOLDER: $exefile"
        continue
    fi

    echo " Signing $(basename "$exefile")"
    storj-sign "$exefile"
done

echo "All files signed"
