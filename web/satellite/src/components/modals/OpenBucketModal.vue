// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <Icon />
                <h1 class="modal__title">Open a Bucket</h1>
                <p class="modal__info">
                    To open a bucket and view your files, please enter the encryption passphrase you saved upon creating this bucket.
                </p>
                <VInput
                    class="modal__input"
                    label="Bucket Name"
                    :init-value="bucketName"
                    role-description="bucket"
                    disabled="true"
                />
                <VInput
                    label="Encryption Passphrase"
                    placeholder="Enter a passphrase here"
                    :error="enterError"
                    role-description="passphrase"
                    is-password="true"
                    :disabled="isLoading"
                    @setData="setPassphrase"
                />
                <div class="modal__buttons">
                    <VButton
                        label="Cancel"
                        height="48px"
                        is-transparent="true"
                        :on-press="closeModal"
                        :is-disabled="isLoading"
                    />
                    <VButton
                        label="Continue ->"
                        height="48px"
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
import { RouteConfig } from '@/router';
import { OBJECTS_ACTIONS } from '@/store/modules/objects';
import { MetaUtils } from '@/utils/meta';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { AnalyticsHttpApi } from '@/api/analytics';

import VModal from '@/components/common/VModal.vue';
import VInput from '@/components/common/VInput.vue';
import VButton from '@/components/common/VButton.vue';

import Icon from '@/../static/images/objects/openBucket.svg';

// @vue/component
@Component({
    components: {
        VInput,
        VModal,
        Icon,
        VButton,
    },
})
export default class OpenBucketModal extends Vue {
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

        this.isLoading = true;

        try {
            await this.setAccess();
            this.isLoading = false;

            if (this.enterError) return;

            this.closeModal();
            this.analytics.pageVisit(RouteConfig.Buckets.with(RouteConfig.UploadFile).path);
            await this.$router.push(RouteConfig.UploadFile.path);
        } catch (e) {
            await this.$notify.error(e.message);
            this.isLoading = false;
        }
    }

    /**
     * Sets access to S3 client.
     */
    public async setAccess(): Promise<void> {
        if (!this.passphrase) {
            this.enterError = 'Passphrase can\'t be empty';

            return;
        }

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
            'buckets': this.bucketName ? [this.bucketName] : [],
            'apiKey': this.apiKey,
        });

        const grantEvent: MessageEvent = await new Promise(resolve => this.worker.onmessage = resolve);
        if (grantEvent.data.error) {
            throw new Error(grantEvent.data.error);
        }

        const satelliteNodeURL: string = MetaUtils.getMetaContent('satellite-nodeurl');
        this.worker.postMessage({
            'type': 'GenerateAccess',
            'apiKey': grantEvent.data.value,
            'passphrase': this.passphrase,
            'projectID': this.$store.getters.selectedProject.id,
            'satelliteNodeURL': satelliteNodeURL,
        });

        const accessGrantEvent: MessageEvent = await new Promise(resolve => this.worker.onmessage = resolve);
        if (accessGrantEvent.data.error) {
            throw new Error(accessGrantEvent.data.error);
        }

        const accessGrant = accessGrantEvent.data.value;

        const gatewayCredentials: EdgeCredentials = await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.GET_GATEWAY_CREDENTIALS, { accessGrant });
        await this.$store.dispatch(OBJECTS_ACTIONS.SET_GATEWAY_CREDENTIALS, gatewayCredentials);
    }

    /**
     * Sets local worker with worker instantiated in store.
     */
    public setWorker(): void {
        this.worker = this.$store.state.accessGrantsModule.accessGrantsWebWorker;
        this.worker.onerror = (error: ErrorEvent) => {
            this.$notify.error(error.message);
        };
    }

    /**
     * Closes open bucket modal.
     */
    public closeModal(): void {
        if (this.isLoading) return;

        this.$store.commit(APP_STATE_MUTATIONS.TOGGLE_OPEN_BUCKET_MODAL_SHOWN);
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
     * Returns chosen bucket name from store.
     */
    public get bucketName(): string {
        return this.$store.state.objectsModule.fileComponentBucketName;
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

        &__input {
            margin-bottom: 21px;
        }

        &__buttons {
            display: flex;
            column-gap: 20px;
            margin-top: 31px;
            width: 100%;
        }
    }
</style>
