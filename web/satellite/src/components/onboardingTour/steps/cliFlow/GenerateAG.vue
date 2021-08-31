// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <CLIFlowContainer
        :on-back-click="onBackClick"
        :on-next-click="onNextClick"
        title="Generate an Access Grant"
    >
        <template #icon>
            <Icon />
        </template>
        <template #content class="generate-ag">
            <p class="generate-ag__msg">
                Generate an Access Grant by running
                <b class="generate-ag__msg__bold">uplink share</b>
                with no restrictions. If you chose an access name, you'll need to specify it in the following command as
                <b class="generate-ag__msg__bold">--access=name</b>
            </p>
            <OSContainer>
                <template #windows>
                    <TabWithCopy value="./uplink.exe share --readonly=false" />
                </template>
                <template #linux>
                    <TabWithCopy value="uplink share --readonly=false" />
                </template>
                <template #macos>
                    <TabWithCopy value="uplink share --readonly=false" />
                </template>
            </OSContainer>
            <p class="generate-ag__msg">Your Access Grant should have been output.</p>
        </template>
    </CLIFlowContainer>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import CLIFlowContainer from "@/components/onboardingTour/steps/common/CLIFlowContainer.vue";
import OSContainer from "@/components/onboardingTour/steps/common/OSContainer.vue";
import TabWithCopy from "@/components/onboardingTour/steps/common/TabWithCopy.vue";

import Icon from '@/../static/images/onboardingTour/generateAGStep.svg';
import {RouteConfig} from "@/router";

// @vue/component
@Component({
    components: {
        CLIFlowContainer,
        Icon,
        OSContainer,
        TabWithCopy,
    }
})
export default class GenerateAG extends Vue {
    /**
     * Holds on back button click logic.
     */
    public async onBackClick(): Promise<void> {
        await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.CLISetup)).path);
    }

    /**
     * Holds on next button click logic.
     */
    public async onNextClick(): Promise<void> {
        await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.CreateBucket)).path);
    }
}
</script>

<style scoped lang="scss">
    .generate-ag {
        font-family: 'font_regular', sans-serif;

        &__msg {
            font-size: 18px;
            line-height: 32px;
            letter-spacing: 0.15px;
            color: #4e4b66;

            &__bold {
                font-family: 'font_medium', sans-serif;
                white-space: nowrap;
            }
        }
    }
</style>
