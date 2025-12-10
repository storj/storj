// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { watch } from 'vue';
import { RouteRecordRaw, createRouter, createWebHistory, Router, RouteLocation } from 'vue-router';

import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useAppStore } from '@/store/modules/appStore';
import { NavigationLink } from '@/types/navigation';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

enum RouteName {
    Account = 'Account',
    Billing = 'Billing',
    APIKeys = 'API Keys',
    AccountSettings = 'Account Settings',
    Projects = 'Projects',
    Project = 'Project',
    Dashboard = 'Dashboard',
    Buckets = 'Buckets',
    Bucket = 'Bucket',
    Access = 'Access Keys',
    Team = 'Team',
    Domains = 'Domains',
    Usage = 'Usage',
    CunoFS = 'cunoFS',
    ObjectMount = 'Object Mount',
    Applications = 'Applications',
    ProjectSettings = 'Project Settings',
    Login = 'Login',
    Signup = 'Signup',
    SignupConfirmation = 'Signup Confirmation',
    ForgotPassword = 'Forgot Password',
    PasswordResetConfirmation = 'Password Reset Confirmation',
    PasswordRecovery = 'Password Recovery',
    Activate = 'Activate Account',
    ComputeOverview = 'Overview',
    ComputeInstances = 'Instances',
    ComputeKeys = 'SSH Keys',
    ComputeDeployInstance = 'Deploy Instance',
}

export abstract class ROUTES {
    public static Account = new NavigationLink('/account', RouteName.Account);
    public static Billing = new NavigationLink('billing', RouteName.Billing);
    public static APIKeys = new NavigationLink('api-keys', RouteName.APIKeys);
    public static AccountSettings = new NavigationLink('settings', RouteName.AccountSettings);

    public static Projects = new NavigationLink('/projects', RouteName.Projects);
    public static Project = new NavigationLink(':id', RouteName.Project);
    public static Dashboard = new NavigationLink('dashboard', RouteName.Dashboard);
    public static Buckets = new NavigationLink('buckets', RouteName.Buckets);
    public static Bucket = new NavigationLink(':browserPath(.*)+', RouteName.Bucket);
    public static Access = new NavigationLink('access', RouteName.Access);
    public static Team = new NavigationLink('team', RouteName.Team);
    public static Domains = new NavigationLink('domains', RouteName.Domains);
    public static Usage = new NavigationLink('usage', RouteName.Usage);
    public static CunoFSBeta = new NavigationLink('cuno-fs-beta', RouteName.CunoFS);
    public static ObjectMount = new NavigationLink('object-mount', RouteName.ObjectMount);
    public static Applications = new NavigationLink('applications', RouteName.Applications);
    public static ProjectSettings = new NavigationLink('settings', RouteName.ProjectSettings);

    public static Login = new NavigationLink('/login', RouteName.Login);
    public static Signup = new NavigationLink('/signup', RouteName.Signup);
    public static SignupConfirmation = new NavigationLink('/signup-confirmation', RouteName.SignupConfirmation);
    public static ForgotPassword = new NavigationLink('/forgot-password', RouteName.ForgotPassword);
    public static PasswordResetConfirmation = new NavigationLink('/password-reset-confirmation', RouteName.PasswordResetConfirmation);
    public static PasswordRecovery = new NavigationLink('/password-recovery', RouteName.PasswordRecovery);
    public static Activate = new NavigationLink('/activate', RouteName.Activate);

    public static ComputeOverview = new NavigationLink('compute-overview', RouteName.ComputeOverview);
    public static ComputeInstances = new NavigationLink('compute-instances', RouteName.ComputeInstances);
    public static ComputeKeys = new NavigationLink('compute-keys', RouteName.ComputeKeys);
    public static ComputeDeployInstance = new NavigationLink('compute-deploy-instance', RouteName.ComputeDeployInstance);

    public static AuthRoutes = [
        ROUTES.Login.path,
        ROUTES.Signup.path,
        ROUTES.ForgotPassword.path,
        ROUTES.Activate.path,
        ROUTES.PasswordRecovery.path,
        ROUTES.SignupConfirmation.path,
        ROUTES.PasswordResetConfirmation.path,
    ];
}

const routes: RouteRecordRaw[] = [
    {
        path: '/',
        redirect: { path: ROUTES.Projects.path }, // redirect
    },
    {
        path: '/',
        component: () => import('@/layouts/default/Auth.vue'),
        children: [
            {
                path: ROUTES.Login.path,
                name: ROUTES.Login.name,
                component: () => import(/* webpackChunkName: "Login" */ '@/views/Login.vue'),
            },
            {
                path: ROUTES.Signup.path,
                name: ROUTES.Signup.name,
                component: () => import(/* webpackChunkName: "Signup" */ '@/views/Signup.vue'),
            },
            {
                path: ROUTES.SignupConfirmation.path,
                name: ROUTES.SignupConfirmation.name,
                component: () => import(/* webpackChunkName: "SignupConfirmation" */ '@/views/SignupConfirmation.vue'),
            },
            {
                path: ROUTES.ForgotPassword.path,
                name: ROUTES.ForgotPassword.name,
                component: () => import(/* webpackChunkName: "ForgotPassword" */ '@/views/ForgotPassword.vue'),
            },
            {
                path: ROUTES.PasswordResetConfirmation.path,
                name: ROUTES.PasswordResetConfirmation.name,
                component: () => import(/* webpackChunkName: "PasswordResetConfirmation" */ '@/views/PasswordResetConfirmation.vue'),
            },
            {
                path: ROUTES.PasswordRecovery.path,
                name: ROUTES.PasswordRecovery.name,
                component: () => import(/* webpackChunkName: "PasswordRecovery" */ '@/views/PasswordRecovery.vue'),
            },
            {
                path: ROUTES.Activate.path,
                name: ROUTES.Activate.name,
                component: () => import(/* webpackChunkName: "ActivateAccountRequest" */ '@/views/ActivateAccountRequest.vue'),
            },
        ],
    },
    {
        path: ROUTES.Account.path,
        component: () => import('@/layouts/default/Account.vue'),
        beforeEnter: (_, from) => useAppStore().setPathBeforeAccountPage(from.path),
        children: [
            {
                path: '',
                redirect: { path: ROUTES.Account.with(ROUTES.AccountSettings).path }, // redirect
            },
            {
                path: ROUTES.Billing.path,
                name: ROUTES.Billing.name,
                component: () => import(/* webpackChunkName: "Billing" */ '@/views/Billing.vue'),
            },
            {
                path: ROUTES.APIKeys.path,
                name: ROUTES.APIKeys.name,
                component: () => import(/* webpackChunkName: "Billing" */ '@/views/RestApiKeys.vue'),
            },
            {
                path: ROUTES.AccountSettings.path,
                name: ROUTES.AccountSettings.name,
                component: () => import(/* webpackChunkName: "MyAccount" */ '@/views/AccountSettings.vue'),
            },
        ],
    },
    {
        path: ROUTES.Projects.path,
        component: () => import('@/layouts/default/AllProjects.vue'),
        children: [
            {
                path: '',
                name: ROUTES.Projects.name,
                component: () => import(/* webpackChunkName: "Projects" */ '@/views/Projects.vue'),
            },
        ],
    },
    {
        path: ROUTES.Projects.with(ROUTES.Project).path,
        component: () => import('@/layouts/default/Default.vue'),
        children: [
            {
                path: '',
                name: RouteName.Project,
                redirect: (to: RouteLocation) => {
                    const projRoute = new NavigationLink(to.params.id as string, RouteName.Project);
                    return { path: ROUTES.Projects.with(projRoute).with(ROUTES.Dashboard).path };
                },
            },
            {
                path: ROUTES.Dashboard.path,
                name: ROUTES.Dashboard.name,
                component: () => import(/* webpackChunkName: "home" */ '@/views/Dashboard.vue'),
            },
            {
                path: ROUTES.Buckets.path,
                name: ROUTES.Buckets.name,
                component: () => import(/* webpackChunkName: "Buckets" */ '@/views/Buckets.vue'),
            },
            {
                path: ROUTES.Buckets.with(ROUTES.Bucket).path,
                name: ROUTES.Bucket.name,
                component: () => import(/* webpackChunkName: "Bucket" */ '@/views/Bucket.vue'),
            },
            {
                path: ROUTES.Access.path,
                name: ROUTES.Access.name,
                component: () => import(/* webpackChunkName: "Access" */ '@/views/Access.vue'),
            },
            {
                path: ROUTES.Usage.path,
                name: ROUTES.Usage.name,
                component: () => import(/* webpackChunkName: "Usage" */ '@/views/Usage.vue'),
            },
            {
                path: ROUTES.Domains.path,
                name: ROUTES.Domains.name,
                component: () => import(/* webpackChunkName: "Domains" */ '@/views/Domains.vue'),
            },
            {
                path: ROUTES.CunoFSBeta.path,
                name: ROUTES.CunoFSBeta.name,
                component: () => import(/* webpackChunkName: "CunoFS" */ '@/views/CunoFS.vue'),
            },
            {
                path: ROUTES.ObjectMount.path,
                name: ROUTES.ObjectMount.name,
                component: () => import(/* webpackChunkName: "ObjectMount" */ '@/views/ObjectMount.vue'),
            },
            {
                path: ROUTES.Team.path,
                name: ROUTES.Team.name,
                component: () => import(/* webpackChunkName: "Team" */ '@/views/Team.vue'),
            },
            {
                path: ROUTES.Applications.path,
                name: ROUTES.Applications.name,
                component: () => import(/* webpackChunkName: "Applications" */ '@/views/Applications.vue'),
            },
            {
                path: ROUTES.ProjectSettings.path,
                name: ROUTES.ProjectSettings.name,
                component: () => import(/* webpackChunkName: "ProjectSettings" */ '@/views/ProjectSettings.vue'),
            },
            // TODO: enable when we have more compute features.
            // {
            //     path: ROUTES.ComputeOverview.path,
            //     name: ROUTES.ComputeOverview.name,
            //     component: () => import(/* webpackChunkName: "ComputeOverview" */ '@/views/ComputeOverview.vue'),
            // },
            {
                path: ROUTES.ComputeInstances.path,
                name: ROUTES.ComputeInstances.name,
                component: () => import(/* webpackChunkName: "ComputeInstances" */ '@/views/ComputeInstances.vue'),
            },
            {
                path: ROUTES.ComputeKeys.path,
                name: ROUTES.ComputeKeys.name,
                component: () => import(/* webpackChunkName: "ComputeKeys" */ '@/views/ComputeKeys.vue'),
            },
            // {
            //     path: ROUTES.ComputeDeployInstance.path,
            //     name: ROUTES.ComputeDeployInstance.name,
            //     component: () => import(/* webpackChunkName: "ComputeDeployInstance" */ '@/views/ComputeDeployInstance.vue'),
            // },
        ],
    },
];

export function setupRouter(): Router {
    const base = import.meta.env.PROD ? '' : '/';
    const history = createWebHistory(base);
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

    router.afterEach((to, from) => {
        useAppStore().setIsNavigating(false);

        if (!configStore.state.config.analyticsEnabled) {
            return;
        }

        if (to.name === ROUTES.Bucket.name && from.name === ROUTES.Bucket.name) {
            // we are navigating within the same bucket, do not track the page visit
            return;
        }
        useAnalyticsStore().pageVisit(to.matched[to.matched.length - 1].path, configStore.state.config.satelliteName);
    });

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
