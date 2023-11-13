#!/usr/bin/env bash

set -eu
set -o pipefail

# Ensure the directory exists
mkdir -p release/$TAG/wasm/

# Copy wasm javascript to match the go version
cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" release/$TAG/wasm/

# Compress wasm javascript using brotli
brotli -k release/$TAG/wasm/wasm_exec.js

# Build wasm code
GOOS=js GOARCH=wasm go build -o release/$TAG/wasm/access.wasm storj.io/storj/satellite/console/wasm

# Take a hash of generated wasm code
hash=$(sha256sum release/$TAG/wasm/access.wasm | awk '{print $1}')

# Define new file name
filename="access.${hash:0:8}.wasm"

# Rename the file to include the hash
mv release/$TAG/wasm/access.wasm release/$TAG/wasm/$filename

# Compress wasm code using brotli
brotli -k release/$TAG/wasm/$filename

# Generate the manifest which would contain our new file name
echo "{\"fileName\": \"$filename\"}" > release/$TAG/wasm/wasm-manifest.json
