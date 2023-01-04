// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <EnterPassphraseIcon />
                <h1 class="modal__title">Enter your encryption passphrase</h1>
                <p class="modal__info">
                    To open a project and view your encrypted files, <br>please enter your encryption passphrase.
                </p>
                <VInput
                    label="Encryption Passphrase"
                    placeholder="Enter your passphrase"
                    :error="enterError"
                    role-description="passphrase"
                    is-password
                    :disabled="isLoading"
                    @setData="setPassphrase"
                />
                <div class="modal__buttons">
                    <VButton
                        label="Enter without passphrase"
                        height="48px"
                        font-size="14px"
                        :is-transparent="true"
                        :on-press="closeModal"
                        :is-disabled="isLoading"
                    />
                    <VButton
                        label="Continue ->"
                        height="48px"
                        font-size="14px"
                        :on-press="onContinue"
                        :is-disabled="isLoading"
                    />
                </div>
            </div>
        </template>
    </VModal>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { OBJECTS_ACTIONS, OBJECTS_MUTATIONS } from '@/store/modules/objects';
import { MetaUtils } from '@/utils/meta';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { AnalyticsHttpApi } from '@/api/analytics';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

import VModal from '@/components/common/VModal.vue';
import VInput from '@/components/common/VInput.vue';
import VButton from '@/components/common/VButton.vue';

import EnterPassphraseIcon from '@/../static/images/buckets/openBucket.svg';

// @vue/component
@Component({
    components: {
        VInput,
        VModal,
        VButton,
        EnterPassphraseIcon,
    },
})
export default class EnterPassphraseModal extends Vue {
    private worker: Worker;
    private readonly FILE_BROWSER_AG_NAME: string = 'Web file browser API key';
    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    public enterError = '';
    public passphrase = '';
    public isLoading = false;

    /**
     * Lifecycle hook after initial render.
     * Sets local worker.
     */
    public mounted(): void {
        this.setWorker();
    }

    /**
     * Sets access and navigates to object browser.
     */
    public async onContinue(): Promise<void> {
        if (this.isLoading) return;

        if (!this.passphrase) {
            this.enterError = 'Passphrase can\'t be empty';
            this.analytics.errorEventTriggered(AnalyticsErrorEventSource.OPEN_BUCKET_MODAL);

            return;
        }

        this.isLoading = true;

        try {
            await this.setAccess();
            this.isLoading = false;

            this.closeModal();
        } catch (error) {
            await this.$notify.error(error.message, AnalyticsErrorEventSource.OPEN_BUCKET_MODAL);
            this.isLoading = false;
        }
    }

    /**
     * Sets access to S3 client.
     */
    public async setAccess(): Promise<void> {
        if (!this.apiKey) {
            await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.DELETE_BY_NAME_AND_PROJECT_ID, this.FILE_BROWSER_AG_NAME);
            const cleanAPIKey: AccessGrant = await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.CREATE, this.FILE_BROWSER_AG_NAME);
            await this.$store.dispatch(OBJECTS_ACTIONS.SET_API_KEY, cleanAPIKey.secret);
        }

        const now = new Date();
        const inThreeDays = new Date(now.setDate(now.getDate() + 3));

        await this.worker.postMessage({
            'type': 'SetPermission',
            'isDownload': true,
            'isUpload': true,
            'isList': true,
            'isDelete': true,
            'notAfter': inThreeDays.toISOString(),
            'buckets': [],
            'apiKey': this.apiKey,
        });

        const grantEvent: MessageEvent = await new Promise(resolve => this.worker.onmessage = resolve);
        if (grantEvent.data.error) {
            throw new Error(grantEvent.data.error);
        }

        const salt = await this.$store.dispatch(PROJECTS_ACTIONS.GET_SALT, this.$store.getters.selectedProject.id);
        const satelliteNodeURL: string = MetaUtils.getMetaContent('satellite-nodeurl');

        this.worker.postMessage({
            'type': 'GenerateAccess',
            'apiKey': grantEvent.data.value,
            'passphrase': this.passphrase,
            'salt': salt,
            'satelliteNodeURL': satelliteNodeURL,
        });

        const accessGrantEvent: MessageEvent = await new Promise(resolve => this.worker.onmessage = resolve);
        if (accessGrantEvent.data.error) {
            throw new Error(accessGrantEvent.data.error);
        }

        const accessGrant = accessGrantEvent.data.value;

        const gatewayCredentials: EdgeCredentials = await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.GET_GATEWAY_CREDENTIALS, { accessGrant });
        await this.$store.dispatch(OBJECTS_ACTIONS.SET_GATEWAY_CREDENTIALS, gatewayCredentials);
        await this.$store.dispatch(OBJECTS_ACTIONS.SET_S3_CLIENT);
        await this.$store.commit(OBJECTS_MUTATIONS.SET_PROMPT_FOR_PASSPHRASE, false);
    }

    /**
     * Sets local worker with worker instantiated in store.
     */
    public setWorker(): void {
        this.worker = this.$store.state.accessGrantsModule.accessGrantsWebWorker;
        this.worker.onerror = (error: ErrorEvent) => {
            this.$notify.error(error.message, AnalyticsErrorEventSource.OPEN_BUCKET_MODAL);
        };
    }

    /**
     * Closes open bucket modal.
     */
    public closeModal(): void {
        if (this.isLoading) return;

        this.$store.commit(APP_STATE_MUTATIONS.REMOVE_ACTIVE_MODAL);
    }

    /**
     * Sets passphrase from child component.
     */
    public setPassphrase(passphrase: string): void {
        if (this.enterError) this.enterError = '';

        this.passphrase = passphrase;
        this.$store.dispatch(OBJECTS_ACTIONS.SET_PASSPHRASE, this.passphrase);
    }

    /**
     * Returns apiKey from store.
     */
    private get apiKey(): string {
        return this.$store.state.objectsModule.apiKey;
    }
}
</script>

<style scoped lang="scss">
    .modal {
        font-family: 'font_regular', sans-serif;
        display: flex;
        flex-direction: column;
        align-items: center;
        padding: 62px 62px 54px;
        max-width: 500px;

        @media screen and (max-width: 600px) {
            padding: 62px 24px 54px;
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 26px;
            line-height: 31px;
            color: #131621;
            margin: 30px 0 15px;
        }

        &__info {
            font-size: 16px;
            line-height: 21px;
            text-align: center;
            color: #354049;
            margin-bottom: 32px;
        }

        &__buttons {
            display: flex;
            column-gap: 20px;
            margin-top: 31px;
            width: 100%;

            @media screen and (max-width: 500px) {
                flex-direction: column-reverse;
                column-gap: unset;
                row-gap: 20px;
            }
        }
    }
</style>
