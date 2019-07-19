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
        path: '/account',
        name: 'Account'
    },
    PROJECT_OVERVIEW: {
        path: '/project-overview',
        name: 'ProjectOverview'
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
        path: 'usage-report',
        name: 'UsageReport'
    },
    BILLING_HISTORY: {
        path: 'billing-history',
        name: 'BillingHistory'
    },
    PROJECT_DETAILS: {
        path: 'details',
        name: 'ProjectDetails'
    },
    PAYMENT_METHODS: {
        path: 'payment-methods',
        name: 'ProjectPaymentMethods.vue'
    },
    BUCKETS: {
        path: '/buckets',
        name: 'Buckets'
    },
    PROFILE: {
        path: 'profile',
        name: 'Profile'
    },
    REFERRAL: {
        path: '/ref/:ids',
        name: 'Referral'
    },
};

export default ROUTES;
