// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        transition="fade-transition"
        class="preview-dialog"
        fullscreen
        theme="dark"
        no-click-animation
    >
        <v-card class="preview-card">
            <v-toolbar
                color="rgba(0, 0, 0, 0.3)"
                theme="dark"
            >
                <v-toolbar-title class="text-subtitle-2">
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
                    <v-btn icon size="small" color="white" @click="isGeographicDistributionDialogShown = true">
                        <icon-distribution size="22" />
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
                        <v-menu activator="parent">
                            <v-list class="pa-1">
                                <v-list-item density="comfortable" link rounded="lg" base-color="error" @click="onDeleteFileClick">
                                    <template #prepend>
                                        <icon-trash bold />
                                    </template>
                                    <v-list-item-title class="pl-2 ml-2 text-body-2 font-weight-medium">
                                        Delete
                                    </v-list-item-title>
                                </v-list-item>
                            </v-list>
                        </v-menu>
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
            <v-carousel
                ref="carousel"
                v-model="constCarouselIndex"
                tabindex="0"
                hide-delimiters
                show-arrows="hover"
                class="h-100 no-outline"
                @keydown.right="onNext"
                @keydown.left="onPrevious"
            >
                <template #prev>
                    <v-btn
                        v-if="files.length > 1"
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
                        v-if="files.length > 1"
                        color="default"
                        class="rounded-circle"
                        icon
                        @click="onNext"
                    >
                        <v-icon icon="mdi-chevron-right" size="x-large" />
                    </v-btn>
                </template>

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
    <geographic-distribution-dialog v-model="isGeographicDistributionDialogShown" />
    <delete-file-dialog
        v-if="fileToDelete"
        v-model="isDeleteFileDialogShown"
        :file="fileToDelete"
        @file-deleted="onDeleteComplete"
    />
</template>

<script setup lang="ts">
import { computed, h, nextTick, ref, watch } from 'vue';
import {
    VBtn,
    VCard,
    VCarousel,
    VCarouselItem,
    VDialog,
    VIcon,
    VList,
    VListItem,
    VListItemTitle,
    VMenu,
    VToolbar,
    VToolbarTitle,
    VTooltip,
} from 'vuetify/components';

import { BrowserObject, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

import IconShare from '@poc/components/icons/IconShare.vue';
import IconDistribution from '@poc/components/icons/IconDistribution.vue';
import FilePreviewItem from '@poc/components/dialogs/filePreviewComponents/FilePreviewItem.vue';
import ShareDialog from '@poc/components/dialogs/ShareDialog.vue';
import GeographicDistributionDialog from '@poc/components/dialogs/GeographicDistributionDialog.vue';
import IconTrash from '@poc/components/icons/IconTrash.vue';
import DeleteFileDialog from '@poc/components/dialogs/DeleteFileDialog.vue';

const obStore = useObjectBrowserStore();
const bucketsStore = useBucketsStore();
const notify = useNotify();

const carousel = ref<VCarousel | null>(null);
const isDownloading = ref<boolean>(false);
const isShareDialogShown = ref<boolean>(false);
const isGeographicDistributionDialogShown = ref<boolean>(false);
const fileToDelete = ref<BrowserObject | null>(null);
const isDeleteFileDialogShown = ref<boolean>(false);

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
        notify.success(
            () => ['Keep this download link private.', h('br'), 'If you want to share, use the Share option.'],
            'Download Started',
        );
    } catch (error) {
        error.message = `Error downloading file. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.GALLERY_VIEW);
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
 * Sets focus on carousel so that keyboard navigation works.
 */
async function focusOnCarousel(): Promise<void> {
    await nextTick();
    carousel.value?.$el.focus();
}

/**
 * Handles delete button click event for files.
 */
function onDeleteFileClick(): void {
    fileToDelete.value = currentFile.value;
    isDeleteFileDialogShown.value = true;
}

/**
 * Closes the preview on file delete.
 */
function onDeleteComplete(): void {
    fileToDelete.value = null;
    model.value = false;
}

/**
 * Watch for changes on the filepath and changes the current carousel item accordingly.
 */
watch(filePath, () => {
    if (!filePath.value) return;

    carouselIndex.value = fileIndex.value;
});

watch(() => props.modelValue, async (shown) => {
    if (!shown) {
        return;
    }

    carouselIndex.value = fileIndex.value;
    await focusOnCarousel();
}, { immediate: true });

watch(isShareDialogShown, async () => {
    if (isShareDialogShown.value) {
        return;
    }

    await focusOnCarousel();
});

watch(isGeographicDistributionDialogShown, async () => {
    if (isGeographicDistributionDialogShown.value) {
        return;
    }

    await focusOnCarousel();
});
</script>

<style scoped lang="scss">
.no-outline {
    outline: none;
}
</style>
