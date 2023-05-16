// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { createApp } from 'vue';
import { createPinia, setActivePinia } from 'pinia';

import App from './App.vue';
import { router } from './router';

import NotificatorPlugin from '@/utils/plugins/notificator';

const pinia = createPinia();
setActivePinia(pinia);

const app = createApp(App);
app.config.performance = true;

app.use(NotificatorPlugin);
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

/**
 * Binds closing action to outside popups area.
 */
app.directive('click-outside', {
    mounted(el, binding) {
        const handler = event => {
            if (el !== event.target && !el.contains(event.target)) {
                binding.value(event);
            }
        };

        handlers.set(el, handler);
    },

    unmounted(el) {
        handlers.delete(el);
    },
});

/**
 * Number directive allow user to type only numbers in input.
 */
app.directive('number', {
    mounted (el: HTMLElement) {
        el.addEventListener('keydown', (e: KeyboardEvent) => {
            const keyCode = parseInt(e.key);

            if (!isNaN(keyCode) || e.key === 'Delete' || e.key === 'Backspace') {
                return;
            }

            e.preventDefault();
        });
    },
});

app.mount('#app');
