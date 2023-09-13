// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog v-model="model" transition="fade-transition" class="preview-dialog" fullscreen theme="dark">
        <v-card class="preview-card">
            <v-carousel v-model="constCarouselIndex" hide-delimiters show-arrows="hover" height="100vh">
                <template #prev>
                    <v-btn
                        color="default"
                        class="rounded-circle"
                        icon
                        @click="onPrevious"
                    >
                        <v-icon icon="mdi-chevron-left" size="x-large" />
                    </v-btn>
                </template>
                <template #next>
                    <v-btn
                        color="default"
                        class="rounded-circle"
                        icon
                        @click="onNext"
                    >
                        <v-icon icon="mdi-chevron-right" size="x-large" />
                    </v-btn>
                </template>
                <v-toolbar
                    color="rgba(0, 0, 0, 0.3)"
                    theme="dark"
                >
                    <v-toolbar-title>
                        {{ fileName }}
                    </v-toolbar-title>
                    <template #append>
                        <v-btn :loading="isDownloading" icon size="small" color="white" @click="download">
                            <img src="@poc/assets/icon-download.svg" width="22" alt="Download">
                            <v-tooltip
                                activator="parent"
                                location="bottom"
                            >
                                Download
                            </v-tooltip>
                        </v-btn>
                        <v-btn icon size="small" color="white" @click="isShareDialogShown = true">
                            <icon-share size="22" />
                            <v-tooltip
                                activator="parent"
                                location="bottom"
                            >
                                Share
                            </v-tooltip>
                        </v-btn>
                        <v-btn icon size="small" color="white">
                            <img src="@poc/assets/icon-geo-distribution.svg" width="22" alt="Geographic Distribution">
                            <v-tooltip
                                activator="parent"
                                location="bottom"
                            >
                                Geographic Distribution
                            </v-tooltip>
                        </v-btn>
                        <v-btn icon size="small" color="white">
                            <img src="@poc/assets/icon-more.svg" width="22" alt="More">
                            <v-tooltip
                                activator="parent"
                                location="bottom"
                            >
                                More
                            </v-tooltip>
                        </v-btn>
                        <v-btn icon size="small" color="white" @click="model = false">
                            <img src="@poc/assets/icon-close.svg" width="18" alt="Close">
                            <v-tooltip
                                activator="parent"
                                location="bottom"
                            >
                                Close
                            </v-tooltip>
                        </v-btn>
                    </template>
                </v-toolbar>

                <v-carousel-item v-for="(file, i) in files" :key="file.Key">
                    <!-- v-carousel will mount all items at the same time -->
                    <!-- so :active will tell file-preview-item if it is the current. -->
                    <!-- If it is, it'll load the preview. -->
                    <file-preview-item :active="i === fileIndex" :file="file" @download="download" />
                </v-carousel-item>
            </v-carousel>
        </v-card>
    </v-dialog>

    <share-dialog v-model="isShareDialogShown" :bucket-name="bucketName" :file="currentFile" />
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import {
    VBtn,
    VCard,
    VCarousel,
    VCarouselItem,
    VDialog,
    VIcon,
    VToolbar,
    VToolbarTitle,
    VTooltip,
} from 'vuetify/components';

import { BrowserObject, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

import IconShare from '@poc/components/icons/IconShare.vue';
import FilePreviewItem from '@poc/components/dialogs/filePreviewComponents/FilePreviewItem.vue';
import ShareDialog from '@poc/components/dialogs/ShareDialog.vue';

const obStore = useObjectBrowserStore();
const bucketsStore = useBucketsStore();
const notify = useNotify();

const isDownloading = ref<boolean>(false);
const isShareDialogShown = ref<boolean>(false);

const folderType = 'folder';

const props = defineProps<{
  modelValue: boolean,
}>();

const emit = defineEmits<{
  (event: 'update:modelValue', value: boolean): void,
}>();

const model = computed<boolean>({
    get: () => props.modelValue,
    set: value => emit('update:modelValue', value),
});

const constCarouselIndex = computed(() => carouselIndex.value);
const carouselIndex = ref(0);

/**
 * Retrieve the file object that the modal is set to from the store.
 */
const currentFile = computed((): BrowserObject => {
    return obStore.sortedFiles[fileIndex.value];
});

const files = computed((): BrowserObject[] => {
    return obStore.sortedFiles;
});

/**
 * Retrieve the file index that the modal is set to from the store.
 */
const fileIndex = computed((): number => {
    return files.value.findIndex(f => f.Key === filePath.value.split('/').pop());
});

/**
 * Retrieve the name of the current file.
 */
const fileName = computed((): string | undefined => {
    return filePath.value.split('/').pop();
});

/**
 * Retrieve the current filepath.
 */
const filePath = computed((): string => {
    return obStore.state.objectPathForModal;
});

/**
 * Returns current path without object key.
 */
const currentPath = computed((): string => {
    return obStore.state.path;
});

/**
 * Returns the name of the current bucket from the store.
 */
const bucketName = computed((): string => {
    return bucketsStore.state.fileComponentBucketName;
});

/**
 * Download the current opened file.
 */
async function download(): Promise<void> {
    if (isDownloading.value) {
        return;
    }
    isDownloading.value = true;
    try {
        await obStore.download(currentFile.value);
        notify.success('Keep this download link private. If you want to share, use the Share option.');
    } catch (error) {
        notify.error('Can not download your file', AnalyticsErrorEventSource.OBJECT_DETAILS_MODAL);
    }
    isDownloading.value = false;
}

/**
 * Handles on previous click logic.
 */
function onPrevious(): void {
    const currentIndex = fileIndex.value;
    const sortedFilesLength = obStore.sortedFiles.length;

    let newFile: BrowserObject;
    if (currentIndex <= 0) {
        newFile = obStore.sortedFiles[sortedFilesLength - 1];
    } else {
        newFile = obStore.sortedFiles[currentIndex - 1];
        if (newFile.type === folderType) {
            newFile = obStore.sortedFiles[sortedFilesLength - 1];
        }
    }
    setNewObjectPath(newFile.Key);
}

/**
 * Handles on next click logic.
 */
function onNext(): void {
    let newFile: BrowserObject | undefined = obStore.sortedFiles[fileIndex.value + 1];
    if (!newFile || newFile.type === folderType) {
        newFile = obStore.sortedFiles.find(f => f.type !== folderType);

        if (!newFile) return;
    }
    setNewObjectPath(newFile.Key);
}

/**
 * Sets new object path.
 */
function setNewObjectPath(objectKey: string): void {
    obStore.setObjectPathForModal(`${currentPath.value}${objectKey}`);
}

/**
 * Watch for changes on the filepath and changes the current carousel item accordingly.
 */
watch(filePath, () => {
    if (!filePath.value) return;

    carouselIndex.value = fileIndex.value;
});

watch(() => props.modelValue, shown => {
    if (!shown) {
        return;
    }
    carouselIndex.value = fileIndex.value;
}, { immediate: true });
</script>
