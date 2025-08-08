// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="text-no-wrap" :class="alignClass">
        <v-tooltip v-if="deleting" location="top" text="Deleting object">
            <template #activator="{ props: loaderProps }">
                <v-progress-circular class="text-right" width="2" size="22" color="error" indeterminate v-bind="loaderProps" />
            </template>
        </v-tooltip>
        <template v-else>
            <v-btn
                v-if="!file.isDeleteMarker || (file.type === 'folder' && downloadPrefixEnabled)"
                variant="text"
                color="default"
                size="small"
                rounded="md"
                class="mr-1 text-caption"
                density="comfortable"
                title="Download"
                icon
                :loading="isDownloading"
                @click="onDownloadClick"
            >
                <component :is="Download" :size="18" />
                <v-tooltip
                    activator="parent"
                    location="top"
                >
                    Download
                </v-tooltip>
            </v-btn>

            <v-btn
                v-if="(!isVersion && !file.isDeleteMarker) || file.isLatest"
                variant="text"
                color="default"
                size="small"
                class="mr-1 text-caption"
                density="comfortable"
                title="Share"
                icon
                @click="emit('shareClick')"
            >
                <component :is="Share2" :size="17" />
            </v-btn>

            <v-btn
                variant="text"
                color="default"
                size="small"
                class="text-caption"
                density="comfortable"
                title="More Actions"
                icon
            >
                <v-icon :icon="Ellipsis" />
                <v-menu v-model="isMenuOpen" activator="parent">
                    <v-list class="pa-1">
                        <template v-if="file.type !== 'folder' && !file.isDeleteMarker">
                            <v-list-item density="comfortable" link @click="emit('previewClick')">
                                <template #prepend>
                                    <component :is="ZoomIn" :size="18" />
                                </template>
                                <v-list-item-title class="ml-3 text-body-2 font-weight-medium">
                                    Preview
                                </v-list-item-title>
                            </v-list-item>

                            <v-list-item
                                v-if="!file.isDeleteMarker"
                                density="comfortable"
                                :link="!isDownloading"
                                @click="onDownloadClick"
                            >
                                <template #prepend>
                                    <component :is="Download" :size="18" />
                                </template>
                                <v-fade-transition>
                                    <v-list-item-title v-show="!isDownloading" class="ml-3 text-body-2 font-weight-medium">
                                        Download
                                    </v-list-item-title>
                                </v-fade-transition>
                                <div v-if="isDownloading" class="browser_actions_menu__loader">
                                    <v-progress-circular indeterminate size="23" width="2" />
                                </div>
                            </v-list-item>
                            <v-list-item
                                v-if="objectLockEnabledForBucket"
                                :variant="lockStatus?.retention.active ? 'tonal' : 'text'"
                                :disabled="isGettingLockStatus"
                                density="comfortable"
                                link
                                @click="emit('lockObjectClick')"
                            >
                                <template #prepend>
                                    <v-progress-circular v-if="isGettingLockStatus" color="primary" indeterminate size="18" width="2" />
                                    <component :is="Lock" v-else :size="18" />
                                </template>
                                <v-list-item-title class="ml-3 text-body-2 font-weight-medium">
                                    Lock{{ lockStatus?.retention.active ? 'ed' : '' }}
                                </v-list-item-title>
                            </v-list-item>
                            <v-list-item
                                v-if="objectLockEnabledForBucket"
                                :variant="lockStatus?.legalHold ? 'tonal' : 'text'"
                                :class="{ 'mt-1' : lockStatus?.retention.active && lockStatus?.legalHold }"
                                :disabled="isGettingLockStatus"
                                density="comfortable"
                                link
                                @click="emit('legalHoldClick')"
                            >
                                <template #prepend>
                                    <v-progress-circular v-if="isGettingLockStatus" color="primary" indeterminate size="18" width="2" />
                                    <component :is="FileLock2" v-else :size="18" />
                                </template>
                                <v-list-item-title class="ml-3 text-body-2 font-weight-medium">
                                    {{ lockStatus?.legalHold ? 'On' : '' }} Legal Hold
                                </v-list-item-title>
                            </v-list-item>
                            <v-list-item v-if="isVersion && !isFileDeleted && !file.isLatest && !file.isDeleteMarker" density="comfortable" link @click="emit('restoreObjectClick')">
                                <template #prepend>
                                    <component :is="Redo2" :size="18" />
                                </template>
                                <v-list-item-title class="ml-3 text-body-2 font-weight-medium">
                                    Restore
                                </v-list-item-title>
                            </v-list-item>
                        </template>

                        <template v-if="file.type === 'folder'">
                            <v-list-item density="comfortable" link @click="emit('previewClick')">
                                <template #prepend>
                                    <component :is="FolderOpen" :size="18" />
                                </template>
                                <v-list-item-title class="ml-3 text-body-2 font-weight-medium">
                                    Open Folder
                                </v-list-item-title>
                            </v-list-item>
                            <v-list-item v-if="downloadPrefixEnabled" density="comfortable" link @click="emit('downloadFolderClick')">
                                <template #prepend>
                                    <component :is="Download" :size="18" />
                                </template>
                                <v-list-item-title class="ml-3 text-body-2 font-weight-medium">
                                    Download
                                </v-list-item-title>
                            </v-list-item>
                        </template>

                        <v-list-item v-if="!isVersion && !file.isDeleteMarker" density="comfortable" link @click="emit('shareClick')">
                            <template #prepend>
                                <component :is="Share2" :size="18" />
                            </template>
                            <v-list-item-title class="ml-3 text-body-2 font-weight-medium">
                                Share
                            </v-list-item-title>
                        </v-list-item>

                        <v-divider v-if="!file.isDeleteMarker" class="my-1" />

                        <template v-if="(!file.isDeleteMarker) || (file.isDeleteMarker && isVersion)">
                            <v-list-item :disabled="isGettingLockStatus" density="comfortable" link base-color="error" @click="onDeleteClick">
                                <template #prepend>
                                    <v-progress-circular v-if="isGettingLockStatus" indeterminate size="18" width="2" />
                                    <component :is="Trash2" v-else :size="18" />
                                </template>
                                <v-list-item-title class="ml-3 text-body-2 font-weight-medium">
                                    Delete
                                </v-list-item-title>
                            </v-list-item>
                        </template>
                    </v-list>
                </v-menu>
            </v-btn>
        </template>
    </div>
</template>

<script setup lang="ts">
import { ref, h, computed, watch } from 'vue';
import {
    VMenu,
    VList,
    VListItem,
    VListItemTitle,
    VDivider,
    VProgressCircular,
    VFadeTransition,
    VIcon,
    VBtn, VTooltip,
} from 'vuetify/components';
import { Ellipsis, Share2, Download, ZoomIn, Trash2, Redo2, Lock, FileLock2, FolderOpen } from 'lucide-vue-next';

import {
    BrowserObject,
    FullBrowserObject,
    useObjectBrowserStore,
} from '@/store/modules/objectBrowserStore';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { ProjectLimits } from '@/types/projects';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { BucketMetadata } from '@/types/buckets';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { ObjectLockStatus } from '@/types/objectLock';
import { usePreCheck } from '@/composables/usePreCheck';

const bucketsStore = useBucketsStore();
const configStore = useConfigStore();
const obStore = useObjectBrowserStore();
const projectsStore = useProjectsStore();

const notify = useNotify();
const { withTrialCheck } = usePreCheck();

const props = defineProps<{
    isVersion?: boolean;
    isFileDeleted?: boolean;
    file: BrowserObject;
    align: 'left' | 'right';
    deleting?: boolean;
}>();

const emit = defineEmits<{
    previewClick: [];
    deleteFileClick: [];
    shareClick: [];
    downloadFolderClick: [];
    restoreObjectClick: [];
    lockObjectClick: [];
    legalHoldClick: [];
    lockedObjectDelete: [FullBrowserObject];
}>();

const isMenuOpen = ref<boolean>(false);
const isDownloading = ref<boolean>(false);
const isGettingLockStatus = ref<boolean>(false);
const lockStatus = ref<ObjectLockStatus>();

const alignClass = computed<string>(() => {
    return 'text-' + props.align;
});

const downloadPrefixEnabled = computed<boolean>(() => configStore.state.config.downloadPrefixEnabled);

/**
 * Returns metadata of the current bucket.
 */
const bucket = computed<BucketMetadata | undefined>(() => {
    return bucketsStore.state.allBucketMetadata.find(b => b.name === bucketsStore.state.fileComponentBucketName);
});

/**
 * Whether object lock is enabled for current bucket.
 */
const objectLockEnabledForBucket = computed<boolean>(() => {
    return configStore.state.config.objectLockUIEnabled && !!bucket.value?.objectLockEnabled;
});

/**
 * Returns current limits from store.
 */
const limits = computed((): ProjectLimits => {
    return projectsStore.state.currentLimits;
});

const disableDownload = computed<boolean>(() => {
    const diff = (limits.value.userSetBandwidthLimit ?? limits.value.bandwidthLimit) - limits.value.bandwidthUsed;
    return props.file?.Size > diff;
});

async function onDownloadClick(): Promise<void> {
    withTrialCheck(async () => {
        if (disableDownload.value) {
            notify.error('Bandwidth limit exceeded, can not download this object.');
            return;
        }

        if (props.file.type === 'folder' && downloadPrefixEnabled.value) {
            emit('downloadFolderClick');
            return;
        }

        if (isDownloading.value) {
            return;
        }

        isDownloading.value = true;
        try {
            await obStore.download(props.file);
            notify.success(
                () => ['Keep this download link private.', h('br'), 'If you want to share, use the Share option.'],
                'Download started',
            );
        } catch (error) {
            error.message = `Error downloading object. ${error.message}`;
            notify.notifyError(error, AnalyticsErrorEventSource.FILE_BROWSER_ENTRY);
        }
        isDownloading.value = false;
    });
}

async function onDeleteClick(): Promise<void> {
    if (!lockStatus.value?.retention.active && !lockStatus.value?.legalHold) {
        emit('deleteFileClick');
        return;
    }
    emit('lockedObjectDelete', { ...props.file, retention: lockStatus.value?.retention, legalHold: lockStatus.value?.legalHold });
}

async function getLockStatus() {
    if (!objectLockEnabledForBucket.value || props.file.type === 'folder' || props.file.isDeleteMarker) {
        return;
    }
    if (isGettingLockStatus.value) {
        return;
    }
    isGettingLockStatus.value = true;
    try {
        lockStatus.value = await obStore.getObjectLockStatus(props.file);
    } catch (error) {
        error.message = `Error getting object lock status. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.FILE_BROWSER_ENTRY);
    } finally {
        isGettingLockStatus.value = false;
    }
}

watch(isMenuOpen, isOpen => {
    if (!isOpen) {
        lockStatus.value = undefined;
        return;
    }
    getLockStatus();
});

</script>

<style scoped lang="scss">
.browser_actions_menu__loader {
    inset: 0;
    position: absolute;
    display: flex;
    align-items: center;
    justify-content: center;
}
</style>
