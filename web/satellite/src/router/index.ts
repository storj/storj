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
import ProjectDetails from '@/components/project/ProjectDetailsArea.vue';
import TeamArea from '@/components/team/TeamArea.vue';
import Page404 from '@/components/errors/Page404.vue';
import ApiKeysArea from '@/components/apiKeys/ApiKeysArea.vue';
import UsageReport from '@/components/project/UsageReport.vue';
import ReportTable from '@/components/project/ReportTable.vue';
import BucketArea from '@/components/buckets/BucketArea.vue';
import { getToken } from '@/utils/tokenManager';

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
            path: ROUTES.FORGOT_PASSWORD.path,
            name: ROUTES.FORGOT_PASSWORD.name,
            component: ForgotPassword
        },
        {
            path: ROUTES.DASHBOARD.path,
            name: ROUTES.DASHBOARD.name,
            meta: {
                requiresAuth: true
            },
            component: Dashboard,
            children: [
                {
                    path: ROUTES.ACCOUNT_SETTINGS.path,
                    name: ROUTES.ACCOUNT_SETTINGS.name,
                    component: AccountArea
                },
                {
                    path: ROUTES.PROJECT_DETAILS.path,
                    name: ROUTES.PROJECT_DETAILS.name,
                    component: ProjectDetails,
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
                    path: ROUTES.USAGE_REPORT.path,
                    name: ROUTES.USAGE_REPORT.name,
                    component: UsageReport,
                },
                {
                    path: ROUTES.BUCKETS.path,
                    name: ROUTES.BUCKETS.name,
                    component: BucketArea
                },
                // {
                //     path: '/',
                //     name: 'dashboardArea',
                //     component: DashboardArea
                // },
            ]
        },
        {
          path: ROUTES.REPORT_TABLE.path,
          name: ROUTES.REPORT_TABLE.name,
          component: ReportTable,
        },
        {
            path: '*',
            name: '404',
            component: Page404
        },
    ]
});

// Makes check that Token exist at session storage before any route except Login and Register
router.beforeEach((to, from, next) => {
    if (to.matched.some(route => route.meta.requiresAuth)) {
        if (!getToken()) {
            next(ROUTES.LOGIN);

            return;
        }
    }

    next();
});

export default router;
