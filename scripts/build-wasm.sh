#!/usr/bin/env bash

set -eu
set -o pipefail

# Ensure the directory exists
mkdir -p release/$TAG/wasm/


# Copy wasm helper file
LOCALGOROOT=$(GOTOOLCHAIN=local go env GOROOT)
if test -f "$LOCALGOROOT/lib/wasm/wasm_exec.js"; then
	cp "$LOCALGOROOT/lib/wasm/wasm_exec.js" release/$TAG/wasm/
else
	cp "$LOCALGOROOT/misc/wasm/wasm_exec.js" release/$TAG/wasm/
fi

# Take a hash of a wasm helper file
helper_hash=$(sha256sum release/$TAG/wasm/wasm_exec.js | awk '{print $1}')

# Define new helper file name
helper_filename="wasm_exec.${helper_hash:0:8}.js"

# Rename helper file to include the hash
mv release/$TAG/wasm/wasm_exec.js release/$TAG/wasm/$helper_filename

# Compress wasm helper file using brotli
brotli -k release/$TAG/wasm/$helper_filename

# Build wasm module
GOOS=js GOARCH=wasm go build -o release/$TAG/wasm/access.wasm storj.io/storj/satellite/console/wasm

# Take a hash of generated wasm module
module_hash=$(sha256sum release/$TAG/wasm/access.wasm | awk '{print $1}')

# Define new module file name
module_filename="access.${module_hash:0:8}.wasm"

# Rename module file to include the hash
mv release/$TAG/wasm/access.wasm release/$TAG/wasm/$module_filename

# Compress wasm module using brotli
brotli -k release/$TAG/wasm/$module_filename

# Generate the manifest which would contain new helper and module file names
echo "{\"helperFileName\": \"$helper_filename\", \"moduleFileName\": \"$module_filename\"}" > release/$TAG/wasm/wasm-manifest.json
