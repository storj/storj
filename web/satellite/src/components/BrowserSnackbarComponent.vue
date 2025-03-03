// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-snackbar
        :model-value="isObjectsUploadModal"
        vertical
        :timeout="-1"
        color="default"
        elevation="24"
        rounded="lg"
        class="upload-snackbar"
        width="100%"
        max-width="400px"
    >
        <v-row>
            <v-col>
                <v-expansion-panels theme="dark" @update:model-value="v => isExpanded = v !== undefined">
                    <v-expansion-panel
                        color="default"
                        rounded="lg"
                    >
                        <v-expansion-panel-title class="pr-5">
                            <span>{{ statusLabel }}</span>
                            <template v-if="isClosable" #actions>
                                <v-row class="ma-0 align-center">
                                    <v-icon v-if="!isExpanded" :icon="ChevronUp" class="mr-3" />
                                    <v-icon v-else :icon="ChevronDown" class="mr-3" />
                                    <v-btn variant="outlined" color="default" size="x-small" :icon="X" title="Close" @click="closeDialog" />
                                </v-row>
                            </template>
                        </v-expansion-panel-title>
                        <v-progress-linear
                            v-if="!isClosable"
                            rounded
                            :indeterminate="!progress"
                            :model-value="progress"
                            height="6"
                            color="success"
                        />
                        <v-expansion-panel-text v-if="!isClosable && objectsInProgress.length > 1">
                            <v-row justify="space-between" class="pt-2">
                                <v-col cols="auto">
                                    <p class="text-medium-emphasis">{{ remainingTimeString }}</p>
                                </v-col>
                                <v-col cols="auto">
                                    <v-tooltip text="Cancel all uploads">
                                        <template #activator="{ props: activatorProps }">
                                            <v-icon
                                                v-bind="activatorProps"
                                                :icon="CircleX"
                                                @click="cancelAll"
                                            />
                                        </template>
                                    </v-tooltip>
                                </v-col>
                            </v-row>
                        </v-expansion-panel-text>
                        <v-divider />
                        <v-expand-transition>
                            <div v-show="isExpanded" class="uploading-content">
                                <UploadItem
                                    v-for="item in uploading"
                                    :key="item.Key"
                                    :item="item"
                                    @click="item.status === UploadingStatus.Finished && onFileClick(item)"
                                />
                            </div>
                        </v-expand-transition>
                    </v-expansion-panel>
                </v-expansion-panels>
            </v-col>
        </v-row>
    </v-snackbar>

    <file-preview-dialog
        v-model="previewDialog"
        v-model:current-file="fileToPreview"
    />
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import {
    VSnackbar,
    VRow,
    VCol,
    VExpansionPanels,
    VExpansionPanel,
    VExpansionPanelTitle,
    VProgressLinear,
    VExpansionPanelText,
    VTooltip,
    VIcon,
    VDivider,
    VExpandTransition,
    VBtn,
} from 'vuetify/components';
import { useRouter } from 'vue-router';
import { ChevronDown, ChevronUp, CircleX, X } from 'lucide-vue-next';

import { BrowserObject, UploadingBrowserObject, UploadingStatus, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { Duration } from '@/utils/time';
import { useNotify } from '@/composables/useNotify';
import { useAppStore } from '@/store/modules/appStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { ROUTES } from '@/router';

import UploadItem from '@/components/UploadItem.vue';
import FilePreviewDialog from '@/components/dialogs/FilePreviewDialog.vue';

const appStore = useAppStore();
const obStore = useObjectBrowserStore();
const bucketsStore = useBucketsStore();
const projectsStore = useProjectsStore();

const router = useRouter();

const remainingTimeString = ref<string>('');
const interval = ref<NodeJS.Timer>();
const notify = useNotify();
const startDate = ref<number>(Date.now());
const isExpanded = ref<boolean>(false);
const previewDialog = ref<boolean>(false);
const fileToPreview = ref<BrowserObject | undefined>();

/**
 * Indicates whether objects upload modal should be shown.
 */
const isObjectsUploadModal = computed<boolean>(() => appStore.state.isUploadingModal);

/**
 * Returns header's status label.
 */
const statusLabel = computed((): string => {
    if (!uploading.value.length) return 'No items to upload';
    let inProgress = 0, failed = 0, cancelled = 0;
    uploading.value.forEach(u => {
        switch (u.status) {
        case UploadingStatus.InProgress:
            inProgress++;
            break;
        case UploadingStatus.Failed:
            failed++;
            break;
        case UploadingStatus.Cancelled:
            cancelled++;
        }
    });

    if (failed === uploading.value.length) return 'Uploading failed';
    if (cancelled === uploading.value.length) return 'Uploading cancelled';
    if (inProgress) return `Uploading ${inProgress} item${inProgress > 1 ? 's' : ''}`;

    const statuses = [
        failed ? `${failed} failed` : '',
        cancelled ? `${cancelled} cancelled` : '',
    ].filter(s => s).join(', ');

    return `Uploading completed${statuses ? ` (${statuses})` : ''}`;
});

/**
 * Returns upload progress.
 */
const progress = computed((): number => {
    const activeUploads = uploading.value.filter(f => f.status === UploadingStatus.InProgress);
    return activeUploads.reduce((total: number, item: UploadingBrowserObject) => {
        total += item.progress || 0;
        return total;
    }, 0) / activeUploads.length;
});

/**
 * Returns uploading objects from store.
 */
const uploading = computed((): UploadingBrowserObject[] => {
    return obStore.state.uploading;
});

/**
 * Returns the current path within the selected bucket.
 */
const filePath = computed<string>(() => bucketsStore.state.fileComponentPath);

/**
 * Returns the name of the selected bucket.
 */
const bucketName = computed<string>(() => bucketsStore.state.fileComponentBucketName);

/**
 * Calculates remaining seconds.
 */
function calculateRemainingTime(): void {
    const progress = uploading.value.reduce((total: number, item: UploadingBrowserObject) => {
        if (item.progress && item.progress !== 100) {
            total += item.progress;
        }
        return total;
    }, 0);

    const remainingProgress = 100 - progress;
    const averageProgressPerNanosecond = progress / ((Date.now() - startDate.value) * 1000000);
    const remainingNanoseconds = remainingProgress / averageProgressPerNanosecond;
    if (!isFinite(remainingNanoseconds) || remainingNanoseconds < 0) {
        remainingTimeString.value = 'Unknown ETA';
        return;
    }

    remainingTimeString.value = `Estimated time remaining: ${new Duration(remainingNanoseconds).remainingFormatted}`;
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
        router.push(`${ROUTES.Projects.path}/${projectsStore.state.selectedProject.urlId}/${ROUTES.Buckets.path}/${bucketName.value}/${pathAndKey}`);
        return;
    }

    const objectToPreview = { ...file };
    const pathParts = objectToPreview.Key.split('/');
    const key = pathParts.pop();
    const path = pathParts.length ? `${pathParts.join('/')}/` : '';

    objectToPreview.Key = key ?? file.Key;
    objectToPreview.path = path;

    obStore.setObjectPathForModal((objectToPreview.path ?? '') + objectToPreview.Key);
    fileToPreview.value = objectToPreview;
    previewDialog.value = true;
}

/**
 * Cancels all uploads in progress.
 */
function cancelAll(): void {
    objectsInProgress.value.forEach(item => {
        try {
            obStore.cancelUpload(item.Key);
        } catch (error) {
            notify.error(`Unable to cancel upload for '${item.Key}'. ${error.message}`, AnalyticsErrorEventSource.OBJECTS_UPLOAD_MODAL);
        }
    });
}

/**
 * Returns uploading objects with InProgress status.
 */
const objectsInProgress = computed((): UploadingBrowserObject[] => {
    return uploading.value.filter(f => f.status === UploadingStatus.InProgress);
});

/**
 * Indicates if modal is closable.
 */
const isClosable = computed((): boolean => {
    return !objectsInProgress.value.length;
});

/**
 * Starts interval for recalculating remaining time.
 */
function startInterval(): void {
    const int = setInterval(() => {
        if (isClosable.value) {
            clearInterval(int);
            interval.value = undefined;
            remainingTimeString.value = '';
            return;
        }

        calculateRemainingTime();
    }, 2000); // recalculate every 2 seconds.

    interval.value = int;
}

function closeDialog(): void {
    isExpanded.value = false;
    appStore.setUploadingModal(false);
    obStore.clearUploading();
}

watch(() => objectsInProgress.value.length, () => {
    if (!interval.value) {
        startDate.value = Date.now();
        startInterval();
    }
});

watch(() => projectsStore.state.selectedProject, (value, oldValue) => {
    if (value.id === oldValue.id || !isObjectsUploadModal.value) {
        return;
    }
    closeDialog();
});

watch(bucketName, (value, oldValue) => {
    if (value === oldValue || !isObjectsUploadModal.value) {
        return;
    }
    closeDialog();
});

/**
 * Close the snackbar if nothing is uploading.
 */
watch(uploading, (value, oldValue) => {
    if (value.length === oldValue.length) {
        return;
    }

    if (!value.length) {
        closeDialog();
    }
});

onMounted(() => {
    startInterval();
});
</script>

<style scoped lang="scss">
.uploading-content {
    overflow-y: auto;
    max-height: 200px;
}
</style>
