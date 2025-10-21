// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { createApp } from 'vue';
import Papa from 'papaparse';
import PAPA_PARSE_WORKER_URL from 'virtual:papa-parse-worker';

import App from './App.vue';

import { registerPlugins } from '@/plugins';
const app = createApp(App);

registerPlugins(app);

app.mount('#app');

// By default, Papa Parse uses a blob URL for loading its worker.
// This isn't supported by our content security policy, so we override the URL.
// In dev mode, PAPA_PARSE_WORKER_URL will be null and Papa Parse will use its default blob URL.
if (PAPA_PARSE_WORKER_URL) {
    Object.assign(Papa, { BLOB_URL: PAPA_PARSE_WORKER_URL });
}
