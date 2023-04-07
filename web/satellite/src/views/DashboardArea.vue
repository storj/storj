// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="dashboard">
        <BrandedLoader v-if="isLoading" />
        <div v-else class="dashboard__wrap">
            <div class="dashboard__wrap__main-area">
                <NavigationArea v-if="!isNavigationHidden" class="dashboard__wrap__main-area__navigation" />
                <MobileNavigation v-if="!isNavigationHidden" class="dashboard__wrap__main-area__mobile-navigation" />
                <div
                    class="dashboard__wrap__main-area__content-wrap"
                    :class="{ 'no-nav': isNavigationHidden }"
                >
                    <div ref="dashboardContent" class="dashboard__wrap__main-area__content-wrap__container">
                        <BetaSatBar v-if="isBetaSatellite" />
                        <MFARecoveryCodeBar v-if="showMFARecoveryCodeBar" :open-generate-modal="generateNewMFARecoveryCodes" />
                        <div class="banner-container dashboard__wrap__main-area__content-wrap__container__content">
                            <UpgradeNotification
                                v-if="isPaidTierBannerShown"
                                :open-add-p-m-modal="togglePMModal"
                            />

                            <ProjectLimitBanner
                                v-if="isProjectLimitBannerShown"
                                :dashboard-ref="dashboardContent"
                                :on-upgrade-clicked="togglePMModal"
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
                                v-if="limitState.hundredIsShown && !isLoading && dashboardContent"
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
                        <router-view class="dashboard__wrap__main-area__content-wrap__container__content" />
                    </div>
                </div>
            </div>
        </div>
        <div v-if="debugTimerShown && !isLoading" class="dashboard__debug-timer">
            <p>Remaining session time: <b class="dashboard__debug-timer__bold">{{ debugTimerText }}</b></p>
        </div>
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
        <!-- IMPORTANT! Make sure this modal is positioned as the last element here so that it is shown on top of everything else -->
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
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue';

import { ErrorUnauthorized } from '@/api/errors/ErrorUnauthorized';
import { RouteConfig } from '@/router';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { CouponType } from '@/types/coupons';
import { Project } from '@/types/projects';
import { APP_STATE_ACTIONS, NOTIFICATION_ACTIONS } from '@/utils/constants/actionNames';
import { FetchState } from '@/utils/constants/fetchStateEnum';
import { LocalData } from '@/utils/localData';
import { User } from '@/types/users';
import { AuthHttpApi } from '@/api/auth';
import { MetaUtils } from '@/utils/meta';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { OBJECTS_ACTIONS } from '@/store/modules/objects';
import { useNotify, useRouter, useStore } from '@/utils/hooks';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { useABTestingStore } from '@/store/modules/abTestingStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useProjectMembersStore } from '@/store/modules/projectMembersStore';

import NavigationArea from '@/components/navigation/NavigationArea.vue';
import InactivityModal from '@/components/modals/InactivityModal.vue';
import BetaSatBar from '@/components/infoBars/BetaSatBar.vue';
import MFARecoveryCodeBar from '@/components/infoBars/MFARecoveryCodeBar.vue';
import AllModals from '@/components/modals/AllModals.vue';
import MobileNavigation from '@/components/navigation/MobileNavigation.vue';
import LimitWarningModal from '@/components/modals/LimitWarningModal.vue';
import VBanner from '@/components/common/VBanner.vue';
import UpgradeNotification from '@/components/notifications/UpgradeNotification.vue';
import ProjectLimitBanner from '@/components/notifications/ProjectLimitBanner.vue';
import BrandedLoader from '@/components/common/BrandedLoader.vue';

const {
    SETUP_ACCOUNT,
    GET_CREDIT_CARDS,
} = PAYMENTS_ACTIONS;

const pmStore = useProjectMembersStore();
const usersStore = useUsersStore();
const abTestingStore = useABTestingStore();
const store = useStore();
// TODO: will be swapped with useRouter from new version of router. remove after vue-router version upgrade.
const nativeRouter = useRouter();
const notify = useNotify();

const router = reactive(nativeRouter);

const auth: AuthHttpApi = new AuthHttpApi();
const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();
const resetActivityEvents: string[] = ['keypress', 'mousemove', 'mousedown', 'touchmove'];
const inactivityModalTime = 60000;
const sessionDuration: number = parseInt(MetaUtils.getMetaContent('inactivity-timer-duration')) * 1000;
const sessionRefreshInterval: number = sessionDuration / 2;
const debugTimerShown = MetaUtils.getMetaContent('inactivity-timer-viewer-enabled') === 'true';
// Minimum number of recovery codes before the recovery code warning bar is shown.
const recoveryCodeWarningThreshold = 4;

const inactivityTimerId = ref<ReturnType<typeof setTimeout> | null>();
const sessionRefreshTimerId = ref<ReturnType<typeof setTimeout> | null>();
const debugTimerId = ref<ReturnType<typeof setTimeout> | null>();
const inactivityModalShown = ref<boolean>(false);
const isSessionActive = ref<boolean>(false);
const isSessionRefreshing = ref<boolean>(false);
const isHundredLimitModalShown = ref<boolean>(false);
const isEightyLimitModalShown = ref<boolean>(false);
const debugTimerText = ref<string>('');

const dashboardContent = ref<HTMLElement | null>(null);

/**
 * Indicates if account was frozen due to billing issues.
 */
const isAccountFrozen = computed((): boolean => {
    return usersStore.state.user.isFrozen;
});

/**
 * Returns all needed information for limit banner and modal when bandwidth or storage close to limits.
 */
type LimitedState = {
    eightyIsShown: boolean;
    hundredIsShown: boolean;
    eightyLabel: string;
    eightyModalLimitType: string;
    eightyModalTitle: string;
    hundredLabel: string;
    hundredModalTitle: string;
    hundredModalLimitType: string;
}

const limitState = computed((): LimitedState => {
    const result: LimitedState = {
        eightyIsShown: false,
        hundredIsShown: false,
        eightyLabel: '',
        eightyModalLimitType: '',
        eightyModalTitle: '',
        hundredLabel: '',
        hundredModalTitle: '',
        hundredModalLimitType: '',
    };

    if (usersStore.state.user.paidTier || isAccountFrozen.value) {
        return result;
    }

    const { currentLimits } = store.state.projectsModule;

    const limitTypeArr = [
        { name: 'bandwidth', usedPercent: Math.round(currentLimits.bandwidthUsed * 100 / currentLimits.bandwidthLimit) },
        { name: 'storage', usedPercent: Math.round(currentLimits.storageUsed * 100 / currentLimits.storageLimit) },
        { name: 'segment', usedPercent: Math.round(currentLimits.segmentUsed * 100 / currentLimits.segmentLimit) },
    ];

    const hundredPercent = [] as string[];
    const eightyPercent = [] as string[];

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
 * Indicates if navigation sidebar is hidden.
 */
const isNavigationHidden = computed((): boolean => {
    return (!isAllProjectsDashboard.value && isOnboardingTour.value)
        || isCreateProjectPage.value;
});

/* whether all projects dashboard should be used */
const isAllProjectsDashboard = computed((): boolean => {
    return store.state.appStateModule.isAllProjectsDashboard;
});

/* whether the project limit banner should be shown. */
const isProjectLimitBannerShown = computed((): boolean => {
    return !LocalData.getProjectLimitBannerHidden()
        && isProjectListPage.value
        && (hasReachedProjectLimit.value || !usersStore.state.user.paidTier);
});

/**
 * Returns whether the user has reached project limits.
 */
const hasReachedProjectLimit = computed((): boolean => {
    const projectLimit: number = usersStore.state.user.projectLimit;
    const projectsCount: number = store.getters.projectsCount(usersStore.state.user.id);

    return projectsCount === projectLimit;
});

/* whether the paid tier banner should be shown */
const isPaidTierBannerShown = computed((): boolean => {
    return !usersStore.state.user.paidTier
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
 * Indicates if current route is projects list page.
 */
const isProjectListPage = computed((): boolean => {
    return router.currentRoute.name === RouteConfig.ProjectsList.name;
});

/**
 * Indicates if current route is onboarding tour.
 */
const isOnboardingTour = computed((): boolean => {
    return router.currentRoute.path.includes(RouteConfig.OnboardingTour.path);
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
    const user: User = usersStore.state.user;
    return user.isMFAEnabled && user.mfaRecoveryCodeCount < recoveryCodeWarningThreshold;
});

/**
 * Indicates if current route is create project page.
 */
const isCreateProjectPage = computed((): boolean => {
    return router.currentRoute.name === RouteConfig.CreateProject.name;
});

/**
 * Indicates if current route is the dashboard page.
 */
const isDashboardPage = computed((): boolean => {
    return router.currentRoute.name === RouteConfig.ProjectDashboard.name;
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
        pmStore.clear(),
        store.dispatch(PROJECTS_ACTIONS.CLEAR),
        usersStore.clear(),
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

function setIsEightyLimitModalShown(value: boolean): void {
    isEightyLimitModalShown.value = value;
}

function setIsHundredLimitModalShown(value: boolean): void {
    isHundredLimitModalShown.value = value;
}

/**
 * Toggles MFA recovery modal visibility.
 */
function toggleMFARecoveryModal(): void {
    store.commit(APP_STATE_MUTATIONS.UPDATE_ACTIVE_MODAL, MODALS.mfaRecovery);
}

/**
 * Generates new MFA recovery codes and toggles popup visibility.
 */
async function generateNewMFARecoveryCodes(): Promise<void> {
    try {
        await usersStore.generateUserMFARecoveryCodes();
        toggleMFARecoveryModal();
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
    }
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
 * Lifecycle hook after initial render.
 * Pre fetches user`s and project information.
 */
onMounted(async () => {
    usersStore.$onAction((action) => {
        if (action.name === 'clear') clearSessionTimers();
    });

    try {
        await usersStore.getUser();
        await usersStore.getFrozenStatus();
        await abTestingStore.fetchValues();
        await usersStore.getSettings();
        setupSessionTimers();
    } catch (error) {
        usersStore.$onAction((action) => {
            if (action.name === 'login') setupSessionTimers();
        });

        if (!(error instanceof ErrorUnauthorized)) {
            await store.dispatch(APP_STATE_ACTIONS.CHANGE_FETCH_STATE, FetchState.ERROR);
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

    if (!store.state.appStateModule.isAllProjectsDashboard) {
        try {
            if (!projects.length) {
                await store.dispatch(PROJECTS_ACTIONS.CREATE_DEFAULT_PROJECT, usersStore.state.user.id);
            } else {
                selectProject(projects);
            }
            if (usersStore.shouldOnboard) {
                const onboardingPath = RouteConfig.OnboardingTour.with(RouteConfig.FirstOnboardingStep).path;

                await analytics.pageVisit(onboardingPath);
                await router.push(onboardingPath);

                await store.dispatch(APP_STATE_ACTIONS.CHANGE_FETCH_STATE, FetchState.LOADED);
                return;
            }
        } catch (error) {
            notify.error(error.message, AnalyticsErrorEventSource.OVERALL_APP_WRAPPER_ERROR);
            return;
        }
    }

    await store.dispatch(APP_STATE_ACTIONS.CHANGE_FETCH_STATE, FetchState.LOADED);
});

onBeforeUnmount(() => {
    clearSessionTimers();
    resetActivityEvents.forEach((eventName: string) => {
        document.removeEventListener(eventName, onSessionActivity);
    });
});
</script>

<style scoped lang="scss">
    :deep(.notification-wrap) {
        margin-top: 1rem;
    }

    .banner-container:empty {
        display: none;
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
                            padding: 48px 48px 0;
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
