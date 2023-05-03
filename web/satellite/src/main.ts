// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import { createPinia, setActivePinia, PiniaVuePlugin } from 'pinia';

import App from './App.vue';
import { router } from './router';

import { Size } from '@/utils/bytesSize';
import { NotificatorPlugin } from '@/utils/plugins/notificator';

window['VueNextTick'] = function(callback) {
    return Vue.nextTick(callback);
};

Vue.config.devtools = true;
Vue.config.performance = true;
Vue.config.productionTip = false;

Vue.use(new NotificatorPlugin());
Vue.use(PiniaVuePlugin);
const pinia = createPinia();
setActivePinia(pinia);

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
Vue.directive('click-outside', {
    bind(el, binding) {
        const handler = event => {
            if (el !== event.target && !el.contains(event.target)) {
                binding.value(event);
            }
        };

        handlers.set(el, handler);
    },

    unbind(el) {
        handlers.delete(el);
    },
});

/**
 * number directive allow user to type only numbers in input.
 */
Vue.directive('number', {
    bind (el: HTMLElement) {
        el.addEventListener('keydown', (e: KeyboardEvent) => {
            const keyCode = parseInt(e.key);

            if (!isNaN(keyCode) || e.key === 'Delete' || e.key === 'Backspace') {
                return;
            }

            e.preventDefault();
        });
    },
});

new Vue({
    router,
    pinia,
    render: (h) => h(App),
}).$mount('#app');
