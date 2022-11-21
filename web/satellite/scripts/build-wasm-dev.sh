#!/usr/bin/env bash

# Copy wasm javascript to match the go version
cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" ./static/wasm

# Build wasm code
GOOS=js GOARCH=wasm go build -o ./static/wasm/access.wasm storj.io/storj/satellite/console/wasm

# Allow execution of wasm_exec.js
chmod +x ./static/wasm/wasm_exec.js