// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Router from 'vue-router';

import App from '@/app/App.vue';
import { router } from '@/app/router';
import { store } from '@/app/store';
import { Currency } from '@/app/utils/currency';
import { Size } from '@/app/utils/size';

Vue.config.productionTip = false;

Vue.use(Router);

/**
 * centsToDollars is a Vue filter that converts amount of cents in dollars string.
 */
Vue.filter('centsToDollars', (cents: number): string => {
    return Currency.dollarsFromCents(cents);
});

/**
 * Converts bytes to base-10 size.
 */
Vue.filter('bytesToBase10String', (amountInBytes: number): string => {
    return Size.toBase10String(amountInBytes);
});

const app = new Vue({
    router,
    store,
    render: (h) => h(App),
});

app.$mount('#app');
