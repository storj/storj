// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

const ROUTES = {
    LOGIN: {
        path: '/login',
        name: 'Login'
    },
    REGISTER: {
        path: '/register',
        name: 'Register'
    },
    FORGOT_PASSWORD: {
        path: '/forgot-password',
        name: 'ForgotPassword'
    },
    DASHBOARD: {
        path: '/',
        name: 'Dashboard'
    },
    ACCOUNT_SETTINGS: {
        path: '/account-settings',
        name: 'AccountSettings'
    },
    PROJECT_DETAILS: {
        path: '/project-details',
        name: 'ProjectDetails'
    },
    TEAM: {
        path: '/team',
        name: 'Team'
    },
    API_KEYS: {
        path: '/api-keys',
        name: 'ApiKeys'
    },
    USAGE_REPORT: {
        path: '/project-details/usage-report',
        name: 'UsageReport'
    },
    REPORT_TABLE: {
        path: '/project-details/usage-report/detailed-report',
        name: 'ReportTable'
    },
    BUCKETS: {
        path: '/buckets',
        name: 'Buckets'
    },
};

export default ROUTES;
