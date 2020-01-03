// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue, { VNode } from 'vue';
import { DirectiveBinding } from 'vue/types/options';

import { NotificatorPlugin } from '@/utils/plugins/notificator';
import { SegmentioPlugin } from '@/utils/plugins/segment';

import App from './App.vue';
import { router } from './router';
import { store } from './store';

Vue.config.devtools = true;
Vue.config.performance = true;
Vue.config.productionTip = false;

const notificator = new NotificatorPlugin();
const segment = new SegmentioPlugin();

Vue.use(notificator);
Vue.use(segment);

let clickOutsideEvent: EventListener;

Vue.directive('click-outside', {
    bind: function (el: HTMLElement, binding: DirectiveBinding, vnode: VNode) {
        clickOutsideEvent = function(event: Event): void {
            if (el === event.target) {
                return;
            }

            if (vnode.context) {
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
 * number directive allow user to type only numbers in input
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

/**
 * leadingZero adds zero to the start of single digit number
 */
Vue.filter('leadingZero', function (value: number): string {
    if (value <= 9) {
        return `0${value}`;
    }

    return `${value}`;
});

/**
 * centsToDollars is a Vue filter that converts amount of cents in dollars string.
 */
Vue.filter('centsToDollars', (cents: number): string => {
    return `USD $${(cents / 100).toFixed(2)}`;
});

new Vue({
    router,
    store,
    render: (h) => h(App),
}).$mount('#app');
