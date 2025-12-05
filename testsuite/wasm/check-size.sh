#!/usr/bin/env bash
set -ueo pipefail
set +x

cleanup(){
    rm main.wasm
    echo "cleaned up test successfully"
}
trap cleanup EXIT

cd satellite/console/wasm && pwd && GOOS=js GOARCH=wasm go build -o main.wasm .
BUILD_SIZE=$(stat -c %s main.wasm)
CURRENT_SIZE=6100000
if [ $BUILD_SIZE -gt $CURRENT_SIZE ]; then
    echo "Wasm size is too big, was $CURRENT_SIZE but now it is $BUILD_SIZE"
    exit 1
fi

echo "Wasm size did not increase and it is $BUILD_SIZE (limit: $CURRENT_SIZE)"
