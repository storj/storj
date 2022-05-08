// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="overview-area">
        <div class="overview-area__header">
            <WelcomeLeft />
            <h1 class="overview-area__header__title" aria-roledescription="title">Welcome</h1>
            <WelcomeRight />
        </div>
        <p class="overview-area__subtitle">Let's get you started using Storj</p>
        <p class="overview-area__question">Do you want to use web browser or command-line interface?</p>
        <div class="overview-area__routes">
            <OverviewContainer
                class="overview-area__routes__left-cont"
                is-web="true"
                title="Web browser"
                info="Start uploading files in the browser and instantly see how your data gets distributed over the Storj network around the world."
                button-label="Continue in web"
                :on-click="onUploadInBrowserClick"
                :is-disabled="isLoading"
            />
            <OverviewContainer
                title="Command line"
                info="The Uplink CLI is a command-line interface which allows you to upload and download files from the network, manage permissions and sharing."
                button-label="Continue in cli"
                :on-click="onUplinkCLIClick"
                :is-disabled="isLoading"
            />
        </div>
        <router-link
            class="overview-area__skip-button"
            :to="projectDashboardPath"
        >
            Skip and go directly to dashboard
        </router-link>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import OverviewContainer from '@/components/onboardingTour/steps/common/OverviewContainer.vue';
import WelcomeLeft from '@/../static/images/onboardingTour/welcome-left.svg';
import WelcomeRight from '@/../static/images/onboardingTour/welcome-right.svg';

import { AnalyticsHttpApi } from '@/api/analytics';
import { RouteConfig } from '@/router';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';

// @vue/component
@Component({
    components: {
        OverviewContainer,
        WelcomeLeft,
        WelcomeRight
    },
})
export default class OverviewStep extends Vue {
    public isLoading = false;
    public projectDashboardPath = RouteConfig.ProjectDashboard.path;

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
     * Holds button click logic.
     * Redirects to next step (creating access grant).
     */
    public async onUplinkCLIClick(): Promise<void> {
        if (this.isLoading) return;

        this.isLoading = true;

        await this.analytics.linkEventTriggered(AnalyticsEvent.PATH_SELECTED, 'CLI');
        await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.CLIStep).with(RouteConfig.APIKey).path);

        this.isLoading = false;
    }

    /**
     * Redirects to objects page.
     */
    public async onUploadInBrowserClick(): Promise<void> {
        if (this.isLoading) return;

        this.isLoading = true;

        await this.analytics.linkEventTriggered(AnalyticsEvent.PATH_SELECTED, 'Continue in Browser');
        await this.$router.push(RouteConfig.Buckets.path).catch(() => {return; });

        this.isLoading = false;
    }
}
</script>

<style scoped lang="scss">
    .overview-area {
        display: flex;
        flex-direction: column;
        align-items: center;
        font-family: 'font_regular', sans-serif;

        &__header {
            display: flex;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 48px;
                line-height: 48px;
                letter-spacing: 1px;
                color: #14142b;
                margin: 0 20px 20px;
            }
        }

        &__subtitle,
        &__question {
            font-family: 'font_medium', sans-serif;
            font-size: 16px;
            line-height: 32px;
            text-align: center;
            letter-spacing: 0.75px;
            color: #14142a;
        }

        &__question {
            font-family: 'font_regular', sans-serif;
        }

        &__routes {
            margin-top: 70px;
            display: flex;
            align-items: center;

            &__left-cont {
                margin-right: 45px;
            }
        }

        &__skip-button {
            margin-top: 50px;
            color: #b7c1ca;
            cursor: pointer;
            text-decoration: underline !important;

            &:hover {
                text-decoration: underline;
            }
        }
    }
</style>
