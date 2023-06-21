// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { createPinia, setActivePinia } from 'pinia';
import { App } from 'vue';

import { router } from '../router';

import { loadFonts } from './webfontloader';
import vuetify from './vuetify';

const pinia = createPinia();
setActivePinia(pinia);

export function registerPlugins(app: App<Element>): void {
    loadFonts();
    app
        .use(vuetify)
        .use(router)
        .use(pinia);
}
