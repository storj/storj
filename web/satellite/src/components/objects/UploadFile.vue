// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <p @click="goToBuckets" class="back"><- Back to Buckets</p>
        <div class="file-browser">
            <FileBrowser></FileBrowser>
        </div>
    </div>
</template>

<script lang="ts">
import { FileBrowser } from 'browser';
import { Component, Vue } from 'vue-property-decorator';

import { RouteConfig } from '@/router';
import { ACCESS_GRANTS_ACTIONS } from '@/store/modules/accessGrants';
import { AccessGrant, GatewayCredentials } from '@/types/accessGrants';
import { MetaUtils } from '@/utils/meta';

@Component({
    components: {
        FileBrowser,
    },
})
export default class UploadFile extends Vue {
    private linksharingURL = '';
    private worker: Worker;

    /**
     * Lifecycle hook after initial render.
     * Checks if bucket is chosen.
     * Sets local worker.
     */
    public mounted(): void {
        if (!this.bucket) {
            this.$router.push(RouteConfig.Objects.with(RouteConfig.EnterPassphrase).path);

            return;
        }

        this.linksharingURL = MetaUtils.getMetaContent('linksharing-url');

        this.setWorker();
    }

    /**
     * Lifecycle hook after vue instance was created.
     * Initiates file browser.
     */
    public created(): void {
        this.$store.commit('files/init', {
            endpoint: this.$store.state.objectsModule.gatewayCredentials.endpoint,
            accessKey: this.$store.state.objectsModule.gatewayCredentials.accessKeyId,
            secretKey: this.$store.state.objectsModule.gatewayCredentials.secretKey,
            bucket: this.bucket,
            browserRoot: RouteConfig.Objects.with(RouteConfig.UploadFile).path,
            getObjectMapUrl: async (path: string) => await this.generateObjectMapUrl(path),
            getSharedLink: async (path: string) => await this.generateShareLinkUrl(path),
        });
    }

    /**
     * Redirects to buckets list view.
     */
    public goToBuckets(): void {
        this.$router.push(RouteConfig.Objects.with(RouteConfig.BucketsManagement).path);
    }

    /**
     * Generates a URL for an object map.
     */
    public async generateObjectMapUrl(path: string): Promise<string> {
        path = `${this.bucket}/${path}`;
        const now = new Date();
        const inADay = new Date(now.setDate(now.getDate() + 1));

        try {
            const key: string = await this.accessKey(this.apiKey, inADay, path);

            path = encodeURIComponent(path.trim());

            return `${this.linksharingURL}/s/${key}/${path}?map=1`;
        } catch (error) {
            await this.$notify.error(error.message);

            return '';
        }
    }

    /**
     * Generates a URL for a link sharing service.
     */
    public async generateShareLinkUrl(path: string): Promise<string> {
        path = `${this.bucket}/${path}`;
        const now = new Date();
        const notAfter = new Date(now.setFullYear(now.getFullYear() + 100));
        const LINK_SHARING_AG_NAME = `${path}_shared-object_${now.toISOString()}`;
        const cleanAPIKey: AccessGrant = await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.CREATE, LINK_SHARING_AG_NAME);

        try {
            const key: string = await this.accessKey(cleanAPIKey.secret, notAfter, path);

            path = encodeURIComponent(path.trim());

            return `${this.linksharingURL}/${key}/${path}`;
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
     * Generates public access key.
     */
    private async accessKey(cleanApiKey: string, notAfter: Date, path: string): Promise<string> {
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

            return '';
        }

        this.worker.postMessage({
            'type': 'RestrictGrant',
            'isDownload': true,
            'isUpload': true,
            'isList': true,
            'isDelete': true,
            'paths': [path],
            'grant': grantData.value,
            'notBefore': new Date().toISOString(),
            'notAfter': notAfter.toISOString(),
        });

        const event: MessageEvent = await new Promise(resolve => this.worker.onmessage = resolve);
        const data = event.data;
        if (data.error) {
            await this.$notify.error(data.error);

            return '';
        }

        const gatewayCredentials: GatewayCredentials = await this.$store.dispatch(ACCESS_GRANTS_ACTIONS.GET_GATEWAY_CREDENTIALS, {accessGrant: data.value, isPublic: true});

        return gatewayCredentials.accessKeyId;
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
}
</script>

<style scoped>
    @import '../../../node_modules/browser/dist/browser.css';

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
