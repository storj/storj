// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card v-if="!allFiles.length" title="No objects uploaded" cols="12" sm="6" md="4" lg="3" variant="flat" :border="true" rounded="xlg">
        <v-card-text>
            <v-divider class="mt-1 mb-4" />
            <v-btn color="primary" size="small" class="mr-2" @click="emit('uploadClick')">Upload</v-btn>
            <v-btn size="small" class="mr-2" @click="emit('newFolderClick')">New Folder</v-btn>
        </v-card-text>
    </v-card>
    <v-row v-else>
        <v-col v-for="item in allFiles" :key="item.browserObject.Key" cols="12" sm="6" md="4" lg="3">
            <file-card
                :item="item"
                class="h-100"
                @preview-click="onFileClick(item.browserObject)"
                @delete-file-click="onDeleteFileClick(item.browserObject)"
                @share-click="onShareClick(item.browserObject)"
            />
        </v-col>
    </v-row>

    <file-preview-dialog v-model="previewDialog" />

    <delete-file-dialog
        v-if="fileToDelete"
        v-model="isDeleteFileDialogShown"
        :file="fileToDelete"
        @content-removed="fileToDelete = null"
    />
    <share-dialog
        v-model="isShareDialogShown"
        :bucket-name="bucketName"
        :file="fileToShare || undefined"
        @content-removed="fileToShare = null"
    />
    <browser-snackbar-component v-model="isObjectsUploadModal" @file-click="onFileClick" />
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import { VBtn, VCard, VCardText, VCol, VDivider, VRow } from 'vuetify/components';

import {
    BrowserObject,
    useObjectBrowserStore,
} from '@/store/modules/objectBrowserStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { LocalData } from '@/utils/localData';
import { useAppStore } from '@/store/modules/appStore';
import { BrowserObjectTypeInfo, BrowserObjectWrapper, EXTENSION_INFOS, FILE_INFO, FOLDER_INFO } from '@/types/browser';

import FilePreviewDialog from '@poc/components/dialogs/FilePreviewDialog.vue';
import DeleteFileDialog from '@poc/components/dialogs/DeleteFileDialog.vue';
import ShareDialog from '@poc/components/dialogs/ShareDialog.vue';
import BrowserSnackbarComponent from '@poc/components/BrowserSnackbarComponent.vue';
import FileCard from '@poc/components/FileCard.vue';

const props = defineProps<{
    forceEmpty?: boolean;
}>();

const emit = defineEmits<{
    uploadClick: [];
    newFolderClick: [];
}>();

const config = useConfigStore();
const obStore = useObjectBrowserStore();
const projectsStore = useProjectsStore();
const bucketsStore = useBucketsStore();
const appStore = useAppStore();

const notify = useNotify();
const router = useRouter();

const isFetching = ref<boolean>(false);
const search = ref<string>('');
const selected = ref([]);
const previewDialog = ref<boolean>(false);
const fileToDelete = ref<BrowserObject | null>(null);
const isDeleteFileDialogShown = ref<boolean>(false);
const fileToShare = ref<BrowserObject | null>(null);
const isShareDialogShown = ref<boolean>(false);
const isFileGuideShown = ref<boolean>(false);

/**
 * Returns the name of the selected bucket.
 */
const bucketName = computed<string>(() => bucketsStore.state.fileComponentBucketName);

/**
 * Returns the current path within the selected bucket.
 */
const filePath = computed<string>(() => bucketsStore.state.fileComponentPath);

/**
 * Indicates whether objects upload modal should be shown.
 */
const isObjectsUploadModal = computed<boolean>(() => appStore.state.isUploadingModal);

/**
 * Returns every file under the current path.
 */
const allFiles = computed<BrowserObjectWrapper[]>(() => {
    if (props.forceEmpty) return [];

    const objects = obStore.state.files;
    return objects.map<BrowserObjectWrapper>(file => {
        const lowerName = file.Key.toLowerCase();
        const dotIdx = lowerName.lastIndexOf('.');
        const ext = dotIdx === -1 ? '' : file.Key.slice(dotIdx + 1);
        return {
            browserObject: file,
            typeInfo: getFileTypeInfo(ext, file.type),
            lowerName,
            ext,
        };
    });
});

/**
 * Returns the title and icon representing a file's type.
 */
function getFileTypeInfo(ext: string, type: BrowserObject['type']): BrowserObjectTypeInfo {
    if (!type) return FILE_INFO;
    if (type === 'folder') return FOLDER_INFO;

    ext = ext.toLowerCase();
    for (const [exts, info] of EXTENSION_INFOS.entries()) {
        if (exts.indexOf(ext) !== -1) return info;
    }
    return FILE_INFO;
}

/**
 * Handles file click.
 */
function onFileClick(file: BrowserObject): void {
    if (!file.type) return;

    if (file.type === 'folder') {
        const uriParts = [file.Key];
        if (filePath.value) {
            uriParts.unshift(...filePath.value.split('/'));
        }
        const pathAndKey = uriParts.map(part => encodeURIComponent(part)).join('/');
        router.push(`/projects/${projectsStore.state.selectedProject.urlId}/buckets/${bucketName.value}/${pathAndKey}`);
        return;
    }

    obStore.setObjectPathForModal(file.path + file.Key);
    previewDialog.value = true;
    isFileGuideShown.value = false;
    LocalData.setFileGuideHidden();
}

/**
 * Fetches all files in the current directory.
 */
async function fetchFiles(): Promise<void> {
    if (isFetching.value || props.forceEmpty) return;
    isFetching.value = true;

    try {
        const path = filePath.value ? filePath.value + '/' : '';

        await obStore.list(path);

        selected.value = [];
    } catch (err) {
        err.message = `Error fetching files. ${err.message}`;
        notify.notifyError(err, AnalyticsErrorEventSource.FILE_BROWSER_LIST_CALL);
    }

    isFetching.value = false;
}

/**
 * Handles delete button click event for files.
 */
function onDeleteFileClick(file: BrowserObject): void {
    fileToDelete.value = file;
    isDeleteFileDialogShown.value = true;
}

/**
 * Handles Share button click event.
 */
function onShareClick(file: BrowserObject): void {
    fileToShare.value = file;
    isShareDialogShown.value = true;
}

watch(filePath, fetchFiles, { immediate: true });
watch(() => props.forceEmpty, v => !v && fetchFiles());
</script>