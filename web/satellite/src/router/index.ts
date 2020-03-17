// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Router, { RouteRecord } from 'vue-router';

import AccountArea from '@/components/account/AccountArea.vue';
import AccountBilling from '@/components/account/billing/BillingArea.vue';
import BillingHistory from '@/components/account/billing/billingHistory/BillingHistory.vue';
import SettingsArea from '@/components/account/SettingsArea.vue';
import ApiKeysArea from '@/components/apiKeys/ApiKeysArea.vue';
import BucketArea from '@/components/buckets/BucketArea.vue';
import Page404 from '@/components/errors/Page404.vue';
import OverviewArea from '@/components/overview/OverviewArea.vue';
import ProjectDashboard from '@/components/project/ProjectDashboard.vue';
import ProjectDetails from '@/components/project/ProjectDetails.vue';
import UsageReport from '@/components/project/UsageReport.vue';
import ProjectMembersArea from '@/components/team/ProjectMembersArea.vue';

import store from '@/store';
import { NavigationLink } from '@/types/navigation';
const DashboardArea = () => import('@/views/DashboardArea.vue');
const ForgotPassword = () => import('@/views/forgotPassword/ForgotPassword.vue');
const LoginArea = () => import('@/views/login/LoginArea.vue');
const RegisterArea = () => import('@/views/register/RegisterArea.vue');

Vue.use(Router);

/**
 * RouteConfig contains information about all routes and subroutes
 */
export abstract class RouteConfig {
    // root paths
    public static Root = new NavigationLink('/', 'Root');
    public static Login = new NavigationLink('/login', 'Login');
    public static Register = new NavigationLink('/register', 'Register');
    public static ForgotPassword = new NavigationLink('/forgot-password', 'Forgot Password');
    public static Account = new NavigationLink('/account', 'Account');
    public static ProjectDashboard = new NavigationLink('/project-dashboard', 'Dashboard');
    public static Team = new NavigationLink('/project-members', 'Team');
    public static ApiKeys = new NavigationLink('/api-keys', 'API Keys');
    public static Buckets = new NavigationLink('/buckets', 'Buckets');
    public static Overview = new NavigationLink('/overview', 'Initial Overview');

    // child paths
    public static ProjectDetails = new NavigationLink('details', 'Project Details');
    public static UsageReport = new NavigationLink('usage-report', 'Usage Report');
    public static Settings = new NavigationLink('settings', 'Settings');
    public static Billing = new NavigationLink('billing', 'Billing');
    public static BillingHistory = new NavigationLink('billing-history', 'Billing History');
    // TODO: disabled until implementation
    // public static Referral = new NavigationLink('referral', 'Referral');

    // not in project yet
    // public static Referral = new NavigationLink('//ref/:ids', 'Referral');
}

export const notProjectRelatedRoutes = [
    RouteConfig.Login.name,
    RouteConfig.Register.name,
    RouteConfig.Billing.name,
    RouteConfig.BillingHistory.name,
    RouteConfig.Settings.name,
    // RouteConfig.Referral.name,
];

export const router = new Router({
    mode: 'history',
    routes: [
        {
            path: RouteConfig.Login.path,
            name: RouteConfig.Login.name,
            component: LoginArea,
        },
        {
            path: RouteConfig.Register.path,
            name: RouteConfig.Register.name,
            component: RegisterArea,
        },
        {
            path: RouteConfig.ForgotPassword.path,
            name: RouteConfig.ForgotPassword.name,
            component: ForgotPassword,
        },
        {
            path: RouteConfig.Root.path,
            meta: {
                requiresAuth: true,
            },
            component: DashboardArea,
            children: [
                {
                    path: RouteConfig.Account.path,
                    name: RouteConfig.Account.name,
                    component: AccountArea,
                    children: [
                        {
                            path: RouteConfig.Settings.path,
                            name: RouteConfig.Settings.name,
                            component: SettingsArea,
                        },
                        {
                            path: RouteConfig.Billing.path,
                            name: RouteConfig.Billing.name,
                            component: AccountBilling,
                        },
                        {
                            path: RouteConfig.BillingHistory.path,
                            name: RouteConfig.BillingHistory.name,
                            component: BillingHistory,
                        },
                        // {
                        //     path: RouteConfig.Referral.path,
                        //     name: RouteConfig.Referral.name,
                        //     component: ReferralArea,
                        // },
                    ],
                },
                {
                    path: RouteConfig.ProjectDashboard.path,
                    name: RouteConfig.ProjectDashboard.name,
                    component: ProjectDashboard,
                    children: [
                        {
                            path: RouteConfig.UsageReport.path,
                            name: RouteConfig.UsageReport.name,
                            component: UsageReport,
                        },
                        {
                            path: RouteConfig.ProjectDetails.path,
                            name: RouteConfig.ProjectDetails.name,
                            component: ProjectDetails,
                        },
                    ],
                },
                {
                    path: RouteConfig.Root.path,
                    name: 'default',
                    component: ProjectDashboard,
                },
                {
                    path: RouteConfig.Team.path,
                    name: RouteConfig.Team.name,
                    component: ProjectMembersArea,
                },
                {
                    path: RouteConfig.ApiKeys.path,
                    name: RouteConfig.ApiKeys.name,
                    component: ApiKeysArea,
                },
                {
                    path: RouteConfig.Buckets.path,
                    name: RouteConfig.Buckets.name,
                    component: BucketArea,
                },
                {
                    path: RouteConfig.Overview.path,
                    name: RouteConfig.Overview.name,
                    component: OverviewArea,
                },
            ],
        },
        {
            path: '*',
            name: '404',
            component: Page404,
        },
    ],
});

router.beforeEach((to, from, next) => {
    if (navigateToDefaultSubTab(to.matched, RouteConfig.Account)) {
        next(RouteConfig.Account.with(RouteConfig.Settings).path);

        return;
    }

    if (navigateToDefaultSubTab(to.matched, RouteConfig.ProjectDashboard)) {
        next(RouteConfig.ProjectDashboard.with(RouteConfig.ProjectDetails).path);

        return;
    }

    if (to.name === 'default') {
        next(RouteConfig.ProjectDashboard.with(RouteConfig.ProjectDetails).path);

        return;
    }

    next();
});

router.afterEach(({name}, from) => {
    if (!name) {
        return;
    }

    if (notProjectRelatedRoutes.includes(name)) {
        document.title = `${router.currentRoute.name} | ${store.state.appStateModule.satelliteName}`;

        return;
    }

    const selectedProjectName = store.state.projectsModule.selectedProject.name ?
        `${store.state.projectsModule.selectedProject.name} | ` : '';

    document.title = `${selectedProjectName + router.currentRoute.name} | ${store.state.appStateModule.satelliteName}`;
});

/**
 * if our route is a tab and has no sub tab route - we will navigate to default subtab.
 * F.E. /account/ -> /account/profile/; /project-overview/ -> /project-overview/details/
 * @param routes - array of RouteRecord from vue-router
 * @param next - callback to process next route
 * @param tabRoute - tabNavigator route
 */
function navigateToDefaultSubTab(routes: RouteRecord[], tabRoute: NavigationLink): boolean {
    return routes.length === 2 && (routes[1].name as string) === tabRoute.name;
}
