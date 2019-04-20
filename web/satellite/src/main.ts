// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import App from './App.vue';
import router from './router';
import store from './store';
import VueSegmentAnalytics from 'vue-segment-analytics';

Vue.config.productionTip = false;
declare module 'vue/types/vue' {
    interface Vue {
        $segment: any; // define real typings here if you want
    }
}

Vue.use(VueSegmentAnalytics, {
    id: process.env.VUE_APP_SEGMENTID,
    router,
});

new Vue({
    router,
    store,
    render: (h) => h(App),
}).$mount('#app');
