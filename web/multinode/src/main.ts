// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Router from 'vue-router';
import Vuex from 'vuex';

import App from '@/app/App.vue';
import { router } from '@/app/router';
import { store } from '@/app/store';

Vue.config.productionTip = false;

Vue.use(Router);
Vue.use(Vuex);

const app = new Vue({
    router,
    store,
    render: (h) => h(App),
});

app.$mount('#app');
