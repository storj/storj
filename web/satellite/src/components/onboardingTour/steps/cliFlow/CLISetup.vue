// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <CLIFlowContainer
        :on-back-click="onBackClick"
        :on-next-click="onNextClick"
        title="Uplink setup"
    >
        <template #icon>
            <Icon />
        </template>
        <template #content class="cli">
            <p class="cli__msg">
                To configure your Uplink CLI, run
                <b class="cli__msg__bold">uplink setup</b>.
            </p>
            <OSContainer>
                <template #windows>
                    <TabWithCopy value="./uplink.exe setup" aria-role-description="windows-cli-setup" />
                </template>
                <template #linux>
                    <TabWithCopy value="uplink setup" aria-role-description="linux-cli-setup" />
                </template>
                <template #macos>
                    <TabWithCopy value="uplink setup" aria-role-description="macos-cli-setup" />
                </template>
            </OSContainer>
            <p class="cli__msg">Follow the prompts. When asked for your API Key, enter the token from the previous step.</p>
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

import Icon from '@/../static/images/onboardingTour/cliSetupStep.svg';

// @vue/component
@Component({
    components: {
        CLIFlowContainer,
        Icon,
        OSContainer,
        TabWithCopy,
    },
})
export default class CLISetup extends Vue {

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
     * Holds on back button click logic.
     */
    public async onBackClick(): Promise<void> {
        this.analytics.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.CLIInstall)).path);
        await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.CLIInstall)).path);
    }

    /**
     * Holds on next button click logic.
     */
    public async onNextClick(): Promise<void> {
        this.analytics.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.CreateBucket)).path);
        await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.CreateBucket)).path);
    }
}
</script>

<style scoped lang="scss">
    .cli {
        font-family: 'font_regular', sans-serif;

        &__msg {
            font-size: 16px;
            line-height: 24px;
            color: #1b2533;
            align-self: flex-start;

            &__bold {
                font-family: 'font_medium', sans-serif;
            }

            &:first-of-type {
                margin-bottom: 40px;
            }

            &:last-of-type {
                margin-top: 40px;
            }
        }
    }
</style>
