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
instantiateStreaming(response, go.importObject).then(result => {
    go.run(result.instance)
    self.postMessage('configured');
}).catch(err => self.postMessage(new Error(err.message)));

self.onmessage = function (event) {
    const data = event.data;
    let result;
    let apiKey;
    switch (data.type) {
        case 'GenerateAccess':
            apiKey = data.apiKey;
            const passphrase = data.passphrase;
            const projectID = data.projectID;
            const nodeURL = data.satelliteNodeURL;

            result = self.generateAccessGrant(nodeURL, apiKey, passphrase, projectID);

            self.postMessage(result);
            break;
        case 'SetPermission': // fallthrough
        case 'RestrictGrant':
            const isDownload = data.isDownload;
            const isUpload = data.isUpload;
            const isList = data.isList;
            const isDelete = data.isDelete;
            const notBefore = data.notBefore;
            const notAfter = data.notAfter;

            let permission = self.newPermission().value;

            permission.AllowDownload = isDownload;
            permission.AllowUpload = isUpload;
            permission.AllowDelete = isDelete;
            permission.AllowList = isList;
            permission.NotBefore = notBefore;
            permission.NotAfter = notAfter;

            if (data.type == "SetPermission") {
                const buckets = data.buckets;
                apiKey = data.apiKey;
                result = self.setAPIKeyPermission(apiKey, buckets, permission);
            } else {
                const paths = data.paths;
                const accessGrant = data.grant;
                result = self.restrictGrant(accessGrant, paths, permission);
            }

            self.postMessage(result);
            break;
        case 'Stop':
            self.close();
            break;
        default:
            self.postMessage(new Error('provided message event type is not supported'));
    }
};
