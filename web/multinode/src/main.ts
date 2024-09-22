// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue, { VNode } from 'vue';
import VueClipboard from 'vue-clipboard2';
import Router from 'vue-router';
import { DirectiveBinding } from 'vue/types/options';

import { vuetify } from '@/app/plugins';

import App from '@/app/App.vue';
import { router } from '@/app/router';
import { store } from '@/app/store';
import { Currency } from '@/app/utils/currency';
import { Percentage } from '@/app/utils/percentage';
import { Size } from '@/private/memory/size';

Vue.config.productionTip = false;

Vue.use(VueClipboard);

Vue.use(Router);

let clickOutsideEvent: EventListener;

/**
 * Binds closing action to outside popups area.
 */
Vue.directive('click-outside', {
    bind: function(el: HTMLElement, binding: DirectiveBinding, vnode: VNode) {
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
Vue.filter('centsToDollars', (cents: number): string => Currency.dollarsFromCents(cents));

/**
 * Converts bytes to base-10 size.
 */
Vue.filter('bytesToBase10String', (amountInBytes: number): string => Size.toBase10String(amountInBytes));

/**
 * Converts float number to percents.
 */
Vue.filter('floatToPercentage', (number: number): string => Percentage.fromFloat(number));

const app = new Vue({
    router,
    store,
    vuetify,
    render: (h) => h(App),
});

app.$mount('#app');
