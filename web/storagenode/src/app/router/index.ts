// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Router from 'vue-router';

import { NavigationLink } from '@/app/types/navigation';
import DashboardArea from '@/app/views/DashboardArea.vue';
import NotificationsArea from '@/app/views/NotificationsArea.vue';

Vue.use(Router);

export abstract class RouteConfig {
    public static Root = new NavigationLink('', 'Root');
    public static Notifications = new NavigationLink('/notifications', 'Notifications');
}

const router = new Router({
    mode: 'history',
    routes: [
        {
            path: RouteConfig.Root.path,
            name: RouteConfig.Root.name,
            component: DashboardArea
        },
        {
            path: RouteConfig.Notifications.path,
            name: RouteConfig.Notifications.name,
            component: NotificationsArea
        },
    ]
});

export default router;
