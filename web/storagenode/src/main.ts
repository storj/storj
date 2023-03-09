// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue, { VNode } from 'vue';
import VueClipboard from 'vue-clipboard2';
import { DirectiveBinding } from 'vue/types/options';

import App from '@/app/App.vue';
import { router } from '@/app/router';
import { store } from '@/app/store';
import { Size } from '@/private/memory/size';

Vue.config.productionTip = false;
VueClipboard.config.autoSetContainer = true;

Vue.use(VueClipboard);

let clickOutsideEvent: EventListener;

/**
 * Binds closing action to outside popups area.
 */
Vue.directive('click-outside', {
    bind: function (el: HTMLElement, binding: DirectiveBinding, vnode: VNode) {
        clickOutsideEvent = function(event: Event): void {
            if (el === event.target || el.contains(event.target as Node)) {
                return;
            }

            if (vnode.context && binding.expression) {
                vnode.context[binding.expression](event);
            }
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
    render: (h) => h(App),
    store,
}).$mount('#app');
