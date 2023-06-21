// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { createApp } from 'vue';

import App from './App.vue';

import { registerPlugins } from '@poc/plugins';

import './styles/settings.scss';

const app = createApp(App);
app.config.performance = true;

registerPlugins(app);

app.mount('#app');
