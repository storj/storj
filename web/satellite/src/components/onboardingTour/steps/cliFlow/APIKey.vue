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
            <ValueWithCopy label="Satellite Address" :value="satelliteAddress" />
            <h3 class="key__label">API Key</h3>
            <ValueWithCopy label="API Key" :value="apiKey" />
        </template>
    </CLIFlowContainer>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from "@/router";
import { MetaUtils } from "@/utils/meta";

import CLIFlowContainer from "@/components/onboardingTour/steps/common/CLIFlowContainer.vue";
import ValueWithCopy from "@/components/onboardingTour/steps/common/ValueWithCopy.vue";

import Icon from '@/../static/images/onboardingTour/apiKeyStep.svg';

// @vue/component
@Component({
    components: {
        Icon,
        CLIFlowContainer,
        ValueWithCopy,
    }
})
export default class APIKey extends Vue {
    public satelliteAddress: string = MetaUtils.getMetaContent('satellite-nodeurl');

    /**
     * Lifecycle hook before initial render.
     * Redirects to encrypt your data step if there is no API key to show.
     */
    public async beforeMount(): Promise<void> {
        if (!this.apiKey) {
            await this.onBackClick();
        }
    }

    /**
     * Holds on back button click logic.
     */
    public async onBackClick(): Promise<void> {
        await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.EncryptYourData)).path);
    }

    /**
     * Holds on next button click logic.
     */
    public async onNextClick(): Promise<void> {
        await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.CLISetup)).path);
    }

    /**
     * Returns API key from store.
     */
    public get apiKey(): string {
        return this.$store.state.accessGrantsModule.onboardingCLIApiKey;
    }
}
</script>

<style scoped lang="scss">
    .key {
        font-family: 'font_regular', sans-serif;

        &__msg {
            font-size: 18px;
            line-height: 32px;
            letter-spacing: 0.15px;
            color: #4e4b66;
        }

        &__label {
            font-family: 'font_bold', sans-serif;
            font-size: 16px;
            line-height: 21px;
            color: #354049;
            margin: 20px 0;
        }
    }
</style>