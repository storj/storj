// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div v-if="isLoading" class="loading-overlay active">
        <div class="load" />
        <LoaderImage class="loading-icon" />
    </div>
    <div v-else ref="dashboardContent" class="all-dashboard">
        <div class="all-dashboard__bars">
            <BetaSatBar v-if="isBetaSatellite" />
            <MFARecoveryCodeBar v-if="showMFARecoveryCodeBar" :open-generate-modal="generateNewMFARecoveryCodes" />
        </div>

        <heading class="all-dashboard__heading" />

        <div class="all-dashboard__content">
            <div class="all-dashboard__content__divider" />

            <div class="all-dashboard__banners">
                <BillingNotification v-if="isBillingNotificationShown" class="all-dashboard__banners__billing" />

                <UpgradeNotification
                    v-if="isPaidTierBannerShown"
                    class="all-dashboard__banners__upgrade"
                    :open-add-p-m-modal="togglePMModal"
                />

                <ProjectLimitBanner
                    v-if="isProjectLimitBannerShown"
                    class="all-dashboard__banners__project-limit"
                    :dashboard-ref="dashboardContent"
                    :on-upgrade-clicked="togglePMModal"
                />

                <v-banner
                    v-if="isAccountFrozen && !isLoading && dashboardContent"
                    class="all-dashboard__banners__freeze"
                    severity="critical"
                    :dashboard-ref="dashboardContent"
                >
                    <template #text>
                        <p class="medium">Your account was frozen due to billing issues. Please update your payment information.</p>
                        <p class="link" @click.stop.self="redirectToBillingPage">To Billing Page</p>
                    </template>
                </v-banner>

                <v-banner
                    v-if="limitState.hundredIsShown && !isLoading && dashboardContent"
                    class="all-dashboard__banners__hundred-limit"
                    severity="critical"
                    :on-click="() => setIsHundredLimitModalShown(true)"
                    :dashboard-ref="dashboardContent"
                >
                    <template #text>
                        <p class="medium">{{ limitState.hundredLabel }}</p>
                        <p class="link" @click.stop.self="togglePMModal">Upgrade now</p>
                    </template>
                </v-banner>

                <v-banner
                    v-if="limitState.eightyIsShown && !isLoading && dashboardContent"
                    class="all-dashboard__banners__eighty-limit"
                    severity="warning"
                    :on-click="() => setIsEightyLimitModalShown(true)"
                    :dashboard-ref="dashboardContent"
                >
                    <template #text>
                        <p class="medium">{{ limitState.eightyLabel }}</p>
                        <p class="link" @click.stop.self="togglePMModal">Upgrade now</p>
                    </template>
                </v-banner>
            </div>

            <router-view />

            <limit-warning-modal
                v-if="isHundredLimitModalShown && !isLoading"
                severity="critical"
                :on-close="() => setIsHundredLimitModalShown(false)"
                :title="limitState.hundredModalTitle"
                :limit-type="limitState.hundredModalLimitType"
                :on-upgrade="togglePMModal"
            />
            <limit-warning-modal
                v-if="isEightyLimitModalShown && !isLoading"
                severity="warning"
                :on-close="() => setIsEightyLimitModalShown(false)"
                :title="limitState.eightyModalTitle"
                :limit-type="limitState.eightyModalLimitType"
                :on-upgrade="togglePMModal"
            />
            <AllModals />
        </div>
        <InactivityModal
            v-if="inactivityModalShown"
            :on-continue="refreshSession"
            :on-logout="handleInactive"
            :on-close="closeInactivityModal"
            :initial-seconds="inactivityModalTime / 1000"
        />
    </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';

import { MODALS } from '@/utils/constants/appStatePopUps';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { User } from '@/types/users';
import {
    AnalyticsErrorEventSource,
} from '@/utils/constants/analyticsEventNames';
import { AnalyticsHttpApi } from '@/api/analytics';
import { useNotify, useRoute, useRouter, useStore } from '@/utils/hooks';
import { RouteConfig } from '@/router';
import { USER_ACTIONS } from '@/store/modules/users';
import {
    APP_STATE_ACTIONS,
    NOTIFICATION_ACTIONS,
    PM_ACTIONS,
} from '@/utils/constants/actionNames';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { OBJECTS_ACTIONS } from '@/store/modules/objects';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { ErrorUnauthorized } from '@/api/errors/ErrorUnauthorized';
import { MetaUtils } from '@/utils/meta';
import { FetchState } from '@/utils/constants/fetchStateEnum';
import { LocalData } from '@/utils/localData';
import { CouponType } from '@/types/coupons';
import { AuthHttpApi } from '@/api/auth';
import Heading from '@/views/all-dashboard/components/Heading.vue';
import { useABTestingStore } from '@/store/modules/abTestingStore';

import BillingNotification from '@/components/notifications/BillingNotification.vue';
import InactivityModal from '@/components/modals/InactivityModal.vue';
import BetaSatBar from '@/components/infoBars/BetaSatBar.vue';
import MFARecoveryCodeBar from '@/components/infoBars/MFARecoveryCodeBar.vue';
import AllModals from '@/components/modals/AllModals.vue';
import LimitWarningModal from '@/components/modals/LimitWarningModal.vue';
import VBanner from '@/components/common/VBanner.vue';
import UpgradeNotification from '@/components/notifications/UpgradeNotification.vue';
import ProjectLimitBanner from '@/components/notifications/ProjectLimitBanner.vue';

import LoaderImage from '@/../static/images/common/loadIcon.svg';

const {
    SETUP_ACCOUNT,
    GET_CREDIT_CARDS,
} = PAYMENTS_ACTIONS;

const router = useRouter();
const store = useStore();
const notify = useNotify();
const abTestingStore = useABTestingStore();
const analytics = new AnalyticsHttpApi();
const auth: AuthHttpApi = new AuthHttpApi();

const inactivityModalTime = 60000;
const sessionDuration: number = parseInt(MetaUtils.getMetaContent('inactivity-timer-duration')) * 1000;
const sessionRefreshInterval: number = sessionDuration / 2;
const debugTimerShown = MetaUtils.getMetaContent('inactivity-timer-viewer-enabled') === 'true';
// Minimum number of recovery codes before the recovery code warning bar is shown.
const recoveryCodeWarningThreshold = 4;

const inactivityTimerId = ref<ReturnType<typeof setTimeout> | null>(null);
const sessionRefreshTimerId = ref<ReturnType<typeof setTimeout> | null>(null);
const debugTimerId = ref<ReturnType<typeof setTimeout> | null>(null);
const debugTimerText = ref<string>('');
const resetActivityEvents: string[] = ['keypress', 'mousemove', 'mousedown', 'touchmove'];
const inactivityModalShown = ref<boolean>(false);
const isSessionActive = ref<boolean>(false);
const isSessionRefreshing = ref<boolean>(false);
const isHundredLimitModalShown = ref<boolean>(false);
const isEightyLimitModalShown = ref<boolean>(false);
const dashboardContent = ref<HTMLElement | null>(null);

/**
 * Indicates if account was frozen due to billing issues.
 */
const isAccountFrozen = computed((): boolean => {
    return store.state.usersModule.user.isFrozen;
});

/**
 * Returns all needed information for limit banner and modal when bandwidth or storage close to limits.
 */
const limitState = computed((): { eightyIsShown: boolean, hundredIsShown: boolean, eightyLabel?: string, eightyModalTitle?: string, eightyModalLimitType?: string, hundredLabel?: string, hundredModalTitle?: string, hundredModalLimitType?: string  } => {
    if (store.state.usersModule.user.paidTier || isAccountFrozen.value) return { eightyIsShown: false, hundredIsShown: false };

    const result:
        {
            eightyIsShown: boolean,
            hundredIsShown: boolean,
            eightyLabel?: string,
            eightyModalTitle?: string,
            eightyModalLimitType?: string,
            hundredLabel?: string,
            hundredModalTitle?: string,
            hundredModalLimitType?: string

        } = { eightyIsShown: false, hundredIsShown: false, eightyLabel: '', hundredLabel: '' };

    const { currentLimits } = store.state.projectsModule;

    const limitTypeArr = [
        { name: 'bandwidth', usedPercent: Math.round(currentLimits.bandwidthUsed * 100 / currentLimits.bandwidthLimit) },
        { name: 'storage', usedPercent: Math.round(currentLimits.storageUsed * 100 / currentLimits.storageLimit) },
        { name: 'segment', usedPercent: Math.round(currentLimits.segmentUsed * 100 / currentLimits.segmentLimit) },
    ];

    const hundredPercent: string[] = [];
    const eightyPercent: string[] = [];

    limitTypeArr.forEach((limitType) => {
        if (limitType.usedPercent >= 80) {
            if (limitType.usedPercent >= 100) {
                hundredPercent.push(limitType.name);
            } else {
                eightyPercent.push(limitType.name);
            }
        }
    });

    if (eightyPercent.length !== 0) {
        result.eightyIsShown = true;

        const eightyPercentString = eightyPercent.join(' and ');

        result.eightyLabel = `You've used 80% of your ${eightyPercentString} limit. Avoid interrupting your usage by upgrading your account.`;
        result.eightyModalTitle = `80% ${eightyPercentString} limit used`;
        result.eightyModalLimitType = eightyPercentString;
    }

    if (hundredPercent.length !== 0) {
        result.hundredIsShown = true;

        const hundredPercentString = hundredPercent.join(' and ');

        result.hundredLabel = `URGENT: You’ve reached the ${hundredPercentString} limit for your project. Upgrade to avoid any service interruptions.`;
        result.hundredModalTitle = `URGENT: You’ve reached the ${hundredPercentString} limit for your project.`;
        result.hundredModalLimitType = hundredPercentString;
    }

    return result;
});

/**
 * Whether the current route is the billing page.
 */
const isBillingPage = computed(() => {
    return useRoute().path.includes(RouteConfig.Billing2.path);
});

/**
 * Indicates whether the billing relocation notification should be shown.
 */
const isBillingNotificationShown = computed((): boolean => {
    return !isBillingPage.value
    && store.state.appStateModule.viewsState.isBillingNotificationShown;
});

/**
 * Indicates if satellite is in beta.
 */
const isBetaSatellite = computed((): boolean => {
    return store.state.appStateModule.isBetaSatellite;
});

/**
 * Indicates if loading screen is active.
 */
const isLoading = computed((): boolean => {
    return store.state.appStateModule.viewsState.fetchState === FetchState.LOADING;
});

/**
 * Indicates whether the MFA recovery code warning bar should be shown.
 */
const showMFARecoveryCodeBar = computed((): boolean => {
    const user: User = store.getters.user;
    return user.isMFAEnabled && user.mfaRecoveryCodeCount < recoveryCodeWarningThreshold;
});

/* whether the project limit banner should be shown. */
const isProjectLimitBannerShown = computed((): boolean => {
    return !LocalData.getProjectLimitBannerHidden()
        && !isBillingPage.value
        && (hasReachedProjectLimit.value || !store.state.usersModule.user.paidTier);
});

/**
 * Returns whether the user has reached project limits.
 */
const hasReachedProjectLimit = computed((): boolean => {
    const projectLimit: number = store.getters.user.projectLimit;
    const projectsCount: number = store.getters.projectsCount;

    return projectsCount === projectLimit;
});

/* whether the paid tier banner should be shown */
const isPaidTierBannerShown = computed((): boolean => {
    return !store.state.usersModule.user.paidTier
        && !isBillingPage.value
        && joinedWhileAgo.value;
});

/* whether the user joined more than 7 days ago */
const joinedWhileAgo = computed((): boolean => {
    const createdAt = store.state.usersModule.user.createdAt as Date | null;
    if (!createdAt) return true; // true so we can show the banner regardless
    const millisPerDay = 24 * 60 * 60 * 1000;
    return ((Date.now() - createdAt.getTime()) / millisPerDay) > 7;
});

function setIsEightyLimitModalShown(value: boolean): void {
    isEightyLimitModalShown.value = value;
}

function setIsHundredLimitModalShown(value: boolean): void {
    isHundredLimitModalShown.value = value;
}

/**
 * Redirects to Billing Page.
 */
async function redirectToBillingPage(): Promise<void> {
    await router.push(RouteConfig.AccountSettings.with(RouteConfig.Billing2.with(RouteConfig.BillingPaymentMethods2)).path);
}

/**
 * Performs logout and cleans event listeners and session timers.
 */
async function handleInactive(): Promise<void> {
    await analytics.pageVisit(RouteConfig.Login.path);
    await router.push(RouteConfig.Login.path);

    await Promise.all([
        store.dispatch(PM_ACTIONS.CLEAR),
        store.dispatch(PROJECTS_ACTIONS.CLEAR),
        store.dispatch(USER_ACTIONS.CLEAR),
        store.dispatch(ACCESS_GRANTS_ACTIONS.STOP_ACCESS_GRANTS_WEB_WORKER),
        store.dispatch(ACCESS_GRANTS_ACTIONS.CLEAR),
        store.dispatch(NOTIFICATION_ACTIONS.CLEAR),
        store.dispatch(BUCKET_ACTIONS.CLEAR),
        store.dispatch(OBJECTS_ACTIONS.CLEAR),
        store.dispatch(APP_STATE_ACTIONS.CLOSE_POPUPS),
        store.dispatch(PAYMENTS_ACTIONS.CLEAR_PAYMENT_INFO),
        abTestingStore.reset(),
        store.dispatch('files/clear'),
    ]);

    resetActivityEvents.forEach((eventName: string) => {
        document.removeEventListener(eventName, onSessionActivity);
    });
    clearSessionTimers();
    inactivityModalShown.value = false;

    try {
        await auth.logout();
    } catch (error) {
        if (error instanceof ErrorUnauthorized) return;

        await notify.error(error.message, AnalyticsErrorEventSource.OVERALL_SESSION_EXPIRED_ERROR);
    }
}

/**
 * Generates new MFA recovery codes and toggles popup visibility.
 */
async function generateNewMFARecoveryCodes(): Promise<void> {
    try {
        await store.dispatch(USER_ACTIONS.GENERATE_USER_MFA_RECOVERY_CODES);
        toggleMFARecoveryModal();
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.ALL_PROJECT_DASHBOARD);
    }
}

/**
 * Toggles MFA recovery modal visibility.
 */
function toggleMFARecoveryModal(): void {
    store.commit(APP_STATE_MUTATIONS.UPDATE_ACTIVE_MODAL, MODALS.mfaRecovery);
}

/**
 * Disables session inactivity modal visibility.
 */
function closeInactivityModal(): void {
    inactivityModalShown.value = false;
}

/**
 * Opens add payment method modal.
 */
function togglePMModal(): void {
    isHundredLimitModalShown.value = false;
    isEightyLimitModalShown.value = false;
    store.commit(APP_STATE_MUTATIONS.UPDATE_ACTIVE_MODAL, MODALS.addPaymentMethod);
}

/**
 * Clears timers associated with session refreshing and inactivity.
 */
function clearSessionTimers(): void {
    [inactivityTimerId.value, sessionRefreshTimerId.value, debugTimerId.value].forEach(id => {
        if (id !== null) clearTimeout(id);
    });
}

/**
 * Adds DOM event listeners and starts session timers.
 */
function setupSessionTimers(): void {
    const isInactivityTimerEnabled = MetaUtils.getMetaContent('inactivity-timer-enabled');

    if (isInactivityTimerEnabled === 'false') return;

    const expiresAt = LocalData.getSessionExpirationDate();

    if (expiresAt) {
        resetActivityEvents.forEach((eventName: string) => {
            document.addEventListener(eventName, onSessionActivity, false);
        });

        if (expiresAt.getTime() - sessionDuration + sessionRefreshInterval < Date.now()) {
            refreshSession();
        }

        restartSessionTimers();
    }
}

/**
 * Restarts timers associated with session refreshing and inactivity.
 */
function restartSessionTimers(): void {
    sessionRefreshTimerId.value = setTimeout(async () => {
        sessionRefreshTimerId.value = null;
        if (isSessionActive.value) {
            await refreshSession();
        }
    }, sessionRefreshInterval);

    inactivityTimerId.value = setTimeout(async () => {
        if (store.getters['files/uploadingLength']) {
            await refreshSession();
            return;
        }

        if (isSessionActive.value) return;
        inactivityModalShown.value = true;
        inactivityTimerId.value = setTimeout(async () => {
            await handleInactive();
            await notify.notify('Your session was timed out.');
        }, inactivityModalTime);
    }, sessionDuration - inactivityModalTime);

    if (!debugTimerShown) return;

    const debugTimer = () => {
        const expiresAt = LocalData.getSessionExpirationDate();

        if (expiresAt) {
            const ms = Math.max(0, expiresAt.getTime() - Date.now());
            const secs = Math.floor(ms / 1000) % 60;

            debugTimerText.value = `${Math.floor(ms / 60000)}:${(secs < 10 ? '0' : '') + secs}`;

            if (ms > 1000) {
                debugTimerId.value = setTimeout(debugTimer, 1000);
            }
        }
    };

    debugTimer();
}

/**
 * Refreshes session and resets session timers.
 */
async function refreshSession(): Promise<void> {
    isSessionRefreshing.value = true;

    try {
        LocalData.setSessionExpirationDate(await auth.refreshSession());
    } catch (error) {
        await notify.error((error instanceof ErrorUnauthorized) ? 'Your session was timed out.' : error.message, AnalyticsErrorEventSource.ALL_PROJECT_DASHBOARD);
        await handleInactive();
        isSessionRefreshing.value = false;
        return;
    }

    clearSessionTimers();
    restartSessionTimers();
    inactivityModalShown.value = false;
    isSessionActive.value = false;
    isSessionRefreshing.value = false;
}

/**
 * Resets inactivity timer and refreshes session if necessary.
 */
async function onSessionActivity(): Promise<void> {
    if (inactivityModalShown.value || isSessionActive.value) return;

    if (sessionRefreshTimerId.value === null && !isSessionRefreshing.value) {
        await refreshSession();
    }

    isSessionActive.value = true;
}

/**
 * Lifecycle hook after initial render.
 * Pre fetches user`s and project information.
 */
onMounted(async () => {
    store.subscribeAction((action) => {
        if (action.type === USER_ACTIONS.CLEAR) clearSessionTimers();
    });

    if (LocalData.getBillingNotificationAcknowledged()) {
        store.commit(APP_STATE_MUTATIONS.CLOSE_BILLING_NOTIFICATION);
    } else {
        const unsub = store.subscribe((action) => {
            if (action.type === APP_STATE_MUTATIONS.CLOSE_BILLING_NOTIFICATION) {
                LocalData.setBillingNotificationAcknowledged();
                unsub();
            }
        });
    }

    try {
        await store.dispatch(USER_ACTIONS.GET);
        await store.dispatch(USER_ACTIONS.GET_FROZEN_STATUS);
        await abTestingStore.fetchValues();
        await store.dispatch(USER_ACTIONS.GET_SETTINGS);
        setupSessionTimers();
    } catch (error) {
        store.subscribeAction((action) => {
            if (action.type === USER_ACTIONS.LOGIN) setupSessionTimers();
        });

        if (!(error instanceof ErrorUnauthorized)) {
            await store.dispatch(APP_STATE_ACTIONS.CHANGE_FETCH_STATE, FetchState.ERROR);
            await notify.error(error.message, AnalyticsErrorEventSource.ALL_PROJECT_DASHBOARD);
        }

        setTimeout(async () => await router.push(RouteConfig.Login.path), 1000);

        return;
    }

    try {
        await store.dispatch(ACCESS_GRANTS_ACTIONS.STOP_ACCESS_GRANTS_WEB_WORKER);
        await store.dispatch(ACCESS_GRANTS_ACTIONS.SET_ACCESS_GRANTS_WEB_WORKER);
    } catch (error) {
        await notify.error(`Unable to set access grants wizard. ${error.message}`, AnalyticsErrorEventSource.ALL_PROJECT_DASHBOARD);
    }

    try {
        const couponType = await store.dispatch(SETUP_ACCOUNT);
        if (couponType === CouponType.NoCoupon) {
            await notify.error(`The coupon code was invalid, and could not be applied to your account`, AnalyticsErrorEventSource.ALL_PROJECT_DASHBOARD);
        }

        if (couponType === CouponType.SignupCoupon) {
            await notify.success(`The coupon code was added successfully`);
        }
    } catch (error) {
        await notify.error(`Unable to setup account. ${error.message}`, AnalyticsErrorEventSource.ALL_PROJECT_DASHBOARD);
    }

    try {
        await store.dispatch(GET_CREDIT_CARDS);
    } catch (error) {
        await notify.error(`Unable to get credit cards. ${error.message}`, AnalyticsErrorEventSource.ALL_PROJECT_DASHBOARD);
    }

    try {
        await store.dispatch(PROJECTS_ACTIONS.FETCH);
    } catch (error) {
        return;
    }

    await store.dispatch(APP_STATE_ACTIONS.CHANGE_FETCH_STATE, FetchState.LOADED);

    if (store.getters.shouldOnboard && !store.state.appStateModule.viewsState.hasShownPricingPlan) {
        store.commit(APP_STATE_MUTATIONS.SET_HAS_SHOWN_PRICING_PLAN, true);
        // if the user is not legible for a pricing plan, they'll automatically be
        // navigated back to all projects dashboard.
        analytics.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.PricingPlanStep).path);
        await router.push(RouteConfig.OnboardingTour.with(RouteConfig.PricingPlanStep).path);
    }
});

onBeforeUnmount(() => {
    clearSessionTimers();
    resetActivityEvents.forEach((eventName: string) => {
        document.removeEventListener(eventName, onSessionActivity);
    });
});
</script>

<style scoped lang="scss">
@keyframes rotate {

    from {
        transform: rotate(0deg);
    }

    to {
        transform: rotate(360deg);
    }
}

.all-dashboard {
    box-sizing: border-box;
    overflow-y: auto;
    width: 100%;
    height: 100%;

    &__bars {
        display: contents;
        position: fixed;
        width: 100%;
        top: 0;
        z-index: 1000;
    }

    &__heading {
        margin: 50px auto 0;
        padding: 0 20px;
        max-width: 1200px;
        box-sizing: border-box;

        @media screen and (max-width: 500px) {
            margin-top: 0;
            padding: 0;
        }
    }

    &__content {
        padding: 0 20px 50px;
        margin: 0 auto;
        max-width: 1200px;
        box-sizing: border-box;

        &__divider {
            margin: 20px 0;
            border: 0.5px solid var(--c-grey-2);

            @media screen and (max-width: 500px) {
                display: none;
            }
        }
    }

    &__banners {
        margin-bottom: 20px;

        &__billing {
            position: initial;
            margin-top: 20px;

            & :deep(.notification-wrap__content) {
                position: initial;
            }
        }

        &__upgrade,
        &__project-limit,
        &__freeze,
        &__hundred-limit,
        &__eighty-limit {
            margin: 20px 0 0;
        }
    }
}

.load {
    width: 90px;
    height: 90px;
    margin: auto 0;
    border: solid 3px var(--c-blue-3);
    border-radius: 50%;
    border-right-color: transparent;
    border-bottom-color: transparent;
    border-left-color: transparent;
    transition: all 0.5s ease-in;
    animation-name: rotate;
    animation-duration: 1.2s;
    animation-iteration-count: infinite;
    animation-timing-function: linear;
}

.loading-overlay {
    display: flex;
    justify-content: center;
    align-items: center;
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background-color: var(--c-white);
    visibility: hidden;
    opacity: 0;
    transition: all 0.5s linear;
}

.loading-overlay.active {
    visibility: visible;
    opacity: 1;
}

.loading-icon {
    position: absolute;
    top: 0;
    bottom: 0;
    left: 0;
    right: 0;
    margin: auto;
}

:deep(div.account-area-container) {
    padding: 0;
}
</style>
