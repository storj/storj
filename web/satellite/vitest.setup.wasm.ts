// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { execSync } from 'child_process';
import fs from 'fs';
import os from 'os';
import path from 'path';

import { beforeAll } from 'vitest';

const buildDir = fs.mkdtempSync(path.join(os.tmpdir(), 'storj-wasmtest-'));

beforeAll(async () => {
    // Build the wasm binary.
    execSync(
        'GOOS=js GOARCH=wasm go build -o ' +
      path.join(buildDir, 'main.wasm') +
      ' .',
        { cwd: path.resolve(__dirname, 'wasm'), stdio: 'inherit' },
    );

    // Copy wasm_exec.js from GOROOT.
    const goroot = execSync('go env GOROOT', { encoding: 'utf-8' }).trim();
    let wasmExecSrc = path.join(goroot, 'lib/wasm/wasm_exec.js');
    if (!fs.existsSync(wasmExecSrc)) {
        wasmExecSrc = path.join(goroot, 'misc/wasm/wasm_exec.js');
    }
    const wasmExecDst = path.join(buildDir, 'wasm_exec.js');
    fs.copyFileSync(wasmExecSrc, wasmExecDst);

    // Load the Go wasm helper. It registers `Go` on globalThis.
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    require(wasmExecDst);

    // Instantiate the wasm module.
    const go = new (
        globalThis as unknown as Record<string, unknown> & {
            Go: new () => {
                importObject: WebAssembly.Imports;
                run: (instance: WebAssembly.Instance) => void;
            };
        }
    ).Go();
    const buffer = fs.readFileSync(path.join(buildDir, 'main.wasm'));
    const result = await WebAssembly.instantiate(buffer, go.importObject);
    go.run(result.instance);
});
