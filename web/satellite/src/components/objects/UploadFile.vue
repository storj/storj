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
import { useRouter } from 'vue-router';

import { AnalyticsHttpApi } from '@/api/analytics';
import { RouteConfig } from '@/router';
import { AccessGrant, EdgeCredentials } from '@/types/accessGrants';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { BucketPage } from '@/types/buckets';
import { useNotify } from '@/utils/hooks';
import { useAppStore } from '@/store/modules/appStore';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useConfigStore } from '@/store/modules/configStore';

import FileBrowser from '@/components/browser/FileBrowser.vue';
import UploadCancelPopup from '@/components/objects/UploadCancelPopup.vue';

const obStore = useObjectBrowserStore();
const bucketsStore = useBucketsStore();
const appStore = useAppStore();
const configStore = useConfigStore();
const agStore = useAccessGrantsStore();
const projectsStore = useProjectsStore();
const router = useRouter();
const notify = useNotify();

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

const worker = ref<Worker | null>(null);

/**
 * Indicates if upload cancel popup is visible.
 */
const isCancelUploadPopupVisible = computed((): boolean => {
    return appStore.state.activeModal === MODALS.uploadCancelPopup;
});

/**
 * Returns passphrase from store.
 */
const passphrase = computed((): string => {
    return bucketsStore.state.passphrase;
});

/**
 * Returns apiKey from store.
 */
const apiKey = computed((): string => {
    return bucketsStore.state.apiKey;
});

/**
 * Returns bucket name from store.
 */
const bucket = computed((): string => {
    return bucketsStore.state.fileComponentBucketName;
});

/**
 * Returns current bucket page from store.
 */
const bucketPage = computed((): BucketPage => {
    return bucketsStore.state.page;
});

/**
 * Returns edge credentials from store.
 */
const edgeCredentials = computed((): EdgeCredentials => {
    return bucketsStore.state.edgeCredentials;
});

/**
 * Returns linksharing URL from store.
 */
const linksharingURL = computed((): string => {
    return configStore.state.config.linksharingURL;
});

/**
 * Returns public linksharing URL from store.
 */
const publicLinksharingURL = computed((): string => {
    return configStore.state.config.publicLinksharingURL;
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
    const cleanAPIKey: AccessGrant = await agStore.createAccessGrant(LINK_SHARING_AG_NAME, projectsStore.state.selectedProject.id);

    try {
        const credentials: EdgeCredentials = await generateCredentials(cleanAPIKey.secret, path, true);

        path = encodeURIComponent(path.trim());

        await analytics.eventTriggered(AnalyticsEvent.LINK_SHARED);

        return `${publicLinksharingURL.value}/${credentials.accessKeyId}/${path}`;
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.UPLOAD_FILE_VIEW);

        return '';
    }
}

/**
 * Sets local worker with worker instantiated in store.
 */
function setWorker(): void {
    worker.value = agStore.state.accessGrantsWebWorker;
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

    const satelliteNodeURL = configStore.state.config.satelliteNodeURL;
    const salt = await projectsStore.getProjectSalt(projectsStore.state.selectedProject.id);

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

    return await agStore.getEdgeCredentials(data.value, undefined, true);
}

/**
 * Initiates file browser.
 */
onBeforeMount(() => {
    setWorker();

    obStore.init({
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
    const projectID = projectsStore.state.selectedProject.id;
    if (!projectID) return;

    if (!passphrase.value) {
        await router.push(RouteConfig.Buckets.with(RouteConfig.BucketsManagement).path).catch(() => {return;});
        return;
    }

    try {
        await bucketsStore.setS3Client(projectID);
    } catch (error) {
        notify.error(error.message, AnalyticsErrorEventSource.UPLOAD_FILE_VIEW);
        return;
    }

    router.push(RouteConfig.Buckets.with(RouteConfig.UploadFile).path).catch(() => {return;});
    obStore.reinit({
        endpoint: edgeCredentials.value.endpoint,
        accessKey: edgeCredentials.value.accessKeyId,
        secretKey: edgeCredentials.value.secretKey,
    });

    try {
        await Promise.all([
            bucketsStore.getBuckets(bucketPage.value.currentPage, projectID),
            obStore.list(''),
            obStore.getObjectCount(),
        ]);
    } catch (error) {
        notify.error(error.message, AnalyticsErrorEventSource.UPLOAD_FILE_VIEW);
    }
});
</script>

<style scoped>
    .file-browser {
        font-family: 'font_regular', sans-serif;
    }
</style>
