// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * main.ts
 *
 * Bootstraps Vuetify and other plugins then mounts the App
 */
// Components
import { createApp } from 'vue';
import Papa from 'papaparse';
import PAPA_PARSE_WORKER_URL from 'virtual:papa-parse-worker';

import App from './App.vue';

// Plugins
import { registerPlugins } from '@poc/plugins';

const app = createApp(App);

registerPlugins(app).then(() => {
    const loader = document.getElementById('pre-app-loader');
    loader?.remove();

    app.mount('#app');
});

// By default, Papa Parse uses a blob URL for loading its worker.
// This isn't supported by our content security policy, so we override the URL.
Object.assign(Papa, { BLOB_URL: PAPA_PARSE_WORKER_URL });
