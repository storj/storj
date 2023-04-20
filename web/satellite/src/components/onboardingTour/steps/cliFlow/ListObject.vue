// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <CLIFlowContainer
        :on-back-click="onBackClick"
        :on-next-click="onNextClick"
        title="Listing a bucket"
    >
        <template #icon>
            <Icon />
        </template>
        <template #content class="list-object">
            <p class="list-object__msg">
                To view the cheesecake photo in our bucket, let's use the list command:
            </p>
            <OSContainer>
                <template #windows>
                    <TabWithCopy value="./uplink.exe ls sj://cakes" aria-role-description="windows-list" />
                </template>
                <template #linux>
                    <TabWithCopy value="uplink ls sj://cakes" aria-role-description="linux-list" />
                </template>
                <template #macos>
                    <TabWithCopy value="uplink ls sj://cakes" aria-role-description="macos-list" />
                </template>
            </OSContainer>
        </template>
    </CLIFlowContainer>
</template>

<script setup lang="ts">
import { RouteConfig } from '@/router';
import { AnalyticsHttpApi } from '@/api/analytics';
import { useRouter } from '@/utils/hooks';

import CLIFlowContainer from '@/components/onboardingTour/steps/common/CLIFlowContainer.vue';
import OSContainer from '@/components/onboardingTour/steps/common/OSContainer.vue';
import TabWithCopy from '@/components/onboardingTour/steps/common/TabWithCopy.vue';

import Icon from '@/../static/images/onboardingTour/listObjectStep.svg';

const router = useRouter();

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

/**
 * Holds on back button click logic.
 */
async function onBackClick(): Promise<void> {
    analytics.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.UploadObject)).path);
    await router.push(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.UploadObject)).path);
}

/**
 * Holds on next button click logic.
 */
async function onNextClick(): Promise<void> {
    analytics.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.DownloadObject)).path);
    await router.push(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.DownloadObject)).path);
}
</script>

<style scoped lang="scss">
    .list-object {
        font-family: 'font_regular', sans-serif;

        &__msg {
            font-size: 16px;
            line-height: 24px;
            color: #1b2533;
        }
    }
</style>
