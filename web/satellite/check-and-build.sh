#!/usr/bin/env bash
# Copyright (C) 2022 Storj Labs, Inc.
# See LICENSE for copying information.

cd "$(dirname "${BASH_SOURCE[0]}")"
set -euxo pipefail

npm install --prefer-offline --no-audit

# The steps below are independent, so run them concurrently. Output is buffered
# per-step and replayed in order afterwards, to keep the CI log readable.
log=$(mktemp -d)
trap 'rm -rf "$log"' EXIT

npm run build            >"$log/build"   2>&1 & build=$!
npm run lint-ci          >"$log/lint"    2>&1 & lint=$!
npm run test             >"$log/test"    2>&1 & test=$!
(npm run wasm-dev && npm run test-wasm) \
                         >"$log/wasm"    2>&1 & wasm=$!
npm audit                >"$log/audit"   2>&1 & audit=$!

set +e
wait $build; rc_build=$?
wait $lint;  rc_lint=$?
wait $test;  rc_test=$?
wait $wasm;  rc_wasm=$?
wait $audit
set -e

for f in build lint test wasm audit; do
    echo "=== $f ==="
    cat "$log/$f"
done

# ponytail: npm audit is advisory here, as it was before.
exit $((rc_build | rc_lint | rc_test | rc_wasm))
