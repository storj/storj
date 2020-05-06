// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="dashboard-container">
        <div v-if="isLoading" class="loading-overlay active">
            <img class="loading-image" src="@/../static/images/register/Loading.gif" alt="Company logo loading gif">
        </div>
        <div v-else class="dashboard-container__wrap">
            <NavigationArea class="regular-navigation"/>
            <div class="dashboard-container__wrap__column">
                <DashboardHeader/>
                <div class="dashboard-container__main-area">
                    <div class="dashboard-container__main-area__bar-area">
                        <VInfoBar
                            v-if="isInfoBarShown"
                            :first-value="storageRemaining"
                            :second-value="bandwidthRemaining"
                            first-description="of Storage Remaining"
                            second-description="of Bandwidth Remaining"
                            :path="projectDashboardPath"
                            link="https://support.tardigrade.io/hc/en-us/requests/new?ticket_form_id=360000683212"
                            link-label="Request Limit Increase ->"
                        />
                    </div>
                    <div class="dashboard-container__main-area__content">
                        <router-view/>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VInfoBar from '@/components/common/VInfoBar.vue';
import DashboardHeader from '@/components/header/HeaderArea.vue';
import NavigationArea from '@/components/navigation/NavigationArea.vue';

import { ErrorUnauthorized } from '@/api/errors/ErrorUnauthorized';
import { RouteConfig } from '@/router';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { USER_ACTIONS } from '@/store/modules/users';
import { ApiKeysPage } from '@/types/apiKeys';
import { Project } from '@/types/projects';
import { User } from '@/types/users';
import { Size } from '@/utils/bytesSize';
import {
    API_KEYS_ACTIONS,
    APP_STATE_ACTIONS,
    PM_ACTIONS,
} from '@/utils/constants/actionNames';
import { AppState } from '@/utils/constants/appStateEnum';
import { ProjectOwning } from '@/utils/projectOwning';

const {
    SETUP_ACCOUNT,
    GET_BALANCE,
    GET_CREDIT_CARDS,
    GET_BILLING_HISTORY,
    GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP,
    GET_PROJECT_USAGE_AND_CHARGES_PREVIOUS_ROLLUP,
} = PAYMENTS_ACTIONS;

@Component({
    components: {
        NavigationArea,
        DashboardHeader,
        VInfoBar,
    },
})
export default class DashboardArea extends Vue {
    /**
     * Holds router link to project dashboard page.
     */
    public readonly projectDashboardPath: string = RouteConfig.ProjectDashboard.path;

    /**
     * Lifecycle hook after initial render.
     * Pre fetches user`s and project information.
     */
    public async mounted(): Promise<void> {
        let user: User;

        // TODO: combine all project related requests in one
        try {
            user = await this.$store.dispatch(USER_ACTIONS.GET);
        } catch (error) {
            if (!(error instanceof ErrorUnauthorized)) {
                await this.$store.dispatch(APP_STATE_ACTIONS.CHANGE_STATE, AppState.ERROR);
                await this.$notify.error(error.message);
            }

            setTimeout(async () => await this.$router.push(RouteConfig.Login.path), 1000);

            return;
        }

        try {
            await this.$store.dispatch(SETUP_ACCOUNT);
        } catch (error) {
            await this.$notify.error(`Unable to setup account. ${error.message}`);
        }

        try {
            await this.$store.dispatch(GET_BALANCE);
        } catch (error) {
            await this.$notify.error(`Unable to get account balance. ${error.message}`);
        }

        try {
            await this.$store.dispatch(GET_CREDIT_CARDS);
        } catch (error) {
            await this.$notify.error(`Unable to get credit cards. ${error.message}`);
        }

        try {
            await this.$store.dispatch(GET_BILLING_HISTORY);
        } catch (error) {
            await this.$notify.error(`Unable to get account billing history. ${error.message}`);
        }

        try {
            await this.$store.dispatch(GET_PROJECT_USAGE_AND_CHARGES_PREVIOUS_ROLLUP);
        } catch (error) {
            await this.$notify.error(`Unable to get usage and charges for previous billing period. ${error.message}`);
        }

        try {
            await this.$store.dispatch(GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP);
        } catch (error) {
            await this.$notify.error(`Unable to get usage and charges for current billing period. ${error.message}`);
        }

        let projects: Project[] = [];

        try {
            projects = await this.$store.dispatch(PROJECTS_ACTIONS.FETCH);
        } catch (error) {
            await this.$notify.error(error.message);

            return;
        }

        if (!projects.length) {
            await this.$store.dispatch(APP_STATE_ACTIONS.CHANGE_STATE, AppState.LOADED);

            try {
                await this.$router.push(RouteConfig.OnboardingTour.path);
            } catch (error) {
                return;
            }

            return;
        }

        await this.$store.dispatch(PROJECTS_ACTIONS.SELECT, projects[0].id);

        let apiKeysPage: ApiKeysPage = new ApiKeysPage();

        try {
            apiKeysPage = await this.$store.dispatch(API_KEYS_ACTIONS.FETCH, 1);
        } catch (error) {
            await this.$notify.error(`Unable to fetch api keys. ${error.message}`);
        }

        if (projects.length === 1 && projects[0].ownerId === user.id && apiKeysPage.apiKeys.length === 0) {
            await this.$store.dispatch(APP_STATE_ACTIONS.CHANGE_STATE, AppState.LOADED);

            try {
                await this.$router.push(RouteConfig.OnboardingTour.path);
            } catch (error) {
                return;
            }

            return;
        }

        try {
            await this.$store.dispatch(PM_ACTIONS.FETCH, 1);
        } catch (error) {
            await this.$notify.error(`Unable to fetch project members. ${error.message}`);
        }

        try {
            await this.$store.dispatch(PROJECTS_ACTIONS.GET_LIMITS, this.$store.getters.selectedProject.id);
        } catch (error) {
            await this.$notify.error(`Unable to fetch project limits. ${error.message}`);
        }

        try {
            await this.$store.dispatch(BUCKET_ACTIONS.FETCH, 1);
        } catch (error) {
            await this.$notify.error(`Unable to fetch buckets. ${error.message}`);
        }

        await this.$store.dispatch(APP_STATE_ACTIONS.CHANGE_STATE, AppState.LOADED);
    }

    /**
     * Indicates if info bar is shown.
     */
    public get isInfoBarShown(): boolean {
        const isBillingPage = this.$route.name === RouteConfig.Billing.name;

        return isBillingPage && new ProjectOwning(this.$store).userHasOwnProject();
    }

    /**
     * Returns formatted string of remaining storage.
     */
    public get storageRemaining(): string {
        const storageUsed = this.$store.state.projectsModule.currentLimits.storageUsed;
        const storageLimit = this.$store.state.projectsModule.currentLimits.storageLimit;

        const difference = storageLimit - storageUsed;
        if (difference < 0) {
            return '0 Bytes';
        }

        const remaining = new Size(difference, 2);

        return `${remaining.formattedBytes}${remaining.label}`;
    }

    /**
     * Returns formatted string of remaining bandwidth.
     */
    public get bandwidthRemaining(): string {
        const bandwidthUsed = this.$store.state.projectsModule.currentLimits.bandwidthUsed;
        const bandwidthLimit = this.$store.state.projectsModule.currentLimits.bandwidthLimit;

        const difference = bandwidthLimit - bandwidthUsed;
        if (difference < 0) {
            return '0 Bytes';
        }

        const remaining = new Size(difference, 2);

        return `${remaining.formattedBytes}${remaining.label}`;
    }

    /**
     * Indicates if loading screen is active.
     */
    public get isLoading(): boolean {
        return this.$store.state.appStateModule.appState.fetchState === AppState.LOADING;
    }
}
</script>

<style scoped lang="scss">
    .dashboard-container {
        position: fixed;
        max-width: 100%;
        width: 100%;
        height: 100%;
        left: 0;
        top: 0;
        background-color: #f5f6fa;
        z-index: 10;

        &__wrap {
            display: flex;

            &__column {
                display: flex;
                flex-direction: column;
                width: 100%;
            }
        }

        &__main-area {
            position: relative;
            width: 100%;
            height: calc(100vh - 50px);
            overflow-y: scroll;
            display: flex;
            flex-direction: column;

            &__bar-area {
                flex: 0 1 auto;
            }

            &__content {
                flex: 1 1 auto;
            }
        }
    }

    @media screen and (max-width: 1280px) {

        .regular-navigation {
            display: none;
        }
    }

    .loading-overlay {
        display: flex;
        justify-content: center;
        align-items: center;
        position: absolute;
        top: 0;
        left: 0;
        right: 0;
        height: 100vh;
        z-index: 100;
        background-color: rgba(134, 134, 148, 1);
        visibility: hidden;
        opacity: 0;
        -webkit-transition: all 0.5s linear;
        -moz-transition: all 0.5s linear;
        -o-transition: all 0.5s linear;
        transition: all 0.5s linear;

        .loading-image {
            z-index: 200;
        }
    }

    .loading-overlay.active {
        visibility: visible;
        opacity: 1;
    }
</style>
