// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue, { VNode } from 'vue';
import { DirectiveBinding } from 'vue/types/options';

import App from './App.vue';
import router from './router';
import store from './store';

Vue.config.devtools = true;
Vue.config.performance = true;
Vue.config.productionTip = false;

let clickOutsideEvent: EventListener;

Vue.directive('click-outside', {
    bind: function (el: HTMLElement, binding: DirectiveBinding, vnode: VNode) {
        clickOutsideEvent = function (event: Event) {
            console.log('entered', el, event);
            if (el === event.target) {
                console.log('event target');

                return;
            }

            if (vnode.context) {
                console.log('context');
                vnode.context[binding.expression](event);
            }
        };
        document.body.addEventListener('click', clickOutsideEvent);
    },
    unbind: function () {
        document.body.removeEventListener('click', clickOutsideEvent);
    },
});

new Vue({
    router,
    store,
    render: (h) => h(App),
}).$mount('#app');
