#!/usr/bin/env bash

# Copy wasm javascript to match the go version
cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" ./static/wasm

# Build wasm code
GOOS=js GOARCH=wasm go build -o ./static/wasm/access.wasm storj.io/storj/satellite/console/wasm

# Take a hash of generated wasm code
if command -v sha256sum > /dev/null; then
    hash=$(sha256sum ./static/wasm/access.wasm | awk '{print $1}')
else
    hash=$(shasum -a 256 ./static/wasm/access.wasm | awk '{print $1}')
fi

# Define new file name
filename="access.${hash:0:8}.wasm"

# Rename the file to include the hash
mv ./static/wasm/access.wasm ./static/wasm/$filename

# Generate the manifest which would contain our new file name
echo "{\"fileName\": \"$filename\"}" > ./static/wasm/wasm-manifest.json
