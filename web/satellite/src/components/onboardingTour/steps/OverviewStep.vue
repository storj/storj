// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="overview-area">
        <h1 class="overview-area__title" aria-roledescription="title">Welcome to Storj {{ titleLabel }}</h1>
        <p class="overview-area__subtitle">Get started using the web browser, or the command line.</p>
        <div class="overview-area__routes">
            <OverviewContainer
                is-web="true"
                title="Start with web browser"
                info="Start uploading files in the browser and instantly see how your data gets distributed over the Storj network around the world."
                button-label="Continue in web ->"
                :on-click="onUploadInBrowserClick"
                :is-disabled="isLoading"
            />
            <OverviewContainer
                title="Start with Uplink CLI"
                info="The Uplink CLI is a command-line interface tool which allows you to upload and download files from the network, manage permissions and share files."
                button-label="Continue in cli ->"
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

import { AnalyticsHttpApi } from '@/api/analytics';
import { RouteConfig } from '@/router';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { MetaUtils } from '@/utils/meta';
import { PartneredSatellite } from '@/types/common';

import OverviewContainer from '@/components/onboardingTour/steps/common/OverviewContainer.vue';

// @vue/component
@Component({
    components: {
        OverviewContainer,
    },
})
export default class OverviewStep extends Vue {
    public isLoading = false;
    public projectDashboardPath = RouteConfig.ProjectDashboard.path;
    public titleLabel = '';

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
     * Mounted hook after initial render.
     * Sets correct title label.
     */
    public mounted(): void {
        const partneredSatellites = MetaUtils.getMetaContent('partnered-satellites');
        if (!partneredSatellites) {
            this.titleLabel = 'OSP';
            return;
        }

        const partneredSatellitesJSON = JSON.parse(partneredSatellites);
        const isPartnered = partneredSatellitesJSON.find((el: PartneredSatellite) => {
            return el.name === this.satelliteName;
        });
        if (isPartnered) {
            this.titleLabel = 'DCS';
            return;
        }

        this.titleLabel = 'OSP';
    }

    /**
     * Holds button click logic.
     * Redirects to next step (creating access grant).
     */
    public async onUplinkCLIClick(): Promise<void> {
        if (this.isLoading) return;

        this.isLoading = true;

        await this.analytics.linkEventTriggered(AnalyticsEvent.PATH_SELECTED, 'CLI');
        this.analytics.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.CLIStep).with(RouteConfig.AGName).path);
        await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.CLIStep).with(RouteConfig.AGName).path);

        this.isLoading = false;
    }

    /**
     * Redirects to buckets page.
     */
    public async onUploadInBrowserClick(): Promise<void> {
        if (this.isLoading) return;

        this.isLoading = true;

        await this.analytics.linkEventTriggered(AnalyticsEvent.PATH_SELECTED, 'Continue in Browser');
        this.analytics.pageVisit(RouteConfig.Buckets.path);
        await this.$router.push(RouteConfig.Buckets.path).catch(() => {return; });

        this.isLoading = false;
    }

    private get satelliteName(): string {
        return this.$store.state.appStateModule.satelliteName;
    }
}
</script>

<style scoped lang="scss">
.overview-area {
    display: flex;
    flex-direction: column;
    align-items: center;
    font-family: 'font_regular', sans-serif;

    &__title {
        font-family: 'font_bold', sans-serif;
        color: #14142b;
        font-size: 32px;
        line-height: 39px;
        margin-bottom: 12.5px;
    }

    &__subtitle {
        font-family: 'font_regular', sans-serif;
        font-weight: 400;
        text-align: center;
        color: #354049;
        font-size: 16px;
        line-height: 21px;
    }

    &__routes {
        margin-top: 35px;
        display: flex;
        align-items: center;
        justify-content: center;
        flex-wrap: wrap;
        column-gap: 38px;
        row-gap: 38px;
    }

    &__skip-button {
        margin-top: 58px;
        color: #b7c1ca;
        cursor: pointer;
        text-decoration: underline !important;

        &:hover {
            text-decoration: underline;
        }
    }
}

@media screen and (max-width: 760px) {

    .overview-area {
        width: 250px;
        text-align: center;
    }
}
</style>
