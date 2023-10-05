// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-snackbar
        vertical
        :timeout="4000"
        color="default"
        elevation="24"
        rounded="lg"
        class="upload-snackbar"
        width="100%"
        max-width="400px"
    >
        <v-row>
            <v-col>
                <v-expansion-panels theme="dark" @update:model-value="v => isExpanded = v != undefined">
                    <v-expansion-panel
                        color="default"
                        rounded="lg"
                    >
                        <v-expansion-panel-title color="">
                            <span>{{ statusLabel }}</span>
                        </v-expansion-panel-title>
                        <v-progress-linear
                            v-if="!isClosable"
                            rounded
                            :indeterminate="!progress"
                            :model-value="progress"
                            height="6"
                            color="success"
                            class="mt-1"
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
                                                icon="mdi-close-circle"
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
                                    @click="item.status === UploadingStatus.Finished && emit('fileClick', item)"
                                />
                            </div>
                        </v-expand-transition>
                    </v-expansion-panel>
                </v-expansion-panels>
            </v-col>
        </v-row>
    </v-snackbar>
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
} from 'vuetify/components';

import { BrowserObject, UploadingBrowserObject, UploadingStatus, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { Duration } from '@/utils/time';
import { useNotify } from '@/utils/hooks';

import UploadItem from '@poc/components/UploadItem.vue';

const obStore = useObjectBrowserStore();
const remainingTimeString = ref<string>('');
const interval = ref<NodeJS.Timer>();
const notify = useNotify();
const startDate = ref<number>(Date.now());
const isExpanded = ref<boolean>(false);

const emit = defineEmits<{
    'fileClick': [file: BrowserObject],
}>();

/**
 * Returns header's status label.
 */
const statusLabel = computed((): string => {
    let inProgress = 0, finished = 0, failed = 0, cancelled = 0;
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
            break;
        default:
            finished++;
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
    return uploading.value.reduce((total: number, item: UploadingBrowserObject) => {
        total += item.progress || 0;
        return total;
    }, 0) / uploading.value.length;
});

/**
 * Returns uploading objects from store.
 */
const uploading = computed((): UploadingBrowserObject[] => {
    return obStore.state.uploading;
});

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

    remainingTimeString.value = new Duration(remainingNanoseconds).remainingFormatted;
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

watch(() => objectsInProgress.value.length, () => {
    if (!interval.value) {
        startDate.value = Date.now();
        startInterval();
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