// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <h1 class="modal__title">Are you sure?</h1>
                <p class="modal__subtitle">
                    Deleting bucket will delete all metadata related to this bucket.
                </p>
                <VInput
                    class="modal__input"
                    label="Bucket Name"
                    placeholder="Enter bucket name"
                    :is-loading="isLoading"
                    @setData="onChangeName"
                />
                <VButton
                    label="Confirm Delete Bucket"
                    width="100%"
                    height="48px"
                    :on-press="onDelete"
                    :is-disabled="isLoading || !name"
                />
            </div>
        </template>
    </VModal>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { OBJECTS_ACTIONS } from '@/store/modules/objects';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { AnalyticsHttpApi } from '@/api/analytics';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { MetaUtils } from '@/utils/meta';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';

import VModal from '@/components/common/VModal.vue';
import VButton from '@/components/common/VButton.vue';
import VInput from '@/components/common/VInput.vue';

// @vue/component
@Component({
    components: {
        VInput,
        VButton,
        VModal,
    },
})
export default class DeleteBucketModal extends Vue {
    private worker: Worker;
    private name = '';
    private readonly FILE_BROWSER_AG_NAME: string = 'Web file browser API key';
    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    public isLoading = false;

    /**
     * Lifecycle hook after initial render.
     * Sets local worker.
     */
    public mounted(): void {
        this.setWorker();
    }

    /**
     * Holds on delete button click logic.
     * Creates unrestricted access grant and deletes bucket.
     */
    public async onDelete(): Promise<void> {
        if (this.isLoading) return;

        this.isLoading = true;

        try {
            if (!this.apiKey) {
                await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.DELETE_BY_NAME_AND_PROJECT_ID, this.FILE_BROWSER_AG_NAME);
                const cleanAPIKey: AccessGrant = await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.CREATE, this.FILE_BROWSER_AG_NAME);
                await this.$store.dispatch(OBJECTS_ACTIONS.SET_API_KEY, cleanAPIKey.secret);
            }

            const now = new Date();
            const inOneHour = new Date(now.setHours(now.getHours() + 1));

            await this.worker.postMessage({
                'type': 'SetPermission',
                'isDownload': false,
                'isUpload': false,
                'isList': true,
                'isDelete': true,
                'notAfter': inOneHour.toISOString(),
                'buckets': [this.name],
                'apiKey': this.apiKey,
            });

            const grantEvent: MessageEvent = await new Promise(resolve => this.worker.onmessage = resolve);
            if (grantEvent.data.error) {
                await this.$notify.error(grantEvent.data.error, AnalyticsErrorEventSource.DELETE_BUCKET_MODAL);
                return;
            }

            const salt = await this.$store.dispatch(PROJECTS_ACTIONS.GET_SALT, this.$store.getters.selectedProject.id);
            const satelliteNodeURL: string = MetaUtils.getMetaContent('satellite-nodeurl');

            this.worker.postMessage({
                'type': 'GenerateAccess',
                'apiKey': grantEvent.data.value,
                'passphrase': '',
                'salt': salt,
                'satelliteNodeURL': satelliteNodeURL,
            });

            const accessGrantEvent: MessageEvent = await new Promise(resolve => this.worker.onmessage = resolve);
            if (accessGrantEvent.data.error) {
                await this.$notify.error(accessGrantEvent.data.error, AnalyticsErrorEventSource.DELETE_BUCKET_MODAL);
                return;
            }

            const accessGrant = accessGrantEvent.data.value;

            const gatewayCredentials: EdgeCredentials = await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.GET_GATEWAY_CREDENTIALS, { accessGrant });
            await this.$store.dispatch(OBJECTS_ACTIONS.SET_GATEWAY_CREDENTIALS_FOR_DELETE, gatewayCredentials);
            await this.$store.dispatch(OBJECTS_ACTIONS.DELETE_BUCKET, this.name);
            this.analytics.eventTriggered(AnalyticsEvent.BUCKET_DELETED);
            await this.fetchBuckets();
        } catch (error) {
            await this.$notify.error(error.message, AnalyticsErrorEventSource.DELETE_BUCKET_MODAL);
            return;
        } finally {
            this.isLoading = false;
        }

        this.closeModal();
    }

    /**
     * Fetches bucket using api.
     */
    private async fetchBuckets(page = 1): Promise<void> {
        try {
            await this.$store.dispatch(BUCKET_ACTIONS.FETCH, page);
        } catch (error) {
            await this.$notify.error(`Unable to fetch buckets. ${error.message}`, AnalyticsErrorEventSource.DELETE_BUCKET_MODAL);
        }
    }

    /**
     * Sets local worker with worker instantiated in store.
     */
    public setWorker(): void {
        this.worker = this.$store.state.accessGrantsModule.accessGrantsWebWorker;
        this.worker.onerror = (error: ErrorEvent) => {
            this.$notify.error(error.message, AnalyticsErrorEventSource.DELETE_BUCKET_MODAL);
        };
    }

    /**
     * Sets name from input.
     */
    public onChangeName(value: string): void {
        this.name = value;
    }

    /**
     * Closes modal.
     */
    public closeModal(): void {
        this.$store.commit(APP_STATE_MUTATIONS.UPDATE_ACTIVE_MODAL, MODALS.deleteBucket);
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
    padding: 45px 70px;
    border-radius: 10px;
    font-family: 'font_regular', sans-serif;
    font-style: normal;
    display: flex;
    flex-direction: column;
    align-items: center;
    background-color: #fff;
    max-width: 480px;

    @media screen and (max-width: 700px) {
        padding: 45px;
    }

    &__title {
        font-family: 'font_bold', sans-serif;
        font-size: 22px;
        line-height: 27px;
        color: #000;
        margin: 0 0 18px;
    }

    &__subtitle {
        font-size: 18px;
        line-height: 30px;
        text-align: center;
        letter-spacing: -0.1007px;
        color: rgb(37 37 37 / 70%);
        margin: 0 0 24px;
    }

    &__input {
        margin-bottom: 18px;
    }
}
</style>
