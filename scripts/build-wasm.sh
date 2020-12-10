#!/usr/bin/env bash

set -eu
set -o pipefail

# Ensure the directory exists
mkdir -p release/$TAG/wasm/

# Copy wasm javascript to match the go version
cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" release/$TAG/wasm/

# Build wasm code
GOOS=js GOARCH=wasm exec go build -o release/$TAG/wasm/access.wasm satellite/console/wasm/main.go
