// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue, { VNode } from 'vue';
import Router from 'vue-router';
import { DirectiveBinding } from 'vue/types/options';
import { createPinia, PiniaVuePlugin } from 'pinia';

import App from '@/app/App.vue';
import { vuetify } from '@/app/plugins';
import { router } from '@/app/router';
import { store } from '@/app/store';

Vue.config.productionTip = false;

Vue.use(PiniaVuePlugin);
const pinia = createPinia();

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

const app = new Vue({
    router,
    store,
    pinia,
    vuetify,
    render: (h) => h(App),
});

app.$mount('#app');
