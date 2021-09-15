// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <CLIFlowContainer
        :on-back-click="onBackClick"
        :on-next-click="onNextClick"
        title="Create a bucket"
    >
        <template #icon>
            <Icon />
        </template>
        <template #content class="create-bucket">
            <p class="create-bucket__msg">
                Let's create a bucket to store your data.
                You can name your bucket using only lowercase alphanumeric characters (no spaces), like “cakes”.
            </p>
            <OSContainer>
                <template #windows>
                    <TabWithCopy value="./uplink.exe mb sj://cakes" />
                </template>
                <template #linux>
                    <TabWithCopy value="uplink mb sj://cakes" />
                </template>
                <template #macos>
                    <TabWithCopy value="uplink mb sj://cakes" />
                </template>
            </OSContainer>
        </template>
    </CLIFlowContainer>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from "@/router";

import CLIFlowContainer from "@/components/onboardingTour/steps/common/CLIFlowContainer.vue";
import OSContainer from "@/components/onboardingTour/steps/common/OSContainer.vue";
import TabWithCopy from "@/components/onboardingTour/steps/common/TabWithCopy.vue";

import Icon from "@/../static/images/onboardingTour/bucketStep.svg";

// @vue/component
@Component({
    components: {
        CLIFlowContainer,
        Icon,
        OSContainer,
        TabWithCopy,
    }
})
export default class CreateBucket extends Vue {
    /**
     * Holds on back button click logic.
     */
    public async onBackClick(): Promise<void> {
        await this.$router.push(this.backRoute);
    }

    /**
     * Holds on next button click logic.
     */
    public async onNextClick(): Promise<void> {
        await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.UploadObject)).path);
    }

    /**
     * Returns back route path from store.
     */
    private get backRoute(): string {
        return this.$store.state.appStateModule.appState.onbCLIFlowCreateBucketBackRoute;
    }
}
</script>

<style scoped lang="scss">
    .create-bucket {
        font-family: 'font_regular', sans-serif;

        &__msg {
            font-size: 18px;
            line-height: 32px;
            letter-spacing: 0.15px;
            color: #4e4b66;
        }
    }
</style>
