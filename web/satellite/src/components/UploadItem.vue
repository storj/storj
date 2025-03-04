// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-list-item :title="item.Key" class="px-4" height="54" :link="props.item.status === UploadingStatus.Finished">
        <template #append>
            <v-tooltip v-if="isBetween5GBand30GB" location="top">
                <p>
                    For files over 5GB, we recommend using
                    <a href="https://storj.dev/dcs/api/uplink-cli" target="_blank" rel="noopener noreferrer">Storj CLI</a>
                </p>
                <p>
                    for better reliability than browser upload.
                </p>
                <template #activator="{ props: activatorProps }">
                    <v-icon
                        class="mr-2"
                        v-bind="activatorProps"
                        :icon="InfoIcon"
                        color="warning"
                    />
                </template>
            </v-tooltip>

            <v-tooltip :text="uploadStatus" location="left">
                <template #activator="{ props: activatorProps }">
                    <v-progress-circular
                        v-if="props.item.status === UploadingStatus.InProgress"
                        v-bind="activatorProps"
                        :indeterminate="!item.progress"
                        :size="20"
                        color="success"
                        :model-value="item.progress"
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
                        :icon="CircleX"
                        @click="cancelUpload"
                    />
                </template>
            </v-tooltip>
        </template>
    </v-list-item>
</template>

<script setup lang="ts">
import { computed, FunctionalComponent } from 'vue';
import { VListItem, VIcon, VProgressCircular, VTooltip } from 'vuetify/components';
import { Ban, CircleX, CircleCheck, Info, InfoIcon } from 'lucide-vue-next';

import {
    UploadingBrowserObject,
    UploadingStatus,
    useObjectBrowserStore,
} from '@/store/modules/objectBrowserStore';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

const obStore = useObjectBrowserStore();
const notify = useNotify();

const props = defineProps<{
    item: UploadingBrowserObject
}>();

const isBetween5GBand30GB = computed((): boolean => {
    const gb5 = 5 * 1024 * 1024 * 1024;
    const gb30 = 30 * 1024 * 1024 * 1024;
    return props.item.Size > gb5 && props.item.Size < gb30;
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

const icon = computed((): FunctionalComponent | undefined => {
    if (props.item.status === UploadingStatus.Finished) {
        return CircleCheck;
    } else if (props.item.status === UploadingStatus.Failed) {
        return Info;
    } else if (props.item.status === UploadingStatus.Cancelled) {
        return Ban;
    } else {
        return undefined;
    }
});

const iconColor = computed((): string => {
    if (props.item.status === UploadingStatus.Finished) {
        return 'success';
    } else if (props.item.status === UploadingStatus.Failed) {
        return 'error';
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
