// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="enter-passphrase">
        <BackIcon class="enter-passphrase__back-icon" @click="onBackClick" />
        <h1 class="enter-passphrase__title">Enter Encryption Passphrase</h1>
        <p class="enter-passphrase__sub-title">Enter the passphrase you most recently generated for Access Grants</p>
        <VInput
            label="Encryption Passphrase"
            placeholder="Enter your passphrase here"
            :error="errorMessage"
            @setData="onChangePassphrase"
        />
        <VButton
            class="enter-passphrase__next-button"
            label="Next"
            width="100%"
            height="48px"
            :on-press="onNextClick"
            :is-disabled="isLoading"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';
import { MetaUtils } from '@/utils/meta';
import { AnalyticsHttpApi } from '@/api/analytics';

import VInput from '@/components/common/VInput.vue';
import VButton from '@/components/common/VButton.vue';

import BackIcon from '@/../static/images/accessGrants/back.svg';

// @vue/component
@Component({
    components: {
        VInput,
        VButton,
        BackIcon,
    },
})
export default class EnterPassphraseStep extends Vue {
    private key = '';
    private restrictedKey = '';
    private access = '';
    private worker: Worker;
    private isLoading = true;

    public passphrase = '';
    public errorMessage = '';

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
     * Lifecycle hook after initial render.
     * Sets local key from props value.
     */
    public async mounted(): Promise<void> {
        if (!this.$route.params.key && !this.$route.params.restrictedKey) {
            this.analytics.pageVisit(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.NameStep)).path);
            await this.$router.push(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.NameStep)).path);

            return;
        }

        this.key = this.$route.params.key;
        this.restrictedKey = this.$route.params.restrictedKey;

        this.setWorker();

        this.isLoading = false;
    }

    /**
     * Changes passphrase data from input value.
     * @param value
     */
    public onChangePassphrase(value: string): void {
        this.passphrase = value.trim();
        this.errorMessage = '';
    }

    /**
     * Holds on next button click logic.
     * Generates access grant and redirects to next step.
     */
    public async onNextClick(): Promise<void> {
        if (!this.passphrase) {
            this.errorMessage = 'Passphrase can`t be empty';

            return;
        }

        this.isLoading = true;

        const satelliteNodeURL = MetaUtils.getMetaContent('satellite-nodeurl');

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

        this.analytics.pageVisit(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.ResultStep)).path);
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
     * Holds on back button click logic.
     * Redirects to previous step.
     */
    public onBackClick(): void {
        this.analytics.pageVisit(RouteConfig.AccessGrants.with(RouteConfig.CreateAccessGrant.with(RouteConfig.PermissionsStep)).path);
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
    .enter-passphrase {
        height: calc(100% - 60px);
        padding: 30px 65px;
        max-width: 475px;
        min-width: 475px;
        font-family: 'font_regular', sans-serif;
        font-style: normal;
        display: flex;
        flex-direction: column;
        align-items: center;
        position: relative;
        background-color: #fff;
        border-radius: 0 6px 6px 0;

        &__back-icon {
            position: absolute;
            top: 30px;
            left: 65px;
            cursor: pointer;
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-weight: bold;
            font-size: 22px;
            line-height: 27px;
            color: #000;
            margin: 0 0 30px;
        }

        &__sub-title {
            font-weight: normal;
            font-size: 16px;
            line-height: 21px;
            color: #000;
            text-align: center;
            margin: 0 0 75px;
            max-width: 340px;
        }

        &__next-button {
            margin-top: 93px;
        }
    }

    .border-radius {
        border-radius: 6px;
    }
</style>

