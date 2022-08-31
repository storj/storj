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
                <p>If you don’t see the objects you’re looking for, <router-link class="link" :to="bucketsManagementPath">try opening the bucket again</router-link> with a different passphrase.</p>
            </template>
        </info-notification>
        <UploadCancelPopup v-if="isCancelUploadPopupVisible" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import { SignatureV4 } from "@aws-sdk/signature-v4";
import { Sha256 } from "@aws-crypto/sha256-browser";
import { HttpRequest, Credentials } from "@aws-sdk/types";

import UploadCancelPopup from '@/components/objects/UploadCancelPopup.vue';
import FileBrowser from '@/components/browser/FileBrowser.vue';

import { AnalyticsHttpApi } from '@/api/analytics';
import { RouteConfig } from '@/router';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { MetaUtils } from '@/utils/meta';
import { Bucket } from "@/types/buckets";
import InfoNotification from "@/components/common/InfoNotification.vue";

// @vue/component
@Component({
    components: {
        InfoNotification,
        FileBrowser,
        UploadCancelPopup,
    },
})
export default class UploadFile extends Vue {
    private credentials: EdgeCredentials;
    private linksharingURL = '';
    private worker: Worker;
    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    public readonly bucketsManagementPath: string = RouteConfig.Buckets.with(RouteConfig.BucketsManagement).path;

    /**
     * Indicates if we have objects in this bucket but not for inputted passphrase.
     */
    public get isMultiplePassphraseNotificationShown(): boolean {
        const name: string = this.$store.state.files.bucket;
        const data: Bucket = this.$store.state.bucketUsageModule.page.buckets.find((bucket: Bucket) => bucket.name === name);

        const objectCount: number = data?.objectCount || 0;
        const ownObjects = this.$store.getters["files/sortedFiles"];

        return objectCount > 0 && !ownObjects.length;
    }

    /**
     * Lifecycle hook after vue instance was created.
     * Initiates file browser.
     */
    public created(): void {
        this.linksharingURL = MetaUtils.getMetaContent('linksharing-url');

        this.setWorker();

        this.credentials = this.$store.state.objectsModule.gatewayCredentials;

        this.$store.commit('files/init', {
            endpoint: this.credentials.endpoint,
            accessKey: this.credentials.accessKeyId,
            secretKey: this.credentials.secretKey,
            bucket: this.bucket,
            browserRoot: RouteConfig.Buckets.with(RouteConfig.UploadFile).path,
            fetchObjectMap: this.fetchObjectMap,
            fetchObjectPreview: this.fetchObjectPreview,
            fetchSharedLink: this.generateShareLinkUrl,
        });
    }

    /**
     * Generates a URL for an object map.
     */
    public async fetchObjectMap(path: string): Promise<Blob | null> {
        try {
            return await this.getObjectViewOrMapBySignedRequest(path, true)
        } catch (error) {
            await this.$notify.error('Failed to fetch object map. Bandwidth limit may be exceeded');

            return null
        }
    }

    /**
     * Generates a URL for an object map.
     */
    public async fetchObjectPreview(path: string): Promise<Blob | null> {
        try {
            return await this.getObjectViewOrMapBySignedRequest(path, false)
        } catch (error) {
            await this.$notify.error('Failed to fetch object view. Bandwidth limit may be exceeded');

            return null
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
            const credentials: EdgeCredentials = await this.generateShareCredentials(cleanAPIKey.secret, path, true);

            path = encodeURIComponent(path.trim());

            await this.analytics.eventTriggered(AnalyticsEvent.LINK_SHARED);

            return `${this.linksharingURL}/${credentials.accessKeyId}/${path}`;
        } catch (error) {
            await this.$notify.error(error.message);

            return '';
        }
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
     * Returns a URL for an object or a map.
     */
    private async getObjectViewOrMapBySignedRequest(path: string, isMap: boolean): Promise<Blob | null> {
        path = `${this.bucket}/${path}`;
        path = encodeURIComponent(path.trim());

        const url = new URL(`${this.linksharingURL}/s/${this.credentials.accessKeyId}/${path}`)

        let request: HttpRequest = {
            method: 'GET',
            protocol: url.protocol,
            hostname: url.hostname,
            port: parseFloat(url.port),
            path: url.pathname,
            headers: {
                'host': url.host,
            }
        }

        if (isMap) {
            request = Object.assign(request, {query: { 'map': '1' }});
        } else {
            request = Object.assign(request, {query: { 'view': '1' }});
        }

        const signerCredentials: Credentials = {
            accessKeyId: this.credentials.accessKeyId,
            secretAccessKey: this.credentials.secretKey,
        };

        const signer = new SignatureV4({
            applyChecksum: true,
            uriEscapePath: false,
            credentials: signerCredentials,
            region: "eu1",
            service: "linksharing",
            sha256: Sha256,
        });

        let signedRequest: HttpRequest;
        try {
            signedRequest = await signer.sign(request);
        } catch (error) {
            await this.$notify.error(error.message);

            return null;
        }

        let requestURL = `${this.linksharingURL}${signedRequest.path}`;
        if (isMap) {
            requestURL = `${requestURL}?map=1`;
        } else {
            requestURL = `${requestURL}?view=1`;
        }

        const response = await fetch(requestURL, signedRequest);
        if (response.ok) {
            return await response.blob();
        }

        return null;
    }

    /**
     * Generates share gateway credentials.
     */
    private async generateShareCredentials(cleanApiKey: string, path: string, isPublic: boolean): Promise<EdgeCredentials> {
        const satelliteNodeURL = MetaUtils.getMetaContent('satellite-nodeurl');

        this.worker.postMessage({
            'type': 'GenerateAccess',
            'apiKey': cleanApiKey,
            'passphrase': this.passphrase,
            'projectID': this.$store.getters.selectedProject.id,
            'satelliteNodeURL': satelliteNodeURL,
        });

        const grantEvent: MessageEvent = await new Promise(resolve => this.worker.onmessage = resolve);
        const grantData = grantEvent.data;
        if (grantData.error) {
            await this.$notify.error(grantData.error);

            return new EdgeCredentials();
        }

        this.worker.postMessage({
            'type': 'RestrictGrant',
            'isDownload': true,
            'isUpload': false,
            'isList': true,
            'isDelete': false,
            'paths': [path],
            'grant': grantData.value,
        });

        const event: MessageEvent = await new Promise(resolve => this.worker.onmessage = resolve);
        const data = event.data;
        if (data.error) {
            await this.$notify.error(data.error);

            return new EdgeCredentials();
        }

        return await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.GET_GATEWAY_CREDENTIALS, {accessGrant: data.value, isPublic});
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
     * Returns bucket name from store.
     */
    private get bucket(): string {
        return this.$store.state.objectsModule.fileComponentBucketName;
    }
}
</script>

<style scoped>
    .back {
        font-family: 'font_medium', sans-serif;
        color: #000;
        font-size: 20px;
        cursor: pointer;
        margin: 0 0 30px 15px;
        display: inline-block;
    }

    .back:hover {
        color: #007bff;
        text-decoration: underline;
    }

    .file-browser {
        font-family: 'font_regular', sans-serif;
        padding-bottom: 200px;
    }
</style>
