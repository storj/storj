// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="text-no-wrap" :class="alignClass">
        <v-tooltip v-if="deleting" location="top" text="Deleting file">
            <template #activator="{ props }">
                <v-progress-circular class="text-right" width="2" size="22" color="error" indeterminate v-bind="props" />
            </template>
        </v-tooltip>
        <template v-else>
            <v-btn
                v-if="file.type !== 'folder'"
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
                v-if="!isVersion"
                variant="text"
                color="default"
                size="small"
                class="mr-1 text-caption"
                density="comfortable"
                title="Share"
                icon
                @click="emit('shareClick')"
            >
                <component :is="Share" :size="17" />
            </v-btn>

            <v-btn
            variant="text"
            color="default"
                size="small"
                class="mr-1 text-caption"
                density="comfortable"
                title="More Actions"
                icon
            >
                <v-icon :icon="Ellipsis" />
                <v-menu activator="parent">
                    <v-list class="pa-1">
                        <template v-if="file.type !== 'folder'">
                            <v-list-item density="comfortable" link @click="emit('previewClick')">
                                <template #prepend>
                                    <component :is="ZoomIn" :size="18" />
                                </template>
                                <v-list-item-title class="ml-3 text-body-2 font-weight-medium">
                                    Preview
                                </v-list-item-title>
                            </v-list-item>

                            <v-list-item
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
                            <v-list-item v-if="isVersion" density="comfortable" link @click="emit('restoreObjectClick')">
                                <template #prepend>
                                    <component :is="Redo2" :size="18" />
                                </template>
                                <v-list-item-title class="ml-3 text-body-2 font-weight-medium">
                                    Restore
                                </v-list-item-title>
                            </v-list-item>
                        </template>

                        <v-list-item v-if="!isVersion" density="comfortable" link @click="emit('shareClick')">
                            <template #prepend>
                                <component :is="Share" :size="18" />
                            </template>
                            <v-list-item-title class="ml-3 text-body-2 font-weight-medium">
                                Share
                            </v-list-item-title>
                        </v-list-item>

                        <v-divider class="my-1" />

                        <v-list-item density="comfortable" link base-color="error" @click="emit('deleteFileClick')">
                            <template #prepend>
                                <component :is="Trash2" :size="18" />
                            </template>
                            <v-list-item-title class="ml-3 text-body-2 font-weight-medium">
                                Delete
                            </v-list-item-title>
                        </v-list-item>
                    </v-list>
                </v-menu>
            </v-btn>
        </template>
    </div>
</template>

<script setup lang="ts">
import { ref, h, computed } from 'vue';
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
import { Ellipsis, Share, Download, ZoomIn, Trash2, Redo2 } from 'lucide-vue-next';

import { BrowserObject, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { ProjectLimits } from '@/types/projects';
import { useProjectsStore } from '@/store/modules/projectsStore';

const obStore = useObjectBrowserStore();
const projectsStore = useProjectsStore();

const notify = useNotify();

const props = defineProps<{
    isVersion?: boolean;
    file: BrowserObject;
    align: 'left' | 'right';
    deleting?: boolean;
}>();

const emit = defineEmits<{
    previewClick: [];
    deleteFileClick: [];
    shareClick: [];
    restoreObjectClick: [];
}>();

const isDownloading = ref<boolean>(false);

const alignClass = computed<string>(() => {
    return 'text-' + props.align;
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
    if (disableDownload.value) {
        notify.error('Bandwidth limit exceeded, can not download this file.');
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
        error.message = `Error downloading file. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.FILE_BROWSER_ENTRY);
    }
    isDownloading.value = false;
}

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
