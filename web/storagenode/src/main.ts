// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.\

import Vue from 'vue';

import App from './app/App.vue';
import router from './app/router';
import store from './app/store';

Vue.config.productionTip = false;

new Vue({
    router,
    render: (h) => h(App),
    store,
}).$mount('#app');
