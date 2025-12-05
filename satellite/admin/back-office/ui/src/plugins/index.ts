// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { createPinia, setActivePinia } from 'pinia';

import vuetify from './vuetify';

import NotificatorPlugin from '@/plugins/notificator';
import { setupRouter } from '@/router';

const pinia = createPinia();
setActivePinia(pinia);

export function registerPlugins (app) {
    app.use(vuetify).use(pinia).use(NotificatorPlugin);

    const router = setupRouter();
    app.use(router);
}
