// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Router from 'vue-router';
import ROUTES from '@/utils/constants/routerConstants';
import Login from '@/views/login/Login.vue';
import Register from '@/views/register/Register.vue';
import ForgotPassword from '@/views/forgotPassword/ForgotPassword.vue';
import Dashboard from '@/views/Dashboard.vue';
import AccountArea from '@/components/account/AccountArea.vue';
import Profile from '@/components/account/Profile.vue';
import AccountBillingHistory from '@/components/account/billing/BillingArea.vue';
import AccountPaymentMethods from '@/components/account/AccountPaymentMethods.vue';
import ProjectOverviewArea from '@/components/project/ProjectOverviewArea.vue';
import TeamArea from '@/components/team/TeamArea.vue';
import Page404 from '@/components/errors/Page404.vue';
import ApiKeysArea from '@/components/apiKeys/ApiKeysArea.vue';
import UsageReport from '@/components/project/UsageReport.vue';
import ProjectDetails from '@/components/project/ProjectDetails.vue';
import ProjectBillingHistory from '@/components/project/billing/BillingArea.vue';
import ProjectPaymentMethods from '@/components/project/ProjectPaymentMethods.vue';
import BucketArea from '@/components/buckets/BucketArea.vue';
import { AuthToken } from '@/utils/authToken';
import store from '@/store';

Vue.use(Router);

let router = new Router({
    mode: 'history',
    routes: [
        {
            path: ROUTES.LOGIN.path,
            name: ROUTES.LOGIN.name,
            component: Login
        },
        {
            path: ROUTES.REGISTER.path,
            name: ROUTES.REGISTER.name,
            component: Register
        },
        {
            path: ROUTES.REFERRAL.path,
            name: ROUTES.REFERRAL.name,
            component: Register
        },
        {
            path: ROUTES.FORGOT_PASSWORD.path,
            name: ROUTES.FORGOT_PASSWORD.name,
            component: ForgotPassword
        },
        {
            path: ROUTES.DASHBOARD.path,
            meta: {
                requiresAuth: true
            },
            component: Dashboard,
            children: [
                {
                    path: ROUTES.ACCOUNT_SETTINGS.path,
                    name: ROUTES.ACCOUNT_SETTINGS.name,
                    component: AccountArea,
                    children: [
                        {
                            path: ROUTES.PROFILE.path,
                            name: ROUTES.PROFILE.name,
                            component: Profile,
                        },
                        {
                            path: ROUTES.PAYMENT_METHODS.path,
                            name: ROUTES.PAYMENT_METHODS.name,
                            component: AccountPaymentMethods,
                        },
                        {
                            path: ROUTES.BILLING_HISTORY.path,
                            name: ROUTES.BILLING_HISTORY.name,
                            component: AccountBillingHistory,
                        },
                    ]
                },
                {
                    path: ROUTES.PROJECT_OVERVIEW.path,
                    name: ROUTES.PROJECT_OVERVIEW.name,
                    component: ProjectOverviewArea,
                    children: [
                        {
                            path: ROUTES.USAGE_REPORT.path,
                            name: ROUTES.USAGE_REPORT.name,
                            component: UsageReport,
                        },
                        {
                            path: ROUTES.PROJECT_DETAILS.path,
                            name: ROUTES.PROJECT_DETAILS.name,
                            component: ProjectDetails
                        },
                        {
                            path: ROUTES.BILLING_HISTORY.path,
                            name: ROUTES.BILLING_HISTORY.name,
                            component: ProjectBillingHistory
                        },
                        {
                            path: ROUTES.PAYMENT_METHODS.path,
                            name: ROUTES.PAYMENT_METHODS.name,
                            component: ProjectPaymentMethods
                        },
                    ]
                },
                // Remove when dashboard will be created
                {
                    path: '/',
                    name: 'default',
                    component: ProjectOverviewArea
                },
                {
                    path: ROUTES.TEAM.path,
                    name: ROUTES.TEAM.name,
                    component: TeamArea
                },
                {
                    path: ROUTES.API_KEYS.path,
                    name: ROUTES.API_KEYS.name,
                    component: ApiKeysArea
                },
                {
                    path: ROUTES.BUCKETS.path,
                    name: ROUTES.BUCKETS.name,
                    component: BucketArea
                },
                // {
                //     path: ROUTES.BUCKETS.path,
                //     name: ROUTES.BUCKETS.name,
                //     component: BucketArea
                // },
                // {
                //     path: '/',
                //     name: 'dashboardArea',
                //     component: DashboardArea
                // },
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
