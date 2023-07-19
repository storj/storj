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

import { RouteConfig } from '@/types/router';
import { EdgeCredentials } from '@/types/accessGrants';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { BucketPage } from '@/types/buckets';
import { useNotify } from '@/utils/hooks';
import { useAppStore } from '@/store/modules/appStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useObjectBrowserStore } from '@/store/modules/objectBrowserStore';

import FileBrowser from '@/components/browser/FileBrowser.vue';
import UploadCancelPopup from '@/components/objects/UploadCancelPopup.vue';

const obStore = useObjectBrowserStore();
const bucketsStore = useBucketsStore();
const appStore = useAppStore();
const projectsStore = useProjectsStore();
const router = useRouter();
const notify = useNotify();

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
 * Initiates file browser.
 */
onBeforeMount(() => {
    obStore.init({
        endpoint: edgeCredentials.value.endpoint,
        accessKey: edgeCredentials.value.accessKeyId,
        secretKey: edgeCredentials.value.secretKey,
        bucket: bucket.value,
        browserRoot: RouteConfig.Buckets.with(RouteConfig.UploadFile).path,
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
        notify.notifyError(error, AnalyticsErrorEventSource.UPLOAD_FILE_VIEW);
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
