// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Router, { RouteRecord } from 'vue-router';

import AccountArea from '@/components/account/AccountArea.vue';
import AccountBilling from '@/components/account/billing/BillingArea.vue';
import BillingHistory from '@/components/account/billing/BillingHistory.vue';
import ProfileArea from '@/components/account/ProfileArea.vue';
import ApiKeysArea from '@/components/apiKeys/ApiKeysArea.vue';
import BucketArea from '@/components/buckets/BucketArea.vue';
import Page404 from '@/components/errors/Page404.vue';
import ProjectDetails from '@/components/project/ProjectDetails.vue';
import ProjectOverviewArea from '@/components/project/ProjectOverviewArea.vue';
import UsageReport from '@/components/project/UsageReport.vue';
import ProjectMembersArea from '@/components/team/ProjectMembersArea.vue';

import { NavigationLink } from '@/types/navigation';
import { AuthToken } from '@/utils/authToken';
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
    public static ProjectOverview = new NavigationLink('/project-overview', 'Overview');
    public static Team = new NavigationLink('/project-members', 'Team');
    public static ApiKeys = new NavigationLink('/api-keys', 'API Keys');
    public static Buckets = new NavigationLink('/buckets', 'Buckets');

    // child paths
    public static ProjectDetails = new NavigationLink('details', 'Project Details');
    public static UsageReport = new NavigationLink('usage-report', 'Usage Report');
    public static Profile = new NavigationLink('profile', 'Profile');
    public static Billing = new NavigationLink('billing', 'Billing');
    public static BillingHistory = new NavigationLink('billing-history', 'Billing History');

    // not in project yet
    // public static Referral = new NavigationLink('//ref/:ids', 'Referral');
}

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
                            path: RouteConfig.Profile.path,
                            name: RouteConfig.Profile.name,
                            component: ProfileArea,
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
                    ],
                },
                {
                    path: RouteConfig.ProjectOverview.path,
                    name: RouteConfig.ProjectOverview.name,
                    component: ProjectOverviewArea,
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
                    component: ProjectOverviewArea,
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
    if (to.matched.some(route => route.meta.requiresAuth)) {
        if (!AuthToken.get()) {
            next(RouteConfig.Login.path);

            return;
        }
    }

    if (navigateToDefaultSubTab(to.matched, RouteConfig.Account)) {
        next(RouteConfig.Account.with(RouteConfig.Profile).path);

        return;
    }

    if (navigateToDefaultSubTab(to.matched, RouteConfig.ProjectOverview)) {
        next(RouteConfig.ProjectOverview.with(RouteConfig.ProjectDetails).path);

        return;
    }

    if (to.name === 'default') {
        next(RouteConfig.ProjectOverview.with(RouteConfig.ProjectDetails).path);

        return;
    }

    next();
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
