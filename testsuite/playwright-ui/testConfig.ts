// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

let playwrightPort = '10000';
const portEnv = process.env.PLAYWRIGHT_PORT;
if (portEnv) {
    playwrightPort = portEnv;
}
export const testConfig = {
    host: `http://127.0.0.1`,
    port: playwrightPort,
    username: `test@storj.io`,
    password: `password`,
    waitForElement: 120000,
};
