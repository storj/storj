// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { watch } from 'vue';
import { RouteRecordRaw, createRouter, createWebHistory, Router } from 'vue-router';

import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useAppStore } from '@poc/store/appStore';
import { FrontendConfig } from '@/types/config.gen';

export enum RouteName {
    Billing = 'Billing',
    AccountSettings = 'Account Settings',
    DesignLibrary = 'Design Library',
    Projects = 'Projects',
    Project = 'Project',
    Dashboard = 'Dashboard',
    Buckets = 'Buckets',
    Bucket = 'Bucket',
    Access = 'Access',
    Team = 'Team',
    ProjectSettings = 'Project Settings',
    Login = 'Login',
}

const routes: RouteRecordRaw[] = [
    {
        path: '/',
        redirect: { path: '/projects' }, // redirect
    },
    {
        path: '/',
        component: () => import('@poc/layouts/default/Auth.vue'),
        children: [
            {
                path: '/login',
                name: RouteName.Login,
                component: () => import(/* webpackChunkName: "Login" */ '@poc/views/Login.vue'),
            },
            {
                path: '/signup',
                name: 'Signup',
                component: () => import(/* webpackChunkName: "Signup" */ '@poc/views/Signup.vue'),
            },
            {
                path: '/signup-confirmation',
                name: 'Signup Confirmation',
                component: () => import(/* webpackChunkName: "SignupConfirmation" */ '@poc/views/SignupConfirmation.vue'),
            },
            {
                path: '/password-reset',
                name: 'Password Reset',
                component: () => import(/* webpackChunkName: "PasswordReset" */ '@poc/views/PasswordReset.vue'),
            },
            {
                path: '/password-reset-confirmation',
                name: 'Password Reset Confirmation',
                component: () => import(/* webpackChunkName: "PasswordResetConfirmation" */ '@poc/views/PasswordResetConfirmation.vue'),
            },
            {
                path: '/password-reset-new',
                name: 'Password Reset New',
                component: () => import(/* webpackChunkName: "PasswordResetNew" */ '@poc/views/PasswordResetNew.vue'),
            },
            {
                path: '/password-reset-new-confirmation',
                name: 'Password Reset New Confirmation',
                component: () => import(/* webpackChunkName: "PasswordResetNewConfirmation" */ '@poc/views/PasswordResetNewConfirmation.vue'),
            },
        ],
    },
    {
        path: '/account',
        component: () => import('@poc/layouts/default/Account.vue'),
        beforeEnter: (_, from) => useAppStore().setPathBeforeAccountPage(from.path),
        children: [
            {
                path: 'billing',
                name: RouteName.Billing,
                component: () => import(/* webpackChunkName: "Billing" */ '@poc/views/Billing.vue'),
            },
            {
                path: 'settings',
                name: RouteName.AccountSettings,
                component: () => import(/* webpackChunkName: "MyAccount" */ '@poc/views/AccountSettings.vue'),
            },
            {
                path: 'design-library',
                name: RouteName.DesignLibrary,
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
                name: RouteName.Projects,
                component: () => import(/* webpackChunkName: "Projects" */ '@poc/views/Projects.vue'),
            },
        ],
    },
    {
        path: '/projects/:id',
        name: RouteName.Project,
        component: () => import('@poc/layouts/default/Default.vue'),
        children: [
            {
                path: 'dashboard',
                name: RouteName.Dashboard,
                component: () => import(/* webpackChunkName: "home" */ '@poc/views/Dashboard.vue'),
            },
            {
                path: 'buckets',
                name: RouteName.Buckets,
                component: () => import(/* webpackChunkName: "Buckets" */ '@poc/views/Buckets.vue'),
            },
            {
                path: 'buckets/:browserPath+',
                name: RouteName.Bucket,
                component: () => import(/* webpackChunkName: "Bucket" */ '@poc/views/Bucket.vue'),
            },
            {
                path: 'access',
                name: RouteName.Access,
                component: () => import(/* webpackChunkName: "Access" */ '@poc/views/Access.vue'),
            },
            {
                path: 'team',
                name: RouteName.Team,
                component: () => import(/* webpackChunkName: "Team" */ '@poc/views/Team.vue'),
            },
            {
                path: 'settings',
                name: RouteName.ProjectSettings,
                component: () => import(/* webpackChunkName: "ProjectSettings" */ '@poc/views/ProjectSettings.vue'),
            },
        ],
    },
];

export function setupRouter(config: FrontendConfig): Router {
    const basePath = !config.prefixVuetifyUI ? '' : import.meta.env.VITE_VUETIFY_PREFIX;
    const history = createWebHistory(basePath);
    const router = createRouter({
        history,
        routes,
    });

    router.beforeEach((to, from, next) => {
        const appStore = useAppStore();
        appStore.setIsNavigating(true);

        if (!to.matched.length) {
            appStore.setErrorPage(404);
            return;
        } else if (appStore.state.error.visible) {
            appStore.removeErrorPage();
        }

        if (to.name === RouteName.Projects && from.name === RouteName.Login) {
            appStore.toggleHasJustLoggedIn(true);
        }

        next();
    });

    router.afterEach(() => useAppStore().setIsNavigating(false));

    const projectsStore = useProjectsStore();
    const configStore = useConfigStore();

    watch(
        () => [router.currentRoute.value, projectsStore.state.selectedProject.name] as const,
        ([route, projectName]) => {
            const parts = [configStore.state.config.satelliteName];

            if (route.name) parts.unshift(route.name as string);
            if (route.matched.some(route => route.name === RouteName.Project) && projectName) {
                parts.unshift(projectName);
            }

            document.title = parts.join(' | ');
        },
    );

    return router;
}