// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * plugins/index.ts
 *
 * Automatically included in `./src/main.ts`
 */

// Plugins
// import { loadFonts } from './webfontloader'
import pinia from '../store';
import router from '../router';

import vuetify from './vuetify';

export function registerPlugins (app) {
    // loadFonts()
    app
        .use(vuetify)
        .use(router)
        .use(pinia);
}
