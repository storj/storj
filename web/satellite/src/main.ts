// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import App from './App.vue';
import router from './router';
import store from './store';
import Analytics from './plugins/analytics';

Vue.config.devtools = true;
Vue.config.performance = true;
Vue.config.productionTip = false;

Vue.use(Analytics, {
    id: process.env.VUE_APP_SEGMENTID,
    router,
});

new Vue({
    router,
    store,
    render: (h) => h(App),
}).$mount('#app');
