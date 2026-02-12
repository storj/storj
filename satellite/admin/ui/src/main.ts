// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { createApp } from 'vue';

import App from './App.vue';

import { registerPlugins } from '@/plugins';

const app = createApp(App);

registerPlugins(app);

app.mount('#app');
