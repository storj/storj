#!/usr/bin/env bash

# Copyright (C) 2025 Storj Labs, Inc.
# See LICENSE for copying information.

set -euo pipefail

# This script generates WASM resources for the satellite.
#
# It builds the WASM module and helper files to load the module.
# It adds hash as a suffix to the helper and module files for versioning.
# In addition it will compress the WASM module.

# Check if argument is provided
if [ $# -ne 1 ]; then
    echo "Usage: $0 <wasm_dir>" >&2
    exit 1
fi

WASM_DIR="$1"

# Ensure the directory exists.
mkdir -p "${WASM_DIR}"

# Get GOROOT
GOROOT="$(go env GOROOT)"
if [ -z "${GOROOT}" ]; then
    echo "Error: GOROOT not found" >&2
    exit 1
fi

cp "${GOROOT}/lib/wasm/wasm_exec.js" "${WASM_DIR}/"

# Add hash as a suffix.
helper_hash=$(sha256sum "${WASM_DIR}/wasm_exec.js" | awk '{print $1}')
helper_filename="wasm_exec.${helper_hash:0:8}.js"
mv "${WASM_DIR}/wasm_exec.js" "${WASM_DIR}/${helper_filename}"

# Build wasm module
echo "Building WASM module..."
GOOS=js GOARCH=wasm go build -o "${WASM_DIR}/access.wasm" storj.io/storj/satellite/console/wasm

# Add hash as a suffix.
module_hash=$(sha256sum "${WASM_DIR}/access.wasm" | awk '{print $1}')
module_filename="access.${module_hash:0:8}.wasm"
mv "${WASM_DIR}/access.wasm" "${WASM_DIR}/${module_filename}"

# Compress the files:
echo "Compressing files..."
brotli -k "${WASM_DIR}/${helper_filename}"
brotli -k "${WASM_DIR}/${module_filename}"

# Generate the manifest which would contain new helper and module file names
echo "Generating manifest..."
cat > "${WASM_DIR}/wasm-manifest.json" <<EOF
{
  "helperFileName": "${helper_filename}",
  "moduleFileName": "${module_filename}"
}
EOF

echo "WASM build complete. Output in: ${WASM_DIR}"
