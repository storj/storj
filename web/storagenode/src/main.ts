// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import VueClipboard from 'vue-clipboard2';
import { DirectiveBinding } from 'vue/types/options';
import { createPinia, PiniaVuePlugin } from 'pinia';

import App from '@/app/App.vue';
import { router } from '@/app/router';
import { store } from '@/app/store';
import { Size } from '@/private/memory/size';

Vue.config.productionTip = false;
VueClipboard.config.autoSetContainer = true;

Vue.use(VueClipboard);
Vue.use(PiniaVuePlugin);
const pinia = createPinia();

let clickOutsideEvent: EventListener;

/**
 * Binds closing action to outside popups area.
 */
Vue.directive('click-outside', {
    bind: function (el: HTMLElement, binding: DirectiveBinding) {
        clickOutsideEvent = function(event: Event): void {
            if (el === event.target || el.contains(event.target as Node)) {
                return;
            }

            binding.value(event);
        };

        document.body.addEventListener('click', clickOutsideEvent);
    },
    unbind: function(): void {
        document.body.removeEventListener('click', clickOutsideEvent);
    },
});

/**
 * centsToDollars is a Vue filter that converts amount of cents in dollars string.
 */
Vue.filter('centsToDollars', (cents: number): string => {
    return `$${(cents / 100).toFixed(2)}`;
});

/**
 * Converts bytes to base-10 types.
 */
Vue.filter('bytesToBase10String', (amountInBytes: number): string => {
    return `${Size.toBase10String(amountInBytes)}`;
});

new Vue({
    router,
    store,
    pinia,
    render: (h) => h(App),
}).$mount('#app');
