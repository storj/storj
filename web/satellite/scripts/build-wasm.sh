#!/usr/bin/env bash

# Copy wasm javascript to match the go version
cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" ./static/wasm

# Compress wasm javascript using brotli
brotli -k -f ./static/wasm/wasm_exec.js

# Build wasm code
GOOS=js GOARCH=wasm go build -o ./static/wasm/access.wasm storj.io/storj/satellite/console/wasm

# Compress wasm code using brotli
brotli -k -f ./static/wasm/access.wasm
