// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="dashboard">
        <div v-if="isLoading" class="loading-overlay active">
            <LoaderImage class="loading-icon" />
        </div>
        <div v-if="!isLoading" class="dashboard__wrap">
            <template v-if="!isNewNavStructure">
                <BetaSatBar v-if="isBetaSatellite" />
                <PaidTierBar v-if="!creditCards.length && !isOnboardingTour" :open-add-p-m-modal="togglePMModal" />
                <ProjectInfoBar v-if="isProjectListPage" />
                <MFARecoveryCodeBar v-if="showMFARecoveryCodeBar" :open-generate-modal="generateNewMFARecoveryCodes" />
            </template>
            <template v-if="isNewNavStructure">
                <div class="dashboard__wrap__new-main-area">
                    <NewNavigationArea v-if="!isNavigationHidden" />
                    <div
                        class="dashboard__wrap__new-main-area__content-wrap"
                        :class="{
                            'with-one-bar': amountOfInfoBars === 1,
                            'with-two-bars': amountOfInfoBars === 2,
                            'with-three-bars': amountOfInfoBars === 3,
                            'with-four-bars': amountOfInfoBars === 4,
                        }"
                    >
                        <BetaSatBar v-if="isBetaSatellite" />
                        <PaidTierBar v-if="!creditCards.length && !isOnboardingTour" :open-add-p-m-modal="togglePMModal" />
                        <ProjectInfoBar v-if="isProjectListPage" />
                        <MFARecoveryCodeBar v-if="showMFARecoveryCodeBar" :open-generate-modal="generateNewMFARecoveryCodes" />
                        <router-view class="dashboard__wrap__new-main-area__content-wrap__content" />
                    </div>
                </div>
            </template>
            <template v-else>
                <DashboardHeader />
                <div
                    class="dashboard__wrap__main-area"
                    :class="{
                        'with-one-bar-old': amountOfInfoBars === 1,
                        'with-two-bars-old': amountOfInfoBars === 2,
                        'with-three-bars-old': amountOfInfoBars === 3,
                        'with-four-bars-old': amountOfInfoBars === 4,
                    }"
                >
                    <NavigationArea class="regular-navigation" />
                    <router-view class="dashboard__wrap__main-area__content" />
                </div>
            </template>
        </div>
        <AddPaymentMethodModal v-if="isAddPMModal" :on-close="togglePMModal" />
        <MFARecoveryCodesPopup v-if="isMFACodesPopup" :toggle-modal="toggleMFACodesPopup" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import AddPaymentMethodModal from '@/components/account/billing/paidTier/AddPaymentMethodModal.vue';
import PaidTierBar from '@/components/infoBars/PaidTierBar.vue';
import MFARecoveryCodeBar from '@/components/infoBars/MFARecoveryCodeBar.vue';
import BetaSatBar from '@/components/infoBars/BetaSatBar.vue';
import MFARecoveryCodesPopup from '@/components/account/mfa/MFARecoveryCodesPopup.vue';
import DashboardHeader from '@/components/header/HeaderArea.vue';
import NavigationArea from '@/components/navigation/NavigationArea.vue';
import NewNavigationArea from '@/components/navigation/newNavigationStructure/NewNavigationArea.vue';
import ProjectInfoBar from "@/components/infoBars/ProjectInfoBar.vue";

import LoaderImage from '@/../static/images/common/loader.svg';

import { ErrorUnauthorized } from '@/api/errors/ErrorUnauthorized';
import { RouteConfig } from '@/router';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { PAYMENTS_ACTIONS, PAYMENTS_MUTATIONS } from '@/store/modules/payments';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { USER_ACTIONS } from '@/store/modules/users';
import { CouponType } from '@/types/coupons';
import { CreditCard } from '@/types/payments';
import { Project } from '@/types/projects';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { AppState } from '@/utils/constants/appStateEnum';
import { LocalData } from '@/utils/localData';
import { User } from "@/types/users";

const {
    SETUP_ACCOUNT,
    GET_CREDIT_CARDS,
} = PAYMENTS_ACTIONS;

// @vue/component
@Component({
    components: {
        NavigationArea,
        NewNavigationArea,
        DashboardHeader,
        LoaderImage,
        PaidTierBar,
        MFARecoveryCodeBar,
        BetaSatBar,
        ProjectInfoBar,
        MFARecoveryCodesPopup,
        AddPaymentMethodModal,
    },
})
export default class DashboardArea extends Vue {
    // Minimum number of recovery codes before the recovery code warning bar is shown.
    public recoveryCodeWarningThreshold = 4;

    public isMFACodesPopup = false;

    /**
     * Lifecycle hook after initial render.
     * Pre fetches user`s and project information.
     */
    public async mounted(): Promise<void> {
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
            this.toggleMFACodesPopup();
        } catch (error) {
            await this.$notify.error(error.message);
        }
    }

    /**
     * Opens add payment method modal.
     */
    public togglePMModal(): void {
        this.$store.commit(PAYMENTS_MUTATIONS.TOGGLE_IS_ADD_PM_MODAL_SHOWN);
    }

    /**
     * Toggles MFA recovery codes popup visibility.
     */
    public toggleMFACodesPopup(): void {
        this.isMFACodesPopup = !this.isMFACodesPopup;
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
     * Indicates if add payment method modal is shown.
     */
    public get isAddPMModal(): boolean {
        return this.$store.state.paymentsModule.isAddPMModalShown;
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
        const user : User = this.$store.getters.user;
        return user.isMFAEnabled && user.mfaRecoveryCodeCount < this.recoveryCodeWarningThreshold;
    }

    /**
     * Indicates if new navigation structure is used.
     */
    public get isNewNavStructure(): boolean {
        return this.$store.state.appStateModule.isNewNavStructure;
    }

    /**
     * Indicates if navigation side bar is hidden.
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
        background-color: rgba(134, 134, 148, 0.3);
        visibility: hidden;
        opacity: 0;
        -webkit-transition: all 0.5s linear;
        -moz-transition: all 0.5s linear;
        -o-transition: all 0.5s linear;
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
                height: calc(100% - 62px);

                &__content {
                    overflow-y: auto;
                    width: 100%;
                    position: relative;
                }
            }

            &__new-main-area {
                display: flex;
                width: 100%;
                height: 100%;

                &__content-wrap {
                    width: 100%;

                    &__content {
                        overflow-y: auto;
                    }
                }
            }
        }
    }

    .with-one-bar {
        height: calc(100% - 26px);
    }

    .with-two-bars {
        height: calc(100% - 52px);
    }

    .with-three-bars {
        height: calc(100% - 78px);
    }

    .with-four-bars {
        height: calc(100% - 104px);
    }

    .with-one-bar-old {
        height: calc(100% - 62px - 26px);
    }

    .with-two-bars-old {
        height: calc(100% - 62px - 52px);
    }

    .with-three-bars-old {
        height: calc(100% - 62px - 78px);
    }

    .with-four-bars-old {
        height: calc(100% - 62px - 104px);
    }

    @media screen and (max-width: 1280px) {

        .regular-navigation {
            display: none;
        }
    }
</style>
