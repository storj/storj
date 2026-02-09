#!/usr/bin/env bash
# Copyright (C) 2022 Storj Labs, Inc.
# See LICENSE for copying information.

cd "$(dirname "${BASH_SOURCE[0]}")"
set -euxo pipefail

npm install --prefer-offline --no-audit --logleve verbose
npm run build
npm run wasm-dev

npm run lint-ci
npm audit || true
npm run test
npm run test-wasm
