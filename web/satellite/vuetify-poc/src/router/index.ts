// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { RouteRecordRaw, createRouter, createWebHistory } from 'vue-router';

import { useAppStore } from '@poc/store/appStore';

const routes: RouteRecordRaw[] = [
    {
        path: '/vuetifypoc',
        redirect: { path: '/projects' }, // redirect
    },
    {
        path: '/account',
        component: () => import('@poc/layouts/default/Account.vue'),
        beforeEnter: (_, from) => useAppStore().setPathBeforeAccountPage(from.path),
        children: [
            {
                path: 'billing',
                name: 'Billing',
                component: () => import(/* webpackChunkName: "Billing" */ '@poc/views/Billing.vue'),
            },
            {
                path: 'settings',
                name: 'Account Settings',
                component: () => import(/* webpackChunkName: "MyAccount" */ '@poc/views/AccountSettings.vue'),
            },
            {
                path: 'design-library',
                name: 'Design Library',
                component: () => import(/* webpackChunkName: "DesignLibrary" */ '@poc/views/DesignLibrary.vue'),
            },
        ],
    },
    {
        path: '/projects',
        component: () => import('@poc/layouts/default/AllProjects.vue'),
        children: [
            {
                path: '',
                name: 'Projects',
                component: () => import(/* webpackChunkName: "Projects" */ '@poc/views/Projects.vue'),
            },
        ],
    },
    {
        path: '/projects/:projectId',
        component: () => import('@poc/layouts/default/Default.vue'),
        children: [
            {
                path: 'dashboard',
                name: 'Dashboard',
                component: () => import(/* webpackChunkName: "home" */ '@poc/views/Dashboard.vue'),
            },
            {
                path: 'buckets',
                name: 'Buckets',
                component: () => import(/* webpackChunkName: "Buckets" */ '@poc/views/Buckets.vue'),
            },
            {
                path: 'buckets/:bucketName',
                name: 'Bucket',
                component: () => import(/* webpackChunkName: "Bucket" */ '@poc/views/Bucket.vue'),
            },
            {
                path: 'access',
                name: 'Access',
                component: () => import(/* webpackChunkName: "Access" */ '@poc/views/Access.vue'),
            },
            {
                path: 'team',
                name: 'Team',
                component: () => import(/* webpackChunkName: "Team" */ '@poc/views/Team.vue'),
            },
        ],
    },
];

const router = createRouter({
    history: createWebHistory(),
    routes,
});

export default router;
