// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

importScripts('/static/static/wasm/wasm_exec.js')

if (!WebAssembly.instantiate) {
    self.postMessage(new Error('web assembly is not supported'));
}

const go = new Go();
const instantiateStreaming = WebAssembly.instantiateStreaming || async function (resp, importObject) {
    const response = await resp;
    const source = await response.arrayBuffer();

    return await WebAssembly.instantiate(source, importObject);
};
const response = fetch('/static/static/wasm/access.wasm');
instantiateStreaming(response, go.importObject).then(result => go.run(result.instance)).catch(err => self.postMessage(new Error(err.message)));

self.onmessage = function (event) {
    const type = event.data.type;
    switch (type) {
        case 'GenerateAccess':
            const result = self.generateAccessGrant();

            self.postMessage(result);
            break;
        default:
            self.postMessage(new Error('provided message event type is not supported'));
    }
};
