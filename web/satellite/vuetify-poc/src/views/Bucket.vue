// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container
        class="bucket-view"
        @dragover.prevent="isDragging = true"
    >
        <dropzone-dialog v-model="isDragging" :bucket="bucketName" @file-drop="upload" />
        <page-title-component title="Browse Files" />

        <browser-breadcrumbs-component />

        <v-col>
            <v-row class="mt-2 mb-4">
                <v-menu v-model="menu" location="bottom" transition="scale-transition" offset="5">
                    <template #activator="{ props }">
                        <v-btn
                            color="primary"
                            min-width="120"
                            :disabled="!isInitialized"
                            v-bind="props"
                        >
                            <browser-snackbar-component :on-cancel="() => { snackbar = false }" />
                            <IconUpload />
                            Upload
                        </v-btn>
                    </template>
                    <v-list class="pa-2">
                        <v-list-item rounded="lg" :disabled="!isInitialized" @click.stop="buttonFileUpload">
                            <template #prepend>
                                <IconFile />
                            </template>
                            <v-list-item-title class="text-body-2 ml-3">
                                Upload File
                            </v-list-item-title>
                        </v-list-item>

                        <v-divider class="my-2" />

                        <v-list-item class="mt-1" rounded="lg" :disabled="!isInitialized" @click.stop="buttonFolderUpload">
                            <template #prepend>
                                <icon-folder />
                            </template>
                            <v-list-item-title class="text-body-2 ml-3">
                                Upload Folder
                            </v-list-item-title>
                        </v-list-item>
                    </v-list>
                </v-menu>

                <input
                    ref="fileInput"
                    type="file"
                    aria-roledescription="file-upload"
                    hidden
                    multiple
                    @change="upload"
                >
                <input
                    ref="folderInput"
                    type="file"
                    aria-roledescription="folder-upload"
                    hidden
                    multiple
                    webkitdirectory
                    mozdirectory
                    @change="upload"
                >
                <v-btn
                    variant="outlined"
                    color="default"
                    class="mx-4"
                    :disabled="!isInitialized"
                >
                    <icon-folder />
                    New Folder
                    <browser-new-folder-dialog />
                </v-btn>
            </v-row>
        </v-col>

        <browser-table-component :loading="isFetching" :force-empty="!isInitialized" />
    </v-container>

    <enter-bucket-passphrase-dialog v-model="isBucketPassphraseDialogOpen" @passphrase-entered="initObjectStore" />
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import {
    VContainer,
    VCol,
    VRow,
    VBtn,
    VMenu,
    VList,
    VListItem,
    VListItemTitle,
    VDivider,
} from 'vuetify/components';

import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { EdgeCredentials } from '@/types/accessGrants';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import PageTitleComponent from '@poc/components/PageTitleComponent.vue';
import BrowserBreadcrumbsComponent from '@poc/components/BrowserBreadcrumbsComponent.vue';
import BrowserSnackbarComponent from '@poc/components/BrowserSnackbarComponent.vue';
import BrowserTableComponent from '@poc/components/BrowserTableComponent.vue';
import BrowserNewFolderDialog from '@poc/components/dialogs/BrowserNewFolderDialog.vue';
import IconUpload from '@poc/components/icons/IconUpload.vue';
import IconFolder from '@poc/components/icons/IconFolder.vue';
import IconFile from '@poc/components/icons/IconFile.vue';
import EnterBucketPassphraseDialog from '@poc/components/dialogs/EnterBucketPassphraseDialog.vue';
import DropzoneDialog from '@poc/components/dialogs/DropzoneDialog.vue';

const bucketsStore = useBucketsStore();
const obStore = useObjectBrowserStore();
const projectsStore = useProjectsStore();
const analyticsStore = useAnalyticsStore();

const router = useRouter();
const route = useRoute();
const notify = useNotify();

const folderInput = ref<HTMLInputElement>();
const fileInput = ref<HTMLInputElement>();
const menu = ref<boolean>(false);
const isFetching = ref<boolean>(true);
const isInitialized = ref<boolean>(false);
const isDragging = ref<boolean>(false);
const snackbar = ref<boolean>(false);
const isBucketPassphraseDialogOpen = ref<boolean>(false);

/**
 * Returns the name of the selected bucket.
 */
const bucketName = computed<string>(() => bucketsStore.state.fileComponentBucketName);

/**
 * Returns edge credentials from store.
 */
const edgeCredentials = computed((): EdgeCredentials => bucketsStore.state.edgeCredentials);

/**
 * Returns ID of selected project from store.
 */
const projectId = computed<string>(() => projectsStore.state.selectedProject.id);

/**
 * Returns whether the user should be prompted to enter the passphrase.
 */
const isPromptForPassphrase = computed<boolean>(() => bucketsStore.state.promptForPassphrase);

/**
 * Open the operating system's file system for file upload.
 */
async function buttonFileUpload(): Promise<void> {
    menu.value = false;
    const fileInputElement = fileInput.value as HTMLInputElement;
    fileInputElement.showPicker();
    analyticsStore.eventTriggered(AnalyticsEvent.UPLOAD_FILE_CLICKED);
}

/**
 * Open the operating system's file system for folder upload.
 */
async function buttonFolderUpload(): Promise<void> {
    menu.value = false;
    const folderInputElement = folderInput.value as HTMLInputElement;
    folderInputElement.showPicker();
    analyticsStore.eventTriggered(AnalyticsEvent.UPLOAD_FOLDER_CLICKED);
}

/**
 * Initializes object browser store.
 */
function initObjectStore(): void {
    obStore.init({
        endpoint: edgeCredentials.value.endpoint,
        accessKey: edgeCredentials.value.accessKeyId,
        secretKey: edgeCredentials.value.secretKey,
        bucket: bucketName.value,
        browserRoot: '', // unused
    });
    isInitialized.value = true;
}

/**
 * Upload the current selected or dragged-and-dropped file.
 */
async function upload(e: Event): Promise<void> {
    if (isDragging.value) {
        isDragging.value = false;
    }

    await obStore.upload({ e });
    analyticsStore.eventTriggered(AnalyticsEvent.OBJECT_UPLOADED);
    const target = e.target as HTMLInputElement;
    target.value = '';
}

watch(isBucketPassphraseDialogOpen, isOpen => {
    if (isOpen || !isPromptForPassphrase.value) return;
    router.push(`/projects/${projectId.value}/buckets`);
});

watch(() => route.params.browserPath, browserPath => {
    if (browserPath === undefined) return;

    let bucketName = '', filePath = '';
    if (typeof browserPath === 'string') {
        bucketName = browserPath;
    } else {
        bucketName = browserPath[0];
        filePath = browserPath.slice(1).join('/');
    }

    bucketsStore.setFileComponentBucketName(bucketName);
    bucketsStore.setFileComponentPath(filePath);
}, { immediate: true });

watch(() => bucketsStore.state.passphrase, async newPass => {
    if (isBucketPassphraseDialogOpen.value) return;

    const bucketsURL = `/projects/${projectId.value}/buckets`;

    if (!newPass) {
        router.push(bucketsURL);
        return;
    }

    isInitialized.value = false;
    try {
        await bucketsStore.setS3Client(projectId.value);
        obStore.reinit({
            endpoint: edgeCredentials.value.endpoint,
            accessKey: edgeCredentials.value.accessKeyId,
            secretKey: edgeCredentials.value.secretKey,
        });
        await router.push(`${bucketsURL}/${bucketsStore.state.fileComponentBucketName}`);
        isInitialized.value = true;
    } catch (error) {
        error.message = `Error setting S3 client. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.UPLOAD_FILE_VIEW);
        router.push(bucketsURL);
    }
});

/**
 * Initializes file browser.
 */
onMounted(async () => {
    try {
        await bucketsStore.getAllBucketsNames(projectId.value);
    } catch (error) {
        error.message = `Error fetching bucket names. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.UPLOAD_FILE_VIEW);
        return;
    }

    if (bucketsStore.state.allBucketNames.indexOf(bucketName.value) === -1) {
        router.push(`/projects/${projectId.value}/buckets`);
        return;
    }

    if (isPromptForPassphrase.value) {
        isBucketPassphraseDialogOpen.value = true;
    } else {
        initObjectStore();
    }

    isFetching.value = false;
});
</script>

<style scoped lang="scss">
.bucket-view {
    height: 100%;
}
</style>