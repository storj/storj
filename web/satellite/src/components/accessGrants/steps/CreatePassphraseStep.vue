// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="create-passphrase">
        <BackIcon class="create-passphrase__back-icon" @click="onBackClick" />
        <GeneratePassphrase
            :is-loading="isLoading"
            :on-button-click="onNextClick"
            :set-parent-passphrase="setPassphrase"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import GeneratePassphrase from '@/components/common/GeneratePassphrase.vue';

import BackIcon from '@/../static/images/accessGrants/back.svg';

import { RouteConfig } from '@/router';
import { MetaUtils } from '@/utils/meta';

// @vue/component
@Component({
    components: {
        BackIcon,
        GeneratePassphrase,
    },
})
export default class CreatePassphraseStep extends Vue {
    private key = '';
    private restrictedKey = '';
    private access = '';
    private worker: Worker;
    private isLoading = true;

    public passphrase = '';

    /**
     * Lifecycle hook after initial render.
     * Sets local key from props value.
     */
    public async mounted(): Promise<void> {
        if (!this.$route.params.key && !this.$route.params.restrictedKey) {
            await this.$router.push(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.NameStep)).path);

            return;
        }

        this.key = this.$route.params.key;
        this.restrictedKey = this.$route.params.restrictedKey;

        this.setWorker();

        this.isLoading = false;
    }

    /**
     * Sets local worker with worker instantiated in store.
     * Also sets worker's onmessage and onerror logic.
     */
    public setWorker(): void {
        this.worker = this.$store.state.accessGrantsModule.accessGrantsWebWorker;
        this.worker.onerror = (error: ErrorEvent) => {
            this.$notify.error(error.message);
        };
    }

    /**
     * Sets passphrase from child component.
     */
    public setPassphrase(passphrase: string): void {
        this.passphrase = passphrase;
    }

    /**
     * Holds on next button click logic.
     * Generates access grant and redirects to next step.
     */
    public async onNextClick(): Promise<void> {
        if (this.isLoading) return;

        this.isLoading = true;

        const satelliteNodeURL: string = MetaUtils.getMetaContent('satellite-nodeurl');

        this.worker.postMessage({
            'type': 'GenerateAccess',
            'apiKey': this.restrictedKey,
            'passphrase': this.passphrase,
            'projectID': this.$store.getters.selectedProject.id,
            'satelliteNodeURL': satelliteNodeURL,
        });

        const accessEvent: MessageEvent = await new Promise(resolve => this.worker.onmessage = resolve);
        if (accessEvent.data.error) {
            await this.$notify.error(accessEvent.data.error);
            this.isLoading = false;

            return;
        }

        this.access = accessEvent.data.value;
        await this.$notify.success('Access Grant was generated successfully');

        this.isLoading = false;

        await this.$router.push({
            name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.ResultStep)).name,
            params: {
                access: this.access,
                key: this.key,
                restrictedKey: this.restrictedKey,
            },
        });
    }

    /**
     * Holds on back button click logic.
     * Redirects to previous step.
     */
    public onBackClick(): void {
        this.$router.push({
            name: RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.PermissionsStep)).name,
            params: {
                key: this.key,
            },
        });
    }
}
</script>

<style scoped lang="scss">
    .create-passphrase {
        position: relative;

        &__back-icon {
            position: absolute;
            top: 30px;
            left: 65px;
            cursor: pointer;
        }
    }
</style>
