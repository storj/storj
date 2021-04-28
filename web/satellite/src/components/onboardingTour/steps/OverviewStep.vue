// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="overview-area">
        <h2 class="overview-area__header">Welcome to Storj DCS</h2>
        <div class="overview-area__continue__area">
            <img class="overview-area__continue__img" src="@/../static/images/onboardingTour/continue-bg.png" alt="continue image">
            <div class="overview-area__continue__text-area">
                <div class="overview-area__continue__container">
                    <p class="overview-area__label continue-label server-side-label">Server-Side Encrypted</p>
                    <h3 class="overview-area__continue__header">Upload in Browser</h3>
                    <p class="overview-area__continue__text">
                        Start uploading files in the browser and instantly see how your data gets distributed over our
                        global storage network. You can always use other upload methods later.
                    </p>
                    <VButton
                        class="overview-area__continue__button"
                        label="Upload in Browser"
                        width="234px"
                        height="48px"
                        :on-press="onUploadInBrowserClick"
                        :is-disabled="isLoading"
                    />
                </div>
            </div>
        </div>
        <h3 class="overview-area__second-header">More Ways To Upload</h3>
        <div class="overview-area__path-area">
            <div class="overview-area__path-section">
                <GatewayIcon class="overview-area__path-section__icon" />
                <p class="overview-area__label server-side-label">Server-Side Encrypted</p>
                <h4 class="overview-area__path-section__title">GatewayMT</h4>
                <p class="overview-area__path-section__text">Backwards S3-Compatible API for uploading data programatically.</p>
                <VButton
                    class="overview-area__path-section__button"
                    label="Continue"
                    width="calc(100% - 4px)"
                    :on-press="onGatewayMTClick"
                    :is-blue-white="true"
                    :is-disabled="isLoading"
                />
            </div>
            <div class="overview-area__path-section">
                <img src="@/../static/images/onboardingTour/command-line-icon.png" alt="uplink icon">
                <p class="overview-area__label">End-to-End Encrypted</p>
                <h4 class="overview-area__path-section__title">Uplink CLI</h4>
                <p class="overview-area__path-section__text">Natively installed client for interacting with the Storj Network.</p>
                <VButton
                    class="overview-area__path-section__button"
                    label="Continue"
                    width="calc(100% - 4px)"
                    :on-press="onUplinkCLIClick"
                    :is-blue-white="true"
                    :is-disabled="isLoading"
                />
            </div>
            <div class="overview-area__path-section">
                <img class="overview-area__path-section__icon" src="@/../static/images/onboardingTour/rclone.png" alt="rclone image">
                <p class="overview-area__label">End-to-End Encrypted</p>
                <h4 class="overview-area__path-section__title">Sync with Rclone</h4>
                <p class="overview-area__path-section__text">Map your filesystem to the decentralized cloud.</p>
                <a
                    class="overview-area__path-section__button"
                    :class="{ 'link-disabled': isLoading }"
                    :href="!isLoading ? 'https://docs.storj.io/how-tos/sync-files-with-rclone' : '' "
                    target="_blank"
                    rel="noopener noreferrer"
                    @click="onRcloneClick"
                >
                    Continue
                </a>
            </div>
        </div>
        <a
            class="overview-area__integrations-button"
            href="https://storj.io/integrations/"
            target="_blank"
            rel="noopener noreferrer"
        >
            More Integrations
        </a>
        <VButton
            class="overview-area__skip-button"
            label="Skip"
            width="100px"
            :on-press="onSkipClick"
            :is-blue-white="true"
            :is-disabled="isLoading"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VButton from '@/components/common/VButton.vue';

import GatewayIcon from '@/../static/images/onboardingTour/s3-gateway.svg';

import { AnalyticsHttpApi } from '@/api/analytics';
import { RouteConfig } from '@/router';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { ProjectFields } from '@/types/projects';
import { PM_ACTIONS } from '@/utils/constants/actionNames';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';

@Component({
    components: {
        VButton,
        GatewayIcon,
    },
})
export default class OverviewStep extends Vue {
    public isLoading: boolean = false;

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    public onRcloneClick(): void {
        this.analytics.linkEventTriggered(AnalyticsEvent.PATH_SELECTED, 'Rclone Sync');
    }

    /**
     * Lifecycle hook after initial render.
     * Sets area to needed state.
     */
    public mounted(): void {
        if (this.userHasProject) {
            this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.AccessGrant).path).catch(() => {return; });
        }
    }

    /**
     * Holds button click logic.
     * Creates untitled project and redirects to next step (creating access grant).
     */
    public async onGatewayMTClick(): Promise<void> {
        if (this.isLoading) return;

        this.isLoading = true;

        await this.analytics.linkEventTriggered(AnalyticsEvent.PATH_SELECTED, 'GatewayMT');

        try {
            await this.createFirstProject();

            this.isLoading = false;

            await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.AccessGrant).with(RouteConfig.AccessGrantName).path);
        } catch (error) {
            await this.$notify.error(error.message);
            this.isLoading = false;
        }
    }

    /**
     * Holds button click logic.
     * Creates untitled project and redirects to next step (creating access grant).
     */
    public async onUplinkCLIClick(): Promise<void> {
        if (this.isLoading) return;

        this.isLoading = true;

        await this.analytics.linkEventTriggered(AnalyticsEvent.PATH_SELECTED, 'CLI');

        try {
            await this.createFirstProject();

            this.isLoading = false;

            await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.AccessGrant).with(RouteConfig.AccessGrantName).path);
        } catch (error) {
            await this.$notify.error(error.message);
            this.isLoading = false;
        }
    }

    /**
     * Creates untitled project and redirects to objects page.
     */
    public async onUploadInBrowserClick(): Promise<void> {
        if (this.isLoading) return;

        this.isLoading = true;

        await this.analytics.linkEventTriggered(AnalyticsEvent.PATH_SELECTED, 'Continue in Browser');

        try {
            await this.createFirstProject();

            this.isLoading = false;

            await this.$router.push(RouteConfig.Objects.path).catch(() => {return; });
        } catch (error) {
            await this.$notify.error(error.message);
            this.isLoading = false;
        }
    }

    /**
     * Holds button click logic.
     * Creates untitled project and redirects to project dashboard.
     */
    public async onSkipClick(): Promise<void> {
        if (this.isLoading) return;

        this.isLoading = true;

        try {
            await this.createFirstProject();

            this.isLoading = false;

            await this.$router.push(RouteConfig.ProjectDashboard.path);
        } catch (error) {
            await this.$notify.error(error.message);
            this.isLoading = false;
        }
    }

    /**
     * Creates untitled project in a background.
     */
    private async createFirstProject(): Promise<void> {
        const FIRST_PAGE = 1;
        const UNTITLED_PROJECT_NAME = 'My First Project';
        const UNTITLED_PROJECT_DESCRIPTION = '___';
        const project = new ProjectFields(
            UNTITLED_PROJECT_NAME,
            UNTITLED_PROJECT_DESCRIPTION,
            this.$store.getters.user.id,
        );
        const createdProject = await this.$store.dispatch(PROJECTS_ACTIONS.CREATE, project);
        const createdProjectId = createdProject.id;

        await this.$store.dispatch(PROJECTS_ACTIONS.SELECT, createdProjectId);
        await this.$store.dispatch(PM_ACTIONS.CLEAR);
        await this.$store.dispatch(PM_ACTIONS.FETCH, FIRST_PAGE);
        await this.$store.dispatch(PAYMENTS_ACTIONS.GET_PAYMENTS_HISTORY);
        await this.$store.dispatch(PAYMENTS_ACTIONS.GET_BALANCE);
        await this.$store.dispatch(PAYMENTS_ACTIONS.GET_PROJECT_USAGE_AND_CHARGES_CURRENT_ROLLUP);
        await this.$store.dispatch(PROJECTS_ACTIONS.GET_LIMITS, createdProjectId);
        await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.CLEAR);
        await this.$store.dispatch(BUCKET_ACTIONS.CLEAR);
    }

    /**
     * Indicates if user has at least one project.
     */
    private get userHasProject(): boolean {
        return this.$store.state.projectsModule.projects.length > 0;
    }
}
</script>

<style scoped lang="scss">
    p,
    h1,
    h2 {
        margin: 0;
    }

    .overview-area {
        display: flex;
        flex-direction: column;
        align-items: center;

        &__header,
        &__second-header {
            font-family: 'font_bold', sans-serif;
            font-size: 38px;
            line-height: 46px;
            text-align: center;
        }

        &__header {
            margin: 0 auto 80px auto;
        }

        &__second-header {
            font-size: 28px;
            line-height: 54px;
            margin: 50px auto;
        }

        &__label {
            font-family: 'font_normal', sans-serif;
            font-weight: 600;
            font-size: 16px;
            background: transparent;
            width: 212px;
            height: 22px;
            padding-top: 5px;
            border-radius: 50px;
            margin: 20px auto 25px auto;
            color: #000;
            border: 2px solid #000;
        }

        &__label.continue-label {
            text-align: center;
            margin: 0;
            position: relative;
            top: 10px;
        }

        &__label.server-side-label {
            color: #d63030;
            border: 2px solid #d63030;
        }

        &__continue {

            &__container {
                margin-top: 70px;
            }

            &__area {
                background: #fff;
                max-width: 1120px;
                height: 415px;
                display: flex;
                margin: 0 auto;
                justify-content: space-between;
                border-radius: 20px;
                padding-bottom: 60px;
            }

            &__img {
                width: 50%;
                margin-top: 30px;
            }

            &__text-area {
                width: calc(50% - 80px);
                padding: 0 40px;
            }

            &__header {
                font-family: 'font_bold', sans-serif;
                font-size: 32px;
                line-height: 38px;
                margin-top: 25px;
                margin-bottom: 11px;
            }

            &__text {
                font-family: 'font_regular', sans-serif;
                font-size: 16px;
                line-height: 32px;
            }

            &__button {
                margin-top: 30px;
            }
        }

        &__path-area {
            display: flex;
            justify-content: space-between;
            max-width: 1120px;
            margin: 0 auto;
        }

        &__path-section {
            background: #fff;
            text-align: center;
            border-radius: 20px;
            padding: 60px 50px;
            width: 22%;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 24px;
                line-height: 29px;
                margin: 10px auto 20px auto;
            }

            &__text {
                font-family: 'font_regular', sans-serif;
                font-size: 16px;
                line-height: 24px;
                min-height: 72px;
            }

            &__icon {
                max-height: 57px;
            }

            &__button {
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                margin-top: 40px;
                border: 2px solid #2683ff;
                width: calc(100% - 4px);
                color: #2683ff;
                border-radius: 10px;
                display: flex;
                align-items: center;
                justify-content: center;
                height: 23px;
                padding: 14px 0;

                &:hover {
                    color: #fff;
                    background: #2683ff;
                }
            }
        }

        &__integrations-button {
            font-family: 'font_normal', sans-serif;
            font-size: 16px;
            color: #fff;
            border-radius: 10px;
            display: block;
            text-align: center;
            padding: 16px 0;
            margin: 60px auto 0 auto;
            background: #2683ff;
            width: 355px;

            &:hover {
                background: darken(#2683ff, 10%);
            }
        }

        &__skip-button {
            margin: 20px 0 40px 0;
        }

        .link-disabled {
            background-color: #dadde5;
            border-color: #dadde5;
            color: #fff;
            pointer-events: none;
        }
    }
</style>
