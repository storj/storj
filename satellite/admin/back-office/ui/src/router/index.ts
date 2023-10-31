// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

// Composables
import { createRouter, createWebHistory } from 'vue-router';

const routes = [
    {
        path: '/',
        redirect: '/login', // directly redirect
        component: () => import('@/layouts/default/Login.vue'),
        children: [
            {
                path: '/login',
                name: 'Login',
                component: () => import(/* webpackChunkName: "Login" */ '@/views/Login.vue'),
            },
        ],
    },
    {
        path: '/admin',
        component: () => import('@/layouts/default/Default.vue'),
        children: [
            {
                path: '/dashboard',
                name: 'Dashboard',
                component: () => import(/* webpackChunkName: "Dashboard" */ '@/views/Dashboard.vue'),
            },
            {
                path: '/accounts',
                name: 'Accounts',
                component: () => import(/* webpackChunkName: "Users" */ '@/views/Accounts.vue'),
            },
            {
                path: '/account-details',
                name: 'Account Details',
                component: () => import(/* webpackChunkName: "AccountDetails" */ '@/views/AccountDetails.vue'),
            },
            {
                path: '/projects',
                name: 'Projects',
                component: () => import(/* webpackChunkName: "Projects" */ '@/views/Projects.vue'),
            },
            {
                path: '/project-details',
                name: 'Project Details',
                component: () => import(/* webpackChunkName: "ProjectDetails" */ '@/views/ProjectDetails.vue'),
            },
            {
                path: '/bucket-details',
                name: 'Bucket Details',
                component: () => import(/* webpackChunkName: "BucketDetails" */ '@/views/BucketDetails.vue'),
            },
            {
                path: '/admin-settings',
                name: 'Admin Settings',
                component: () => import(/* webpackChunkName: "AdminSettings" */ '@/views/AdminSettings.vue'),
            },
        ],
    },
];

const router = createRouter({
    history: createWebHistory(process.env.NODE_ENV === 'production' ? '/back-office/' : process.env.BASE_URL),
    routes,
});

export default router;
