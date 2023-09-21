// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="dashboard">
        <BrandedLoader v-if="isLoading" />
        <SessionWrapper v-else>
            <template #default="session">
                <div class="dashboard__wrap">
                    <div class="dashboard__wrap__main-area">
                        <NavigationArea v-if="!isNavigationHidden" class="dashboard__wrap__main-area__navigation" />
                        <MobileNavigation v-if="!isNavigationHidden" class="dashboard__wrap__main-area__mobile-navigation" />
                        <div
                            class="dashboard__wrap__main-area__content-wrap"
                            :class="{ 'no-nav': isNavigationHidden }"
                        >
                            <div ref="dashboardContent" class="dashboard__wrap__main-area__content-wrap__container">
                                <BetaSatBar v-if="isBetaSatellite" />
                                <MFARecoveryCodeBar v-if="showMFARecoveryCodeBar" :open-generate-modal="toggleMFARecoveryModal" />
                                <div class="dashboard__wrap__main-area__content-wrap__container__content banners">
                                    <ProjectInvitationBanner v-if="isProjectInvitationBannerShown" />

                                    <UpgradeNotification
                                        v-if="isPaidTierBannerShown"
                                        :open-add-p-m-modal="togglePMModal"
                                    />

                                    <v-banner
                                        v-if="isAccountFrozen && !isLoading && dashboardContent"
                                        title="Your account was frozen due to billing issues."
                                        message="Please update your payment information."
                                        link-text="To Billing Page"
                                        severity="critical"
                                        :dashboard-ref="dashboardContent"
                                        :on-link-click="redirectToBillingPage"
                                    />

                                    <v-banner
                                        v-if="isAccountWarned && !isLoading && dashboardContent"
                                        title="Your account will be frozen soon due to billing issues."
                                        message="Please update your payment information."
                                        link-text="To Billing Page"
                                        severity="warning"
                                        :dashboard-ref="dashboardContent"
                                        :on-link-click="redirectToBillingPage"
                                    />

                                    <limit-warning-banners
                                        v-if="dashboardContent"
                                        :reached-thresholds="reachedThresholds"
                                        :dashboard-ref="dashboardContent"
                                        :on-upgrade-click="togglePMModal"
                                        :on-banner-click="thresh => limitModalThreshold = thresh"
                                    />
                                </div>
                                <router-view class="dashboard__wrap__main-area__content-wrap__container__content" />
                                <div class="dashboard__wrap__main-area__content-wrap__container__content banners-bottom">
                                    <UploadNotification
                                        v-if="isLargeUploadWarningNotificationShown"
                                        wording-bold="Trying to upload a large file?"
                                        wording="Check the recommendations for your use case"
                                        :notification-icon="WarningIcon"
                                        info-notification
                                        :on-close-click="onWarningNotificationCloseClick"
                                    />
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
                <div v-if="session.debugTimerShown.value && !isLoading" class="dashboard__debug-timer">
                    <p>Remaining session time: <b class="dashboard__debug-timer__bold">{{ session.debugTimerText.value }}</b></p>
                </div>
                <limit-warning-modal
                    v-if="limitModalThreshold && !isLoading"
                    :reached-thresholds="reachedThresholds"
                    :threshold="limitModalThreshold"
                    :on-close="() => limitModalThreshold = null"
                    :on-upgrade="togglePMModal"
                />
                <AllModals />
                <ObjectsUploadingModal v-if="isObjectsUploadModal" />
            </template>
        </SessionWrapper>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, toRaw } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { ErrorUnauthorized } from '@/api/errors/ErrorUnauthorized';
import { RouteConfig } from '@/types/router';
import { CouponType } from '@/types/coupons';
import { LimitThreshold, LimitType, Project, LimitThresholdsReached } from '@/types/projects';
import { FetchState } from '@/utils/constants/fetchStateEnum';
import { LocalData } from '@/utils/localData';
import { User } from '@/types/users';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useABTestingStore } from '@/store/modules/abTestingStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { useAppStore } from '@/store/modules/appStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { DEFAULT_PROJECT_LIMITS, useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { Memory } from '@/utils/bytesSize';

import UploadNotification from '@/components/notifications/UploadNotification.vue';
import NavigationArea from '@/components/navigation/NavigationArea.vue';
import SessionWrapper from '@/components/utils/SessionWrapper.vue';
import BetaSatBar from '@/components/infoBars/BetaSatBar.vue';
import MFARecoveryCodeBar from '@/components/infoBars/MFARecoveryCodeBar.vue';
import AllModals from '@/components/modals/AllModals.vue';
import MobileNavigation from '@/components/navigation/MobileNavigation.vue';
import LimitWarningModal from '@/components/modals/LimitWarningModal.vue';
import VBanner from '@/components/common/VBanner.vue';
import UpgradeNotification from '@/components/notifications/UpgradeNotification.vue';
import ProjectInvitationBanner from '@/components/notifications/ProjectInvitationBanner.vue';
import BrandedLoader from '@/components/common/BrandedLoader.vue';
import ObjectsUploadingModal from '@/components/modals/objectUpload/ObjectsUploadingModal.vue';
import LimitWarningBanners from '@/views/dashboard/components/LimitWarningBanners.vue';

import WarningIcon from '@/../static/images/notifications/circleWarning.svg';

const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();
const appStore = useAppStore();
const agStore = useAccessGrantsStore();
const billingStore = useBillingStore();
const usersStore = useUsersStore();
const abTestingStore = useABTestingStore();
const projectsStore = useProjectsStore();

const notify = useNotify();
const router = useRouter();
const route = useRoute();

// Minimum number of recovery codes before the recovery code warning bar is shown.
const recoveryCodeWarningThreshold = 4;

const limitModalThreshold = ref<LimitThreshold | null>(null);

const dashboardContent = ref<HTMLElement | null>(null);

/**
 * Indicates whether objects upload modal should be shown.
 */
const isObjectsUploadModal = computed((): boolean => {
    return configStore.state.config.newUploadModalEnabled && appStore.state.isUploadingModal;
});

/**
 * Indicates if account was frozen due to billing issues.
 */
const isAccountFrozen = computed((): boolean => {
    return usersStore.state.user.freezeStatus.frozen;
});

/**
 * Indicates if account was warned due to billing issues.
 */
const isAccountWarned = computed((): boolean => {
    return usersStore.state.user.freezeStatus.warned;
});

/**
 * Returns which limit thresholds have been reached by which usage limit type.
 */
const reachedThresholds = computed((): LimitThresholdsReached => {
    const reached: LimitThresholdsReached = {
        Eighty: [],
        Hundred: [],
        CustomEighty: [],
        CustomHundred: [],
    };

    const currentLimits = projectsStore.state.currentLimits;
    const config = configStore.state.config;

    if (isAccountFrozen.value || currentLimits === DEFAULT_PROJECT_LIMITS) return reached;

    type LimitInfo = {
        used: number;
        currentLimit: number;
        paidLimit?: number;
    };

    const info: Record<LimitType, LimitInfo> = {
        Storage: {
            used: currentLimits.storageUsed,
            currentLimit: currentLimits.storageLimit,
            paidLimit: parseConfigLimit(config.defaultPaidStorageLimit),
        },
        Egress: {
            used: currentLimits.bandwidthUsed,
            currentLimit: currentLimits.bandwidthLimit,
            paidLimit: parseConfigLimit(config.defaultPaidBandwidthLimit),
        },
        Segment: {
            used: currentLimits.segmentUsed,
            currentLimit: currentLimits.segmentLimit,
        },
    };

    (Object.entries(info) as [LimitType, LimitInfo][]).forEach(([limitType, info]) => {
        const maxLimit = (isPaidTier.value && info.paidLimit) ? Math.max(info.currentLimit, info.paidLimit) : info.currentLimit;
        if (info.used >= maxLimit) {
            reached.Hundred.push(limitType);
        } else if (info.used >= 0.8 * maxLimit) {
            reached.Eighty.push(limitType);
        } else if (isPaidTier.value) {
            if (info.used >= info.currentLimit) {
                reached.CustomHundred.push(limitType);
            } else if (info.used >= 0.8 * info.currentLimit) {
                reached.CustomEighty.push(limitType);
            }
        }
    });

    return reached;
});

/**
 * Indicates if navigation sidebar is hidden.
 */
const isNavigationHidden = computed((): boolean => {
    return isOnboardingTour.value || isCreateProjectPage.value;
});

/* whether the paid tier banner should be shown */
const isPaidTierBannerShown = computed((): boolean => {
    return !isPaidTier.value
        && !isOnboardingTour.value
        && joinedWhileAgo.value
        && isDashboardPage.value;
});

/* whether the user joined more than 7 days ago */
const joinedWhileAgo = computed((): boolean => {
    const createdAt = usersStore.state.user.createdAt as Date | null;
    if (!createdAt) return true; // true so we can show the banner regardless
    const millisPerDay = 24 * 60 * 60 * 1000;
    return ((Date.now() - createdAt.getTime()) / millisPerDay) > 7;
});

/**
 * Indicates if current route is onboarding tour.
 */
const isOnboardingTour = computed((): boolean => {
    return route.path.includes(RouteConfig.OnboardingTour.path);
});

/**
 * Indicates if satellite is in beta.
 */
const isBetaSatellite = computed((): boolean => {
    return configStore.state.config.isBetaSatellite;
});

/**
 * Indicates if loading screen is active.
 */
const isLoading = computed((): boolean => {
    return appStore.state.fetchState === FetchState.LOADING;
});

/**
 * Indicates whether the MFA recovery code warning bar should be shown.
 */
const showMFARecoveryCodeBar = computed((): boolean => {
    const user: User = usersStore.state.user;
    return user.isMFAEnabled && user.mfaRecoveryCodeCount < recoveryCodeWarningThreshold;
});

/**
 * Indicates whether the large upload warning notification should be shown (file uploading exceeds 1GB).
 */
const isLargeUploadWarningNotificationShown = computed((): boolean => {
    return appStore.state.isLargeUploadWarningNotificationShown;
});

/**
 * Indicates whether the project member invitation banner should be shown.
 */
const isProjectInvitationBannerShown = computed((): boolean => {
    return !configStore.state.config.allProjectsDashboard;
});

/**
 * Indicates if current route is create project page.
 */
const isCreateProjectPage = computed((): boolean => {
    return route.name === RouteConfig.CreateProject.name;
});

/**
 * Indicates if current route is the dashboard page.
 */
const isDashboardPage = computed((): boolean => {
    return route.name === RouteConfig.ProjectDashboard.name;
});

/**
 * Returns whether user is in the paid tier.
 */
const isPaidTier = computed((): boolean => {
    return usersStore.state.user.paidTier;
});

/**
 * Returns the URL for the general request page from the store.
 */
const requestURL = computed((): string => {
    return configStore.state.config.generalRequestURL;
});

/**
 * Closes upload large files warning notification.
 */
function onWarningNotificationCloseClick(): void {
    appStore.setLargeUploadWarningNotification(false);
}

/**
 * Stores project to vuex store and browser's local storage.
 * @param projectID - project id string
 */
function storeProject(projectID: string): void {
    projectsStore.selectProject(projectID);
    LocalData.setSelectedProjectId(projectID);
}

/**
 * Checks if stored project is in fetched projects array and selects it.
 * Selects first fetched project if check is not successful.
 * @param fetchedProjects - fetched projects array
 */
function selectProject(fetchedProjects: Project[]): void {
    const storedProjectID = LocalData.getSelectedProjectId();
    const isProjectInFetchedProjects = fetchedProjects.some(project => project.id === storedProjectID);
    if (storedProjectID && isProjectInFetchedProjects) {
        storeProject(storedProjectID);

        return;
    }

    // Length of fetchedProjects array is checked before selectProject() function call.
    storeProject(fetchedProjects[0].id);
}

/**
 * Toggles MFA recovery modal visibility.
 */
function toggleMFARecoveryModal(): void {
    appStore.updateActiveModal(MODALS.mfaRecovery);
}

/**
 * Opens add payment method modal.
 */
function togglePMModal(): void {
    if (isPaidTier.value) return;
    appStore.updateActiveModal(MODALS.upgradeAccount);
}

/**
 * Redirects to Billing Page.
 */
async function redirectToBillingPage(): Promise<void> {
    await router.push(RouteConfig.Account.with(RouteConfig.Billing.with(RouteConfig.BillingPaymentMethods)).path);
}

/**
 * Parses limit value from config, returning it as a byte amount.
 */
function parseConfigLimit(limit: string): number {
    const [value, unit] = limit.split(' ');
    return parseFloat(value) * Memory[unit === 'B' ? 'Bytes' : unit];
}

/**
 * Lifecycle hook after initial render.
 * Pre-fetches user`s and project information.
 */
onMounted(async () => {
    try {
        await Promise.all([
            usersStore.getUser(),
            abTestingStore.fetchValues(),
            usersStore.getSettings(),
        ]);
    } catch (error) {
        if (!(error instanceof ErrorUnauthorized)) {
            appStore.changeState(FetchState.ERROR);
            notify.notifyError(error, AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
        }

        setTimeout(async () => await router.push(RouteConfig.Login.path), 1000);

        return;
    }

    try {
        agStore.stopWorker();
        await agStore.startWorker();
    } catch (error) {
        notify.error(`Unable to set access grants wizard. ${error.message}`, AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
    }

    try {
        const couponType = await billingStore.setupAccount();
        if (couponType === CouponType.NoCoupon) {
            notify.error(`The coupon code was invalid, and could not be applied to your account`, AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
        }

        if (couponType === CouponType.SignupCoupon) {
            notify.success(`The coupon code was added successfully`);
        }
    } catch (error) {
        error.message = `Unable to setup account. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
    }

    try {
        await projectsStore.getUserInvitations();
    } catch (error) {
        error.message = `Unable to get project invitations. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
    }

    let projects: Project[] = [];

    try {
        projects = await projectsStore.getProjects();
    } catch (error) {
        return;
    }

    if (projects.length) {
        selectProject(projects);
    }

    if (!configStore.state.config.allProjectsDashboard) {
        try {
            if (!projects.length) {
                await projectsStore.createDefaultProject(usersStore.state.user.id);
            }

            const onboardingPath = RouteConfig.OnboardingTour.with(configStore.firstOnboardingStep).path;
            if (usersStore.shouldOnboard && route.path !== onboardingPath) {
                analyticsStore.pageVisit(onboardingPath);
                await router.push(onboardingPath);
            }
        } catch (error) {
            notify.error(error.message, AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
            return;
        }
    }

    appStore.changeState(FetchState.LOADED);
});
</script>

<style scoped lang="scss">
    .dashboard {
        height: 100%;
        background-color: #f5f6fa;
        display: flex;
        flex-direction: column;

        &__wrap {
            display: flex;
            flex-direction: column;
            height: 100%;

            &__main-area {
                position: fixed;
                display: flex;
                width: 100%;
                height: 100%;

                &__mobile-navigation {
                    display: none;
                }

                &__content-wrap {
                    width: 100%;
                    height: 100%;
                    min-width: 0;

                    &__container {
                        height: 100%;
                        overflow-y: auto;
                        display: flex;
                        flex-direction: column;
                        align-items: center;

                        &__content {
                            max-width: 1200px;
                            padding-top: 48px;
                            padding-left: 48px;
                            padding-right: 48px;
                            box-sizing: border-box;
                            width: 100%;

                            &.banners {
                                display: flex;
                                flex-direction: column;
                                gap: 16px;

                                &:empty {
                                    display: none;
                                }
                            }

                            &.banners-bottom {
                                display: flex;
                                flex-direction: column;
                                gap: 16px;
                                padding-top: 16px;
                                padding-bottom: 48px;
                                flex-grow: 1;
                                justify-content: flex-end;

                                &:empty {
                                    padding-top: 0;
                                    padding-bottom: 0;
                                }
                            }
                        }
                    }
                }
            }
        }

        &__debug-timer {
            display: flex;
            position: absolute;
            top: 20px;
            left: 50%;
            transform: translateX(-50%);
            padding: 16px;
            z-index: 10000;
            background-color: #fec;
            font-family: 'font_regular', sans-serif;
            font-size: 14px;
            border: 1px solid var(--c-yellow-2);
            border-radius: 10px;
            box-shadow: 0 7px 20px rgba(0 0 0 / 15%);

            &__bold {
                font-family: 'font_medium', sans-serif;
            }
        }
    }

    .no-nav {
        width: 100%;
    }

    @media screen and (width <= 1280px) {

        .regular-navigation {
            display: none;
        }

        .no-nav {
            width: 100%;
        }
    }

    @media screen and (width <= 800px) {

        .dashboard__wrap__main-area__content-wrap__container__content {
            padding: 32px 24px 0;
        }
    }

    @media screen and (width <= 500px) {

        .dashboard__wrap__main-area {
            flex-direction: column;

            &__content-wrap {
                height: calc(100% - 4rem);

                &__container {
                    height: 100%;
                    margin-bottom: 0;
                }
            }

            &__navigation {
                display: none;
            }

            &__mobile-navigation {
                display: block;
            }
        }
    }
</style>
