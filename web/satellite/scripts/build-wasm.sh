#!/usr/bin/env bash

# Copy wasm helper file
LOCALGOROOT=$(GOTOOLCHAIN=local go env GOROOT)
if test -f "$LOCALGOROOT/lib/wasm/wasm_exec.js"; then
    cp "$LOCALGOROOT/lib/wasm/wasm_exec.js" ./static/wasm
else
    cp "$LOCALGOROOT/misc/wasm/wasm_exec.js" ./static/wasm
fi

# Take a hash of a wasm helper file
if command -v sha256sum > /dev/null; then
    helper_hash=$(sha256sum ./static/wasm/wasm_exec.js | awk '{print $1}')
else
    helper_hash=$(shasum -a 256 ./static/wasm/wasm_exec.js | awk '{print $1}')
fi

# Define new helper file name
helper_filename="wasm_exec.${helper_hash:0:8}.js"

# Rename helper file to include the hash
mv ./static/wasm/wasm_exec.js ./static/wasm/$helper_filename

# Compress wasm helper file using brotli
brotli -k -f ./static/wasm/$helper_filename

# Build wasm module
GOOS=js GOARCH=wasm go build -o ./static/wasm/access.wasm storj.io/storj/satellite/console/wasm

# Take a hash of generated wasm module
if command -v sha256sum > /dev/null; then
    module_hash=$(sha256sum ./static/wasm/access.wasm | awk '{print $1}')
else
    module_hash=$(shasum -a 256 ./static/wasm/access.wasm | awk '{print $1}')
fi

# Define new module file name
module_filename="access.${module_hash:0:8}.wasm"

# Rename module file to include the hash
mv ./static/wasm/access.wasm ./static/wasm/$module_filename

# Compress wasm module using brotli
brotli -k -f ./static/wasm/$module_filename

# Generate the manifest which would contain new helper and module file names
echo "{\"helperFileName\": \"$helper_filename\", \"moduleFileName\": \"$module_filename\"}" > ./static/wasm/wasm-manifest.json
