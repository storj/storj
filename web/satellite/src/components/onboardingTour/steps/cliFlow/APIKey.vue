// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <CLIFlowContainer
        :on-back-click="onBackClick"
        :on-next-click="onNextClick"
        title="API Key Generated"
    >
        <template #icon>
            <Icon />
        </template>
        <template #content class="key">
            <p class="key__msg">Now copy and save the Satellite Address and API Key as it will only appear once.</p>
            <h3 class="key__label">Satellite Address</h3>
            <ValueWithCopy label="Satellite Address" role-description="satellite-address" :value="satelliteAddress" />
            <h3 class="key__label">API Key</h3>
            <ValueWithCopy label="API Key" role-description="api-key" :value="storedAPIKey" />
        </template>
    </CLIFlowContainer>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';
import { MetaUtils } from '@/utils/meta';
import { AnalyticsHttpApi } from '@/api/analytics';

import CLIFlowContainer from '@/components/onboardingTour/steps/common/CLIFlowContainer.vue';
import ValueWithCopy from '@/components/onboardingTour/steps/common/ValueWithCopy.vue';

import Icon from '@/../static/images/onboardingTour/apiKeyStep.svg';

// @vue/component
@Component({
    components: {
        Icon,
        CLIFlowContainer,
        ValueWithCopy,
    },
})
export default class APIKey extends Vue {
    public satelliteAddress: string = MetaUtils.getMetaContent('satellite-nodeurl');

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
     * Lifecycle hook after initial render.
     * Checks if api key was generated during previous step.
     */
    public mounted(): void {
        if (!this.storedAPIKey) {
            this.analytics.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.AGName)).path);
            this.$router.push({ name: RouteConfig.AGName.name });
        }
    }

    /**
     * Holds on back button click logic.
     * Navigates to previous screen.
     */
    public async onBackClick(): Promise<void> {
        if (this.backRoute) {
            this.analytics.pageVisit(this.backRoute);
            await this.$router.push(this.backRoute).catch(() => {return; });

            return;
        }

        this.analytics.pageVisit(RouteConfig.OnboardingTour.path);
        await this.$router.push(RouteConfig.OnboardingTour.path).catch(() => {return; });
    }

    /**
     * Holds on next button click logic.
     */
    public async onNextClick(): Promise<void> {
        this.analytics.pageVisit(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.CLIInstall)).path);
        await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.CLIInstall)).path);
    }

    /**
     * Returns API key from store.
     */
    public get storedAPIKey(): string {
        return this.$store.state.appStateModule.viewsState.onbApiKey;
    }

    /**
     * Returns back route from store.
     */
    private get backRoute(): string {
        return this.$store.state.appStateModule.viewsState.onbAPIKeyStepBackRoute;
    }
}
</script>

<style scoped lang="scss">
    .key {
        font-family: 'font_regular', sans-serif;

        &__msg {
            font-size: 16px;
            line-height: 24px;
            color: #1b2533;
        }

        &__label {
            font-family: 'font_bold', sans-serif;
            font-size: 14px;
            line-height: 20px;
            color: var(--c-grey-6);
            margin: 20px 0 13px;
            width: 100%;
        }
    }
</style>
