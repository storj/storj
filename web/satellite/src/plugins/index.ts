// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { App } from 'vue';
import { createPinia, setActivePinia } from 'pinia';

import vuetify from './vuetify';

import { setupRouter } from '@/router';
import NotificatorPlugin from '@/plugins/notificator';

const pinia = createPinia();
setActivePinia(pinia);

export function registerPlugins(app: App<Element>): void {
    app.use(vuetify).use(pinia).use(NotificatorPlugin);

    const router = setupRouter();
    app.use(router);
}
