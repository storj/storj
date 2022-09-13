// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="dashboard">
        <div v-if="isLoading" class="loading-overlay active">
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
                    <div class="dashboard__wrap__main-area__content-wrap__container">
                        <div class="bars">
                            <BetaSatBar v-if="isBetaSatellite" />
                            <PaidTierBar v-if="!creditCards.length && !isOnboardingTour" :open-add-p-m-modal="togglePMModal" />
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
            :initial-seconds="inactivityModalTime/1000"
        />
        <AllModals />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { ErrorUnauthorized } from '@/api/errors/ErrorUnauthorized';
import { RouteConfig } from '@/router';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { USER_ACTIONS } from '@/store/modules/users';
import { CouponType } from '@/types/coupons';
import { CreditCard } from '@/types/payments';
import { Project } from '@/types/projects';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { AppState } from '@/utils/constants/appStateEnum';
import { LocalData } from '@/utils/localData';
import { User } from '@/types/users';
import { AuthHttpApi } from '@/api/auth';
import { MetaUtils } from '@/utils/meta';
import { AnalyticsHttpApi } from '@/api/analytics';

import ProjectInfoBar from '@/components/infoBars/ProjectInfoBar.vue';
import BillingNotification from '@/components/notifications/BillingNotification.vue';
import NavigationArea from '@/components/navigation/NavigationArea.vue';
import InactivityModal from '@/components/modals/InactivityModal.vue';
import BetaSatBar from '@/components/infoBars/BetaSatBar.vue';
import MFARecoveryCodeBar from '@/components/infoBars/MFARecoveryCodeBar.vue';
import PaidTierBar from '@/components/infoBars/PaidTierBar.vue';
import AllModals from '@/components/modals/AllModals.vue';
import MobileNavigation from '@/components/navigation/MobileNavigation.vue';

import LoaderImage from '@/../static/images/common/loader.svg';

const {
    SETUP_ACCOUNT,
    GET_CREDIT_CARDS,
} = PAYMENTS_ACTIONS;

// @vue/component
@Component({
    components: {
        MobileNavigation,
        AllModals,
        NavigationArea,
        LoaderImage,
        PaidTierBar,
        MFARecoveryCodeBar,
        BetaSatBar,
        ProjectInfoBar,
        BillingNotification,
        InactivityModal,
    },
})
export default class DashboardArea extends Vue {
    private readonly auth: AuthHttpApi = new AuthHttpApi();

    // Properties concerning session refreshing, inactivity notification, and automatic logout
    private readonly resetActivityEvents: string[] = ['keypress', 'mousemove', 'mousedown', 'touchmove'];
    private readonly sessionDuration: number = parseInt(MetaUtils.getMetaContent('inactivity-timer-duration')) * 1000;
    private inactivityTimerId: ReturnType<typeof setTimeout> | null;
    private inactivityModalShown = false;
    private inactivityModalTime = 60000;
    private sessionRefreshInterval: number = this.sessionDuration/2;
    private sessionRefreshTimerId: ReturnType<typeof setTimeout> | null;
    private isSessionActive = false;
    private isSessionRefreshing = false;

    // Properties concerning the session timer popup used for debugging
    private readonly debugTimerShown = MetaUtils.getMetaContent('inactivity-timer-viewer-enabled') == 'true';
    private debugTimerText = '';
    private debugTimerId: ReturnType<typeof setTimeout> | null;

    // Minimum number of recovery codes before the recovery code warning bar is shown.
    public recoveryCodeWarningThreshold = 4;

    public readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
     * Lifecycle hook after initial render.
     * Pre fetches user`s and project information.
     */
    public async mounted(): Promise<void> {
        this.$store.subscribeAction((action) => {
            if (action.type == USER_ACTIONS.CLEAR) this.clearSessionTimers();
        });

        if (LocalData.getBillingNotificationAcknowledged()) {
            this.$store.commit(APP_STATE_MUTATIONS.CLOSE_BILLING_NOTIFICATION);
        } else {
            const unsub = this.$store.subscribe((action) => {
                if (action.type == APP_STATE_MUTATIONS.CLOSE_BILLING_NOTIFICATION) {
                    LocalData.setBillingNotificationAcknowledged();
                    unsub();
                }
            });
        }

        try {
            await this.$store.dispatch(USER_ACTIONS.GET);
            this.setupSessionTimers();
        } catch (error) {
            this.$store.subscribeAction((action) => {
                if (action.type == USER_ACTIONS.LOGIN) this.setupSessionTimers();
            });

            if (!(error instanceof ErrorUnauthorized)) {
                await this.$store.dispatch(APP_STATE_ACTIONS.CHANGE_STATE, AppState.ERROR);
                await this.$notify.error(error.message);
            }

            setTimeout(async () => await this.$router.push(RouteConfig.Login.path), 1000);

            return;
        }

        try {
            await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.STOP_ACCESS_GRANTS_WEB_WORKER);
            await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.SET_ACCESS_GRANTS_WEB_WORKER);
        } catch (error) {
            await this.$notify.error(`Unable to set access grants wizard. ${error.message}`);
        }

        try {
            const couponType = await this.$store.dispatch(SETUP_ACCOUNT);
            if (couponType === CouponType.NoCoupon) {
                await this.$notify.error(`The coupon code was invalid, and could not be applied to your account`);
            }

            if (couponType === CouponType.SignupCoupon) {
                await this.$notify.success(`The coupon code was added successfully`);
            }
        } catch (error) {
            await this.$notify.error(`Unable to setup account. ${error.message}`);
        }

        try {
            await this.$store.dispatch(GET_CREDIT_CARDS);
        } catch (error) {
            await this.$notify.error(`Unable to get credit cards. ${error.message}`);
        }

        let projects: Project[] = [];

        try {
            projects = await this.$store.dispatch(PROJECTS_ACTIONS.FETCH);
        } catch (error) {
            return;
        }

        if (!projects.length) {
            try {
                await this.$store.dispatch(PROJECTS_ACTIONS.CREATE_DEFAULT_PROJECT);

                this.analytics.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.OverviewStep).path);
                await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.OverviewStep).path);

                await this.$store.dispatch(APP_STATE_ACTIONS.CHANGE_STATE, AppState.LOADED);
            } catch (error) {
                return;
            }

            return;
        }

        this.selectProject(projects);

        await this.$store.dispatch(APP_STATE_ACTIONS.CHANGE_STATE, AppState.LOADED);
    }

    /**
     * Generates new MFA recovery codes and toggles popup visibility.
     */
    public async generateNewMFARecoveryCodes(): Promise<void> {
        try {
            await this.$store.dispatch(USER_ACTIONS.GENERATE_USER_MFA_RECOVERY_CODES);
            this.toggleMFARecoveryModal();
        } catch (error) {
            await this.$notify.error(error.message);
        }
    }

    /**
     * Toggles MFA recovery modal visibility.
     */
    public toggleMFARecoveryModal(): void {
        this.$store.commit(APP_STATE_MUTATIONS.TOGGLE_MFA_RECOVERY_MODAL_SHOWN);
    }

    /**
     * Opens add payment method modal.
     */
    public togglePMModal(): void {
        this.$store.commit(APP_STATE_MUTATIONS.TOGGLE_IS_ADD_PM_MODAL_SHOWN);
    }

    /**
     * Disables session inactivity modal visibility.
     */
    public closeInactivityModal(): void {
        this.inactivityModalShown = false;
    }

    /**
     * Checks if stored project is in fetched projects array and selects it.
     * Selects first fetched project if check is not successful.
     * @param fetchedProjects - fetched projects array
     */
    private selectProject(fetchedProjects: Project[]): void {
        const storedProjectID = LocalData.getSelectedProjectId();
        const isProjectInFetchedProjects = fetchedProjects.some(project => project.id === storedProjectID);
        if (storedProjectID && isProjectInFetchedProjects) {
            this.storeProject(storedProjectID);

            return;
        }

        // Length of fetchedProjects array is checked before selectProject() function call.
        this.storeProject(fetchedProjects[0].id);
    }

    /**
     * Stores project to vuex store and browser's local storage.
     * @param projectID - project id string
     */
    private storeProject(projectID: string): void {
        this.$store.dispatch(PROJECTS_ACTIONS.SELECT, projectID);
        LocalData.setSelectedProjectId(projectID);
    }

    /**
     * Indicates if current route is projects list page.
     */
    public get isProjectListPage(): boolean {
        return this.$route.name === RouteConfig.ProjectsList.name;
    }

    /**
     * Returns credit cards from store.
     */
    public get creditCards(): CreditCard[] {
        return this.$store.state.paymentsModule.creditCards;
    }

    /**
     * Indicates if current route is onboarding tour.
     */
    public get isOnboardingTour(): boolean {
        return this.$route.path.includes(RouteConfig.OnboardingTour.path);
    }

    /**
     * Indicates if satellite is in beta.
     */
    public get isBetaSatellite(): boolean {
        return this.$store.state.appStateModule.isBetaSatellite;
    }

    /**
     * Indicates if loading screen is active.
     */
    public get isLoading(): boolean {
        return this.$store.state.appStateModule.appState.fetchState === AppState.LOADING;
    }

    /**
     * Indicates whether the MFA recovery code warning bar should be shown.
     */
    public get showMFARecoveryCodeBar(): boolean {
        const user: User = this.$store.getters.user;
        return user.isMFAEnabled && user.mfaRecoveryCodeCount < this.recoveryCodeWarningThreshold;
    }

    /**
     * Indicates if navigation sidebar is hidden.
     */
    public get isNavigationHidden(): boolean {
        return this.isOnboardingTour || this.isCreateProjectPage;
    }

    /**
     * Indicates whether the billing relocation notification should be shown.
     */
    public get isBillingNotificationShown(): boolean {
        return this.$store.state.appStateModule.appState.isBillingNotificationShown;
    }

    /**
     * Indicates if current route is create project page.
     */
    private get isCreateProjectPage(): boolean {
        return this.$route.name === RouteConfig.CreateProject.name;
    }

    /**
     * Refreshes session and resets session timers.
     */
    private async refreshSession(): Promise<void> {
        this.isSessionRefreshing = true;
        
        try {
            LocalData.setSessionExpirationDate(await this.auth.refreshSession());
        } catch (error) {
            await this.$notify.error((error instanceof ErrorUnauthorized) ? 'Your session was timed out.' : error.message);
            await this.handleInactive();
            this.isSessionRefreshing = false;
            return;
        }

        this.clearSessionTimers();
        this.restartSessionTimers();
        this.inactivityModalShown = false;
        this.isSessionActive = false;
        this.isSessionRefreshing = false;
    }

    /**
     * Performs logout and cleans event listeners and session timers.
     */
    private async handleInactive(): Promise<void> {
        this.analytics.pageVisit(RouteConfig.Login.path);
        await this.$router.push(RouteConfig.Login.path);

        this.resetActivityEvents.forEach((eventName: string) => {
            document.removeEventListener(eventName, this.onSessionActivity);
        });
        this.clearSessionTimers();
        this.inactivityModalShown = false;

        try {
            await this.auth.logout();
        } catch (error) {
            if (error instanceof ErrorUnauthorized) return;

            await this.$notify.error(error.message);
        }
    }

    /**
     * Resets inactivity timer and refreshes session if necessary.
     */
    private async onSessionActivity(): Promise<void> {
        if (this.inactivityModalShown || this.isSessionActive) return;

        if (this.sessionRefreshTimerId == null && !this.isSessionRefreshing) {
            await this.refreshSession();
        }
        this.isSessionActive = true;
    }

    /**
     * Adds DOM event listeners and starts session timers.
     */
    private setupSessionTimers(): void {
        const isInactivityTimerEnabled = MetaUtils.getMetaContent('inactivity-timer-enabled');

        if (isInactivityTimerEnabled === 'false') return;

        const expiresAt = LocalData.getSessionExpirationDate();

        if (expiresAt) {
            this.resetActivityEvents.forEach((eventName: string) => {
                document.addEventListener(eventName, this.onSessionActivity, false);
            });

            if (expiresAt.getTime() - this.sessionDuration + this.sessionRefreshInterval < Date.now()) {
                this.refreshSession();
            }

            this.restartSessionTimers();
        }
    }

    /**
     * Restarts timers associated with session refreshing and inactivity.
     */
    private restartSessionTimers(): void {
        this.sessionRefreshTimerId = setTimeout(async () => {
            this.sessionRefreshTimerId = null;
            if (this.isSessionActive) {
                await this.refreshSession();
            }
        }, this.sessionRefreshInterval);

        this.inactivityTimerId = setTimeout(() => {
            if (this.isSessionActive) return;
            this.inactivityModalShown = true;
            this.inactivityTimerId = setTimeout(async () => {
                this.handleInactive();
                await this.$notify.notify('Your session was timed out.');
            }, this.inactivityModalTime);
        }, this.sessionDuration - this.inactivityModalTime);

        if (!this.debugTimerShown) return;

        const debugTimer = () => {
            const expiresAt = LocalData.getSessionExpirationDate();

            if (expiresAt) {
                const ms = Math.max(0, expiresAt.getTime() - Date.now());
                const secs = Math.floor(ms/1000)%60;

                this.debugTimerText = `${Math.floor(ms/60000)}:${(secs<10 ? '0' : '')+secs}`;

                if (ms > 1000) {
                    this.debugTimerId = setTimeout(debugTimer, 1000);
                }
            }
        };
        debugTimer();
    }

    /**
     * Clears timers associated with session refreshing and inactivity.
     */
    private clearSessionTimers(): void {
        [this.inactivityTimerId, this.sessionRefreshTimerId, this.debugTimerId].forEach(id => {
            if (id != null) clearTimeout(id);
        });
    }
}
</script>

<style scoped lang="scss">
    .loading-overlay {
        display: flex;
        justify-content: center;
        align-items: center;
        position: absolute;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background-color: rgb(134 134 148 / 30%);
        visibility: hidden;
        opacity: 0;
        transition: all 0.5s linear;
    }

    .loading-overlay.active {
        visibility: visible;
        opacity: 1;
    }

    .loading-icon {
        width: 100px;
        height: 100px;
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
            border: 1px solid #ffd78a;
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
