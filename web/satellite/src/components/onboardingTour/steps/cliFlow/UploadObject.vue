// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <CLIFlowContainer
        :on-back-click="onBackClick"
        :on-next-click="onNextClick"
        title="Ready to upload"
    >
        <template #icon>
            <Icon />
        </template>
        <template #content class="upload-object">
            <p class="upload-object__msg">
                Here is an example image you can use for your first upload. Just right-click on it and save as
                cheesecake.jpg to your Desktop.
            </p>
            <img class="upload-object__icon" src="@/../static/images/onboardingTour/cheesecake.jpg" alt="Cheesecake">
            <p class="upload-object__msg">
                Now to upload the photo, use the copy command.
            </p>
            <OSContainer>
                <template #windows>
                    <TabWithCopy value="./uplink.exe cp <FILE_PATH> sj://cakes" aria-role-description="windows-upload" />
                </template>
                <template #linux>
                    <TabWithCopy value="uplink cp ~/Desktop/cheesecake.jpg sj://cakes" aria-role-description="linux-upload" />
                </template>
                <template #macos>
                    <TabWithCopy value="uplink cp ~/Desktop/cheesecake.jpg sj://cakes" aria-role-description="macos-upload" />
                </template>
            </OSContainer>
        </template>
    </CLIFlowContainer>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';
import { AnalyticsHttpApi } from '@/api/analytics';

import CLIFlowContainer from '@/components/onboardingTour/steps/common/CLIFlowContainer.vue';
import OSContainer from '@/components/onboardingTour/steps/common/OSContainer.vue';
import TabWithCopy from '@/components/onboardingTour/steps/common/TabWithCopy.vue';

import Icon from '@/../static/images/onboardingTour/uploadStep.svg';

// @vue/component
@Component({
    components: {
        CLIFlowContainer,
        OSContainer,
        TabWithCopy,
        Icon,
    },
})
export default class UploadObject extends Vue {

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
     * Holds on back button click logic.
     */
    public async onBackClick(): Promise<void> {
        this.analytics.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.CreateBucket)).path);
        await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.CreateBucket)).path);
    }

    /**
     * Holds on next button click logic.
     */
    public async onNextClick(): Promise<void> {
        this.analytics.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.ListObject)).path);
        await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.ListObject)).path);
    }
}
</script>

<style scoped lang="scss">
    .upload-object {
        font-family: 'font_regular', sans-serif;

        &__msg {
            font-size: 16px;
            line-height: 24px;
            color: #1b2533;
            align-self: flex-start;
        }

        &__icon {
            margin: 20px 0;
            width: 100%;
        }
    }
</style>
