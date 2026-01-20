#!/usr/bin/env bash

# Copyright (C) 2025 Storj Labs, Inc.
# See LICENSE for copying information.

set -euo pipefail

# This script generates WASM resources for the satellite.
#
# It builds the WASM module and helper files to load the module.
# It adds hash as a suffix to the helper and module files for versioning.
# Optionally compresses the files with brotli.
#
# Usage:
#   build-wasm.sh <output-dir> [--compress]
#
# Examples:
#   build-wasm.sh /out/wasm --compress   # Docker build (with brotli)
#   build-wasm.sh static/wasm --compress # npm run wasm (production)
#   build-wasm.sh static/wasm            # npm run wasm-dev (no compression)

COMPRESS=false
OUTPUT_DIR=""

# Parse arguments
for arg in "$@"; do
    case $arg in
        --compress)
            COMPRESS=true
            ;;
        *)
            if [ -z "$OUTPUT_DIR" ]; then
                OUTPUT_DIR="$arg"
            else
                echo "Error: unexpected argument: $arg" >&2
                echo "Usage: $0 <output-dir> [--compress]" >&2
                exit 1
            fi
            ;;
    esac
done

if [ -z "$OUTPUT_DIR" ]; then
    echo "Usage: $0 <output-dir> [--compress]" >&2
    exit 1
fi

# Ensure the output directory exists
mkdir -p "${OUTPUT_DIR}"

# Convert to absolute path
OUTPUT_DIR_ABS="$(cd "${OUTPUT_DIR}" && pwd)"

# Get GOROOT
GOROOT="$(go env GOROOT)"
if [ -z "${GOROOT}" ]; then
    echo "Error: GOROOT not found" >&2
    exit 1
fi

# Copy wasm helper file
cp "${GOROOT}/lib/wasm/wasm_exec.js" "${OUTPUT_DIR_ABS}/"

# Helper function for sha256
sha256() {
    if command -v sha256sum > /dev/null; then
        sha256sum "$1" | awk '{print $1}'
    else
        shasum -a 256 "$1" | awk '{print $1}'
    fi
}

# Add hash suffix to helper file
helper_hash=$(sha256 "${OUTPUT_DIR_ABS}/wasm_exec.js")
helper_filename="wasm_exec.${helper_hash:0:8}.js"
mv "${OUTPUT_DIR_ABS}/wasm_exec.js" "${OUTPUT_DIR_ABS}/${helper_filename}"

# Build wasm module from this directory
echo "Building WASM module..."
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GOOS=js GOARCH=wasm go build -C "${SCRIPT_DIR}" -o "${OUTPUT_DIR_ABS}/access.wasm" .

# Add hash suffix to module file
module_hash=$(sha256 "${OUTPUT_DIR_ABS}/access.wasm")
module_filename="access.${module_hash:0:8}.wasm"
mv "${OUTPUT_DIR_ABS}/access.wasm" "${OUTPUT_DIR_ABS}/${module_filename}"

# Compress files if requested
if [ "$COMPRESS" = true ]; then
    echo "Compressing files..."
    brotli -k -f "${OUTPUT_DIR_ABS}/${helper_filename}"
    brotli -k -f "${OUTPUT_DIR_ABS}/${module_filename}"
fi

# Generate manifest
echo "Generating manifest..."
cat > "${OUTPUT_DIR_ABS}/wasm-manifest.json" <<EOF
{
  "helperFileName": "${helper_filename}",
  "moduleFileName": "${module_filename}"
}
EOF

echo "WASM build complete. Output in: ${OUTPUT_DIR_ABS}"
