#!/usr/bin/env bash

LOCALGOROOT=$(GOTOOLCHAIN=local go env GOROOT)
if test -f "$LOCALGOROOT/lib/wasm/wasm_exec.js"; then
    cp "$LOCALGOROOT/lib/wasm/wasm_exec.js" .
else
    cp "$LOCALGOROOT/misc/wasm/wasm_exec.js" .
fi
