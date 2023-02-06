// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="dashboard">
        <div v-if="isLoading" class="loading-overlay active">
            <div class="load" />
            <LoaderImage class="loading-icon" />
        </div>
        <div v-else class="dashboard__wrap">
            <div class="dashboard__wrap__main-area">
                <NavigationArea v-if="!isNavigationHidden" class="dashboard__wrap__main-area__navigation" />
                <MobileNavigation v-if="!isNavigationHidden" class="dashboard__wrap__main-area__mobile-navigation" />
                <div
                    class="dashboard__wrap__main-area__content-wrap"
                    :class="{ 'no-nav': isNavigationHidden }"
                >
                    <UpgradeNotification
                        v-if="isPaidTierBannerShown && abTestValues.hasNewUpgradeBanner"
                        :open-add-p-m-modal="togglePMModal"
                    />
                    <div ref="dashboardContent" class="dashboard__wrap__main-area__content-wrap__container">
                        <div class="bars">
                            <BetaSatBar v-if="isBetaSatellite" />
                            <PaidTierBar
                                v-if="isPaidTierBannerShown && !abTestValues.hasNewUpgradeBanner"
                                :open-add-p-m-modal="togglePMModal"
                            />
                            <ProjectInfoBar v-if="isProjectListPage" />
                            <MFARecoveryCodeBar v-if="showMFARecoveryCodeBar" :open-generate-modal="generateNewMFARecoveryCodes" />
                        </div>
                        <router-view class="dashboard__wrap__main-area__content-wrap__container__content" />
                    </div>
                    <BillingNotification v-if="isBillingNotificationShown" />
                </div>
            </div>
        </div>
        <div v-if="debugTimerShown && !isLoading" class="dashboard__debug-timer">
            <p>Remaining session time: <b class="dashboard__debug-timer__bold">{{ debugTimerText }}</b></p>
        </div>
        <InactivityModal
            v-if="inactivityModalShown"
            :on-continue="refreshSession"
            :on-logout="handleInactive"
            :on-close="closeInactivityModal"
            :initial-seconds="inactivityModalTime / 1000"
        />
        <v-banner
            v-if="isAccountFrozen && !isLoading && dashboardContent"
            severity="critical"
            :dashboard-ref="dashboardContent"
        >
            <template #text>
                <p class="medium">Your account was frozen due to billing issues. Please update your payment information.</p>
                <p class="link" @click.stop.self="redirectToBillingPage">To Billing Page</p>
            </template>
        </v-banner>
        <v-banner
            v-if="limitState.isShown && !isLoading && dashboardContent"
            :severity="limitState.severity"
            :on-click="() => setIsLimitModalShown(true)"
            :dashboard-ref="dashboardContent"
        >
            <template #text>
                <p class="medium">{{ limitState.label }}</p>
                <p class="link" @click.stop.self="togglePMModal">Upgrade now</p>
            </template>
        </v-banner>
        <limit-warning-modal
            v-if="isLimitModalShown && !isLoading"
            :severity="limitState.severity"
            :on-close="() => setIsLimitModalShown(false)"
            :title="limitState.modalTitle"
            :on-upgrade="togglePMModal"
        />
        <AllModals />
    </div>
</template>

<script setup lang="ts">
import { computed, onBeforeMount, onBeforeUnmount, onMounted, reactive, ref } from 'vue';

import { ErrorUnauthorized } from '@/api/errors/ErrorUnauthorized';
import { RouteConfig } from '@/router';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { USER_ACTIONS } from '@/store/modules/users';
import { CouponType } from '@/types/coupons';
import { CreditCard } from '@/types/payments';
import { Project } from '@/types/projects';
import { APP_STATE_ACTIONS, NOTIFICATION_ACTIONS, PM_ACTIONS } from '@/utils/constants/actionNames';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { AppState } from '@/utils/constants/appStateEnum';
import { LocalData } from '@/utils/localData';
import { User } from '@/types/users';
import { AuthHttpApi } from '@/api/auth';
import { MetaUtils } from '@/utils/meta';
import { AnalyticsHttpApi } from '@/api/analytics';
import eventBus from '@/utils/eventBus';
import { ABTestValues } from '@/types/abtesting';
import { AB_TESTING_ACTIONS } from '@/store/modules/abTesting';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { OBJECTS_ACTIONS } from '@/store/modules/objects';
import { useNotify, useRouter, useStore } from '@/utils/hooks';

import ProjectInfoBar from '@/components/infoBars/ProjectInfoBar.vue';
import BillingNotification from '@/components/notifications/BillingNotification.vue';
import NavigationArea from '@/components/navigation/NavigationArea.vue';
import InactivityModal from '@/components/modals/InactivityModal.vue';
import BetaSatBar from '@/components/infoBars/BetaSatBar.vue';
import MFARecoveryCodeBar from '@/components/infoBars/MFARecoveryCodeBar.vue';
import PaidTierBar from '@/components/infoBars/PaidTierBar.vue';
import AllModals from '@/components/modals/AllModals.vue';
import MobileNavigation from '@/components/navigation/MobileNavigation.vue';
import LimitWarningModal from '@/components/modals/LimitWarningModal.vue';
import VBanner from '@/components/common/VBanner.vue';
import UpgradeNotification from '@/components/notifications/UpgradeNotification.vue';

import LoaderImage from '@/../static/images/common/loadIcon.svg';

const {
    SETUP_ACCOUNT,
    GET_CREDIT_CARDS,
} = PAYMENTS_ACTIONS;

const store = useStore();
// TODO: will be swapped with useRouter from new version of router. remove after vue-router version upgrade.
const nativeRouter = useRouter();
const notify = useNotify();

const router = reactive(nativeRouter);

const auth: AuthHttpApi = new AuthHttpApi();
const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();
const resetActivityEvents: string[] = ['keypress', 'mousemove', 'mousedown', 'touchmove'];
const inactivityModalTime = 60000;
const ACTIVITY_REFRESH_TIME_LIMIT = 180000;
const sessionDuration: number = parseInt(MetaUtils.getMetaContent('inactivity-timer-duration')) * 1000;
const sessionRefreshInterval: number = sessionDuration / 2;
const debugTimerShown = MetaUtils.getMetaContent('inactivity-timer-viewer-enabled') == 'true';
// Minimum number of recovery codes before the recovery code warning bar is shown.
const recoveryCodeWarningThreshold = 4;

const inactivityTimerId = ref<ReturnType<typeof setTimeout> | null>();
const sessionRefreshTimerId = ref<ReturnType<typeof setTimeout> | null>();
const debugTimerId = ref<ReturnType<typeof setTimeout> | null>();
const inactivityModalShown = ref<boolean>(false);
const isSessionActive = ref<boolean>(false);
const isSessionRefreshing = ref<boolean>(false);
const isLimitModalShown = ref<boolean>(false);
const debugTimerText = ref<string>('');

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
const limitState = computed((): { isShown: boolean, severity?: 'info' | 'warning' | 'critical', label?: string, modalTitle?: string } => {
    if (store.state.usersModule.user.paidTier || isAccountFrozen.value) return { isShown: false };

    const EIGHTY_PERCENT = 80;
    const HUNDRED_PERCENT = 100;

    const result: { isShown: boolean, severity?: 'info' | 'warning' | 'critical', label?: string, modalTitle?: string  } = { isShown: false, label: '' };
    const { currentLimits } = store.state.projectsModule;

    const bandwidthUsedPercent = Math.round(currentLimits.bandwidthUsed * HUNDRED_PERCENT / currentLimits.bandwidthLimit);
    const storageUsedPercent = Math.round(currentLimits.storageUsed * HUNDRED_PERCENT / currentLimits.storageLimit);

    const isLimitHigh = bandwidthUsedPercent >= EIGHTY_PERCENT || storageUsedPercent >= EIGHTY_PERCENT;
    const isLimitCritical = bandwidthUsedPercent === HUNDRED_PERCENT || storageUsedPercent === HUNDRED_PERCENT;

    if (isLimitHigh) {
        result.isShown = true;
        result.severity = isLimitCritical ? 'critical' : 'warning';

        if (bandwidthUsedPercent > storageUsedPercent) {
            result.label = bandwidthUsedPercent === HUNDRED_PERCENT ?
                'URGENT: You’ve reached the bandwidth limit for your project. Avoid any service interruptions.'
                : `You’ve used ${bandwidthUsedPercent}% of your bandwidth limit. Avoid interrupting your usage by upgrading account.`;
            result.modalTitle = `You’ve used ${bandwidthUsedPercent}% of your free account bandwidth`;
        } else if (bandwidthUsedPercent < storageUsedPercent) {
            result.label = storageUsedPercent === HUNDRED_PERCENT ?
                'URGENT: You’ve reached the storage limit for your project. Avoid any service interruptions.'
                : `You’ve used ${storageUsedPercent}% of your storage limit. Avoid interrupting your usage by upgrading account.`;
            result.modalTitle = `You’ve used ${storageUsedPercent}% of your free account storage`;
        } else {
            result.label = storageUsedPercent === HUNDRED_PERCENT && bandwidthUsedPercent === HUNDRED_PERCENT ?
                'URGENT: You’ve reached the storage and bandwidth limits for your project. Avoid any service interruptions.'
                : `You’ve used ${storageUsedPercent}% of your storage and ${bandwidthUsedPercent}% of bandwidth limit. Avoid interrupting your usage by upgrading account.`;
            result.modalTitle = `You’ve used ${storageUsedPercent}% storage and ${bandwidthUsedPercent}%  of your free account bandwidth`;
        }
    }

    return result;
});

/**
 * Indicates if navigation sidebar is hidden.
 */
const isNavigationHidden = computed((): boolean => {
    return isOnboardingTour.value || isCreateProjectPage.value;
});

const abTestValues = computed((): ABTestValues => {
    return store.state.abTestingModule.abTestValues;
});

/* whether the paid tier banner should be shown */
const isPaidTierBannerShown = computed((): boolean => {
    return !store.state.usersModule.user.paidTier && !isOnboardingTour.value;
});

/**
 * Indicates if current route is projects list page.
 */
const isProjectListPage = computed((): boolean => {
    return router.history.current?.name === RouteConfig.ProjectsList.name;
});

/**
 * Returns credit cards from store.
 */
const creditCards = computed((): CreditCard[] => {
    return store.state.paymentsModule.creditCards;
});

/**
 * Indicates if current route is onboarding tour.
 */
const isOnboardingTour = computed((): boolean => {
    return router.history.current?.path.includes(RouteConfig.OnboardingTour.path);
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
    return store.state.appStateModule.appState.fetchState === AppState.LOADING;
});

/**
 * Indicates whether the MFA recovery code warning bar should be shown.
 */
const showMFARecoveryCodeBar = computed((): boolean => {
    const user: User = store.getters.user;
    return user.isMFAEnabled && user.mfaRecoveryCodeCount < recoveryCodeWarningThreshold;
});

/**
 * Indicates whether the billing relocation notification should be shown.
 */
const isBillingNotificationShown = computed((): boolean => {
    return store.state.appStateModule.appState.isBillingNotificationShown;
});

/**
 * Indicates if current route is create project page.
 */
const isCreateProjectPage = computed((): boolean => {
    return router.history.current?.name === RouteConfig.CreateProject.name;
});

/**
 * Stores project to vuex store and browser's local storage.
 * @param projectID - project id string
 */
function storeProject(projectID: string): void {
    store.dispatch(PROJECTS_ACTIONS.SELECT, projectID);
    LocalData.setSelectedProjectId(projectID);
}

/**
 * Clears timers associated with session refreshing and inactivity.
 */
function clearSessionTimers(): void {
    [inactivityTimerId.value, sessionRefreshTimerId.value, debugTimerId.value].forEach(id => {
        if (id != null) clearTimeout(id);
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

    inactivityTimerId.value = setTimeout(() => {
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
            const secs = Math.floor(ms/1000)%60;

            debugTimerText.value = `${Math.floor(ms/60000)}:${(secs<10 ? '0' : '')+secs}`;

            if (ms > 1000) {
                debugTimerId.value = setTimeout(debugTimer, 1000);
            }
        }
    };

    debugTimer();
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
 * Used to trigger session timer update while doing not UI-related work for a long time.
 */
async function resetSessionOnLogicRelatedActivity(): Promise<void> {
    const isInactivityTimerEnabled = MetaUtils.getMetaContent('inactivity-timer-enabled');

    if (!isInactivityTimerEnabled) {
        return;
    }

    const expiresAt = LocalData.getSessionExpirationDate();

    if (expiresAt) {
        const ms = Math.max(0, expiresAt.getTime() - Date.now());

        // Isn't triggered when decent amount of session time is left.
        if (ms < ACTIVITY_REFRESH_TIME_LIMIT) {
            await refreshSession();
        }
    }
}

/**
 * Refreshes session and resets session timers.
 */
async function refreshSession(): Promise<void> {
    isSessionRefreshing.value = true;

    try {
        LocalData.setSessionExpirationDate(await auth.refreshSession());
    } catch (error) {
        await notify.error((error instanceof ErrorUnauthorized) ? 'Your session was timed out.' : error.message, AnalyticsErrorEventSource.OVERALL_SESSION_EXPIRED_ERROR);
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
        store.dispatch(AB_TESTING_ACTIONS.RESET),
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

function setIsLimitModalShown(value: boolean): void {
    isLimitModalShown.value = value;
}

/**
 * Toggles MFA recovery modal visibility.
 */
function toggleMFARecoveryModal(): void {
    store.commit(APP_STATE_MUTATIONS.TOGGLE_MFA_RECOVERY_MODAL_SHOWN);
}

/**
 * Generates new MFA recovery codes and toggles popup visibility.
 */
async function generateNewMFARecoveryCodes(): Promise<void> {
    try {
        await store.dispatch(USER_ACTIONS.GENERATE_USER_MFA_RECOVERY_CODES);
        toggleMFARecoveryModal();
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
    }
}

/**
 * Opens add payment method modal.
 */
function togglePMModal(): void {
    isLimitModalShown.value = false;
    store.commit(APP_STATE_MUTATIONS.TOGGLE_IS_ADD_PM_MODAL_SHOWN);
}

/**
 * Disables session inactivity modal visibility.
 */
function closeInactivityModal(): void {
    inactivityModalShown.value = false;
}

/**
 * Redirects to Billing Page.
 */
async function redirectToBillingPage(): Promise<void> {
    await router.push(RouteConfig.Account.with(RouteConfig.Billing.with(RouteConfig.BillingPaymentMethods)).path);
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
 * Subscribes to activity events to refresh session timers.
 */
onBeforeMount(() => {
    eventBus.$on('upload_progress', () => {
        resetSessionOnLogicRelatedActivity();
    });
});

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
            if (action.type == APP_STATE_MUTATIONS.CLOSE_BILLING_NOTIFICATION) {
                LocalData.setBillingNotificationAcknowledged();
                unsub();
            }
        });
    }

    try {
        await store.dispatch(USER_ACTIONS.GET);
        await store.dispatch(USER_ACTIONS.GET_FROZEN_STATUS);
        await store.dispatch(AB_TESTING_ACTIONS.FETCH);
        setupSessionTimers();
    } catch (error) {
        store.subscribeAction((action) => {
            if (action.type == USER_ACTIONS.LOGIN) setupSessionTimers();
        });

        if (!(error instanceof ErrorUnauthorized)) {
            await store.dispatch(APP_STATE_ACTIONS.CHANGE_STATE, AppState.ERROR);
            await notify.error(error.message, AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
        }

        setTimeout(async () => await router.push(RouteConfig.Login.path), 1000);

        return;
    }

    try {
        await store.dispatch(ACCESS_GRANTS_ACTIONS.STOP_ACCESS_GRANTS_WEB_WORKER);
        await store.dispatch(ACCESS_GRANTS_ACTIONS.SET_ACCESS_GRANTS_WEB_WORKER);
    } catch (error) {
        await notify.error(`Unable to set access grants wizard. ${error.message}`, AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
    }

    try {
        const couponType = await store.dispatch(SETUP_ACCOUNT);
        if (couponType === CouponType.NoCoupon) {
            await notify.error(`The coupon code was invalid, and could not be applied to your account`, AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
        }

        if (couponType === CouponType.SignupCoupon) {
            await notify.success(`The coupon code was added successfully`);
        }
    } catch (error) {
        await notify.error(`Unable to setup account. ${error.message}`, AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
    }

    try {
        await store.dispatch(GET_CREDIT_CARDS);
    } catch (error) {
        await notify.error(`Unable to get credit cards. ${error.message}`, AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
    }

    let projects: Project[] = [];

    try {
        projects = await store.dispatch(PROJECTS_ACTIONS.FETCH);
    } catch (error) {
        return;
    }

    if (!projects.length) {
        try {
            await store.dispatch(PROJECTS_ACTIONS.CREATE_DEFAULT_PROJECT);

            await analytics.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.OverviewStep).path);
            await router.push(RouteConfig.OnboardingTour.with(RouteConfig.OverviewStep).path);

            await store.dispatch(APP_STATE_ACTIONS.CHANGE_STATE, AppState.LOADED);
        } catch (error) {
            notify.error(error.message, AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
            return;
        }

        return;
    }

    selectProject(projects);

    await store.dispatch(APP_STATE_ACTIONS.CHANGE_STATE, AppState.LOADED);
});

onBeforeUnmount(() => {
    eventBus.$off('upload_progress');
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
        background-color: #fff;
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

                        &__content {
                            max-width: 1200px;
                            margin: 0 auto;
                            padding: 48px 48px 60px;
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

    .bars {
        display: contents;
        position: fixed;
        width: 100%;
        top: 0;
        z-index: 1000;
    }

    @media screen and (max-width: 1280px) {

        .regular-navigation {
            display: none;
        }

        .no-nav {
            width: 100%;
        }
    }

    @media screen and (max-width: 800px) {

        .dashboard__wrap__main-area__content-wrap__container__content {
            padding: 32px 24px 50px;
        }
    }

    @media screen and (max-width: 500px) {

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
