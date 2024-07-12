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
                    <p v-if="showingVersions && currentFile"> Version ID: {{ currentFile.VersionId }} </p>
                </v-toolbar-title>
                <template #append>
                    <v-btn id="Download" :loading="isDownloading" icon size="small" color="white" @click="download">
                        <img src="@/assets/icon-download.svg" width="22" alt="Download">
                        <v-tooltip
                            activator="parent"
                            location="bottom"
                            theme="light"
                        >
                            Download
                        </v-tooltip>
                    </v-btn>
                    <v-btn v-if="showingVersions" id="Delete" icon size="small" color="red" @click="onDeleteFileClick">
                        <icon-trash />
                        <v-tooltip
                            activator="parent"
                            location="bottom"
                            theme="light"
                        >
                            Delete
                        </v-tooltip>
                    </v-btn>
                    <v-btn v-if="!showingVersions" id="Share" icon size="small" color="white" @click="isShareDialogShown = true">
                        <icon-share size="22" />
                        <v-tooltip
                            activator="parent"
                            location="bottom"
                            theme="light"
                        >
                            Share
                        </v-tooltip>
                    </v-btn>
                    <v-btn v-if="!showingVersions" id="Distribution" icon size="small" color="white" @click="isGeographicDistributionDialogShown = true">
                        <icon-distribution size="22" />
                        <v-tooltip
                            activator="parent"
                            location="bottom"
                            theme="light"
                        >
                            Geographic Distribution
                        </v-tooltip>
                    </v-btn>
                    <v-btn v-if="!showingVersions" icon size="small" color="white">
                        <img src="@/assets/icon-more.svg" width="22" alt="More">
                        <v-tooltip
                            activator="parent"
                            location="bottom"
                            theme="light"
                        >
                            More
                        </v-tooltip>
                        <v-menu activator="parent">
                            <v-list class="pa-1" theme="light">
                                <v-list-item density="comfortable" link base-color="error" @click="onDeleteFileClick">
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
                    <v-btn id="close-preview" icon size="small" color="white" @click="model = false">
                        <img src="@/assets/icon-close.svg" width="18" alt="Close">
                        <v-tooltip
                            activator="parent"
                            location="bottom"
                            theme="light"
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
                        <v-icon :icon="ChevronLeft" size="x-large" />
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
                        <v-icon :icon="ChevronRight" size="x-large" />
                    </v-btn>
                </template>

                <v-carousel-item v-for="(file, i) in files" :key="file.Key">
                    <!-- v-carousel will mount all items at the same time -->
                    <!-- so :active will tell file-preview-item if it is the current. -->
                    <!-- If it is, it'll load the preview. -->
                    <file-preview-item
                        :active="i === fileIndex"
                        :file="file"
                        :video-autoplay="videoAutoplay"
                        :showing-version="showingVersions"
                        @download="download"
                    />
                </v-carousel-item>
            </v-carousel>
        </v-card>
    </v-dialog>

    <share-dialog v-if="!showingVersions" v-model="isShareDialogShown" :bucket-name="bucketName" :file="currentFile" />
    <geographic-distribution-dialog v-if="!showingVersions" v-model="isGeographicDistributionDialogShown" />
    <delete-file-dialog
        v-if="fileToDelete"
        v-model="isDeleteFileDialogShown"
        :files="[fileToDelete]"
        @files-deleted="onDeleteComplete"
    />
</template>

<script setup lang="ts">
import { computed, h, nextTick, ref, watch, watchEffect } from 'vue';
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
import { ChevronLeft, ChevronRight } from 'lucide-vue-next';

import { BrowserObject, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { ProjectLimits } from '@/types/projects';
import { useProjectsStore } from '@/store/modules/projectsStore';

import IconShare from '@/components/icons/IconShare.vue';
import IconDistribution from '@/components/icons/IconDistribution.vue';
import FilePreviewItem from '@/components/dialogs/filePreviewComponents/FilePreviewItem.vue';
import ShareDialog from '@/components/dialogs/ShareDialog.vue';
import GeographicDistributionDialog from '@/components/dialogs/GeographicDistributionDialog.vue';
import IconTrash from '@/components/icons/IconTrash.vue';
import DeleteFileDialog from '@/components/dialogs/DeleteFileDialog.vue';

const obStore = useObjectBrowserStore();
const projectsStore = useProjectsStore();
const bucketsStore = useBucketsStore();
const notify = useNotify();

const props = withDefaults(defineProps<{
    videoAutoplay?: boolean
    showingVersions?: boolean
}>(), {
    videoAutoplay: false,
    showingVersions: false,
});

const emit = defineEmits<{
    'fileDeleted': [],
}>();

const carousel = ref<VCarousel | null>(null);
const isDownloading = ref<boolean>(false);
const isShareDialogShown = ref<boolean>(false);
const isGeographicDistributionDialogShown = ref<boolean>(false);
const fileToDelete = ref<BrowserObject | null>(null);
const isDeleteFileDialogShown = ref<boolean>(false);

const folderType = 'folder';

const model = defineModel<boolean>({ required: true });
const currentFile = defineModel<BrowserObject | null>('currentFile', { required: true });

const constCarouselIndex = computed(() => carouselIndex.value);
const carouselIndex = ref(0);

const files = computed((): BrowserObject[] => {
    if (props.showingVersions) {
        return obStore.state.objectVersions.get(filePath.value) ?? [];
    }
    return obStore.sortedFiles;
});

/**
 * Retrieve the file index that the modal is set to from the store.
 */
const fileIndex = computed((): number => {
    if (props.showingVersions) {
        return files.value.findIndex((file) => {
            return file.path + file.Key === filePath.value && file.VersionId === currentFile.value?.VersionId;
        });
    }
    return files.value.findIndex(f => f.Key === filePath.value.split('/').pop());
});

/**
 * Retrieve the name of the current file.
 */
const fileName = computed((): string | undefined => {
    return currentFile.value?.Key;
});

/**
 * Retrieve the current filepath.
 */
const filePath = computed((): string => {
    if (!currentFile.value) return obStore.state.objectPathForModal;
    return currentFile.value.path + currentFile.value.Key;
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
 * Returns current limits from store.
 */
const limits = computed((): ProjectLimits => {
    return projectsStore.state.currentLimits;
});

const disableDownload = computed<boolean>(() => {
    const diff = (limits.value.userSetBandwidthLimit ?? limits.value.bandwidthLimit) - limits.value.bandwidthUsed;
    return (currentFile.value?.Size ?? 0) > diff;
});

/**
 * Download the current opened file.
 */
async function download(): Promise<void> {
    if (disableDownload.value) {
        notify.error('Bandwidth limit exceeded, can not download this file.');
        return;
    }
    if (isDownloading.value || !currentFile.value) {
        return;
    }

    isDownloading.value = true;
    try {
        await obStore.download(currentFile.value);
        notify.success(
            () => ['Keep this download link private.', h('br'), 'If you want to share, use the Share option.'],
            'Download started',
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
    if (files.value.length === 1) return;
    const currentIndex = fileIndex.value;
    const filesLength = files.value.length;

    let newFile = currentIndex <= 0 ? files.value[filesLength - 1] : files.value[currentIndex - 1];
    if (newFile.type === folderType) {
        newFile = files.value[filesLength - 1];
    }
    setNewObjectPath(newFile);
}

/**
 * Handles on next click logic.
 */
function onNext(): void {
    if (files.value.length === 1) return;
    const newIndex = fileIndex.value + 1;
    const filesLength = files.value.length;
    let newFile: BrowserObject | undefined = newIndex >= filesLength ? files.value[0] : files.value[newIndex];
    if (!newFile || newFile.type === folderType) {
        newFile = files.value.find(f => f.type !== folderType);

        if (!newFile) return;
    }
    setNewObjectPath(newFile);
}

/**
 * Sets new object path.
 */
function setNewObjectPath(file: BrowserObject): void {
    obStore.setObjectPathForModal(`${currentPath.value}${file.Key}`);
    currentFile.value = file;
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
    emit('fileDeleted');
}

watchEffect(async () => {
    if (!model.value) {
        return;
    }

    carouselIndex.value = fileIndex.value;
    await focusOnCarousel();
});

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
