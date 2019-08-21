// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import AccountArea from '@/components/account/AccountArea.vue';
import AccountBillingHistory from '@/components/account/billing/BillingArea.vue';
import AccountPaymentMethods from '@/components/account/AccountPaymentMethods.vue';
import ApiKeysArea from '@/components/apiKeys/ApiKeysArea.vue';
import { AuthToken } from '@/utils/authToken';
import BucketArea from '@/components/buckets/BucketArea.vue';
import Dashboard from '@/views/Dashboard.vue';
import ForgotPassword from '@/views/forgotPassword/ForgotPassword.vue';
import Login from '@/views/login/Login.vue';
import Page404 from '@/components/errors/Page404.vue';
import Profile from '@/components/account/Profile.vue';
import ProjectBillingHistory from '@/components/project/billing/BillingArea.vue';
import ProjectDetails from '@/components/project/ProjectDetails.vue';
import ProjectMembersArea from '@/components/team/ProjectMembersArea.vue';
import ProjectOverviewArea from '@/components/project/ProjectOverviewArea.vue';
import ProjectPaymentMethods from '@/components/project/ProjectPaymentMethods.vue';
import Register from '@/views/register/Register.vue';
import Router from 'vue-router';
import store from '@/store';
import UsageReport from '@/components/project/UsageReport.vue';
import { NavigationLink } from '@/types/navigation';

Vue.use(Router);

export abstract class RouteConfig {
    // root paths
    public static Root = new NavigationLink('/', 'Root');
    public static Login = new NavigationLink('/login', 'Login');
    public static Register = new NavigationLink('/register', 'Register');
    public static ForgotPassword = new NavigationLink('/forgot-password', 'Forgot Password');
    public static AccountSettings = new NavigationLink('/account', 'Account');
    public static ProjectOverview = new NavigationLink('/project-overview', 'Overview');
    public static Team = new NavigationLink('/project-members', 'Team');
    public static ApiKeys = new NavigationLink('/api-keys', 'ApiKeys');
    public static Buckets = new NavigationLink('/buckets', 'Buckets');

    // child paths
    public static ProjectDetails = new NavigationLink('/details', 'Project Details');
    public static BillingHistory = new NavigationLink('/billing-history', 'Billing History');
    public static UsageReport = new NavigationLink('/usage-report', 'Usage Report');
    public static PaymentMethods = new NavigationLink('/payment-methods', 'Payment Methods');
    public static Profile = new NavigationLink('/profile', 'Profile');

    // not in project yet
    // public static Referral = new NavigationLink('//ref/:ids', 'Referral');
    
}

let router = new Router({
    mode: 'history',
    routes: [
        {
            path: RouteConfig.Login.path,
            name: RouteConfig.Login.name,
            component: Login
        },
        {
            path: RouteConfig.Register.path,
            name: RouteConfig.Register.name,
            component: Register
        },
        {
            path: RouteConfig.ForgotPassword.path,
            name: RouteConfig.ForgotPassword.name,
            component: ForgotPassword
        },
        {
            path: RouteConfig.Root.path,
            meta: {
                requiresAuth: true
            },
            component: Dashboard,
            children: [
                {
                    path: RouteConfig.AccountSettings.path,
                    name: RouteConfig.AccountSettings.name,
                    component: AccountArea,
                    children: [
                        {
                            path: RouteConfig.Profile.path,
                            name: RouteConfig.Profile.name,
                            component: Profile,
                        },
                        {
                            path: RouteConfig.PaymentMethods.path,
                            name: RouteConfig.PaymentMethods.name,
                            component: AccountPaymentMethods,
                        },
                        {
                            path: RouteConfig.BillingHistory.path,
                            name: RouteConfig.BillingHistory.name,
                            component: AccountBillingHistory,
                        },
                    ]
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
                            component: ProjectDetails
                        },
                        {
                            path: RouteConfig.BillingHistory.path,
                            name: RouteConfig.BillingHistory.name,
                            component: ProjectBillingHistory
                        },
                        {
                            path: RouteConfig.PaymentMethods.path,
                            name: RouteConfig.PaymentMethods.name,
                            component: ProjectPaymentMethods
                        },
                    ]
                },
                {
                    path: RouteConfig.Team.path,
                    name: RouteConfig.Team.name,
                    component: ProjectMembersArea
                },
                {
                    path: RouteConfig.ApiKeys.path,
                    name: RouteConfig.ApiKeys.name,
                    component: ApiKeysArea
                },
                {
                    path: RouteConfig.Buckets.path,
                    name: RouteConfig.Buckets.name,
                    component: BucketArea
                },
            ]
        },
        {
            path: '*',
            name: '404',
            component: Page404
        },
    ]
});

// Makes check that Token exist at session storage before any route except Login and Register
// and if we are able to navigate to page without existing project
router.beforeEach((to, from, next) => {
    if (isUnavailablePageWithoutProject(to.name as string)) {
        next(ROUTES.PROJECT_OVERVIEW.path + '/' + ROUTES.PROJECT_DETAILS.path);

        return;
    }

    if (to.matched.some(route => route.meta.requiresAuth)) {
        if (!AuthToken.get()) {
            next(ROUTES.LOGIN);

            return;
        }
    }

    next();
});

// isUnavailablePageWithoutProject checks if we are able to navigate to page without existing project
function isUnavailablePageWithoutProject(pageName: string): boolean {
    let unavailablePages: string[] = [ROUTES.TEAM.name, ROUTES.API_KEYS.name, ROUTES.BUCKETS.name];
    const state = store.state as any;

    let isProjectSelected = state.projectsModule.selectedProject.id !== '';

    return unavailablePages.includes(pageName) && !isProjectSelected;
}

export default router;
