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
        :persistent="false"
    >
        <v-card class="preview-card">
            <v-toolbar
                color="rgba(0, 0, 0, 0.3)"
                theme="dark"
            >
                <v-toolbar-title class="text-subtitle-2">
                    {{ fileName }}
                    <p v-if="showingVersions && currentFile" class="text-caption text-medium-emphasis"> Version ID: {{ currentFile.VersionId }} </p>
                </v-toolbar-title>
                <template #append>
                    <v-btn id="Download" :loading="isDownloading" icon size="small" color="white" :title="$vuetify.display.smAndDown ? 'Download' : undefined" @click="download">
                        <component :is="Download" :size="20" />
                        <v-tooltip
                            activator="parent"
                            location="bottom"
                            theme="light"
                            class="hidden-sm-and-down"
                        >
                            Download
                        </v-tooltip>
                    </v-btn>
                    <v-btn v-if="showingVersions" id="Delete" :loading="isGettingRetention" icon size="small" color="red" :title="$vuetify.display.smAndDown ? 'Delete' : undefined" @click="onDeleteFileClick">
                        <component :is="Trash2" :size="20" />
                        <v-tooltip
                            activator="parent"
                            location="bottom"
                            theme="light"
                            class="hidden-sm-and-down"
                        >
                            Delete
                        </v-tooltip>
                    </v-btn>
                    <v-btn v-if="!showingVersions" id="Share" icon size="small" color="white" :title="$vuetify.display.smAndDown ? 'Share' : undefined" @click="isShareDialogShown = true">
                        <component :is="Share2" :size="19" />
                        <v-tooltip
                            activator="parent"
                            location="bottom"
                            theme="light"
                            class="hidden-sm-and-down"
                        >
                            Share
                        </v-tooltip>
                    </v-btn>
                    <v-btn v-if="!showingVersions" id="Distribution" icon size="small" color="white" :title="$vuetify.display.smAndDown ? 'Geographic Distribution' : undefined" @click="isGeographicDistributionDialogShown = true">
                        <icon-distribution size="20" />
                        <v-tooltip
                            activator="parent"
                            location="bottom"
                            theme="light"
                            class="hidden-sm-and-down"
                        >
                            Geographic Distribution
                        </v-tooltip>
                    </v-btn>
                    <v-btn v-if="!showingVersions" icon size="small" color="white" title="More Actions">
                        <component :is="EllipsisVertical" :size="20" />
                        <v-menu activator="parent">
                            <v-list class="pa-1" theme="light">
                                <v-list-item :disabled="isGettingRetention" density="comfortable" link base-color="error" @click="onDeleteFileClick">
                                    <template #prepend>
                                        <component :is="Trash2" v-if="!isGettingRetention" :size="18" />
                                        <v-progress-circular v-else size="small" indeterminate />
                                    </template>
                                    <v-list-item-title class="pl-1 ml-2 text-body-2 font-weight-medium">
                                        Delete
                                    </v-list-item-title>
                                </v-list-item>
                            </v-list>
                        </v-menu>
                    </v-btn>
                    <v-btn id="close-preview" icon size="small" color="white" :title="$vuetify.display.smAndDown ? 'Close Preview' : undefined" @click="model = false">
                        <component :is="X" :size="20" />
                        <v-tooltip
                            activator="parent"
                            location="bottom"
                            theme="light"
                            class="hidden-sm-and-down"
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
                        variant="outlined"
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
                        variant="outlined"
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

    <share-dialog v-if="!showingVersions" v-model="isShareDialogShown" :bucket-name="bucketName" :file="currentFile ?? undefined" />
    <geographic-distribution-dialog v-if="!showingVersions" v-model="isGeographicDistributionDialogShown" />
    <delete-versions-dialog
        v-if="showingVersions"
        v-model="isDeleteFileDialogShown"
        :files="fileToDelete ? [fileToDelete] : []"
        @content-removed="onDeleteFileDialogClose"
    />
    <delete-file-dialog
        v-else-if="!isBucketVersioned"
        v-model="isDeleteFileDialogShown"
        :files="fileToDelete ? [fileToDelete] : []"
        @content-removed="onDeleteFileDialogClose"
    />
    <delete-versioned-file-dialog
        v-else
        v-model="isDeleteFileDialogShown"
        :files="fileToDelete ? [fileToDelete] : []"
        @content-removed="onDeleteFileDialogClose"
    />
    <locked-delete-error-dialog
        v-model="isLockedObjectDeleteDialogShown"
        :file="lockActionFile"
        @content-removed="lockActionFile = null"
    />
</template>

<script setup lang="ts">
import { computed, h, nextTick, ref, watch, watchEffect } from 'vue';
import {
    VBtn,
    VCard,
    VCarousel,
    VCarouselItem,
    VProgressCircular,
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
import { ChevronLeft, ChevronRight, Share2, Trash2, Download, X, EllipsisVertical } from 'lucide-vue-next';

import { BrowserObject, FullBrowserObject, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { ProjectLimits } from '@/types/projects';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { Versioning } from '@/types/versioning';
import { BucketMetadata } from '@/types/buckets';
import { useConfigStore } from '@/store/modules/configStore';

import IconDistribution from '@/components/icons/IconDistribution.vue';
import FilePreviewItem from '@/components/dialogs/filePreviewComponents/FilePreviewItem.vue';
import ShareDialog from '@/components/dialogs/ShareDialog.vue';
import GeographicDistributionDialog from '@/components/dialogs/GeographicDistributionDialog.vue';
import DeleteFileDialog from '@/components/dialogs/DeleteFileDialog.vue';
import DeleteVersionedFileDialog from '@/components/dialogs/DeleteVersionedFileDialog.vue';
import DeleteVersionsDialog from '@/components/dialogs/DeleteVersionsDialog.vue';
import LockedDeleteErrorDialog from '@/components/dialogs/LockedDeleteErrorDialog.vue';

const obStore = useObjectBrowserStore();
const projectsStore = useProjectsStore();
const bucketsStore = useBucketsStore();
const configStore = useConfigStore();
const notify = useNotify();

const props = withDefaults(defineProps<{
    videoAutoplay?: boolean
    versions?: BrowserObject[]
}>(), {
    videoAutoplay: false,
    versions: () => [],
});

const emit = defineEmits<{
    'fileDeleteRequested': [],
}>();

const carousel = ref<VCarousel | null>(null);
const isDownloading = ref<boolean>(false);
const isGettingRetention = ref<boolean>(false);
const isShareDialogShown = ref<boolean>(false);
const isGeographicDistributionDialogShown = ref<boolean>(false);
const fileToDelete = ref<BrowserObject | undefined>();
const lockActionFile = ref<FullBrowserObject | null>(null);
const isDeleteFileDialogShown = ref<boolean>(false);
const isLockedObjectDeleteDialogShown = ref<boolean>(false);

const folderType = 'folder';

const model = defineModel<boolean>({ required: true });
const currentFile = defineModel<BrowserObject | undefined>('currentFile', { required: true });

const constCarouselIndex = computed(() => carouselIndex.value);
const carouselIndex = ref(0);

const showingVersions = computed(() => props.versions.length > 0);

/**
 * Returns metadata of the current bucket.
 */
const bucket = computed<BucketMetadata | undefined>(() => {
    return bucketsStore.state.allBucketMetadata.find(b => b.name === bucketName.value);
});

/**
 * Whether object lock is enabled for current bucket.
 */
const objectLockEnabledForBucket = computed<boolean>(() => {
    return configStore.state.config.objectLockUIEnabled && !!bucket.value?.objectLockEnabled;
});

/**
 * Whether this bucket is versioned/version-suspended.
 */
const isBucketVersioned = computed<boolean>(() => {
    return bucket.value?.versioning !== Versioning.NotSupported && bucket.value?.versioning !== Versioning.Unversioned;
});

const files = computed((): BrowserObject[] => {
    if (showingVersions.value) {
        return props.versions;
    }
    return obStore.sortedFiles;
});

/**
 * Retrieve the file index that the modal is set to from the store.
 */
const fileIndex = computed((): number => {
    if (showingVersions.value) {
        return props.versions.findIndex((file) => {
            return file.VersionId === currentFile.value?.VersionId;
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
async function onDeleteFileClick(): Promise<void> {
    function initDelete() {
        fileToDelete.value = currentFile.value;
        isDeleteFileDialogShown.value = true;
    }
    if (!objectLockEnabledForBucket.value) {
        initDelete();
        return;
    }
    if (isGettingRetention.value || !currentFile.value) {
        return;
    }
    isGettingRetention.value = true;
    try {
        const retention = await obStore.getObjectRetention(currentFile.value);
        if (!retention.active) {
            initDelete();
            return;
        }
        lockActionFile.value = { ...currentFile.value, retention };
        isLockedObjectDeleteDialogShown.value = true;
    } catch (error) {
        error.message = `Error deleting file. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.FILE_BROWSER_ENTRY);
    } finally {
        isGettingRetention.value = false;
    }
}

/**
 * Closes the preview on file delete dialog close.
 */
function onDeleteFileDialogClose(): void {
    fileToDelete.value = undefined;
    model.value = false;
    emit('fileDeleteRequested');
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
