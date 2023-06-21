// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <CLIFlowContainer
        :on-back-click="onBackClick"
        :on-next-click="onNextClick"
        title="Download a file"
    >
        <template #icon>
            <Icon />
        </template>
        <template #content class="download-object">
            <p class="download-object__msg">
                To download the cheesecake photo, let's use the copy command:
            </p>
            <OSContainer>
                <template #windows>
                    <TabWithCopy value="./uplink.exe cp sj://cakes/cheesecake.jpg <DESTINATION_PATH>/cheesecake.jpg" aria-role-description="windows-download" />
                </template>
                <template #linux>
                    <TabWithCopy value="uplink cp sj://cakes/cheesecake.jpg ~/Downloads/cheesecake.jpg" aria-role-description="linux-download" />
                </template>
                <template #macos>
                    <TabWithCopy value="uplink cp sj://cakes/cheesecake.jpg ~/Downloads/cheesecake.jpg" aria-role-description="macos-download" />
                </template>
            </OSContainer>
        </template>
    </CLIFlowContainer>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router';

import { RouteConfig } from '@/types/router';
import { AnalyticsHttpApi } from '@/api/analytics';

import CLIFlowContainer from '@/components/onboardingTour/steps/common/CLIFlowContainer.vue';
import OSContainer from '@/components/onboardingTour/steps/common/OSContainer.vue';
import TabWithCopy from '@/components/onboardingTour/steps/common/TabWithCopy.vue';

import Icon from '@/../static/images/onboardingTour/downloadObjectStep.svg';

const router = useRouter();

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

/**
 * Holds on back button click logic.
 */
async function onBackClick(): Promise<void> {
    analytics.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.ListObject)).path);
    await router.push(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.ListObject)).path);
}

/**
 * Holds on next button click logic.
 */
async function onNextClick(): Promise<void> {
    analytics.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.ShareObject)).path);
    await router.push(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.ShareObject)).path);
}
</script>

<style scoped lang="scss">
    .download-object {
        font-family: 'font_regular', sans-serif;

        &__msg {
            font-size: 16px;
            line-height: 24px;
            color: #1b2533;
        }
    }
</style>
