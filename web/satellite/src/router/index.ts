// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vue from 'vue';
import Router from 'vue-router';

import { NavigationLink } from '@/types/navigation';
import { MetaUtils } from '@/utils/meta';
import AllDashboardArea from '@/views/all-dashboard/AllDashboardArea.vue';
import MyProjects from '@/views/all-dashboard/components/MyProjects.vue';

import AccessGrants from '@/components/accessGrants/AccessGrants.vue';
import CreateAccessGrantFlow from '@/components/accessGrants/createFlow/CreateAccessGrantFlow.vue';
import AccountArea from '@/components/account/AccountArea.vue';
import AccountBilling from '@/components/account/billing/BillingArea.vue';
import BillingOverview from '@/components/account/billing/billingTabs/Overview.vue';
import BillingPaymentMethods from '@/components/account/billing/billingTabs/PaymentMethods.vue';
import BillingHistory from '@/components/account/billing/billingTabs/BillingHistory.vue';
import BillingCoupons from '@/components/account/billing/billingTabs/Coupons.vue';
import BucketsView from '@/components/objects/BucketsView.vue';
import ObjectsArea from '@/components/objects/ObjectsArea.vue';
import UploadFile from '@/components/objects/UploadFile.vue';
import OnboardingTourArea from '@/components/onboardingTour/OnboardingTourArea.vue';
import PricingPlanStep from '@/components/onboardingTour/steps/PricingPlanStep.vue';
import OnbCLIStep from '@/components/onboardingTour/steps/CLIStep.vue';
import OverviewStep from '@/components/onboardingTour/steps/OverviewStep.vue';
import CreateProject from '@/components/project/CreateProject.vue';
import EditProjectDetails from '@/components/project/EditProjectDetails.vue';
import ProjectDashboard from '@/components/project/dashboard/ProjectDashboard.vue';
import ProjectsList from '@/components/projectsList/ProjectsList.vue';
import ProjectMembersArea from '@/components/team/ProjectMembersArea.vue';
import CLIInstall from '@/components/onboardingTour/steps/cliFlow/CLIInstall.vue';
import APIKey from '@/components/onboardingTour/steps/cliFlow/APIKey.vue';
import CLISetup from '@/components/onboardingTour/steps/cliFlow/CLISetup.vue';
import CreateBucket from '@/components/onboardingTour/steps/cliFlow/CreateBucket.vue';
import UploadObject from '@/components/onboardingTour/steps/cliFlow/UploadObject.vue';
import ListObject from '@/components/onboardingTour/steps/cliFlow/ListObject.vue';
import DownloadObject from '@/components/onboardingTour/steps/cliFlow/DownloadObject.vue';
import ShareObject from '@/components/onboardingTour/steps/cliFlow/ShareObject.vue';
import RegistrationSuccess from '@/components/common/RegistrationSuccess.vue';
import SuccessScreen from '@/components/onboardingTour/steps/cliFlow/SuccessScreen.vue';
import AGName from '@/components/onboardingTour/steps/cliFlow/AGName.vue';
import AGPermissions from '@/components/onboardingTour/steps/cliFlow/AGPermissions.vue';
import BucketDetails from '@/components/objects/BucketDetails.vue';
import NewSettingsArea from '@/components/account/NewSettingsArea.vue';

const ActivateAccount = () => import('@/views/ActivateAccount.vue');
const AuthorizeArea = () => import('@/views/AuthorizeArea.vue');
const DashboardArea = () => import('@/views/DashboardArea.vue');
const ForgotPassword = () => import('@/views/ForgotPassword.vue');
const LoginArea = () => import('@/views/LoginArea.vue');
const RegisterArea = () => import('@/views/registration/RegisterArea.vue');
const ResetPassword = () => import('@/views/ResetPassword.vue');

Vue.use(Router);

/**
 * RouteConfig contains information about all routes and subroutes
 */
export abstract class RouteConfig {
    // root paths
    public static Root = new NavigationLink('/', 'Root');
    public static AllProjectsDashboard = new NavigationLink('/all-projects', 'All Projects');
    public static Login = new NavigationLink('/login', 'Login');
    public static Register = new NavigationLink('/signup', 'Register');
    public static RegisterSuccess = new NavigationLink('/signup-success', 'RegisterSuccess');
    public static Activate = new NavigationLink('/activate', 'Activate');
    public static ForgotPassword = new NavigationLink('/forgot-password', 'Forgot Password');
    public static ResetPassword = new NavigationLink('/password-recovery', 'Reset Password');
    public static Authorize = new NavigationLink('/oauth/v2/authorize', 'Authorize');
    public static Account = new NavigationLink('/account', 'Account');
    public static AccountSettings = new NavigationLink('/account-settings', 'Account Settings');
    public static ProjectDashboard = new NavigationLink('/project-dashboard', 'Dashboard');
    public static Users = new NavigationLink('/project-members', 'Users');
    public static OnboardingTour = new NavigationLink('/onboarding-tour', 'Onboarding Tour');
    public static CreateProject = new NavigationLink('/create-project', 'Create Project');
    public static EditProjectDetails = new NavigationLink('/edit-project-details', 'Edit Project Details');
    public static AccessGrants = new NavigationLink('/access-grants', 'Access');
    public static ProjectsList = new NavigationLink('/projects', 'Projects');
    public static Buckets = new NavigationLink('/buckets', 'Buckets');

    // account child paths
    public static Settings = new NavigationLink('settings', 'Settings');
    public static Settings2 = new NavigationLink('settings', 'Settings 2');
    public static Billing = new NavigationLink('billing', 'Billing');
    public static Billing2 = new NavigationLink('billing', 'Account Billing');
    public static BillingOverview = new NavigationLink('overview', 'Overview');
    // this duplicates the path of BillingOverview so that they can be used interchangeably in BillingArea.vue
    public static BillingOverview2 = new NavigationLink('overview', 'Billing Overview');
    public static BillingPaymentMethods = new NavigationLink('payment-methods', 'Payment Methods');
    // this duplicates the path of BillingPaymentMethods so that they can be used interchangeably in BillingArea.vue
    public static BillingPaymentMethods2 = new NavigationLink('payment-methods', 'Payment Methods 2');
    public static BillingHistory = new NavigationLink('billing-history', 'Billing History');
    // this duplicates the path of BillingHistory so that they can be used interchangeably in BillingArea.vue
    public static BillingHistory2 = new NavigationLink('billing-history2', 'Billing History 2');
    public static BillingCoupons = new NavigationLink('coupons', 'Coupons');
    public static BillingCoupons2 = new NavigationLink('coupons', 'Billing Coupons');

    // access grant child paths
    public static CreateAccessModal = new NavigationLink('create-access-modal', 'Create Access Modal');

    // onboarding tour child paths
    public static PricingPlanStep = new NavigationLink('pricing', 'Pricing Plan');
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

    public static FirstOnboardingStep = this.OverviewStep;

    // objects child paths.
    public static BucketsManagement = new NavigationLink('management', 'Buckets Management');
    public static BucketsDetails = new NavigationLink('details', 'Bucket Details');
    public static UploadFile = new NavigationLink('upload/', 'Objects Upload');
    public static UploadFileChildren = new NavigationLink('*', 'Objects Upload Children');
}

if (MetaUtils.getMetaContent('pricing-packages-enabled') === 'true') {
    RouteConfig.FirstOnboardingStep = RouteConfig.PricingPlanStep;
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
    RouteConfig.BillingOverview.name,
    RouteConfig.BillingPaymentMethods.name,
    RouteConfig.BillingHistory.name,
    RouteConfig.BillingCoupons.name,
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
                            component: NewSettingsArea,
                        },
                        {
                            path: RouteConfig.Billing.path,
                            name: RouteConfig.Billing.name,
                            component: AccountBilling,
                            children: [
                                {
                                    path: RouteConfig.BillingOverview.path,
                                    name: RouteConfig.BillingOverview.name,
                                    component: BillingOverview,
                                },
                                {
                                    path: RouteConfig.BillingPaymentMethods.path,
                                    name: RouteConfig.BillingPaymentMethods.name,
                                    component: BillingPaymentMethods,
                                },
                                {
                                    path: RouteConfig.BillingHistory.path,
                                    name: RouteConfig.BillingHistory.name,
                                    component: BillingHistory,
                                },
                                {
                                    path: RouteConfig.BillingCoupons.path,
                                    name: RouteConfig.BillingCoupons.name,
                                    component: BillingCoupons,
                                },
                            ],
                        },
                    ],
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
                            path: RouteConfig.PricingPlanStep.path,
                            name: RouteConfig.PricingPlanStep.name,
                            component: PricingPlanStep,
                        },
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
                            path: RouteConfig.CreateAccessModal.path,
                            name: RouteConfig.CreateAccessModal.name,
                            component: CreateAccessGrantFlow,
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
                            path: RouteConfig.BucketsManagement.path,
                            name: RouteConfig.BucketsManagement.name,
                            component: BucketsView,
                            props: true,
                        },
                        {
                            path: RouteConfig.BucketsDetails.path,
                            name: RouteConfig.BucketsDetails.name,
                            component: BucketDetails,
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
            path: RouteConfig.AllProjectsDashboard.path,
            meta: {
                requiresAuth: true,
            },
            component: AllDashboardArea,
            children: [
                {
                    path: RouteConfig.Root.path,
                    name: RouteConfig.AllProjectsDashboard.name,
                    component: MyProjects,
                },
                {
                    path: RouteConfig.AccountSettings.path,
                    name: RouteConfig.AccountSettings.name,
                    component: AccountArea,
                    children: [
                        {
                            path: RouteConfig.Settings2.path,
                            name: RouteConfig.Settings2.name,
                            component: NewSettingsArea,
                        },
                        {
                            path: RouteConfig.Billing2.path,
                            name: RouteConfig.Billing2.name,
                            component: AccountBilling,
                            children: [
                                {
                                    path: RouteConfig.BillingOverview2.path,
                                    name: RouteConfig.BillingOverview2.path,
                                    component: BillingOverview,
                                },
                                {
                                    path: RouteConfig.BillingPaymentMethods2.path,
                                    name: RouteConfig.BillingPaymentMethods2.name,
                                    component: BillingPaymentMethods,
                                },
                                {
                                    path: RouteConfig.BillingHistory2.path,
                                    name: RouteConfig.BillingHistory2.name,
                                    component: BillingHistory,
                                },
                                {
                                    path: RouteConfig.BillingCoupons2.path,
                                    name: RouteConfig.BillingCoupons2.name,
                                    component: BillingCoupons,
                                },
                            ],
                        },
                    ],
                },
            ],
        },
    ],
});
