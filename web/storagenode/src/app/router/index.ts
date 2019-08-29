// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Router from 'vue-router';
import Dashboard from '@/app/views/Dashboard.vue';

Vue.use(Router);

let router = new Router({
    mode: 'history',
    routes: [
        {
            path: '',
            name: '',
            component: Dashboard
        },
    ]
});

export default router;
