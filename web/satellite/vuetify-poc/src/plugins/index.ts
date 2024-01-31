// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { App, watch } from 'vue';
import { createPinia, setActivePinia } from 'pinia';
import THEME_URLS from 'virtual:vuetify-theme-css';

import { setupRouter } from '../router';

import vuetify from './vuetify';

import NotificatorPlugin from '@/utils/plugins/notificator';

const pinia = createPinia();
setActivePinia(pinia);

// Vuetify's way of applying themes uses a dynamic inline stylesheet.
// This is incompatible with our CSP policy, so circumvent it.
function setupTheme() {
    const oldAppend = document.head.appendChild.bind(document.head);
    document.head.appendChild = function<T extends Node>(node: T): T {
        if (node instanceof HTMLStyleElement && node.id === 'vuetify-theme-stylesheet') {
            node.remove();
            return node;
        }
        return oldAppend(node);
    };

    const themeLinks: Record<string, HTMLLinkElement> = {};

    for (const [name, url] of Object.entries(THEME_URLS)) {
        let link = document.createElement('link');
        link.rel = 'stylesheet';
        link.href = url;
        link.disabled = name !== vuetify.theme.global.name.value;
        document.head.appendChild(link);
        themeLinks[name] = link;

        // If we don't preload the style, there will be a delay after
        // toggling to it for the first time.
        link = document.createElement('link');
        link.rel = 'preload';
        link.as = 'style';
        link.href = url;
        document.head.appendChild(link);
    }

    watch(() => vuetify.theme.global.name.value, newName => {
        for (const [name, link] of Object.entries(themeLinks)) {
            link.disabled = name !== newName;
        }
    });
}

export function registerPlugins(app: App<Element>): void {
    setupTheme();
    app.use(vuetify).use(pinia).use(NotificatorPlugin);

    const router = setupRouter();
    app.use(router);
}
