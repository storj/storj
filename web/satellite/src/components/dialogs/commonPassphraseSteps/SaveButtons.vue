// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-col cols="6">
        <v-btn
            variant="outlined"
            block
            :color="justDownloaded ? 'success' : 'default'"
            :prepend-icon="justDownloaded ? Check : DownloadIcon"
            @click="onDownload"
        >
            {{ justDownloaded ? 'Downloaded' : (items.length > 1 ? 'Download All' : 'Download') }}
        </v-btn>
    </v-col>
    <v-col cols="6">
        <v-btn
            variant="outlined"
            block
            :color="justCopied ? 'success' : 'default'"
            :prepend-icon="justCopied ? Check : Copy"
            @click="onCopy"
        >
            {{ justCopied ? 'Copied' : (items.length > 1 ? 'Copy All' : 'Copy') }}
        </v-btn>
    </v-col>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import { VCol, VBtn } from 'vuetify/components';
import { Check, Copy, DownloadIcon } from 'lucide-vue-next';

import { SaveButtonsItem } from '@/types/common';
import { Download } from '@/utils/download';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';

const analyticsStore = useAnalyticsStore();
const projectStore = useProjectsStore();
const configStore = useConfigStore();

const props = defineProps<{
    items: SaveButtonsItem[];
    name: string;
    type: string;
}>();

const successDuration = 2000;
const copiedTimeout = ref<ReturnType<typeof setTimeout> | null>(null);
const downloadedTimeout = ref<ReturnType<typeof setTimeout> | null>(null);

const justCopied = computed<boolean>(() => copiedTimeout.value !== null);
const justDownloaded = computed<boolean>(() => downloadedTimeout.value !== null);

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
        `${configStore.brandName}-${props.type}-${props.name}-${new Date().toISOString()}.txt`,
    );
    analyticsStore.eventTriggered(AnalyticsEvent.DOWNLOAD_TXT_CLICKED, { project_id: projectStore.state.selectedProject.id });

    if (downloadedTimeout.value) clearTimeout(downloadedTimeout.value);
    downloadedTimeout.value = setTimeout(() => {
        downloadedTimeout.value = null;
    }, successDuration);
}
</script>
