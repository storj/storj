// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Router, { RouteRecord } from 'vue-router';

import AccessGrants from '@/components/accessGrants/AccessGrants.vue';
import CreateAccessGrant from '@/components/accessGrants/CreateAccessGrant.vue';
import CLIStep from '@/components/accessGrants/steps/CLIStep.vue';
import CreatePassphraseStep from '@/components/accessGrants/steps/CreatePassphraseStep.vue';
import EnterPassphraseStep from '@/components/accessGrants/steps/EnterPassphraseStep.vue';
import GatewayStep from '@/components/accessGrants/steps/GatewayStep.vue';
import NameStep from '@/components/accessGrants/steps/NameStep.vue';
import PermissionsStep from '@/components/accessGrants/steps/PermissionsStep.vue';
import ResultStep from '@/components/accessGrants/steps/ResultStep.vue';
import AccountArea from '@/components/account/AccountArea.vue';
import AccountBilling from '@/components/account/billing/BillingArea.vue';
import DetailedHistory from '@/components/account/billing/depositAndBillingHistory/DetailedHistory.vue';
import AddCouponCode from '@/components/account/billing/freeCredits/AddCouponCode.vue';
import CreditsHistory from '@/components/account/billing/freeCredits/CreditsHistory.vue';
import SettingsArea from '@/components/account/SettingsArea.vue';
import Page404 from '@/components/errors/Page404.vue';
import BucketsView from '@/components/objects/BucketsView.vue';
import CreatePassphrase from '@/components/objects/CreatePassphrase.vue';
import EnterPassphrase from '@/components/objects/EnterPassphrase.vue';
import ObjectsArea from '@/components/objects/ObjectsArea.vue';
import UploadFile from '@/components/objects/UploadFile.vue';
import WarningView from '@/components/objects/WarningView.vue';
import OnboardingTourArea from '@/components/onboardingTour/OnboardingTourArea.vue';
import CreateAccessGrantStep from '@/components/onboardingTour/steps/CreateAccessGrantStep.vue';
import OverviewStep from '@/components/onboardingTour/steps/OverviewStep.vue';
import CreateProject from '@/components/project/CreateProject.vue';
import EditProjectDetails from '@/components/project/EditProjectDetails.vue';
import ProjectDashboard from '@/components/project/ProjectDashboard.vue';
import ProjectsList from '@/components/projectsList/ProjectsList.vue';
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
    public static OnboardingTour = new NavigationLink('/onboarding-tour', 'Onboarding Tour');
    public static CreateProject = new NavigationLink('/create-project', 'Create Project');
    public static EditProjectDetails = new NavigationLink('/edit-project-details', 'Edit Project Details');
    public static AccessGrants = new NavigationLink('/access-grants', 'Access');
    public static ProjectsList = new NavigationLink('/projects', 'Projects');
    public static Objects = new NavigationLink('/objects', 'Objects');

    // account child paths
    public static Settings = new NavigationLink('settings', 'Settings');
    public static Billing = new NavigationLink('billing', 'Billing');
    public static AddCouponCode = new NavigationLink('add-coupon', 'Get Free Credits');
    public static BillingHistory = new NavigationLink('billing-history', 'Billing History');
    public static DepositHistory = new NavigationLink('deposit-history', 'Deposit History');
    public static CreditsHistory = new NavigationLink('credits-history', 'Credits History');

    // access grant child paths
    public static CreateAccessGrant = new NavigationLink('create-grant', 'Create Access Grant');
    public static NameStep = new NavigationLink('name', 'Name Access Grant');
    public static PermissionsStep = new NavigationLink('permissions', 'Access Grant Permissions');
    public static CreatePassphraseStep = new NavigationLink('create-passphrase', 'Access Grant Create Passphrase');
    public static EnterPassphraseStep = new NavigationLink('enter-passphrase', 'Access Grant Enter Passphrase');
    public static ResultStep = new NavigationLink('result', 'Access Grant Result');
    public static GatewayStep = new NavigationLink('gateway', 'Access Grant Gateway');
    public static CLIStep = new NavigationLink('cli', 'Access Grant In CLI');

    // onboarding tour child paths
    public static OverviewStep = new NavigationLink('overview', 'Onboarding Overview');
    public static AccessGrant = new NavigationLink('access', 'Onboarding Access Grant');
    public static AccessGrantName = new NavigationLink('name', 'Onboarding Name Access Grant');
    public static AccessGrantPermissions = new NavigationLink('permissions', 'Onboarding Access Grant Permissions');
    public static AccessGrantCLI = new NavigationLink('cli', 'Onboarding Access Grant CLI');
    public static AccessGrantPassphrase = new NavigationLink('create-passphrase', 'Onboarding Access Grant Create Passphrase');
    public static AccessGrantResult = new NavigationLink('result', 'Onboarding Access Grant Result');
    public static AccessGrantGateway = new NavigationLink('gateway', 'Onboarding Access Grant Gateway');

    // objects child paths.
    public static Warning = new NavigationLink('warning', 'Objects Warning');
    public static CreatePassphrase = new NavigationLink('create-passphrase', 'Objects Create Passphrase');
    public static EnterPassphrase = new NavigationLink('enter-passphrase', 'Objects Enter Passphrase');
    public static BucketsManagement = new NavigationLink('buckets', 'Buckets Management');
    public static UploadFile = new NavigationLink('upload/', 'Objects Upload');
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
                            children: [
                                {
                                    path: RouteConfig.AddCouponCode.path,
                                    name: RouteConfig.AddCouponCode.name,
                                    component: AddCouponCode,
                                },
                            ],
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
                    path: RouteConfig.OnboardingTour.path,
                    name: RouteConfig.OnboardingTour.name,
                    component: OnboardingTourArea,
                    children: [
                        {
                            path: RouteConfig.OverviewStep.path,
                            name: RouteConfig.OverviewStep.name,
                            component: OverviewStep,
                        },
                        {
                            path: RouteConfig.AccessGrant.path,
                            name: RouteConfig.AccessGrant.name,
                            component: CreateAccessGrantStep,
                            children: [
                                {
                                    path: RouteConfig.AccessGrantName.path,
                                    name: RouteConfig.AccessGrantName.name,
                                    component: NameStep,
                                },
                                {
                                    path: RouteConfig.AccessGrantPermissions.path,
                                    name: RouteConfig.AccessGrantPermissions.name,
                                    component: PermissionsStep,
                                    props: true,
                                },
                                {
                                    path: RouteConfig.AccessGrantCLI.path,
                                    name: RouteConfig.AccessGrantCLI.name,
                                    component: CLIStep,
                                    props: true,
                                },
                                {
                                    path: RouteConfig.AccessGrantPassphrase.path,
                                    name: RouteConfig.AccessGrantPassphrase.name,
                                    component: CreatePassphraseStep,
                                    props: true,
                                },
                                {
                                    path: RouteConfig.AccessGrantResult.path,
                                    name: RouteConfig.AccessGrantResult.name,
                                    component: ResultStep,
                                    props: true,
                                },
                                {
                                    path: RouteConfig.AccessGrantGateway.path,
                                    name: RouteConfig.AccessGrantGateway.name,
                                    component: GatewayStep,
                                    props: true,
                                },
                            ],
                        },
                    ],
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
                    component: AccessGrants,
                    children: [
                        {
                            path: RouteConfig.CreateAccessGrant.path,
                            name: RouteConfig.CreateAccessGrant.name,
                            component: CreateAccessGrant,
                            children: [
                                {
                                    path: RouteConfig.NameStep.path,
                                    name: RouteConfig.NameStep.name,
                                    component: NameStep,
                                },
                                {
                                    path: RouteConfig.PermissionsStep.path,
                                    name: RouteConfig.PermissionsStep.name,
                                    component: PermissionsStep,
                                    props: true,
                                },
                                {
                                    path: RouteConfig.CreatePassphraseStep.path,
                                    name: RouteConfig.CreatePassphraseStep.name,
                                    component: CreatePassphraseStep,
                                    props: true,
                                },
                                {
                                    path: RouteConfig.EnterPassphraseStep.path,
                                    name: RouteConfig.EnterPassphraseStep.name,
                                    component: EnterPassphraseStep,
                                    props: true,
                                },
                                {
                                    path: RouteConfig.ResultStep.path,
                                    name: RouteConfig.ResultStep.name,
                                    component: ResultStep,
                                    props: true,
                                },
                                {
                                    path: RouteConfig.GatewayStep.path,
                                    name: RouteConfig.GatewayStep.name,
                                    component: GatewayStep,
                                    props: true,
                                },
                                {
                                    path: RouteConfig.CLIStep.path,
                                    name: RouteConfig.CLIStep.name,
                                    component: CLIStep,
                                    props: true,
                                },
                            ],
                        },
                    ],
                },
                {
                    path: RouteConfig.ProjectsList.path,
                    name: RouteConfig.ProjectsList.name,
                    component: ProjectsList,
                },
                {
                    path: RouteConfig.Objects.path,
                    name: RouteConfig.Objects.name,
                    component: ObjectsArea,
                    children: [
                        {
                            path: RouteConfig.Warning.path,
                            name: RouteConfig.Warning.name,
                            component: WarningView,
                        },
                        {
                            path: RouteConfig.CreatePassphrase.path,
                            name: RouteConfig.CreatePassphrase.name,
                            component: CreatePassphrase,
                        },
                        {
                            path: RouteConfig.EnterPassphrase.path,
                            name: RouteConfig.EnterPassphrase.name,
                            component: EnterPassphrase,
                        },
                        {
                            path: RouteConfig.BucketsManagement.path,
                            name: RouteConfig.BucketsManagement.name,
                            component: BucketsView,
                            props: true,
                        },
                        {
                            path: RouteConfig.UploadFile.path,
                            name: RouteConfig.UploadFile.name,
                            component: UploadFile,
                            children: [
                                {
                                    path: '*',
                                    component: UploadFile,
                                },
                            ],
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

    if (navigateToDefaultSubTab(to.matched, RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant))) {
        next(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant).with(RouteConfig.NameStep).path);

        return;
    }

    if (navigateToDefaultSubTab(to.matched, RouteConfig.OnboardingTour.with(RouteConfig.AccessGrant))) {
        next(RouteConfig.OnboardingTour.with(RouteConfig.AccessGrant).with(RouteConfig.NameStep).path);

        return;
    }

    if (navigateToDefaultSubTab(to.matched, RouteConfig.OnboardingTour)) {
        next(RouteConfig.OnboardingTour.with(RouteConfig.OverviewStep).path);

        return;
    }

    if (navigateToDefaultSubTab(to.matched, RouteConfig.Objects)) {
        next(RouteConfig.Objects.with(RouteConfig.Warning).path);

        return;
    }

    if (to.name === 'default') {
        next(RouteConfig.ProjectDashboard.path);

        return;
    }

    next();
});

router.afterEach(({ name }, from) => {
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
    return (routes.length === 2 && (routes[1].name as string) === tabRoute.name) ||
        (routes.length === 3 && (routes[2].name as string) === tabRoute.name);
}
