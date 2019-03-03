// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Router from 'vue-router';
import Search from '@/views/Search.vue';

Vue.use(Router);

let router = new Router({
    mode: 'history',
    routes: [
        {
            path: '',
            name: '',
            component: Search
        },
    ]
});

export default router;
