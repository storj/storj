// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="text-no-wrap text-right">
        <v-btn
            v-if="file.type !== 'folder'"
            variant="outlined"
            color="default"
            size="small"
            class="mr-1 text-caption"
            density="comfortable"
            icon
            :loading="isDownloading"
            @click="onDownloadClick"
        >
            <icon-download />
            <v-tooltip activator="parent" location="start">Download</v-tooltip>
        </v-btn>

        <v-btn
            variant="outlined"
            color="default"
            size="small"
            class="mr-1 text-caption"
            density="comfortable"
            icon
        >
            <v-icon icon="mdi-dots-horizontal" />
            <v-menu activator="parent">
                <v-list class="pa-2">
                    <template v-if="file.type !== 'folder'">
                        <v-list-item density="comfortable" link rounded="lg">
                            <template #prepend>
                                <icon-preview />
                            </template>
                            <v-list-item-title class="pl-2 text-body-2 font-weight-medium">
                                Preview
                            </v-list-item-title>
                        </v-list-item>

                        <v-list-item
                            density="comfortable"
                            :link="!isDownloading"
                            rounded="lg"
                            @click="onDownloadClick"
                        >
                            <template #prepend>
                                <icon-download />
                            </template>
                            <v-fade-transition>
                                <v-list-item-title v-show="!isDownloading" class="pl-2 text-body-2 font-weight-medium">
                                    Download
                                </v-list-item-title>
                            </v-fade-transition>
                            <div v-if="isDownloading" class="browser_actions_menu__loader">
                                <v-progress-circular indeterminate size="23" width="2" />
                            </div>
                        </v-list-item>
                    </template>

                    <v-list-item density="comfortable" link rounded="lg">
                        <template #prepend>
                            <icon-share bold />
                        </template>
                        <v-list-item-title class="pl-2 text-body-2 font-weight-medium">
                            Share
                        </v-list-item-title>
                    </v-list-item>

                    <v-divider class="my-2" />

                    <v-list-item density="comfortable" link rounded="lg" base-color="error" @click="onDeleteClick">
                        <template #prepend>
                            <icon-trash bold />
                        </template>
                        <v-list-item-title class="pl-2 text-body-2 font-weight-medium">
                            Delete
                        </v-list-item-title>
                    </v-list-item>
                </v-list>
            </v-menu>
        </v-btn>
    </div>

    <v-overlay
        v-model="isDeleting"
        scrim="surface"
        contained
        persistent
        no-click-animation
        class="align-center justify-center browser-table__loader-overlay"
    >
        <div class="d-flex align-center">
            <v-progress-circular size="23" width="2" color="error" indeterminate />
            <p class="ml-3 text-subtitle-1 font-weight-medium text-error">Deleting...</p>
        </div>
    </v-overlay>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import {
    VMenu,
    VList,
    VListItem,
    VListItemTitle,
    VDivider,
    VProgressCircular,
    VFadeTransition,
    VIcon,
    VBtn,
    VTooltip,
    VOverlay,
} from 'vuetify/components';

import { BrowserObject, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useBucketsStore } from '@/store/modules/bucketsStore';

import IconDownload from '@poc/components/icons/IconDownload.vue';
import IconShare from '@poc/components/icons/IconShare.vue';
import IconPreview from '@poc/components/icons/IconPreview.vue';
import IconTrash from '@poc/components/icons/IconTrash.vue';

const obStore = useObjectBrowserStore();
const bucketsStore = useBucketsStore();
const notify = useNotify();

const props = defineProps<{
    file: BrowserObject;
}>();

const emit = defineEmits<{
    deleteFolderClick: [];
}>();

const isDownloading = ref<boolean>(false);

const filePath = computed<string>(() => bucketsStore.state.fileComponentPath);

const isDeleting = computed((): boolean => {
    return obStore.state.filesToBeDeleted.some(
        file => file.Key === props.file.Key && file.path === props.file.path,
    );
});

async function onDownloadClick(): Promise<void> {
    isDownloading.value = true;
    await obStore.download(props.file).catch((err: Error) => {
        err.message = `Error downloading file. ${err.message}`;
        notify.notifyError(err, AnalyticsErrorEventSource.FILE_BROWSER_ENTRY);
    });
    isDownloading.value = false;
}

async function onDeleteClick(): Promise<void> {
    if (props.file.type === 'folder') {
        emit('deleteFolderClick');
        return;
    }

    obStore.addFileToBeDeleted(props.file);
    await obStore.deleteObject(filePath.value ? filePath.value + '/' : '', props.file).catch((err: Error) => {
        err.message = `Error deleting ${props.file.type}. ${err.message}`;
        notify.notifyError(err, AnalyticsErrorEventSource.FILE_BROWSER_ENTRY);
    });
    obStore.removeFileFromToBeDeleted(props.file);
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
