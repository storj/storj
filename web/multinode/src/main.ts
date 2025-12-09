// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { DirectiveBinding, createApp } from 'vue';
import { createPinia, setActivePinia } from 'pinia';

import App from '@/app/App.vue';
import { vuetify } from '@/app/plugins';
import { router } from '@/app/router';

const pinia = createPinia();
setActivePinia(pinia);

const app = createApp(App);
app.use(pinia);
app.use(vuetify);
app.use(router);

/**
 * Click outside handlers.
 */
const handlers = new Map();
document.addEventListener('click', event => {
    for (const handler of handlers.values()) {
        handler(event);
    }
});

app.directive('click-outside', {
    mounted: function (el: HTMLElement, binding: DirectiveBinding): void {
        const handler = event => {
            if (el !== event.target && !el.contains(event.target)) {
                binding.value(event);
            }
        };

        handlers.set(el, handler);
    },
    unmounted: function(el: HTMLElement): void {
        handlers.delete(el);
    },
});

app.mount('#app');
