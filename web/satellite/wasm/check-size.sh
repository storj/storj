#!/usr/bin/env bash
set -ueo pipefail
set +x

TMPDIR=$(mktemp -d)
cleanup(){
    rm -rf "$TMPDIR"
    echo "cleaned up test successfully"
}
trap cleanup EXIT

cd "$(dirname "${BASH_SOURCE[0]}")"
GOOS=js GOARCH=wasm go build -o "$TMPDIR/main.wasm" .
BUILD_SIZE=$(stat -c %s "$TMPDIR/main.wasm")
CURRENT_SIZE=6200000
if [ $BUILD_SIZE -gt $CURRENT_SIZE ]; then
    echo "Wasm size is too big, was $CURRENT_SIZE but now it is $BUILD_SIZE"
    exit 1
fi

echo "Wasm size did not increase and it is $BUILD_SIZE (limit: $CURRENT_SIZE)"
