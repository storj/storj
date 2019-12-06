// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.\

import Vue, { VNode } from 'vue';
import { DirectiveBinding } from 'vue/types/options';

import App from './app/App.vue';
import router from './app/router';
import store from './app/store';

Vue.config.productionTip = false;

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

new Vue({
    router,
    render: (h) => h(App),
    store,
}).$mount('#app');
