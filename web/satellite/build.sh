#!/usr/bin/env bash
# Copyright (C) 2022 Storj Labs, Inc.
# See LICENSE for copying information.

cd "$(dirname "${BASH_SOURCE[0]}")"
set -euxo pipefail

npm install --prefer-offline --no-audit --logleve verbose
echo "module stub" > ./node_modules/go.mod # prevent Go from scanning this dir
npm run build

npm run lint-ci
npm audit || true
npm run test

: ${TAG:=dev}

# wasm is required to run
if [ ! -f "../../release/$TAG/wasm/access.wasm" ]; then
    echo "access.wasm is missing, but it is required for a working satellite web. Please execute TAG=$TAG ./scripts/build-wasm.sh first."
    exit 255
fi
cp ../../release/$TAG/wasm/* ./static/wasm/
