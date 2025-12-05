// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { createApp, DirectiveBinding } from 'vue';
import { createPinia } from 'pinia';

import App from '@/app/App.vue';
import { router } from '@/app/router';

const pinia = createPinia();

const app = createApp(App);
app.use(pinia);
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
