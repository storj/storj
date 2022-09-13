// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="bucket-creation">
        <bucket-creation-progress class="bucket-creation__progress" :creation-step="creationStep" />
        <bucket-creation-name-step
            v-if="creationStep === BucketCreationSteps.Name"
            @setName="setName"
        />
        <bucket-creation-generate-passphrase
            v-if="creationStep === BucketCreationSteps.Passphrase"
            :on-next-click="onPassphraseGenerationNextClick"
            :on-back-click="onGenerationBackClick"
            :set-parent-passphrase="setPassphrase"
            :is-loading="isLoading"
        />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';
import { OBJECTS_ACTIONS } from '@/store/modules/objects';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { MetaUtils } from '@/utils/meta';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';

import BucketCreationGeneratePassphrase from '@/components/objects/BucketCreationGeneratePassphrase.vue';
import BucketCreationNameStep from '@/components/objects/BucketCreationNameStep.vue';
import BucketCreationProgress from '@/components/objects/BucketCreationProgress.vue';

export enum BucketCreationSteps {
    Name = 0,
    Passphrase,
    Upload
}

// @vue/component
@Component({
    components: {
        BucketCreationProgress,
        BucketCreationNameStep,
        BucketCreationGeneratePassphrase,
    },
})
export default class BucketCreation extends Vue {
    private worker: Worker;

    private readonly FILE_BROWSER_AG_NAME: string = 'Web file browser API key';
    public readonly BucketCreationSteps = BucketCreationSteps;
    public creationStep: BucketCreationSteps = BucketCreationSteps.Name;
    public isLoading = false;
    public bucketName = '';
    public passphrase = '';

    public readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
     * Sets bucket name from child component.
     */
    public setName(name: string): void {
        this.bucketName = name;
        this.creationStep = BucketCreationSteps.Passphrase;
    }

    /**
     * Sets passphrase from child component.
     */
    public setPassphrase(passphrase: string): void {
        this.passphrase = passphrase;
        this.$store.dispatch(OBJECTS_ACTIONS.SET_PASSPHRASE, this.passphrase);
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
     * Holds on next button click logic on passphrase generation step.
     */
    public async onPassphraseGenerationNextClick(): Promise<void> {
        if (this.isLoading) return;

        this.isLoading = true;

        try {
            this.setWorker();
            await this.setAccess();
            await this.$store.dispatch(OBJECTS_ACTIONS.CREATE_BUCKET, this.bucketName);
            await this.fetchBuckets();
            await this.$store.dispatch(OBJECTS_ACTIONS.SET_FILE_COMPONENT_BUCKET_NAME, this.bucketName);
            this.analytics.eventTriggered(AnalyticsEvent.BUCKET_CREATED);
            this.analytics.pageVisit(RouteConfig.UploadFile.path);
            await this.$router.push(RouteConfig.UploadFile.path);
        } catch (e) {
            await this.$notify.error(e.message);
        } finally {
            this.isLoading = false;
        }
    }

    /**
     * Holds on back button click logic on passphrase generation step.
     */
    public onGenerationBackClick(): void {
        this.creationStep = BucketCreationSteps.Name;
    }

    /**
     * Removes temporary created access grant.
     */
    public async removeTemporaryAccessGrant(): Promise<void> {
        try {
            await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.DELETE_BY_NAME_AND_PROJECT_ID, this.FILE_BROWSER_AG_NAME);
        } catch (error) {
            await this.$notify.error(error.message);
        }
    }

    /**
     * Fetches bucket using api.
     */
    public async fetchBuckets(page = 1): Promise<void> {
        try {
            await this.$store.dispatch(BUCKET_ACTIONS.FETCH, page);
        } catch (error) {
            await this.$notify.error(`Unable to fetch buckets. ${error.message}`);
        }
    }

    /**
     * Sets access to S3 client.
     */
    public async setAccess(): Promise<void> {
        if (!this.apiKey) {
            await this.removeTemporaryAccessGrant();
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
            'buckets': [this.bucketName],
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
        await this.$store.dispatch(OBJECTS_ACTIONS.SET_S3_CLIENT);
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
.bucket-creation {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: flex-start;
    font-family: 'font_regular', sans-serif;
    padding-bottom: 30px;

    &__progress {
        margin-bottom: 44px;
        width: 460px;

        @media screen and (max-width: 760px) {
            width: 85%;
        }
    }
}

:deep(.bucket-icon) {
    width: 267px;
}

@media screen and (max-width: 760px) {

    :deep(.label-container__main__label) {
        font-size: 0.875rem !important;
        line-height: 1.285rem !important;
    }
}

@media screen and (max-width: 600px) {

    :deep(.bucket-icon) {
        width: 190px;
    }
}
</style>
