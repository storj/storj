// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Router, { RouteRecord } from 'vue-router';

import AccessGrants from '@/components/accessGrants/AccessGrants.vue';
import CreateAccessNameStep from '@/components/accessGrants/steps/CreateAccessNameStep.vue';
import CreateAccessPassphraseStep from '@/components/accessGrants/steps/CreateAccessPassphraseStep.vue';
import CreateAccessPermissionsStep from '@/components/accessGrants/steps/CreateAccessPermissionsStep.vue';
import CreateAccessUplinkStep from '@/components/accessGrants/steps/CreateAccessUplinkStep.vue';
import AccountArea from '@/components/account/AccountArea.vue';
import AccountBilling from '@/components/account/billing/BillingArea.vue';
import DetailedHistory from '@/components/account/billing/depositAndBillingHistory/DetailedHistory.vue';
import CreditsHistory from '@/components/account/billing/freeCredits/CreditsHistory.vue';
import SettingsArea from '@/components/account/SettingsArea.vue';
import ApiKeysArea from '@/components/apiKeys/ApiKeysArea.vue';
import Page404 from '@/components/errors/Page404.vue';
import OnboardingTourArea from '@/components/onboardingTour/OnboardingTourArea.vue';
import CreateProject from '@/components/project/CreateProject.vue';
import EditProjectDetails from '@/components/project/EditProjectDetails.vue';
import ProjectDashboard from '@/components/project/ProjectDashboard.vue';
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
    public static Register = new NavigationLink('/signup', 'Register');
    public static ForgotPassword = new NavigationLink('/forgot-password', 'Forgot Password');
    public static Account = new NavigationLink('/account', 'Account');
    public static ProjectDashboard = new NavigationLink('/project-dashboard', 'Dashboard');
    public static Users = new NavigationLink('/project-members', 'Users');
    public static ApiKeys = new NavigationLink('/api-keys', 'API Keys');
    public static OnboardingTour = new NavigationLink('/onboarding-tour', 'Onboarding Tour');
    public static CreateProject = new NavigationLink('/create-project', 'Create Project');
    public static EditProjectDetails = new NavigationLink('/edit-project-details', 'Edit Project Details');
    public static AccessGrants = new NavigationLink('/access-grants', 'Access Grants');

    // child paths
    public static Settings = new NavigationLink('settings', 'Settings');
    public static Billing = new NavigationLink('billing', 'Billing');
    public static BillingHistory = new NavigationLink('billing-history', 'Billing History');
    public static DepositHistory = new NavigationLink('deposit-history', 'Deposit History');
    public static CreditsHistory = new NavigationLink('credits-history', 'Credits History');
    public static NameStep = new NavigationLink('access-create-name', 'Name Your Access');
    public static PermissionsStep = new NavigationLink('access-create-permissions', 'Access Permissions');
    public static PassphraseStep = new NavigationLink('access-create-passphrase', 'Encryption Passphrase');
    public static UplinkStep = new NavigationLink('access-create-uplink', 'Upload Data');

    // TODO: disabled until implementation
    // public static Referral = new NavigationLink('referral', 'Referral');

    // not in project yet
    // public static Referral = new NavigationLink('//ref/:ids', 'Referral');
}

export const notProjectRelatedRoutes = [
    RouteConfig.Login.name,
    RouteConfig.Register.name,
    RouteConfig.ForgotPassword.name,
    RouteConfig.Billing.name,
    RouteConfig.BillingHistory.name,
    RouteConfig.DepositHistory.name,
    RouteConfig.CreditsHistory.name,
    RouteConfig.Settings.name,
    RouteConfig.AccessGrants.name,
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
                            component: DetailedHistory,
                        },
                        {
                            path: RouteConfig.DepositHistory.path,
                            name: RouteConfig.DepositHistory.name,
                            component: DetailedHistory,
                        },
                        {
                            path: RouteConfig.CreditsHistory.path,
                            name: RouteConfig.CreditsHistory.name,
                            component: CreditsHistory,
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
                },
                {
                    path: RouteConfig.Root.path,
                    name: 'default',
                    component: ProjectDashboard,
                },
                {
                    path: RouteConfig.Users.path,
                    name: RouteConfig.Users.name,
                    component: ProjectMembersArea,
                },
                {
                    path: RouteConfig.ApiKeys.path,
                    name: RouteConfig.ApiKeys.name,
                    component: ApiKeysArea,
                },
                {
                    path: RouteConfig.OnboardingTour.path,
                    name: RouteConfig.OnboardingTour.name,
                    component: OnboardingTourArea,
                },
                {
                    path: RouteConfig.CreateProject.path,
                    name: RouteConfig.CreateProject.name,
                    component: CreateProject,
                },
                {
                    path: RouteConfig.EditProjectDetails.path,
                    name: RouteConfig.EditProjectDetails.name,
                    component: EditProjectDetails,
                },
                {
                    path: RouteConfig.AccessGrants.path,
                    name: RouteConfig.AccessGrants.name,
                    meta: {
                            requiresAuth: true,
                    },
                    component: AccessGrants,
                    children: [
                        {
                                path: RouteConfig.NameStep.path,
                                name: RouteConfig.NameStep.name,
                                component: CreateAccessNameStep,
                        },
                        {
                                path: RouteConfig.PermissionsStep.path,
                                name: RouteConfig.PermissionsStep.name,
                                component: CreateAccessPermissionsStep,
                        },
                        {
                                path: RouteConfig.PassphraseStep.path,
                                name: RouteConfig.PassphraseStep.name,
                                component: CreateAccessPassphraseStep,
                        },
                        {
                                path: RouteConfig.UplinkStep.path,
                                name: RouteConfig.UplinkStep.name,
                                component: CreateAccessUplinkStep,
                        },
                    ],
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
        next(RouteConfig.Account.with(RouteConfig.Billing).path);

        return;
    }

    if (to.name === 'default') {
        next(RouteConfig.ProjectDashboard.path);

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
 * F.E. /account/ -> /account/billing/;
 * @param routes - array of RouteRecord from vue-router
 * @param next - callback to process next route
 * @param tabRoute - tabNavigator route
 */
function navigateToDefaultSubTab(routes: RouteRecord[], tabRoute: NavigationLink): boolean {
    return routes.length === 2 && (routes[1].name as string) === tabRoute.name;
}
