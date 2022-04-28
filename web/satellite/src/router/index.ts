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
import AddCouponCode from '@/components/account/billing/coupons/AddCouponCode.vue';
import CreditsHistory from '@/components/account/billing/coupons/CouponArea.vue';
import SettingsArea from '@/components/account/SettingsArea.vue';
import Page404 from '@/components/errors/Page404.vue';
import BucketsView from '@/components/objects/BucketsView.vue';
import EncryptData from '@/components/objects/EncryptData.vue';
import ObjectsArea from '@/components/objects/ObjectsArea.vue';
import UploadFile from '@/components/objects/UploadFile.vue';
import OnboardingTourArea from '@/components/onboardingTour/OnboardingTourArea.vue';
import OnbCLIStep from '@/components/onboardingTour/steps/CLIStep.vue';
import OverviewStep from '@/components/onboardingTour/steps/OverviewStep.vue';
import CreateProject from '@/components/project/CreateProject.vue';
import EditProjectDetails from '@/components/project/EditProjectDetails.vue';
import ProjectDashboard from '@/components/project/ProjectDashboard.vue';
import NewProjectDashboard from "@/components/project/newProjectDashboard/NewProjectDashboard.vue";
import ProjectsList from '@/components/projectsList/ProjectsList.vue';
import ProjectMembersArea from '@/components/team/ProjectMembersArea.vue';
import CLIInstall from "@/components/onboardingTour/steps/cliFlow/CLIInstall.vue";
import APIKey from "@/components/onboardingTour/steps/cliFlow/APIKey.vue";
import CLISetup from "@/components/onboardingTour/steps/cliFlow/CLISetup.vue";
import CreateBucket from "@/components/onboardingTour/steps/cliFlow/CreateBucket.vue";
import UploadObject from "@/components/onboardingTour/steps/cliFlow/UploadObject.vue";
import ListObject from "@/components/onboardingTour/steps/cliFlow/ListObject.vue";
import DownloadObject from "@/components/onboardingTour/steps/cliFlow/DownloadObject.vue";
import ShareObject from "@/components/onboardingTour/steps/cliFlow/ShareObject.vue";
import RegistrationSuccess from "@/components/common/RegistrationSuccess.vue";
import SuccessScreen from "@/components/onboardingTour/steps/cliFlow/SuccessScreen.vue";
import AGName from "@/components/onboardingTour/steps/cliFlow/AGName.vue";
import AGPermissions from "@/components/onboardingTour/steps/cliFlow/AGPermissions.vue";

import store from '@/store';
import { OBJECTS_ACTIONS } from '@/store/modules/objects';
import { NavigationLink } from '@/types/navigation';
import { MetaUtils } from "@/utils/meta";

const ActivateAccount = () => import('@/views/ActivateAccount.vue');
const AuthorizeArea = () => import('@/views/AuthorizeArea.vue');
const DashboardArea = () => import('@/views/DashboardArea.vue');
const ForgotPassword = () => import('@/views/ForgotPassword.vue');
const LoginArea = () => import('@/views/LoginArea.vue');
const RegisterArea = () => import('@/views/RegisterArea.vue');
const ResetPassword = () => import('@/views/ResetPassword.vue');

Vue.use(Router);

/**
 * RouteConfig contains information about all routes and subroutes
 */
export abstract class RouteConfig {
    // root paths
    public static Root = new NavigationLink('/', 'Root');
    public static Login = new NavigationLink('/login', 'Login');
    public static Register = new NavigationLink('/signup', 'Register');
    public static RegisterSuccess = new NavigationLink('/signup-success', 'RegisterSuccess');
    public static Activate = new NavigationLink('/activate', 'Activate');
    public static ForgotPassword = new NavigationLink('/forgot-password', 'Forgot Password');
    public static ResetPassword = new NavigationLink('/password-recovery', 'Reset Password');
    public static Authorize = new NavigationLink('/oauth/v2/authorize', 'Authorize')
    public static Account = new NavigationLink('/account', 'Account');
    public static ProjectDashboard = new NavigationLink('/project-dashboard', 'Dashboard');
    public static NewProjectDashboard = new NavigationLink('/new-project-dashboard', 'Dashboard ');
    public static Users = new NavigationLink('/project-members', 'Users');
    public static OnboardingTour = new NavigationLink('/onboarding-tour', 'Onboarding Tour');
    public static CreateProject = new NavigationLink('/create-project', 'Create Project');
    public static EditProjectDetails = new NavigationLink('/edit-project-details', 'Edit Project Details');
    public static AccessGrants = new NavigationLink('/access-grants', 'Access');
    public static ProjectsList = new NavigationLink('/projects', 'Projects');
    public static Buckets = new NavigationLink('/buckets', 'Buckets');

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
    public static OnbCLIStep = new NavigationLink('cli', 'Onboarding CLI');
    public static AGName = new NavigationLink('ag-name', 'Onboarding AG Name');
    public static AGPermissions = new NavigationLink('ag-permissions', 'Onboarding AG Permissions');
    public static APIKey = new NavigationLink('api-key', 'Onboarding API Key');
    public static CLIInstall = new NavigationLink('cli-install', 'Onboarding CLI Install');
    public static CLISetup = new NavigationLink('cli-setup', 'Onboarding CLI Setup');
    public static CreateBucket = new NavigationLink('create-bucket', 'Onboarding Create Bucket');
    public static UploadObject = new NavigationLink('upload-object', 'Onboarding Upload Object');
    public static ListObject = new NavigationLink('list-object', 'Onboarding List Object');
    public static DownloadObject = new NavigationLink('download-object', 'Onboarding Download Object');
    public static ShareObject = new NavigationLink('share-object', 'Onboarding Share Object');
    public static SuccessScreen = new NavigationLink('success', 'Onboarding Success Screen');

    // objects child paths.
    public static EncryptData = new NavigationLink('encrypt-data', 'Objects Encrypt Data');
    public static BucketsManagement = new NavigationLink('management', 'Buckets Management');
    public static UploadFile = new NavigationLink('upload/', 'Objects Upload');
    public static UploadFileChildren = new NavigationLink('*', 'Objects Upload Children');
}

const isNewProjectDashboard = MetaUtils.getMetaContent('new-project-dashboard') === 'true';
if (isNewProjectDashboard) {
    RouteConfig.ProjectDashboard = RouteConfig.NewProjectDashboard
}

export const notProjectRelatedRoutes = [
    RouteConfig.Login.name,
    RouteConfig.Register.name,
    RouteConfig.RegisterSuccess.name,
    RouteConfig.Activate.name,
    RouteConfig.ForgotPassword.name,
    RouteConfig.ResetPassword.name,
    RouteConfig.Authorize.name,
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
            path: RouteConfig.RegisterSuccess.path,
            name: RouteConfig.RegisterSuccess.name,
            component: RegistrationSuccess,
        },
        {
            path: RouteConfig.Activate.path,
            name: RouteConfig.Activate.name,
            component: ActivateAccount,
        },
        {
            path: RouteConfig.ForgotPassword.path,
            name: RouteConfig.ForgotPassword.name,
            component: ForgotPassword,
        },
        {
            path: RouteConfig.ResetPassword.path,
            name: RouteConfig.ResetPassword.name,
            component: ResetPassword,
        },
        {
            path: RouteConfig.Authorize.path,
            name: RouteConfig.Authorize.name,
            component: AuthorizeArea,
        },
        {
            path: RouteConfig.Root.path,
            meta: {
                requiresAuth: true,
            },
            component: DashboardArea,
            children: [
                {
                    path: RouteConfig.Root.path,
                    name: 'default',
                    component: ProjectDashboard,
                },
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
                    path: RouteConfig.NewProjectDashboard.path,
                    name: RouteConfig.NewProjectDashboard.name,
                    component: NewProjectDashboard,
                },
                {
                    path: RouteConfig.ProjectDashboard.path,
                    name: RouteConfig.ProjectDashboard.name,
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
                            path: RouteConfig.OnbCLIStep.path,
                            name: RouteConfig.OnbCLIStep.name,
                            component: OnbCLIStep,
                            children: [
                                {
                                    path: RouteConfig.AGName.path,
                                    name: RouteConfig.AGName.name,
                                    component: AGName,
                                },
                                {
                                    path: RouteConfig.AGPermissions.path,
                                    name: RouteConfig.AGPermissions.name,
                                    component: AGPermissions,
                                },
                                {
                                    path: RouteConfig.APIKey.path,
                                    name: RouteConfig.APIKey.name,
                                    component: APIKey,
                                },
                                {
                                    path: RouteConfig.CLIInstall.path,
                                    name: RouteConfig.CLIInstall.name,
                                    component: CLIInstall,
                                },
                                {
                                    path: RouteConfig.CLISetup.path,
                                    name: RouteConfig.CLISetup.name,
                                    component: CLISetup,
                                },
                                {
                                    path: RouteConfig.CreateBucket.path,
                                    name: RouteConfig.CreateBucket.name,
                                    component: CreateBucket,
                                },
                                {
                                    path: RouteConfig.UploadObject.path,
                                    name: RouteConfig.UploadObject.name,
                                    component: UploadObject,
                                },
                                {
                                    path: RouteConfig.ListObject.path,
                                    name: RouteConfig.ListObject.name,
                                    component: ListObject,
                                },
                                {
                                    path: RouteConfig.DownloadObject.path,
                                    name: RouteConfig.DownloadObject.name,
                                    component: DownloadObject,
                                },
                                {
                                    path: RouteConfig.ShareObject.path,
                                    name: RouteConfig.ShareObject.name,
                                    component: ShareObject,
                                },
                                {
                                    path: RouteConfig.SuccessScreen.path,
                                    name: RouteConfig.SuccessScreen.name,
                                    component: SuccessScreen,
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
                    path: RouteConfig.Buckets.path,
                    name: RouteConfig.Buckets.name,
                    component: ObjectsArea,
                    children: [
                        {
                            path: RouteConfig.EncryptData.path,
                            name: RouteConfig.EncryptData.name,
                            component: EncryptData,
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
                                    path: RouteConfig.UploadFileChildren.path,
                                    name: RouteConfig.UploadFileChildren.name,
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

router.beforeEach(async (to, _, next) => {
    if (!to.path.includes(RouteConfig.UploadFile.path) && !store.state.appStateModule.appState.isUploadCancelPopupVisible) {
        const areUploadsInProgress: boolean = await store.dispatch(OBJECTS_ACTIONS.CHECK_ONGOING_UPLOADS, to.path);
        if (areUploadsInProgress) return;
    }

    if (navigateToDefaultSubTab(to.matched, RouteConfig.Account)) {
        next(RouteConfig.Account.with(RouteConfig.Billing).path);

        return;
    }

    if (navigateToDefaultSubTab(to.matched, RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant))) {
        next(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant).with(RouteConfig.NameStep).path);

        return;
    }

    if (navigateToDefaultSubTab(to.matched, RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep))) {
        next(RouteConfig.OnboardingTour.path);

        return;
    }

    if (navigateToDefaultSubTab(to.matched, RouteConfig.OnboardingTour)) {
        next(RouteConfig.OnboardingTour.with(RouteConfig.OverviewStep).path);

        return;
    }

    if (navigateToDefaultSubTab(to.matched, RouteConfig.Buckets)) {
        if (store.state.appStateModule.isNewObjectsFlow) {
            next(RouteConfig.Buckets.with(RouteConfig.BucketsManagement).path);

            return;
        }

        next(RouteConfig.Buckets.with(RouteConfig.EncryptData).path);

        return;
    }

    if (to.name === 'default') {
        next(RouteConfig.ProjectDashboard.path);

        return;
    }

    next();
});

router.afterEach(({ name }, _from) => {
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
 * @param tabRoute - tabNavigator route
 */
function navigateToDefaultSubTab(routes: RouteRecord[], tabRoute: NavigationLink): boolean {
    return (routes.length === 2 && (routes[1].name as string) === tabRoute.name) ||
        (routes.length === 3 && (routes[2].name as string) === tabRoute.name);
}
