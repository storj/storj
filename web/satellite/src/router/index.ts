// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { RouteRecord, createRouter, useRoute, createWebHistory } from 'vue-router';

import { NavigationLink } from '@/types/navigation';
import { useAppStore } from '@/store/modules/appStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { RouteConfig } from '@/types/router';

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
const DashboardArea = () => import('@/views/dashboard/DashboardArea.vue');
const ForgotPassword = () => import('@/views/ForgotPassword.vue');
const LoginArea = () => import('@/views/LoginArea.vue');
const RegisterArea = () => import('@/views/registration/RegisterArea.vue');
const ResetPassword = () => import('@/views/ResetPassword.vue');

const notProjectRelatedRoutes = [
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

export const router = createRouter({
    history: createWebHistory(),
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
                    path: RouteConfig.Team.path,
                    name: RouteConfig.Team.name,
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
                    path: RouteConfig.AllProjectsDashboard.path,
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

router.beforeEach(async (to, from, next) => {
    const appStore = useAppStore();
    const configStore = useConfigStore();

    if (!to.matched.length) {
        appStore.setErrorPage(404);
        return;
    } else if (appStore.state.error.visible) {
        appStore.removeErrorPage();
    }

    if (to.name === RouteConfig.ProjectDashboard.name && from.name === RouteConfig.Login.name) {
        appStore.toggleHasJustLoggedIn(true);
    }

    if (to.name === RouteConfig.AllProjectsDashboard.name && from.name === RouteConfig.Login.name) {
        appStore.toggleHasJustLoggedIn(true);
    }

    // On very first login we try to redirect user to project dashboard
    // but since there is no project we then redirect user to onboarding flow.
    // That's why we toggle this flag here back to false not show create project passphrase modal again
    // if user clicks 'Continue in web'.
    if (to.name === RouteConfig.ProjectDashboard.name && from.name === RouteConfig.OverviewStep.name) {
        appStore.toggleHasJustLoggedIn(false);
    }
    if (to.name === RouteConfig.ProjectDashboard.name && from.name === RouteConfig.AllProjectsDashboard.name) {
        appStore.toggleHasJustLoggedIn(false);
    }

    if (!configStore.state.config.billingFeaturesEnabled && to.path.includes(RouteConfig.Billing.path)) {
        next(RouteConfig.Account.with(RouteConfig.Settings).path);

        return;
    }

    if (navigateToDefaultSubTab(to.matched, RouteConfig.Account)) {
        next(RouteConfig.Account.with(RouteConfig.Billing).path);

        return;
    }

    if (navigateToDefaultSubTab(to.matched, RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep))) {
        next(RouteConfig.OnboardingTour.path);

        return;
    }

    if (navigateToDefaultSubTab(to.matched, RouteConfig.OnboardingTour)) {
        next(RouteConfig.OnboardingTour.with(configStore.firstOnboardingStep).path);

        return;
    }

    if (navigateToDefaultSubTab(to.matched, RouteConfig.Buckets)) {
        next(RouteConfig.Buckets.with(RouteConfig.BucketsManagement).path);

        return;
    }

    if (to.name === 'default') {
        next(RouteConfig.ProjectDashboard.path);

        return;
    }

    next();
});

router.afterEach(() => {
    updateTitle();
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

/**
 * Updates the title of the webpage.
 */
function updateTitle(): void {
    const configStore = useConfigStore();
    const projectsStore = useProjectsStore();
    const route = useRoute();
    const routeName = route.name as string;
    const parts = [routeName, configStore.state.config.satelliteName];

    if (routeName && !notProjectRelatedRoutes.includes(routeName)) {
        parts.unshift(projectsStore.state.selectedProject.name);
    }

    document.title = parts.filter(s => !!s).join(' | ');
}
