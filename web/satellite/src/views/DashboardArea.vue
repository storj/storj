// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="dashboard">
        <div v-if="isLoading" class="loading-overlay active">
            <img class="loading-image" src="@/../static/images/register/Loading.gif" alt="Company logo loading gif">
        </div>
        <div v-if="isBetaSatellite" class="dashboard__beta-banner">
            <p class="dashboard__beta-banner__message">
                Thanks for testing the {{satelliteName}} Beta satellite | Data may be deleted during this beta | Submit testing feedback
                <a class="dashboard__beta-banner__message__link" :href="betaFeedbackURL" target="_blank" rel="noopener noreferrer">here</a>
                | Request support
                <a class="dashboard__beta-banner__message__link" :href="betaSupportURL" target="_blank" rel="noopener noreferrer">here</a>
            </p>
        </div>
        <div v-if="!isLoading" class="dashboard__wrap">
            <DashboardHeader/>
            <div class="dashboard__wrap__main-area">
                <NavigationArea class="regular-navigation"/>
                <div class="dashboard__wrap__main-area__content">
                    <div class="dashboard__wrap__main-area__content__bar-area">
                        <VInfoBar
                            v-if="isBillingInfoBarShown"
                            :first-value="storageRemaining"
                            :second-value="bandwidthRemaining"
                            first-description="of Storage Remaining"
                            second-description="of Bandwidth Remaining"
                            :path="projectDashboardPath"
                            :link="projectLimitsIncreaseRequestURL"
                            link-label="Request Limit Increase ->"
                        />
                        <VInfoBar
                            v-if="isProjectLimitInfoBarShown"
                            is-blue="true"
                            :first-value="`You have used ${projectsCount}`"
                            first-description="of your"
                            :second-value="projectLimit"
                            second-description="available projects."
                            :link="projectLimitsIncreaseRequestURL"
                            link-label="Request Project Limit Increase"
                        />
                    </div>
                    <router-view/>
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
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { USER_ACTIONS } from '@/store/modules/users';
import { Project } from '@/types/projects';
import { Size } from '@/utils/bytesSize';
import {
    APP_STATE_ACTIONS,
    PM_ACTIONS,
} from '@/utils/constants/actionNames';
import { AppState } from '@/utils/constants/appStateEnum';
import { LocalData } from '@/utils/localData';
import { MetaUtils } from '@/utils/meta';

const {
    GET_PAYWALL_ENABLED_STATUS,
    SETUP_ACCOUNT,
    GET_BALANCE,
    GET_CREDIT_CARDS,
    GET_PAYMENTS_HISTORY,
    GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP,
} = PAYMENTS_ACTIONS;

@Component({
    components: {
        NavigationArea,
        DashboardHeader,
        VInfoBar,
    },
})
export default class DashboardArea extends Vue {
    private FIRST_PAGE: number = 1;

    /**
     * Holds router link to project dashboard page.
     */
    public readonly projectDashboardPath: string = RouteConfig.ProjectDashboard.path;

    /**
     * Lifecycle hook before initial render.
     * Sets access grants web worker.
     */
    public async beforeMount(): Promise<void> {
        try {
            await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.STOP_ACCESS_GRANTS_WEB_WORKER);
            await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.SET_ACCESS_GRANTS_WEB_WORKER);
        } catch (error) {
            await this.$notify.error(`Unable to set access grants wizard. ${error.message}`);
        }
    }

    /**
     * Lifecycle hook after initial render.
     * Pre fetches user`s and project information.
     */
    public async mounted(): Promise<void> {
        // TODO: combine all project related requests in one
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
            await this.$store.dispatch(GET_PAYWALL_ENABLED_STATUS);
        } catch (error) {
            await this.$notify.error(`Unable to get paywall enabled status. ${error.message}`);
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
            await this.$store.dispatch(GET_PAYMENTS_HISTORY);
        } catch (error) {
            await this.$notify.error(`Unable to get account payments history. ${error.message}`);
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
                await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.OverviewStep).path);
            } catch (error) {
                return;
            }

            return;
        }

        this.selectProject(projects);

        try {
            await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.FETCH, this.FIRST_PAGE);
        } catch (error) {
            await this.$notify.error(`Unable to fetch access grants. ${error.message}`);
        }

        try {
            await this.$store.dispatch(PROJECTS_ACTIONS.FETCH_OWNED, this.FIRST_PAGE);
        } catch (error) {
            await this.$notify.error(`Unable to fetch owned projects. ${error.message}`);
        }

        try {
            await this.$store.dispatch(BUCKET_ACTIONS.FETCH_ALL_BUCKET_NAMES);
        } catch (error) {
            await this.$notify.error(`Unable to fetch all bucket names. ${error.message}`);
        }

        await this.$store.dispatch(PM_ACTIONS.SET_SEARCH_QUERY, '');
        try {
            await this.$store.dispatch(PM_ACTIONS.FETCH, this.FIRST_PAGE);
        } catch (error) {
            await this.$notify.error(`Unable to fetch project members. ${error.message}`);
        }

        try {
            await this.$store.dispatch(PROJECTS_ACTIONS.GET_LIMITS, this.$store.getters.selectedProject.id);
        } catch (error) {
            await this.$notify.error(`Unable to fetch project limits. ${error.message}`);
        }

        try {
            await this.$store.dispatch(BUCKET_ACTIONS.FETCH, this.FIRST_PAGE);
        } catch (error) {
            await this.$notify.error(`Unable to fetch buckets. ${error.message}`);
        }

        await this.$store.dispatch(APP_STATE_ACTIONS.CHANGE_STATE, AppState.LOADED);
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
     * Indicates if billing info bar is shown.
     */
    public get isBillingInfoBarShown(): boolean {
        const showBillingInfoBar = (this.$route.name === RouteConfig.Billing.name) || (this.$route.name === RouteConfig.ProjectDashboard.name);

        return showBillingInfoBar && this.projectsCount > 0;
    }

    /**
     * Indicates if project limit info bar is shown.
     */
    public get isProjectLimitInfoBarShown(): boolean {
        return this.$route.name === RouteConfig.ProjectsList.name;
    }

    /**
     * Returns user's projects count.
     */
    public get projectsCount(): number {
        return this.$store.getters.projectsCount;
    }

    /**
     * Returns project limit from store.
     */
    public get projectLimit(): number {
        const projectLimit: number = this.$store.getters.user.projectLimit;
        if (projectLimit < this.projectsCount) return this.projectsCount;

        return projectLimit;
    }

    /**
     * Returns project limits increase request url from config.
     */
    public get projectLimitsIncreaseRequestURL(): string {
        return MetaUtils.getMetaContent('project-limits-increase-request-url');
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

    /**
     * Indicates if paywall is enabled.
     */
    private get isPaywallEnabled(): boolean {
        return this.$store.state.paymentsModule.isPaywallEnabled;
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
        background-color: rgba(134, 134, 148, 1);
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

                    &__bar-area {
                        position: relative;
                    }
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
