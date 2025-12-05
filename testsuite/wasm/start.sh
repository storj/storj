#!/usr/bin/env bash
# Copyright (C) 2022 Storj Labs, Inc.
# See LICENSE for copying information.
set -x
cd "$(dirname "${BASH_SOURCE[0]}")/../.."

mkdir -p .build
rm -rf .build/wasm || true

cp -r satellite/console/wasm/tests/ .build/
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
