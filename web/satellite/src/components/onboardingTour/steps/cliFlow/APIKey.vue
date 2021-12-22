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
            <VLoader v-if="isLoading" width="100px" height="100px" />
            <template v-else>
                <p class="key__msg">Now copy and save the Satellite Address and API Key as it will only appear once.</p>
                <h3 class="key__label">Satellite Address</h3>
                <ValueWithCopy label="Satellite Address" role-description="satellite-address" :value="satelliteAddress" />
                <h3 class="key__label">API Key</h3>
                <ValueWithCopy label="API Key" role-description="api-key" :value="storedAPIKey || restrictedKey" />
            </template>
        </template>
    </CLIFlowContainer>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from "@/router";
import { MetaUtils } from "@/utils/meta";

import {ACCESS_GRANTS_ACTIONS} from "@/store/modules/accessGrants";
import {APP_STATE_MUTATIONS} from "@/store/mutationConstants";
import {AccessGrant} from "@/types/accessGrants";
import CLIFlowContainer from "@/components/onboardingTour/steps/common/CLIFlowContainer.vue";
import ValueWithCopy from "@/components/onboardingTour/steps/common/ValueWithCopy.vue";
import VLoader from "@/components/common/VLoader.vue";

import Icon from '@/../static/images/onboardingTour/apiKeyStep.svg';

// @vue/component
@Component({
    components: {
        Icon,
        CLIFlowContainer,
        ValueWithCopy,
        VLoader,
    }
})
export default class APIKey extends Vue {
    private worker: Worker;

    public satelliteAddress: string = MetaUtils.getMetaContent('satellite-nodeurl');
    public isLoading = true;
    public restrictedKey = '';

    /**
     * Lifecycle hook after initial render.
     * Sets local web worker.
     */
    public async mounted(): Promise<void> {
        if (this.storedAPIKey) {
            this.isLoading = false;

            return;
        }

        this.setWorker();
        await this.generateAPIKey()
    }

    /**
     * Sets local worker with worker instantiated in store.
     * Also sets worker's onerror logic.
     */
    public setWorker(): void {
        this.worker = this.$store.state.accessGrantsModule.accessGrantsWebWorker;
        this.worker.onerror = (error: ErrorEvent) => {
            this.$notify.error(error.message);
        };
    }

    /**
     * Generates CLI flow API key.
     */
    public async generateAPIKey(): Promise<void> {
        try {
            this.restrictedKey = await this.generateRestrictedKey();

            await this.$store.commit(APP_STATE_MUTATIONS.SET_ONB_API_KEY, this.restrictedKey);
        } catch (error) {
            await this.$notify.error(error.message)
        }

        this.isLoading = false;
    }

    /**
     * Generates and returns restricted key from clean API key.
     */
    private async generateRestrictedKey(): Promise<string> {
        const date = new Date().toISOString()
        const onbAGName = `Onboarding Grant ${date}`
        const cleanAPIKey: AccessGrant = await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.CREATE, onbAGName);

        await this.worker.postMessage({
            'type': 'SetPermission',
            'isDownload': true,
            'isUpload': true,
            'isList': true,
            'isDelete': true,
            'buckets': [],
            'apiKey': cleanAPIKey.secret,
        });

        const grantEvent: MessageEvent = await new Promise(resolve => this.worker.onmessage = resolve);
        if (grantEvent.data.error) {
            throw new Error(grantEvent.data.error)
        }

        return grantEvent.data.value;
    }

    /**
     * Holds on back button click logic.
     * Navigates to previous screen.
     */
    public async onBackClick(): Promise<void> {
        if (this.backRoute) {
            await this.$router.push(this.backRoute).catch(() => {return; });

            return;
        }

        await this.$router.push(RouteConfig.OnboardingTour.path).catch(() => {return; })
    }

    /**
     * Holds on next button click logic.
     */
    public async onNextClick(): Promise<void> {
        await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.OnbCLIStep.with(RouteConfig.CLIInstall)).path);
    }

    /**
     * Returns API key from store.
     */
    public get storedAPIKey(): string {
        return this.$store.state.appStateModule.appState.onbApiKey;
    }

    /**
     * Returns back route from store.
     */
    private get backRoute(): string {
        return this.$store.state.appStateModule.appState.onbAPIKeyStepBackRoute;
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