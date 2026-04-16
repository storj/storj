// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { watchEffect } from 'vue';
import { createRouter, createWebHistory, Router } from 'vue-router';

import { NavigationLink } from '@/router/navigation';
import { useAppStore } from '@/store/app';

export abstract class ROUTES {
    public static Accounts = new NavigationLink('/accounts', 'Accounts');
    public static Account = new NavigationLink(':userID', 'Account');
    public static AccountProject = new NavigationLink('projects/:projectID', 'Account Project');

    public static ProjectDetail = new NavigationLink('/projects-details', 'Project Details');

    public static NodeDetail = new NavigationLink('/nodes/:nodeID', 'Node Detail');
}

const routes = [
    {
        path: '/',
        redirect: ROUTES.Accounts.path,
    },
    {
        path: '/admin',
        component: () => import('@/layouts/default/Default.vue'),
        children: [
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
                path: ROUTES.ProjectDetail.path,
                name: ROUTES.ProjectDetail.name,
                component: () => import(/* webpackChunkName: "ProjectDetails" */ '@/views/ProjectDetails.vue'),
            },
            {
                path: ROUTES.NodeDetail.path,
                name: ROUTES.NodeDetail.name,
                component: () => import(/* webpackChunkName: "NodeDetail" */ '@/views/NodeDetail.vue'),
            },
        ],
    },
];

export function setupRouter(): Router {
    const router = createRouter({
        history: createWebHistory(process.env.BASE_URL),
        routes,
    });

    const appStore = useAppStore();
    watchEffect(() => {
        const routeName = router.currentRoute.value.name as string | undefined;
        const brandName = appStore.state.settings?.admin?.branding?.name ?? 'Storj';
        const parts: string[] = [`${brandName} Admin`];
        if (routeName) parts.unshift(routeName);
        document.title = parts.join(' | ');
    });

    return router;
}
