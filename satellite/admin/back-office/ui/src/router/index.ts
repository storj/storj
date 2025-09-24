// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { watch } from 'vue';
import { createRouter, createWebHistory, Router } from 'vue-router';

import { NavigationLink } from '@/router/navigation';

export abstract class ROUTES {
    public static Accounts = new NavigationLink('/accounts', 'Accounts');
    public static Account = new NavigationLink(':userID', 'Account');
    public static AccountProject = new NavigationLink('projects/:projectID', 'Account Project');

    public static Projects = new NavigationLink('/projects', 'Projects');
    public static ProjectDetail = new NavigationLink('/projects-details', 'Project Details');

    public static BucketDetail = new NavigationLink('/bucket-details', 'Bucket Details');
    public static AdminSettings = new NavigationLink('/admin-settings', 'Admin Settings');
}

const routes = [
    {
        path: '/',
        // redirect: '/login', // directly redirect
        // component: () => import('@/layouts/default/Login.vue'),
        // children: [
        //     {
        //         path: '/login',
        //         name: 'Login',
        //         component: () => import(/* webpackChunkName: "Login" */ '@/views/Login.vue'),
        //     },
        // ],
        // TODO: once the switch satellite feature is implemented, remove the redirection below and
        // uncomment the above code.
        redirect: ROUTES.Accounts.path, // directly redirect
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
                path: ROUTES.Accounts.path,
                children: [
                    {
                        path: '',
                        name: ROUTES.Accounts.name,
                        component: () => import(/* webpackChunkName: "Users" */ '@/views/AccountSearch.vue'),
                    },
                    {
                        path: ROUTES.Account.path,
                        children: [
                            {
                                path: '',
                                name: ROUTES.Account.name,
                                component: () => import(/* webpackChunkName: "Users" */ '@/views/AccountDetails.vue'),
                            },
                            {
                                path: ROUTES.AccountProject.path,
                                name: ROUTES.AccountProject.name,
                                component: () => import(/* webpackChunkName: "Users" */ '@/views/ProjectDetails.vue'),
                            },
                        ],
                    },
                ],
            },
            {
                path: ROUTES.Projects.path,
                name: ROUTES.Projects.name,
                component: () => import(/* webpackChunkName: "Projects" */ '@/views/Projects.vue'),
            },
            {
                path: ROUTES.ProjectDetail.path,
                name: ROUTES.ProjectDetail.name,
                component: () => import(/* webpackChunkName: "ProjectDetails" */ '@/views/ProjectDetails.vue'),
            },
            {
                path: ROUTES.BucketDetail.path,
                name: ROUTES.BucketDetail.name,
                component: () => import(/* webpackChunkName: "BucketDetails" */ '@/views/BucketDetails.vue'),
            },
            {
                path: ROUTES.AdminSettings.path,
                name: ROUTES.AdminSettings.name,
                component: () => import(/* webpackChunkName: "AdminSettings" */ '@/views/AdminSettings.vue'),
            },
        ],
    },
];

export function setupRouter(): Router {
    const router = createRouter({
        history: createWebHistory(process.env.BASE_URL),
        routes,
    });

    watch(
        () => router.currentRoute.value.name as string,
        routeName => document.title = 'Storj Admin' + (routeName ? ' - ' + routeName : ''),
    );

    return router;
}
