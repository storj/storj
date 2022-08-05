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
                    <div class="bars">
                        <BetaSatBar v-if="isBetaSatellite" />
                        <PaidTierBar v-if="!creditCards.length && !isOnboardingTour" :open-add-p-m-modal="togglePMModal" />
                        <ProjectInfoBar v-if="isProjectListPage" />
                        <MFARecoveryCodeBar v-if="showMFARecoveryCodeBar" :open-generate-modal="generateNewMFARecoveryCodes" />
                    </div>
                    <router-view
                        class="dashboard__wrap__main-area__content-wrap__content"
                        :class="{
                            'with-one-bar': amountOfInfoBars === 1,
                            'with-two-bars': amountOfInfoBars === 2,
                            'with-three-bars': amountOfInfoBars === 3,
                            'with-four-bars': amountOfInfoBars === 4,
                        }"
                    />
                </div>
            </div>
        </div>
        <AllModals />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import AllModals from "@/components/modals/AllModals.vue";
import PaidTierBar from '@/components/infoBars/PaidTierBar.vue';
import MFARecoveryCodeBar from '@/components/infoBars/MFARecoveryCodeBar.vue';
import BetaSatBar from '@/components/infoBars/BetaSatBar.vue';
import NavigationArea from '@/components/navigation/NavigationArea.vue';
import ProjectInfoBar from "@/components/infoBars/ProjectInfoBar.vue";

import LoaderImage from '@/../static/images/common/loader.svg';

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
import { APP_STATE_MUTATIONS } from "@/store/mutationConstants";
import { AppState } from '@/utils/constants/appStateEnum';
import { LocalData } from '@/utils/localData';
import { User } from "@/types/users";
import { AuthHttpApi } from "@/api/auth";
import { MetaUtils } from "@/utils/meta";

import { AnalyticsHttpApi } from '@/api/analytics';
import MobileNavigation from "@/components/navigation/MobileNavigation.vue";

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
    },
})
export default class DashboardArea extends Vue {
    // List of DOM events that resets inactivity timer.
    private readonly resetActivityEvents: string[] = ['keypress', 'mousemove', 'mousedown', 'touchmove'];
    private readonly auth: AuthHttpApi = new AuthHttpApi();
    private inactivityTimerId: ReturnType<typeof setTimeout>;
    // Minimum number of recovery codes before the recovery code warning bar is shown.
    public recoveryCodeWarningThreshold = 4;

    public readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
     * Lifecycle hook after initial render.
     * Pre fetches user`s and project information.
     */
    public async mounted(): Promise<void> {
        this.setupInactivityTimers();
        try {
            await this.$store.dispatch(USER_ACTIONS.GET);
        } catch (error) {
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
     * Returns amount of rendered info bars.
     * It is used to set height of content's container.
     */
    public get amountOfInfoBars(): number {
        const conditions: boolean[] = [
            this.isBetaSatellite,
            !this.creditCards.length && !this.isOnboardingTour,
            this.isProjectListPage,
            this.showMFARecoveryCodeBar,
        ]

        return conditions.filter(c => c).length;
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
     * Indicates if current route is create project page.
     */
    private get isCreateProjectPage(): boolean {
        return this.$route.name === RouteConfig.CreateProject.name;
    }

    /**
     * Sets up timer id with given delay.
     */
    private startInactivityTimer(): void {
        const inactivityTimerDelayInSeconds = MetaUtils.getMetaContent('inactivity-timer-delay');

        this.inactivityTimerId = setTimeout(this.handleInactive, parseInt(inactivityTimerDelayInSeconds) * 1000);
    }

    /**
     * Performs logout and cleans event listeners.
     */
    private async handleInactive(): Promise<void> {
        try {
            await this.auth.logout();
            this.resetActivityEvents.forEach((eventName: string) => {
                document.removeEventListener(eventName, this.resetInactivityTimer);
            });
            this.analytics.pageVisit(RouteConfig.Login.path);
            await this.$router.push(RouteConfig.Login.path);
            await this.$notify.notify('Your session was timed out.');
        } catch (error) {
            await this.$notify.error(error.message);
        }
    }

    /**
     * Resets inactivity timer.
     */
    private resetInactivityTimer(): void {
        clearTimeout(this.inactivityTimerId);
        this.startInactivityTimer();
    }

    /**
     * Adds DOM event listeners and starts timer.
     */
    private setupInactivityTimers(): void {
        const isInactivityTimerEnabled = MetaUtils.getMetaContent('inactivity-timer-enabled');

        if (isInactivityTimerEnabled === 'false') return;

        this.resetActivityEvents.forEach((eventName: string) => {
            document.addEventListener(eventName, this.resetInactivityTimer, false);
        });

        this.startInactivityTimer();
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
                    height: 95vh;
                    box-sizing: border-box;
                    overflow-y: auto;
                    padding-bottom: 60px;

                    &__content {
                        max-width: 1200px;
                        margin: 0 auto;
                        padding: 48px 48px 60px;
                    }
                }
            }
        }
    }

    .with-one-bar {
        padding-top: 56px;
    }

    .with-two-bars {
        padding-top: 82px;
    }

    .with-three-bars {
        padding-top: 108px;
    }

    .with-four-bars {
        padding-top: 134px;
    }

    .no-nav {
        width: 100%;
    }

    .bars {
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

        .dashboard__wrap__main-area__content-wrap__content {
            padding: 32px 24px 50px;
        }
    }

    @media screen and (max-width: 500px) {

        .dashboard__wrap__main-area {
            flex-direction: column;

            &__content-wrap {
                box-sizing: border-box;
                width: 100%;
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
