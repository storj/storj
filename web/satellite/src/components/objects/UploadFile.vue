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

<script setup lang="ts">
import { computed, onBeforeMount, ref, watch } from 'vue';

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
import { useNotify, useRouter, useStore } from '@/utils/hooks';

import FileBrowser from '@/components/browser/FileBrowser.vue';
import UploadCancelPopup from '@/components/objects/UploadCancelPopup.vue';

const store = useStore();
const router = useRouter();
const notify = useNotify();

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

const worker = ref<Worker | null>(null);
const linksharingURL = ref<string>('');

/**
 * Indicates if upload cancel popup is visible.
 */
const isCancelUploadPopupVisible = computed((): boolean => {
    return store.state.appStateModule.viewsState.activeModal === MODALS.uploadCancelPopup;
});

/**
 * Returns passphrase from store.
 */
const passphrase = computed((): string => {
    return store.state.objectsModule.passphrase;
});

/**
 * Returns apiKey from store.
 */
const apiKey = computed((): string => {
    return store.state.objectsModule.apiKey;
});

/**
 * Returns bucket name from store.
 */
const bucket = computed((): string => {
    return store.state.objectsModule.fileComponentBucketName;
});

/**
 * Returns current bucket page from store.
 */
const bucketPage = computed((): BucketPage => {
    return store.state.bucketUsageModule.page;
});

/**
 * Returns edge credentials from store.
 */
const edgeCredentials = computed((): EdgeCredentials => {
    return store.state.objectsModule.gatewayCredentials;
});

/**
 * Generates a URL for an object map.
 */
async function generateObjectPreviewAndMapUrl(path: string): Promise<string> {
    path = `${bucket.value}/${path}`;

    try {
        const creds: EdgeCredentials = await generateCredentials(apiKey.value, path, false);

        path = encodeURIComponent(path.trim());

        return `${linksharingURL.value}/s/${creds.accessKeyId}/${path}`;
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.UPLOAD_FILE_VIEW);

        return '';
    }
}

/**
 * Generates a URL for a link sharing service.
 */
async function generateShareLinkUrl(path: string): Promise<string> {
    path = `${bucket.value}/${path}`;
    const now = new Date();
    const LINK_SHARING_AG_NAME = `${path}_shared-object_${now.toISOString()}`;
    const cleanAPIKey: AccessGrant = await store.dispatch(ACCESS_GRANTS_ACTIONS.CREATE, LINK_SHARING_AG_NAME);

    try {
        const credentials: EdgeCredentials = await generateCredentials(cleanAPIKey.secret, path, true);

        path = encodeURIComponent(path.trim());

        await analytics.eventTriggered(AnalyticsEvent.LINK_SHARED);

        return `${linksharingURL.value}/${credentials.accessKeyId}/${path}`;
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.UPLOAD_FILE_VIEW);

        return '';
    }
}

/**
 * Sets local worker with worker instantiated in store.
 */
function setWorker(): void {
    worker.value = store.state.accessGrantsModule.accessGrantsWebWorker;
    if (worker.value) {
        worker.value.onerror = (error: ErrorEvent) => {
            notify.error(error.message, AnalyticsErrorEventSource.UPLOAD_FILE_VIEW);
        };
    }
}

/**
 * Generates share gateway credentials.
 */
async function generateCredentials(cleanApiKey: string, path: string, areEndless: boolean): Promise<EdgeCredentials> {
    if (!worker.value) {
        throw new Error('Worker is not defined');
    }

    const satelliteNodeURL = MetaUtils.getMetaContent('satellite-nodeurl');
    const salt = await store.dispatch(PROJECTS_ACTIONS.GET_SALT, store.getters.selectedProject.id);

    worker.value.postMessage({
        'type': 'GenerateAccess',
        'apiKey': cleanApiKey,
        'passphrase': passphrase.value,
        'salt': salt,
        'satelliteNodeURL': satelliteNodeURL,
    });

    const grantEvent: MessageEvent = await new Promise(resolve => {
        if (worker.value) {
            worker.value.onmessage = resolve;
        }
    });
    const grantData = grantEvent.data;
    if (grantData.error) {
        await notify.error(grantData.error, AnalyticsErrorEventSource.UPLOAD_FILE_VIEW);

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

    worker.value.postMessage(permissionsMsg);

    const event: MessageEvent = await new Promise(resolve => {
        if (worker.value) {
            worker.value.onmessage = resolve;
        }
    });
    const data = event.data;
    if (data.error) {
        await notify.error(data.error, AnalyticsErrorEventSource.UPLOAD_FILE_VIEW);

        return new EdgeCredentials();
    }

    return await store.dispatch(ACCESS_GRANTS_ACTIONS.GET_GATEWAY_CREDENTIALS, { accessGrant: data.value, isPublic: true });
}

/**
 * Initiates file browser.
 */
onBeforeMount(() => {
    linksharingURL.value = MetaUtils.getMetaContent('linksharing-url');

    setWorker();

    store.commit('files/init', {
        endpoint: edgeCredentials.value.endpoint,
        accessKey: edgeCredentials.value.accessKeyId,
        secretKey: edgeCredentials.value.secretKey,
        bucket: bucket.value,
        browserRoot: RouteConfig.Buckets.with(RouteConfig.UploadFile).path,
        fetchPreviewAndMapUrl: generateObjectPreviewAndMapUrl,
        fetchSharedLink: generateShareLinkUrl,
    });
});

watch(passphrase, async () => {
    if (!passphrase.value) {
        await router.push(RouteConfig.Buckets.with(RouteConfig.BucketsManagement).path).catch(() => {return;});
        return;
    }

    try {
        await store.dispatch(OBJECTS_ACTIONS.SET_S3_CLIENT);
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.UPLOAD_FILE_VIEW);
        return;
    }

    await router.push(RouteConfig.Buckets.with(RouteConfig.UploadFile).path).catch(() => {return;});
    store.commit('files/reinit', {
        endpoint: edgeCredentials.value.endpoint,
        accessKey: edgeCredentials.value.accessKeyId,
        secretKey: edgeCredentials.value.secretKey,
    });
    try {
        await store.dispatch(BUCKET_ACTIONS.FETCH, bucketPage.value.currentPage);
        await store.dispatch('files/list', '');
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.UPLOAD_FILE_VIEW);
    }
});
</script>

<style scoped>
    .file-browser {
        font-family: 'font_regular', sans-serif;
        padding-bottom: 200px;
    }
</style>
