#!/usr/bin/env bash
# Copyright (C) 2022 Storj Labs, Inc.
# See LICENSE for copying information.
set -x
cd "$(dirname "${BASH_SOURCE[0]}")/../.."

mkdir -p .build
rm -rf .build/wasm || true

cp -r satellite/console/wasm/tests/ .build/
cd .build/tests/
cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" .

npm install
npm run test
