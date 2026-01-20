#!/usr/bin/env bash
# Copyright (C) 2022 Storj Labs, Inc.
# See LICENSE for copying information.
set -x
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/../../.."

mkdir -p .build
rm -rf .build/wasm || true

echo "Running wasm tests at: $(pwd)"
cp -r web/satellite/wasm/tests/ .build/tests/
cd .build/tests/

# Copy wasm helper file
LOCALGOROOT=$(GOTOOLCHAIN=local go env GOROOT)
if test -f "$LOCALGOROOT/lib/wasm/wasm_exec.js"; then
    cp "$LOCALGOROOT/lib/wasm/wasm_exec.js" .
else
    cp "$LOCALGOROOT/misc/wasm/wasm_exec.js" .
fi

npm install
npm run test
