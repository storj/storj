// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <CLIFlowContainer
        :on-back-click="onBackClick"
        :on-next-click="onNextClick"
        title="Import Your  Access Grant"
    >
        <template #icon>
            <Icon />
        </template>
        <template #content class="import">
            <p class="import__msg">
                To import the Access Grant that you created earlier, it needs to be saved in a plain text file.
            </p>
            <OSContainer>
                <template #windows>
                    <TabWithCopy value="./uplink.exe import <file_name>.txt" />
                </template>
                <template #linux>
                    <TabWithCopy value="uplink import <file_name>.txt" />
                </template>
                <template #macos>
                    <TabWithCopy value="uplink import <file_name>.txt" />
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

import Icon from '@/../static/images/onboardingTour/importStep.svg';

// @vue/component
@Component({
    components: {
        CLIFlowContainer,
        Icon,
        OSContainer,
        TabWithCopy,
    }
})
export default class ImportAG extends Vue {
    /**
     * Holds on back button click logic.
     */
    public async onBackClick(): Promise<void> {
        await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.CLIInstall)).path);
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
    .import {
        font-family: 'font_regular', sans-serif;

        &__msg {
            font-size: 18px;
            line-height: 32px;
            letter-spacing: 0.15px;
            color: #4e4b66;
        }
    }
</style>
