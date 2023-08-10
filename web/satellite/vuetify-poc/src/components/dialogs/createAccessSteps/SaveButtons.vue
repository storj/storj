// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-col cols="6">
        <v-btn
            variant="outlined"
            size="small"
            block
            :color="justCopied ? 'success' : 'default'"
            :prepend-icon="justCopied ? 'mdi-check' : 'mdi-content-copy'"
            @click="onCopy"
        >
            {{ justCopied ? 'Copied' : (items.length > 1 ? 'Copy All' : 'Copy') }}
        </v-btn>
    </v-col>
    <v-col cols="6">
        <v-btn
            variant="outlined"
            size="small"
            block
            :color="justDownloaded ? 'success' : 'default'"
            :prepend-icon="justDownloaded ? 'mdi-check' : 'mdi-tray-arrow-down'"
            @click="onDownload"
        >
            {{ justDownloaded ? 'Downloaded' : (items.length > 1 ? 'Download All' : 'Download') }}
        </v-btn>
    </v-col>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import { VCol, VBtn } from 'vuetify/components';

import { SaveButtonsItem } from '@poc/types/createAccessGrant';
import { Download } from '@/utils/download';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';

const props = defineProps<{
    items: SaveButtonsItem[];
    accessName: string;
    fileNameBase: string;
}>();

const successDuration = 2000;
const copiedTimeout = ref<ReturnType<typeof setTimeout> | null>(null);
const downloadedTimeout = ref<ReturnType<typeof setTimeout> | null>(null);

const justCopied = computed<boolean>(() => copiedTimeout.value !== null);
const justDownloaded = computed<boolean>(() => downloadedTimeout.value !== null);

const analyticsStore = useAnalyticsStore();

/**
 * Saves items to clipboard.
 */
function onCopy(): void {
    navigator.clipboard.writeText(props.items.map(item => typeof item === 'string' ? item : item.value).join(' '));
    analyticsStore.eventTriggered(AnalyticsEvent.COPY_TO_CLIPBOARD_CLICKED);

    if (copiedTimeout.value) clearTimeout(copiedTimeout.value);
    copiedTimeout.value = setTimeout(() => {
        copiedTimeout.value = null;
    }, successDuration);
}

/**
 * Downloads items into .txt file.
 */
function onDownload(): void {
    Download.file(
        props.items.map(item => typeof item === 'string' ? item : `${item.name}:\n${item.value}`).join('\n\n'),
        `Storj-${props.fileNameBase}-${props.accessName}-${new Date().toISOString()}.txt`,
    );
    analyticsStore.eventTriggered(AnalyticsEvent.DOWNLOAD_TXT_CLICKED);

    if (downloadedTimeout.value) clearTimeout(downloadedTimeout.value);
    downloadedTimeout.value = setTimeout(() => {
        downloadedTimeout.value = null;
    }, successDuration);
}
</script>
