// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';

import { NotificatorPlugin } from '@/utils/plugins/notificator';

import App from './App.vue';
import router from './router';
import store from './store';

Vue.config.devtools = true;
Vue.config.performance = true;
Vue.config.productionTip = false;

const notificator = new NotificatorPlugin();

Vue.use(notificator);

new Vue({
    router,
    store,
    render: (h) => h(App),
}).$mount('#app');
