// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <div class="file-browser">
            <FileBrowser />
        </div>
        <info-notification v-if="isMultiplePassphraseNotificationShown">
            <template #text>
                <p class="medium">Do you know a bucket can have multiple passphrases?</p>
                <p>
                    If you don’t see the objects you’re looking for,
                    <span class="link" @click="toggleManagePassphrase">try opening the bucket again</span>
                    with a different passphrase.
                </p>
            </template>
        </info-notification>
        <UploadCancelPopup v-if="isCancelUploadPopupVisible" />
    </div>
</template>

<script lang="ts">
import { Component, Vue, Watch } from 'vue-property-decorator';

import { AnalyticsHttpApi } from '@/api/analytics';
import { RouteConfig } from '@/router';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { PROJECTS_ACTIONS } from '@/store/modules/projects';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { MetaUtils } from '@/utils/meta';
import { Bucket } from '@/types/buckets';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';

import FileBrowser from '@/components/browser/FileBrowser.vue';
import UploadCancelPopup from '@/components/objects/UploadCancelPopup.vue';
import InfoNotification from '@/components/common/InfoNotification.vue';

// @vue/component
@Component({
    components: {
        InfoNotification,
        FileBrowser,
        UploadCancelPopup,
    },
})
export default class UploadFile extends Vue {
    private linksharingURL = '';
    private worker: Worker;
    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
     * Lifecycle hook after vue instance was created.
     * Initiates file browser.
     */
    public created(): void {
        this.linksharingURL = MetaUtils.getMetaContent('linksharing-url');

        this.setWorker();

        this.$store.commit('files/init', {
            endpoint: this.edgeCredentials.endpoint,
            accessKey: this.edgeCredentials.accessKeyId,
            secretKey: this.edgeCredentials.secretKey,
            bucket: this.bucket,
            browserRoot: RouteConfig.Buckets.with(RouteConfig.UploadFile).path,
            fetchPreviewAndMapUrl: this.generateObjectPreviewAndMapUrl,
            fetchSharedLink: this.generateShareLinkUrl,
        });
    }

    @Watch('edgeCredentials.secretKey')
    public async reinit(): Promise<void> {
        if (!this.edgeCredentials.secretKey) {
            await this.$router.push(RouteConfig.Buckets.with(RouteConfig.BucketsManagement).path).catch(() => {return;});
            return;
        }

        await this.$router.push(RouteConfig.Buckets.with(RouteConfig.UploadFile).path).catch(() => {return;});
        await this.$store.commit('files/reinit', {
            endpoint: this.edgeCredentials.endpoint,
            accessKey: this.edgeCredentials.accessKeyId,
            secretKey: this.edgeCredentials.secretKey,
        });
        try {
            await this.$store.dispatch('files/list', '');
        } catch (error) {
            await this.$notify.error(error.message, AnalyticsErrorEventSource.UPLOAD_FILE_VIEW);
        }
    }

    /**
     * Generates a URL for an object map.
     */
    public async generateObjectPreviewAndMapUrl(path: string): Promise<string> {
        path = `${this.bucket}/${path}`;

        try {
            const creds: EdgeCredentials = await this.generateCredentials(this.apiKey, path, false);

            path = encodeURIComponent(path.trim());

            return `${this.linksharingURL}/s/${creds.accessKeyId}/${path}`;
        } catch (error) {
            await this.$notify.error(error.message, AnalyticsErrorEventSource.UPLOAD_FILE_VIEW);

            return '';
        }
    }

    /**
     * Generates a URL for a link sharing service.
     */
    public async generateShareLinkUrl(path: string): Promise<string> {
        path = `${this.bucket}/${path}`;
        const now = new Date();
        const LINK_SHARING_AG_NAME = `${path}_shared-object_${now.toISOString()}`;
        const cleanAPIKey: AccessGrant = await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.CREATE, LINK_SHARING_AG_NAME);

        try {
            const credentials: EdgeCredentials = await this.generateCredentials(cleanAPIKey.secret, path, true);

            path = encodeURIComponent(path.trim());

            await this.analytics.eventTriggered(AnalyticsEvent.LINK_SHARED);

            return `${this.linksharingURL}/${credentials.accessKeyId}/${path}`;
        } catch (error) {
            await this.$notify.error(error.message, AnalyticsErrorEventSource.UPLOAD_FILE_VIEW);

            return '';
        }
    }

    /**
     * Toggles manage project passphrase modal visibility.
     */
    public toggleManagePassphrase(): void {
        this.$store.commit(APP_STATE_MUTATIONS.TOGGLE_MANAGE_PROJECT_PASSPHRASE_MODAL_SHOWN);
    }

    /**
     * Sets local worker with worker instantiated in store.
     */
    public setWorker(): void {
        this.worker = this.$store.state.accessGrantsModule.accessGrantsWebWorker;
        this.worker.onerror = (error: ErrorEvent) => {
            this.$notify.error(error.message, AnalyticsErrorEventSource.UPLOAD_FILE_VIEW);
        };
    }

    /**
     * Generates share gateway credentials.
     */
    private async generateCredentials(cleanApiKey: string, path: string, areEndless: boolean): Promise<EdgeCredentials> {
        const satelliteNodeURL = MetaUtils.getMetaContent('satellite-nodeurl');
        const salt = await this.$store.dispatch(PROJECTS_ACTIONS.GET_SALT, this.$store.getters.selectedProject.id);

        this.worker.postMessage({
            'type': 'GenerateAccess',
            'apiKey': cleanApiKey,
            'passphrase': this.passphrase,
            'salt': salt,
            'satelliteNodeURL': satelliteNodeURL,
        });

        const grantEvent: MessageEvent = await new Promise(resolve => this.worker.onmessage = resolve);
        const grantData = grantEvent.data;
        if (grantData.error) {
            await this.$notify.error(grantData.error, AnalyticsErrorEventSource.UPLOAD_FILE_VIEW);

            return new EdgeCredentials();
        }

        let permissionsMsg = {
            'type': 'RestrictGrant',
            'isDownload': true,
            'isUpload': false,
            'isList': true,
            'isDelete': false,
            'paths': [path],
            'grant': grantData.value,
        };

        if (!areEndless) {
            const now = new Date();
            const inOneDay = new Date(now.setDate(now.getDate() + 1));

            permissionsMsg = Object.assign(permissionsMsg, { 'notAfter': inOneDay.toISOString() });
        }

        this.worker.postMessage(permissionsMsg);

        const event: MessageEvent = await new Promise(resolve => this.worker.onmessage = resolve);
        const data = event.data;
        if (data.error) {
            await this.$notify.error(data.error, AnalyticsErrorEventSource.UPLOAD_FILE_VIEW);

            return new EdgeCredentials();
        }

        return await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.GET_GATEWAY_CREDENTIALS, { accessGrant: data.value, isPublic: true });
    }

    /**
     * Indicates if we have objects in this bucket but not for inputted passphrase.
     */
    public get isMultiplePassphraseNotificationShown(): boolean {
        const name: string = this.$store.state.files.bucket;
        const data: Bucket = this.$store.state.bucketUsageModule.page.buckets.find((bucket: Bucket) => bucket.name === name);

        const objectCount: number = data?.objectCount || 0;
        const ownObjects = this.$store.getters['files/sortedFiles'];

        return objectCount > 0 && !ownObjects.length;
    }

    /**
     * Indicates if upload cancel popup is visible.
     */
    public get isCancelUploadPopupVisible(): boolean {
        return this.$store.state.appStateModule.appState.isUploadCancelPopupVisible;
    }

    /**
     * Returns passphrase from store.
     */
    private get passphrase(): string {
        return this.$store.state.objectsModule.passphrase;
    }

    /**
     * Returns apiKey from store.
     */
    private get apiKey(): string {
        return this.$store.state.objectsModule.apiKey;
    }

    /**
     * Returns bucket name from store.
     */
    private get bucket(): string {
        return this.$store.state.objectsModule.fileComponentBucketName;
    }

    /**
     * Returns edge credentials from store.
     */
    private get edgeCredentials(): EdgeCredentials {
        return this.$store.state.objectsModule.gatewayCredentials;
    }
}
</script>

<style scoped>
    .file-browser {
        font-family: 'font_regular', sans-serif;
        padding-bottom: 200px;
    }

    .link {
        cursor: pointer;
    }
</style>
