// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="overview-area">
        <h1 class="overview-area__title" aria-roledescription="title">Welcome to Storj</h1>
        <p class="overview-area__subtitle">Get started using the web browser, or the command line.</p>
        <div class="overview-area__routes">
            <OverviewContainer
                :is-web="true"
                title="Start with web browser"
                info="Start uploading files in the browser and instantly see how your data gets distributed over the Storj network around the world."
                button-label="Continue in web ->"
                :on-click="onUploadInBrowserClick"
            />
            <OverviewContainer
                title="Start with Uplink CLI"
                info="The Uplink CLI is a command-line interface tool which allows you to upload and download files from the network, manage permissions and share files."
                button-label="Continue in cli ->"
                :on-click="onUplinkCLIClick"
            />
        </div>
        <p class="overview-area__skip-button" @click="onSkip">
            Skip and go directly to dashboard
        </p>
    </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';

import { RouteConfig } from '@/types/router';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useNotify } from '@/utils/hooks';
import { useUsersStore } from '@/store/modules/usersStore';
import { useAppStore } from '@/store/modules/appStore';
import { useConfigStore } from '@/store/modules/configStore';
import { PartneredSatellite } from '@/types/config';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { OnboardingOption } from '@/types/common';

import OverviewContainer from '@/components/onboardingTour/steps/common/OverviewContainer.vue';

const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();
const appStore = useAppStore();
const usersStore = useUsersStore();
const notify = useNotify();
const router = useRouter();

const projectDashboardPath = RouteConfig.ProjectDashboard.path;

/**
 * Skips onboarding flow.
 */
async function onSkip(): Promise<void> {
    endOnboarding();
    await router.push(projectDashboardPath);
    appStore.updateActiveModal(MODALS.createProjectPassphrase);
    analyticsStore.linkEventTriggered(AnalyticsEvent.PATH_SELECTED, OnboardingOption.Skip);
}

/**
 * Holds button click logic.
 * Redirects to next step (creating access grant).
 */
function onUplinkCLIClick(): void {
    router.push(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep).with(RouteConfig.AGName).path);
    analyticsStore.linkEventTriggered(AnalyticsEvent.PATH_SELECTED, OnboardingOption.CLI);
    analyticsStore.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep).with(RouteConfig.AGName).path);
}

/**
 * Redirects to buckets page.
 */
async function onUploadInBrowserClick(): Promise<void> {
    endOnboarding();
    appStore.updateActiveModal(MODALS.createProjectPassphrase);
    analyticsStore.linkEventTriggered(AnalyticsEvent.PATH_SELECTED, OnboardingOption.Browser);
}

async function endOnboarding(): Promise<void> {
    try {
        await usersStore.updateSettings({ onboardingEnd: true });
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.ONBOARDING_OVERVIEW_STEP);
    }
}

/**
 * Mounted hook after initial render.
 * Sets correct title label.
 */
onMounted(async (): Promise<void> => {
    try {
        if (!usersStore.state.settings.onboardingStart) {
            await usersStore.updateSettings({ onboardingStart: true });
        }
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.ONBOARDING_OVERVIEW_STEP);
    }
});
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
        gap: 38px 38px;
    }

    &__skip-button {
        margin-top: 58px;
        color: #b7c1ca;
        cursor: pointer;
        text-decoration: underline;

        &:hover {
            text-decoration: underline;
        }
    }
}

@media screen and (width <= 760px) {

    .overview-area {
        width: 250px;
        text-align: center;
    }
}
</style>
