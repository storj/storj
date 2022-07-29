#!/usr/bin/env bash

set -euxo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"

: ${OUTPUT_DIR:=../../.build}
mkdir -p $OUTPUT_DIR || true

go test -parallel 1 -p 1 -short -vet=off -timeout 5m -json -race ./... 2>&1 | tee $OUTPUT_DIR/ui-tests.json 
cat $OUTPUT_DIR/ui-tests.json | xunit -out $OUTPUT_DIR/ui-tests.xml
