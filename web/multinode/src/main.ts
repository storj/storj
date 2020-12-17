// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';

import App from '@/app/App.vue';

Vue.config.productionTip = false;

new Vue({
    // TODO: add router,
    render: (h) => h(App),
    // TODO: add store,
}).$mount('#app');
