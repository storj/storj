// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <div class="file-browser">
            <FileBrowser />
        </div>
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
import { MODALS } from '@/utils/constants/appStatePopUps';
import { MetaUtils } from '@/utils/meta';
import { BucketPage } from '@/types/buckets';
import { BUCKET_ACTIONS } from '@/store/modules/buckets';
import { OBJECTS_ACTIONS } from '@/store/modules/objects';

import FileBrowser from '@/components/browser/FileBrowser.vue';
import UploadCancelPopup from '@/components/objects/UploadCancelPopup.vue';

// @vue/component
@Component({
    components: {
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

    @Watch('passphrase')
    public async reinit(): Promise<void> {
        if (!this.passphrase) {
            await this.$router.push(RouteConfig.Buckets.with(RouteConfig.BucketsManagement).path).catch(() => {return;});
            return;
        }

        try {
            await this.$store.dispatch(OBJECTS_ACTIONS.SET_S3_CLIENT);
        } catch (error) {
            await this.$notify.error(error.message, AnalyticsErrorEventSource.UPLOAD_FILE_VIEW);
            return;
        }

        await this.$router.push(RouteConfig.Buckets.with(RouteConfig.UploadFile).path).catch(() => {return;});
        this.$store.commit('files/reinit', {
            endpoint: this.edgeCredentials.endpoint,
            accessKey: this.edgeCredentials.accessKeyId,
            secretKey: this.edgeCredentials.secretKey,
        });
        try {
            await this.$store.dispatch(BUCKET_ACTIONS.FETCH, this.bucketPage.currentPage);
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
     * Indicates if upload cancel popup is visible.
     */
    public get isCancelUploadPopupVisible(): boolean {
        return this.$store.state.appStateModule.viewsState.activeModal === MODALS.uploadCancelPopup;
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
     * Returns current bucket page from store.
     */
    private get bucketPage(): BucketPage {
        return this.$store.state.bucketUsageModule.page;
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
</style>
