// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="dashboard">
        <div v-if="isLoading" class="loading-overlay active">
            <LoaderImage class="loading-icon" />
        </div>
        <div v-if="isBetaSatellite" class="dashboard__beta-banner">
            <p class="dashboard__beta-banner__message">
                Thanks for testing the {{ satelliteName }} Beta satellite | Data may be deleted during this beta | Submit testing feedback
                <a class="dashboard__beta-banner__message__link" :href="betaFeedbackURL" target="_blank" rel="noopener noreferrer">here</a>
                | Request support
                <a class="dashboard__beta-banner__message__link" :href="betaSupportURL" target="_blank" rel="noopener noreferrer">here</a>
            </p>
        </div>
        <div v-if="!isLoading" class="dashboard__wrap">
            <PaidTierBar v-if="!creditCards.length && !isOnboardingTour" :open-add-p-m-modal="togglePMModal" />
            <MFARecoveryCodeBar v-if="showMFARecoveryCodeBar" :open-generate-modal="generateNewMFARecoveryCodes" />
            <template v-if="isNewNavStructure">
                <div class="dashboard__wrap__new-main-area">
                    <NewNavigationArea />
                    <router-view class="dashboard__wrap__new-main-area__content" />
                </div>
            </template>
            <template v-else>
                <DashboardHeader />
                <div class="dashboard__wrap__main-area">
                    <NavigationArea class="regular-navigation" />
                    <div class="dashboard__wrap__main-area__content">
                        <router-view />
                    </div>
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
import PaidTierBar from '@/components/account/billing/paidTier/PaidTierBar.vue';
import MFARecoveryCodesPopup from '@/components/account/mfa/MFARecoveryCodesPopup.vue';
import MFARecoveryCodeBar from '@/components/account/mfa/MFARecoveryCodeBar.vue';
import DashboardHeader from '@/components/header/HeaderArea.vue';
import NavigationArea from '@/components/navigation/NavigationArea.vue';
import NewNavigationArea from '@/components/navigation/newNavigationStructure/NewNavigationArea.vue';

import LoaderImage from '@/../static/images/common/loader.svg';

import { ErrorUnauthorized } from '@/api/errors/ErrorUnauthorized';
import { RouteConfig } from '@/router';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { PAYMENTS_ACTIONS, PAYMENTS_MUTATIONS } from '@/store/modules/payments';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { USER_ACTIONS } from '@/store/modules/users';
import { CreditCard } from '@/types/payments';
import { Project } from '@/types/projects';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { AppState } from '@/utils/constants/appStateEnum';
import { LocalData } from '@/utils/localData';
import { MetaUtils } from '@/utils/meta';
import { User } from '@/types/users';

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
            await this.$store.dispatch(SETUP_ACCOUNT);
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
            await this.$notify.error(error.message);

            return;
        }

        if (!projects.length) {
            try {
                await this.$store.dispatch(PROJECTS_ACTIONS.CREATE_DEFAULT_PROJECT);
                if (this.isNewOnbCLiFlow) {
                    await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.OverviewStep).path);
                } else {
                    await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.OldOverviewStep).path);
                }

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
     * Returns satellite name from store (config).
     */
    public get satelliteName(): string {
        return MetaUtils.getMetaContent('satellite-name');
    }

    /**
     * Returns feedback URL from config for beta satellites.
     */
    public get betaFeedbackURL(): string {
        return MetaUtils.getMetaContent('beta-satellite-feedback-url');
    }

    /**
     * Returns support URL from config for beta satellites.
     */
    public get betaSupportURL(): string {
        return MetaUtils.getMetaContent('beta-satellite-support-url');
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

    public get isNewNavStructure(): boolean {
        return this.$store.state.appStateModule.isNewNavStructure;
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
     * Returns onboarding CLI flow status from store.
     */
    private get isNewOnbCLiFlow(): boolean {
        return this.$store.state.appStateModule.isNewOnbCLIFlow;
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

        &__beta-banner {
            width: calc(100% - 60px);
            padding: 0 30px;
            display: flex;
            align-items: center;
            justify-content: space-between;
            font-family: 'font_regular', sans-serif;
            background-color: red;

            &__message {
                font-weight: normal;
                font-size: 14px;
                line-height: 16px;
                color: #fff;

                &__link {
                    font-size: 14px;
                    line-height: 16px;
                    color: #fff;
                    text-decoration: underline;

                    &:hover {
                        text-decoration: none;
                    }
                }
            }
        }

        &__wrap {
            display: flex;
            flex-direction: column;
            height: 100%;

            &__main-area {
                display: flex;
                height: 100%;

                &__content {
                    overflow-y: scroll;
                    height: calc(100vh - 62px);
                    width: 100%;
                    position: relative;
                }
            }

            &__new-main-area {
                display: flex;
                width: 100%;
                height: 100%;

                &__content {
                    width: 100%;
                    overflow-y: auto;
                }
            }
        }
    }

    @media screen and (max-width: 1280px) {

        .regular-navigation {
            display: none;
        }
    }
</style>
