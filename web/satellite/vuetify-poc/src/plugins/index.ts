// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * plugins/index.ts
 *
 * Automatically included in `./src/main.ts`
 */

// Plugins
import { createPinia, setActivePinia } from 'pinia';

import router from '../router';

import { loadFonts } from './webfontloader';
import vuetify from './vuetify';

const pinia = createPinia();
setActivePinia(pinia);

export function registerPlugins (app) {
    loadFonts();
    app
        .use(vuetify)
        .use(router)
        .use(pinia);
}
