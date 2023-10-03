// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-row class="pt-2" justify="space-between">
        <v-col cols="9">
            <p class="text-truncate">{{ item.Key }}</p>
        </v-col>
        <v-col cols="auto">
            <v-tooltip :text="uploadStatus">
                <template #activator="{ props: activatorProps }">
                    <v-progress-circular
                        v-if="props.item.status === UploadingStatus.InProgress"
                        v-bind="activatorProps"
                        :indeterminate="!item.progress"
                        :size="20"
                        color="secondary"
                        :model-value="progressStyle"
                    />
                    <v-icon
                        v-else
                        v-bind="activatorProps"
                        :icon="icon"
                        :color="iconColor"
                    />
                </template>
            </v-tooltip>

            <v-tooltip v-if="props.item.status === UploadingStatus.InProgress" text="Cancel upload">
                <template #activator="{ props: activatorProps }">
                    <v-icon
                        class="ml-2"
                        v-bind="activatorProps"
                        icon="mdi-close-circle"
                        @click="cancelUpload"
                    />
                </template>
            </v-tooltip>
        </v-col>
    </v-row>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { VCol, VIcon, VProgressCircular, VRow, VTooltip } from 'vuetify/components';

import {
    UploadingBrowserObject,
    UploadingStatus,
    useObjectBrowserStore,
} from '@/store/modules/objectBrowserStore';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

const obStore = useObjectBrowserStore();
const notify = useNotify();

const props = defineProps<{
    item: UploadingBrowserObject
}>();

const progressStyle = computed((): number => {
    if (props.item.progress) {
        return 360*(props.item.progress/100);
    }
    return 0;
});

const uploadStatus = computed((): string => {
    if (props.item.status === UploadingStatus.InProgress) {
        return 'Uploading...';
    } else if (props.item.status === UploadingStatus.Finished) {
        return 'Upload complete';
    } else if (props.item.status === UploadingStatus.Failed) {
        return 'Upload failed';
    } else if (props.item.status === UploadingStatus.Cancelled) {
        return 'Upload cancelled';
    } else {
        return '';
    }
});

const icon = computed((): string => {
    if (props.item.status === UploadingStatus.Finished) {
        return 'mdi-checkbox-marked-circle';
    } else if (props.item.status === UploadingStatus.Failed) {
        return 'mdi-information';
    } else if (props.item.status === UploadingStatus.Cancelled) {
        return 'mdi-cancel';
    } else {
        return '';
    }
});

const iconColor = computed((): string => {
    if (props.item.status === UploadingStatus.Finished) {
        return 'success';
    } else if (props.item.status === UploadingStatus.Failed) {
        return 'warning';
    } else if (props.item.status === UploadingStatus.Cancelled) {
        return 'error';
    } else {
        return '';
    }
});

/**
 * Cancels active upload.
 */
function cancelUpload(): void {
    try {
        obStore.cancelUpload(props.item.Key);
    } catch (error) {
        notify.error(error.message, AnalyticsErrorEventSource.OBJECTS_UPLOAD_MODAL);
    }
}

</script>