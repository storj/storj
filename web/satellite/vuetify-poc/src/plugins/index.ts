// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * plugins/index.ts
 *
 * Automatically included in `./src/main.ts`
 */

// Plugins
import { App } from 'vue';
import { createPinia, setActivePinia } from 'pinia';

import router from '../router';

import { loadFonts } from './webfontloader';
import vuetify from './vuetify';

import NotificatorPlugin from '@/utils/plugins/notificator';

const pinia = createPinia();
setActivePinia(pinia);

export function registerPlugins(app: App<Element>) {
    loadFonts();
    app
        .use(vuetify)
        .use(router)
        .use(pinia)
        .use(NotificatorPlugin);
}
