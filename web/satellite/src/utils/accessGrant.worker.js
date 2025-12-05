// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

if (!WebAssembly.instantiate) {
    self.postMessage({ error: new Error('Web assembly is not supported') });
}

async function setupWithCacheControl(mode) {
    // Determine the base path for WASM files based on environment.
    // Dev server runs on port 3000, production builds are served differently
    const wasmBasePath = self.location.port === '3000' ? '/wasm' : '/static/static/wasm';

    const manifestResp = await fetch(`${wasmBasePath}/wasm-manifest.json`, { cache: 'no-store' });
    if (!manifestResp.ok) {
        throw new Error('Failed to fetch wasm manifest.');
    }
    const manifest = await manifestResp.json();

    // eslint-disable-next-line no-undef
    importScripts(`${wasmBasePath}/${manifest.helperFileName}`);

    const go = new self.Go();

    const response = await fetch(`${wasmBasePath}/${manifest.moduleFileName}`, { cache: mode });
    if (!response.ok) {
        throw new Error('Failed to fetch wasm module.');
    }

    const buffer = await response.arrayBuffer();
    const module = await WebAssembly.compile(buffer);
    const instance = await WebAssembly.instantiate(module, go.importObject);

    go.run(instance);

    self.postMessage('configured');
}

self.onmessage = async function (event) {
    const data = event.data;
    let result;
    let apiKey;
    switch (data.type) {
    case 'Setup':
        try {
            await setupWithCacheControl('default');
        } catch {
            try {
                await setupWithCacheControl('reload');
            } catch (e) {
                self.postMessage({ error: new Error(e.message) });
            }
        }

        break;
    case 'DeriveAndEncryptRootKey':
        {
            const passphrase = data.passphrase;
            const projectID = data.projectID;
            const aesKey = data.aesKey;

            result = self.deriveAndAESEncryptRootKey(passphrase, projectID, aesKey);
            self.postMessage(result);
        }
        break;
    case 'GenerateAccess':
        {
            apiKey = data.apiKey;
            const passphrase = data.passphrase;
            const salt = data.salt;
            const nodeURL = data.satelliteNodeURL;
            const encryptPath = data.encryptPath;

            if (!encryptPath) {
                if (!self.generateNewAccessGrantWithPathEncryption) {
                    self.postMessage({ error: new Error('This page has an update, hard refresh for it to work correctly.') });
                    return;
                }
                result = self.generateNewAccessGrantWithPathEncryption(nodeURL, apiKey, passphrase, salt, encryptPath);
            } else {
                result = self.generateNewAccessGrant(nodeURL, apiKey, passphrase, salt);
            }
            self.postMessage(result);
        }
        break;
    case 'SetPermission': // fallthrough
    case 'RestrictGrant':
        {
            const isDownload = data.isDownload;
            const isUpload = data.isUpload;
            const isList = data.isList;
            const isDelete = data.isDelete;
            const isPutObjectRetention = data.isPutObjectRetention ?? false;
            const isGetObjectRetention = data.isGetObjectRetention ?? false;
            const isBypassGovernanceRetention = data.isBypassGovernanceRetention ?? false;
            const isPutObjectLegalHold = data.isPutObjectLegalHold ?? false;
            const isGetObjectLegalHold = data.isGetObjectLegalHold ?? false;
            const isPutObjectLockConfiguration = data.isPutObjectLockConfiguration ?? false;
            const isGetObjectLockConfiguration = data.isGetObjectLockConfiguration ?? false;
            const notBefore = data.notBefore;
            const notAfter = data.notAfter;

            const permission = self.newPermission().value;

            permission.AllowDownload = isDownload;
            permission.AllowUpload = isUpload;
            permission.AllowDelete = isDelete;
            permission.AllowList = isList;
            permission.AllowPutObjectRetention = isPutObjectRetention;
            permission.AllowGetObjectRetention = isGetObjectRetention;
            permission.AllowBypassGovernanceRetention = isBypassGovernanceRetention;
            permission.AllowPutObjectLegalHold = isPutObjectLegalHold;
            permission.AllowGetObjectLegalHold = isGetObjectLegalHold;
            permission.AllowPutBucketObjectLockConfiguration = isPutObjectLockConfiguration;
            permission.AllowGetBucketObjectLockConfiguration = isGetObjectLockConfiguration;

            if (notBefore) permission.NotBefore = notBefore;
            if (notAfter) permission.NotAfter = notAfter;

            if (data.type === 'SetPermission') {
                const buckets = JSON.parse(data.buckets);
                apiKey = data.apiKey;
                result = self.setAPIKeyPermission(apiKey, buckets, permission);
            } else {
                const paths = data.paths;
                const accessGrant = data.grant;
                result = self.restrictGrant(accessGrant, paths, permission);
            }

            self.postMessage(result);
        }
        break;
    default:
        self.postMessage({ error: new Error('provided message event type is not supported') });
    }
};
