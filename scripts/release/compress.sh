#!/usr/bin/env bash

# Copyright (C) 2025 Storj Labs, Inc.
# See LICENSE for copying information.

set -euo pipefail

if [ $# -ne 1 ]; then
    echo "Usage: $0 <folder>"
    echo "Compresses all files in the specified folder with zip and removes originals"
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

# Compress all regular files (not directories)
for filepath in "$FOLDER"/*; do
    # Skip if not a regular file
    if [ ! -f "$filepath" ]; then
        continue
    fi

    # Validate file is within FOLDER (prevent traversal attacks)
    filepath_real="$(realpath "$filepath")" || continue
    if [[ "$filepath_real" != "$FOLDER"/* ]]; then
        echo "Warning: Skipping file outside FOLDER: $filepath"
        continue
    fi

    # Skip if it's already a zip file
    if [[ "$filepath" == *.zip ]]; then
        continue
    fi

    # Extract just the filename
    filename=$(basename "$filepath")

    # Remove extension from filename for zip name (handles .exe and other extensions)
    zipname="${filename%.*}"

    echo "Compressing: $filename"

    # Create zip archive in the same folder with -j (junk paths) to store only the file
    zip -q -j "${FOLDER}/${zipname}.zip" "$filepath"

    # Remove the original file only if zip was successful
    if [ $? -eq 0 ]; then
        rm "$filepath"
        echo "  → Created ${zipname}.zip"
    else
        echo "  → Failed to compress $filename"
        exit 1
    fi
done

echo "Compression complete"
