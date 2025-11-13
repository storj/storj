// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { createRouter, createWebHistory } from 'vue-router';

import { NavigationLink } from '@/app/types/navigation';

import DashboardArea from '@/app/views/DashboardArea.vue';
import NotificationsArea from '@/app/views/NotificationsArea.vue';
import PayoutArea from '@/app/views/PayoutArea.vue';
import Page404 from '@/app/components/errors/Page404.vue';

export abstract class RouteConfig {
    public static Root = new NavigationLink('', 'Root');
    public static Notifications = new NavigationLink('/notifications', 'Notifications');
    public static Payout = new NavigationLink('/payout-information', 'Payout');
}

/**
 * Router describes location mapping with components.
 */
export const router = createRouter({
    history: createWebHistory(import.meta.env.PROD ? '' : '/'),
    routes: [
        {
            path: RouteConfig.Root.path,
            name: RouteConfig.Root.name,
            component: DashboardArea,
        },
        {
            path: RouteConfig.Notifications.path,
            name: RouteConfig.Notifications.name,
            component: NotificationsArea,
        },
        {
            path: RouteConfig.Payout.path,
            name: RouteConfig.Payout.name,
            component: PayoutArea,
        },
        {
            path: '/:pathMatch(.*)*',
            name: '404',
            component: Page404,
        },
    ],
});
