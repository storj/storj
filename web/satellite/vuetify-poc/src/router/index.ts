// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { watch } from 'vue';
import { RouteRecordRaw, createRouter, createWebHistory, Router, RouteLocation } from 'vue-router';

import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useAppStore } from '@poc/store/appStore';
import { FrontendConfig } from '@/types/config.gen';
import { NavigationLink } from '@/types/navigation';

enum RouteName {
    Account = 'Account',
    Billing = 'Billing',
    AccountSettings = 'Account Settings',
    Projects = 'Projects',
    Project = 'Project',
    Dashboard = 'Dashboard',
    Buckets = 'Buckets',
    Bucket = 'Bucket',
    Access = 'Access',
    Team = 'Team',
    ProjectSettings = 'Project Settings',
    Login = 'Login',
    Signup = 'Signup',
    SignupConfirmation = 'Signup Confirmation',
    ForgotPassword = 'Forgot Password',
    PasswordResetConfirmation = 'Password Reset Confirmation',
    PasswordRecovery = 'Password Recovery',
    Activate = 'Activate Account',
}

export abstract class ROUTES {
    public static Account = new NavigationLink('/account', RouteName.Account);
    public static Billing = new NavigationLink('billing', RouteName.Billing);
    public static AccountSettings = new NavigationLink('settings', RouteName.AccountSettings);

    public static Projects = new NavigationLink('/projects', RouteName.Projects);
    public static Project = new NavigationLink(':id', RouteName.Project);
    public static Dashboard = new NavigationLink('dashboard', RouteName.Dashboard);
    public static Buckets = new NavigationLink('buckets', RouteName.Buckets);
    public static Bucket = new NavigationLink(':browserPath+', RouteName.Bucket);
    public static Access = new NavigationLink('access', RouteName.Access);
    public static Team = new NavigationLink('team', RouteName.Team);
    public static ProjectSettings = new NavigationLink('settings', RouteName.ProjectSettings);

    public static Login = new NavigationLink('/login', RouteName.Login);
    public static Signup = new NavigationLink('/signup', RouteName.Signup);
    public static SignupConfirmation = new NavigationLink('/signup-confirmation', RouteName.SignupConfirmation);
    public static ForgotPassword = new NavigationLink('/forgot-password', RouteName.ForgotPassword);
    public static PasswordResetConfirmation = new NavigationLink('/password-reset-confirmation', RouteName.PasswordResetConfirmation);
    public static PasswordRecovery = new NavigationLink('/password-recovery', RouteName.PasswordRecovery);
    public static Activate = new NavigationLink('/activate', RouteName.Activate);

    public static DashboardAnalyticsLink = `${this.Projects.path}/${this.Dashboard.path}`;
    public static ProjectSettingsAnalyticsLink = `${this.Projects.path}/${this.ProjectSettings.path}`;
    public static AccessAnalyticsLink = `${this.Projects.path}/${this.Access.path}`;
    public static TeamAnalyticsLink = `${this.Projects.path}/${this.Team.path}`;
    public static BucketsAnalyticsLink = `${this.Projects.path}/${this.Buckets.path}`;
}

const routes: RouteRecordRaw[] = [
    {
        path: '/',
        redirect: { path: ROUTES.Projects.path }, // redirect
    },
    {
        path: '/',
        component: () => import('@poc/layouts/default/Auth.vue'),
        children: [
            {
                path: ROUTES.Login.path,
                name: ROUTES.Login.name,
                component: () => import(/* webpackChunkName: "Login" */ '@poc/views/Login.vue'),
            },
            {
                path: ROUTES.Signup.path,
                name: ROUTES.Signup.name,
                component: () => import(/* webpackChunkName: "Signup" */ '@poc/views/Signup.vue'),
            },
            {
                path: ROUTES.SignupConfirmation.path,
                name: ROUTES.SignupConfirmation.name,
                component: () => import(/* webpackChunkName: "SignupConfirmation" */ '@poc/views/SignupConfirmation.vue'),
            },
            {
                path: ROUTES.ForgotPassword.path,
                name: ROUTES.ForgotPassword.name,
                component: () => import(/* webpackChunkName: "ForgotPassword" */ '@poc/views/ForgotPassword.vue'),
            },
            {
                path: ROUTES.PasswordResetConfirmation.path,
                name: ROUTES.PasswordResetConfirmation.name,
                component: () => import(/* webpackChunkName: "PasswordResetConfirmation" */ '@poc/views/PasswordResetConfirmation.vue'),
            },
            {
                path: ROUTES.PasswordRecovery.path,
                name: ROUTES.PasswordRecovery.name,
                component: () => import(/* webpackChunkName: "PasswordRecovery" */ '@poc/views/PasswordRecovery.vue'),
            },
            {
                path: ROUTES.Activate.path,
                name: ROUTES.Activate.name,
                component: () => import(/* webpackChunkName: "ActivateAccountRequest" */ '@poc/views/ActivateAccountRequest.vue'),
            },
        ],
    },
    {
        path: ROUTES.Account.path,
        component: () => import('@poc/layouts/default/Account.vue'),
        beforeEnter: (_, from) => useAppStore().setPathBeforeAccountPage(from.path),
        children: [
            {
                path: '',
                redirect: { path: ROUTES.Account.with(ROUTES.AccountSettings).path }, // redirect
            },
            {
                path: ROUTES.Billing.path,
                name: ROUTES.Billing.name,
                component: () => import(/* webpackChunkName: "Billing" */ '@poc/views/Billing.vue'),
            },
            {
                path: ROUTES.AccountSettings.path,
                name: ROUTES.AccountSettings.path,
                component: () => import(/* webpackChunkName: "MyAccount" */ '@poc/views/AccountSettings.vue'),
            },
        ],
    },
    {
        path: ROUTES.Projects.path,
        component: () => import('@poc/layouts/default/AllProjects.vue'),
        children: [
            {
                path: '',
                name: ROUTES.Projects.name,
                component: () => import(/* webpackChunkName: "Projects" */ '@poc/views/Projects.vue'),
            },
        ],
    },
    {
        path: ROUTES.Projects.with(ROUTES.Project).path,
        name: RouteName.Project,
        component: () => import('@poc/layouts/default/Default.vue'),
        children: [
            {
                path: '',
                redirect: (to: RouteLocation) => {
                    const projRoute = new NavigationLink(to.params.id as string, RouteName.Project);
                    return { path: ROUTES.Projects.with(projRoute).with(ROUTES.Dashboard).path };
                },
            },
            {
                path: ROUTES.Dashboard.path,
                name: ROUTES.Dashboard.name,
                component: () => import(/* webpackChunkName: "home" */ '@poc/views/Dashboard.vue'),
            },
            {
                path: ROUTES.Buckets.path,
                name: ROUTES.Buckets.name,
                component: () => import(/* webpackChunkName: "Buckets" */ '@poc/views/Buckets.vue'),
            },
            {
                path: ROUTES.Buckets.with(ROUTES.Bucket).path,
                name: ROUTES.Bucket.name,
                component: () => import(/* webpackChunkName: "Bucket" */ '@poc/views/Bucket.vue'),
            },
            {
                path: ROUTES.Access.path,
                name: ROUTES.Access.name,
                component: () => import(/* webpackChunkName: "Access" */ '@poc/views/Access.vue'),
            },
            {
                path: ROUTES.Team.path,
                name: ROUTES.Team.name,
                component: () => import(/* webpackChunkName: "Team" */ '@poc/views/Team.vue'),
            },
            {
                path: ROUTES.ProjectSettings.path,
                name: ROUTES.ProjectSettings.name,
                component: () => import(/* webpackChunkName: "ProjectSettings" */ '@poc/views/ProjectSettings.vue'),
            },
        ],
    },
];

export function setupRouter(): Router {
    const history = createWebHistory('');
    const router = createRouter({
        history,
        routes,
    });

    router.beforeEach((to, _, next) => {
        const appStore = useAppStore();
        appStore.setIsNavigating(true);

        if (!to.matched.length) {
            appStore.setErrorPage(404);
            return;
        } else if (appStore.state.error.visible) {
            appStore.removeErrorPage();
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
