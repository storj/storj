// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Router from 'vue-router';
import Dashboard from '@/app/views/Dashboard.vue';
import { NavigationLink } from '@/app/types/navigation';

Vue.use(Router);

export abstract class RouteConfig {
    public static Root = new NavigationLink('', 'Root');
}

const router = new Router({
    mode: 'history',
    routes: [
        {
            path: RouteConfig.Root.path,
            name: RouteConfig.Root.name,
            component: Dashboard
        },
    ]
});

export default router;
