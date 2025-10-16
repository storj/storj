#!/usr/bin/env bash
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

# TODO: this should be done in parallel

# Walk through each subdirectory
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

    # Process each file in the folder
    for file in "$folder"/*; do
        if [ ! -f "$file" ]; then
            continue
        fi

        # Validate file is within the folder (prevent traversal)
        file_real="$(realpath "$file")" || continue
        if [[ "$file_real" != "$folder_real"/* ]]; then
            echo "Warning: Skipping file outside folder: $file"
            continue
        fi

        binary_name=$(basename "$file")

        # Remove .exe extension if it exists
        binary_clean="${binary_name%.exe}"
        # Replace .msi with _msi for the zip file name
        binary_clean="${binary_clean/.msi/_msi}"

        zip_name="${binary_clean}_${folder_name}.zip"
        zip_path="${BASE_DIR}/${zip_name}"

        echo "  Compressing: $binary_name -> $zip_name"

        # Create zip file containing the binary
        # -j: junk (don't record) directory names
        # -q: quiet mode
        zip -q -j "$zip_path" "${folder}/${binary_name}"
    done
done

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
