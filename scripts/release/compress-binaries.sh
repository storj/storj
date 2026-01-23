#!/usr/bin/env bash

# Copyright (C) 2025 Storj Labs, Inc.
# See LICENSE for copying information.

set -euo pipefail

# Check if directory argument is provided
if [ $# -eq 0 ]; then
    echo "Usage: $0 <directory>"
    echo "Example: $0 ./release/v0.0.1"
    exit 1
fi

BASE_DIR="$1"

# Check if directory exists
if [ ! -d "$BASE_DIR" ]; then
    echo "Error: Directory '$BASE_DIR' does not exist"
    exit 1
fi

# Validate BASE_DIR is an absolute path or convert it
BASE_DIR="$(cd "$BASE_DIR" && pwd)" || {
    echo "Error: Cannot resolve absolute path for '$1'"
    exit 1
}

# Remove existing sha256sums and zip files if they exists
rm -f "$BASE_DIR/sha256sums"
rm -f "$BASE_DIR"/*.zip

echo "Processing binaries in: $BASE_DIR"

# Function to compress a single file
compress_file() {
    local file="$1"
    local folder="$2"
    local base_dir="$3"

    folder_real="$(realpath "$folder")" || return
    folder_name=$(basename "$folder")

    # Validate file is within the folder (prevent traversal)
    file_real="$(realpath "$file")" || return
    if [[ "$file_real" != "$folder_real"/* ]]; then
        echo "Warning: Skipping file outside folder: $file"
        return
    fi

    binary_name=$(basename "$file")

    # Remove .exe extension if it exists
    binary_clean="${binary_name%.exe}"
    # Replace .msi with _msi for the zip file name
    binary_clean="${binary_clean/.msi/_msi}"

    zip_name="${binary_clean}_${folder_name}.zip"
    zip_path="${base_dir}/${zip_name}"

    echo "  Compressing: $binary_name -> $zip_name"

    # Create zip file containing the binary
    # -j: junk (don't record) directory names
    # -q: quiet mode
    zip -q -j "$zip_path" "${folder}/${binary_name}"
}
export -f compress_file

# Collect all files to compress
files_to_compress=()
for folder in "$BASE_DIR"/*; do
    if [ ! -d "$folder" ]; then
        continue
    fi

    # Validate folder is within BASE_DIR (prevent symlink/traversal attacks)
    folder_real="$(realpath "$folder")" || continue
    if [[ "$folder_real" != "$BASE_DIR"/* ]]; then
        echo "Warning: Skipping folder outside BASE_DIR: $folder"
        continue
    fi

    folder_name=$(basename "$folder")
    echo "Processing folder: $folder_name"

    for file in "$folder"/*; do
        if [ ! -f "$file" ]; then
            continue
        fi
        files_to_compress+=("$file|$folder|$BASE_DIR")
    done
done

# Get number of CPU cores (cross-platform)
if command -v nproc &> /dev/null; then
    NPROC=$(nproc)
else
    NPROC=$(sysctl -n hw.ncpu)
fi

# Process files in parallel using xargs (parallel is not available on Mac by default)
printf '%s\n' "${files_to_compress[@]}" | xargs -P "$NPROC" -I {} bash -c 'IFS="|" read -r file folder base_dir <<< "{}"; compress_file "$file" "$folder" "$base_dir"'

# Generate SHA256 checksums for all zip files
echo "Generating SHA256 checksums..."
if compgen -G "$BASE_DIR"/*.zip > /dev/null; then
    (cd "$BASE_DIR" && sha256sum ./*.zip | sort -k 2 > sha256sums)
else
    echo "Warning: No zip files found in $BASE_DIR"
    touch "$BASE_DIR/sha256sums"
fi

echo "Done! Checksums saved to: $BASE_DIR/sha256sums"
echo ""
echo "Summary:"
zip_count=$(find "$BASE_DIR" -type f -name "*.zip" | wc -l)
echo "  Created $zip_count zip file(s)"
echo "  Checksum file: $BASE_DIR/sha256sums"
