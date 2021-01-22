// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information

require('./wasm_exec.js');

const fs = require('fs');
const path = require('path');

describe('main.wasm Tests', () => {
    beforeAll(async () => {
        const go = new Go();
        wasmPath = __dirname;
        if (process.env.WASM_PATH) {
            wasmPath = process.env.WASM_PATH;
        }
        wasmPath = path.resolve(wasmPath, 'main.wasm');
        const buffer = fs.readFileSync(wasmPath);
        await WebAssembly.instantiate(buffer, go.importObject).then(results => {
                go.run(results.instance);
        })
    })

    describe('generateAccessGrant function', () => {
        test('returns an error when called without arguments', async () => {
            const result = generateAccessGrant();
            expect(result["error"]).toContain("not enough argument")
            expect(result["value"]).toBeNull()
        });
        test('happy path returns an access grant', async () => {
            const apiKey = "13YqeGFpvtzbUp1QAfpvy2E5ZqLUFFNhEkv7153UDGDVnSmTuYYa7tKUnENGgvFXCCSFP7zNhKw6fsuQmWG5JGdQJbXVaVYFhoM2LcA"
            const projectID = "b9e663e0-69e0-48e9-8eb2-4079be85e488"
            const result = generateAccessGrant("a",apiKey, "supersecretpassphrase", projectID);
            expect(result["error"]).toEqual("")
            expect(result["value"]).toEqual("158UWUf6FHtCk8RGQn2JAXETNRnVwyoF7yEQQnuvPrLbsCPpttuAVWwzQ2YgD2bpQLpdBnctHssvQsqeju7kn7gz3LEJZSdRqyRG6rA9Da3PLGsawWMtM3NdGVqq9akyEmotsN7eMJVC1mfTsupiYXeHioTTTg11kY")
        });
    });

    describe('setAPIKeyPermission function', () => {
        test('returns an error when called without arguments', async () => {
            const result = setAPIKeyPermission();
            expect(result["error"]).toContain("not enough arguments")
            expect(result["value"]).toBeNull()
        });
        test('default permissions returns an access grant', async () => {
            const apiKey = "13YqeGFpvtzbUp1QAfpvy2E5ZqLUFFNhEkv7153UDGDVnSmTuYYa7tKUnENGgvFXCCSFP7zNhKw6fsuQmWG5JGdQJbXVaVYFhoM2LcA"
            const projectID = "b9e663e0-69e0-48e9-8eb2-4079be85e488"
            const perm = newPermission()
            perm["AllowDownload"] = true
            const result = setAPIKeyPermission(apiKey, [], perm);
            expect(result["error"]).toEqual("")
            expect(result["value"]).toEqual("19JjrwZJK1Ck5PdhRtxujGUnzbbiPYSSPZGyE8xrTbxVaJSEr9JL4vXpca3bSH2igjfeYsWeoe7rzo4QTGnwd29Pa924rtXzRjDzSxvkt4UdFd6iiCg")
        });
    });
});
