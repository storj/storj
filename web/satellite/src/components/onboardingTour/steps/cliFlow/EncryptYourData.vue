// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="encrypt">
        <GeneratePassphrase
            :on-next-click="onNextClick"
            :on-skip-click="onSkipClick"
            :on-back-click="onBackClick"
            :set-parent-passphrase="setPassphrase"
            :is-loading="isLoading"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from "@/router";
import { MetaUtils } from "@/utils/meta";
import { ACCESS_GRANTS_ACTIONS, ACCESS_GRANTS_MUTATIONS } from "@/store/modules/accessGrants";
import { AccessGrant } from "@/types/accessGrants";

import GeneratePassphrase from "@/components/common/GeneratePassphrase.vue";

// @vue/component
@Component({
    components: {
        GeneratePassphrase,
    }
})
export default class EncryptYourData extends Vue {
    private worker: Worker;

    public isLoading = true;
    public passphrase = '';

    /**
     * Lifecycle hook after initial render.
     * Sets local web worker.
     */
    public mounted(): void {
        this.setWorker();

        this.isLoading = false;
    }

    /**
     * Sets passphrase from child component.
     */
    public setPassphrase(passphrase: string): void {
        this.passphrase = passphrase;
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
     * Holds on next button click logic.
     * Generates access grant.
     */
    public async onNextClick(): Promise<void> {
        if (this.isLoading) return;

        this.isLoading = true;

        try {
            const restrictedKey = await this.generateRestrictedKey();

            const satelliteNodeURL: string = MetaUtils.getMetaContent('satellite-nodeurl');
            this.worker.postMessage({
                'type': 'GenerateAccess',
                'apiKey': restrictedKey,
                'passphrase': this.passphrase,
                'projectID': this.$store.getters.selectedProject.id,
                'satelliteNodeURL': satelliteNodeURL,
            });

            const accessGrantEvent: MessageEvent = await new Promise(resolve => this.worker.onmessage = resolve);
            if (accessGrantEvent.data.error) {
                await this.$notify.error(accessGrantEvent.data.error);
            }

            await this.$store.commit(ACCESS_GRANTS_MUTATIONS.SET_ONBOARDING_ACCESS_GRANT, accessGrantEvent.data.value);
            await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.CLIStep.with(RouteConfig.GeneratedAG)).path);
        } catch (error) {
            await this.$notify.error(error.message)
        }

        this.isLoading = false;
    }

    /**
     * Holds on skip button click logic.
     * Generates CLI API key.
     */
    public async onSkipClick(): Promise<void> {
        if (this.isLoading) return;

        this.isLoading = true;

        try {
            const restrictedKey = await this.generateRestrictedKey();

            await this.$store.commit(ACCESS_GRANTS_MUTATIONS.SET_ONBOARDING_CLI_API_KEY, restrictedKey);
            await this.$router.push(RouteConfig.OnboardingTour.with(RouteConfig.CLIStep.with(RouteConfig.APIKey)).path);
        } catch (error) {
            await this.$notify.error(error.message)
        }

        this.isLoading = false;
    }

    /**
     * Holds on back button click logic.
     * Navigates to previous screen.
     */
    public async onBackClick(): Promise<void> {
        await this.$router.push(RouteConfig.OnboardingTour.path).catch(() => {return; })
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
}
</script>
