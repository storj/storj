// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { NavigationLink } from '@/types/navigation';

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
    public static RegisterConfirmation = new NavigationLink('/signup-confirmation', 'RegisterSuccess');
    public static Activate = new NavigationLink('/activate', 'Activate');
    public static ForgotPassword = new NavigationLink('/forgot-password', 'Forgot Password');
    public static ResetPassword = new NavigationLink('/password-recovery', 'Reset Password');
    public static Authorize = new NavigationLink('/oauth/v2/authorize', 'Authorize');
    public static Account = new NavigationLink('/account', 'Account');
    public static AccountSettings = new NavigationLink('/account-settings', 'Account Settings');
    public static ProjectDashboard = new NavigationLink('/project-dashboard', 'Dashboard');
    public static Team = new NavigationLink('/team', 'Team');
    public static OnboardingTour = new NavigationLink('/onboarding-tour', 'Onboarding Tour');
    public static CreateProject = new NavigationLink('/create-project', 'Create Project');
    public static EditProjectDetails = new NavigationLink('/edit-project-details', 'Edit Project Details');
    public static AccessGrants = new NavigationLink('/access-grants', 'Access');
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
    public static BillingHistory2 = new NavigationLink('billing-history', 'Billing History 2');
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

    // buckets child paths.
    public static BucketsManagement = new NavigationLink('management', 'Buckets Management');
    public static BucketsDetails = new NavigationLink('details', 'Bucket Details');
    public static UploadFile = new NavigationLink('upload/', 'Objects Upload');
    public static UploadFileChildren = new NavigationLink(':pathMatch*', 'Objects Upload Children');
}
