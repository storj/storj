// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-list-item :title="item.Key" class="px-6" height="54" :link="props.item.status === UploadingStatus.Finished">
        <template #append>
            <v-tooltip :text="uploadStatus" location="left">
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
                        :icon="mdiCloseCircle"
                        @click="cancelUpload"
                    />
                </template>
            </v-tooltip>
        </template>
    </v-list-item>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { VListItem, VIcon, VProgressCircular, VTooltip } from 'vuetify/components';
import { mdiCancel, mdiCheckboxMarkedCircle, mdiCloseCircle, mdiInformation } from '@mdi/js';

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
        return mdiCheckboxMarkedCircle;
    } else if (props.item.status === UploadingStatus.Failed) {
        return mdiInformation;
    } else if (props.item.status === UploadingStatus.Cancelled) {
        return mdiCancel;
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
