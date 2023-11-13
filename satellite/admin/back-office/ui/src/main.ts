// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * main.ts
 *
 * Bootstraps Vuetify and other plugins then mounts the App`
 */

// Components
import { createApp } from 'vue';

import App from './App.vue';

// Plugins
import { registerPlugins } from '@/plugins';

const app = createApp(App);

registerPlugins(app);

app.mount('#app');
