// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { createRouter, createWebHistory } from 'vue-router';

const routes = [
    {
        path: '/vuetifypoc',
        redirect: { path: '/dashboard' },
        component: () => import('@poc/layouts/Default.vue'),
        children: [
            {
                path: '/dashboard',
                name: 'Dashboard',
                component: () => import('@poc/views/Dashboard.vue'),
            },
            {
                path: '/team',
                name: 'Team',
                component: () => import('@poc/views/Team.vue'),
            },
        ],
    },
];

export const router = createRouter({
    history: createWebHistory(),
    routes,
});
