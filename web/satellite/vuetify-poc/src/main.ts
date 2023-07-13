// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * main.ts
 *
 * Bootstraps Vuetify and other plugins then mounts the App
 */

// Components
import { createApp } from 'vue';

import App from './App.vue';

// Composables

// Styles
import './styles/settings.scss';
// import './styles/styles.scss'

// Plugins
import { registerPlugins } from '@poc/plugins';

const app = createApp(App);

registerPlugins(app);

app.mount('#app');
