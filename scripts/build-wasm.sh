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

# Compress wasm code using brotli
brotli -k release/$TAG/wasm/access.wasm
