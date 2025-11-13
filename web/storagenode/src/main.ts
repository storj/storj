// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import { DirectiveBinding } from 'vue/types/options';
import { createPinia, PiniaVuePlugin } from 'pinia';

import App from '@/app/App.vue';
import { router } from '@/app/router';
import { store } from '@/app/store';

Vue.config.productionTip = false;

Vue.use(PiniaVuePlugin);
const pinia = createPinia();

let clickOutsideEvent: EventListener;

/**
 * Binds closing action to outside popups area.
 */
Vue.directive('click-outside', {
    bind: function (el: HTMLElement, binding: DirectiveBinding): void {
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

new Vue({
    router,
    store,
    pinia,
    render: (h) => h(App),
}).$mount('#app');
