// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="410px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card ref="innerContent">
            <v-card-item class="pa-6">
                <template #prepend>
                    <v-sheet
                        class="border-sm d-flex justify-center align-center"
                        width="40"
                        height="40"
                        rounded="lg"
                    >
                        <icon-trash />
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold text-capitalize">Delete {{ fileTypes }}</v-card-title>
                <template #append>
                    <v-btn
                        icon="$close"
                        variant="text"
                        size="small"
                        color="default"
                        :disabled="isLoading"
                        @click="model = false"
                    />
                </template>
            </v-card-item>

            <v-divider />

            <div class="pa-6">
                <p class="mb-3">The following {{ fileTypes }}<template v-if="isFolder">, and all contained data</template> will be deleted. This action cannot be undone.</p>
                <p v-for="file of files" :key="file.path + file.Key" class="mt-2">
                    <v-chip :title="file.path + file.Key" class="font-weight-bold text-wrap h-100 py-2">
                        {{ file.path + file.Key }}
                    </v-chip>
                </p>
            </div>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn variant="outlined" color="default" block :disabled="isLoading" @click="model = false">
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn color="error" variant="flat" block :loading="isLoading" @click="onDeleteClick">
                            Delete
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref, Component, watch } from 'vue';
import {
    VDialog,
    VCard,
    VCardItem,
    VSheet,
    VCardTitle,
    VDivider,
    VCardActions,
    VRow,
    VCol,
    VBtn,
    VChip,
} from 'vuetify/components';

import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/utils/hooks';
import { BrowserObject, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';

import IconTrash from '@/components/icons/IconTrash.vue';

const props = defineProps<{
    files: BrowserObject[],
}>();

const emit = defineEmits<{
    'contentRemoved': [],
    'filesDeleted': [],
}>();

const model = defineModel<boolean>({ required: true });

const obStore = useObjectBrowserStore();
const bucketsStore = useBucketsStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const innerContent = ref<Component | null>(null);

const filePath = computed<string>(() => bucketsStore.state.fileComponentPath);

const fileCount = computed<number>(() => props.files.filter(file => file.type === 'file' && !file.VersionId).length);

const folderCount = computed<number>(() => props.files.filter(file => file.type === 'folder').length);

const versionsCount = computed<number>(() => props.files.filter(file => !!file.VersionId).length);

/**
 * types of objects to be deleted.
 */
const fileTypes = computed<string>(() => {
    let result = '';

    if (versionsCount.value > 0) {
        result += `version${versionsCount.value > 1 ? 's' : ''}`;
    }

    if (fileCount.value > 0) {
        result += `${result ? ' and' : ''} file${fileCount.value > 1 ? 's' : ''}`;
    }

    if (folderCount.value > 0) {
        result += `${result ? ' and' : ''} folder${folderCount.value > 1 ? 's' : ''}`;
    }

    return result;
});

const isFolder = computed<boolean>(() => {
    return folderCount.value > 0;
});

async function onDeleteClick(): Promise<void> {
    await withLoading(async () => {
        try {
            if (props.files.length === 1) {
                await deleteSingleFile(props.files[0]);
            } else if (props.files.length > 1) {
                // multiple files selected in the file browser.
                await obStore.deleteSelected();
            } else return;
        } catch (error) {
            error.message = `Error deleting ${fileTypes.value}. ${error.message}`;
            notify.notifyError(error, AnalyticsErrorEventSource.FILE_BROWSER);
            return;
        }

        emit('filesDeleted');
        notify.success(`${fileCount.value + folderCount.value + versionsCount.value} ${fileTypes.value} deleted`);
        model.value = false;
    });
}

async function deleteSingleFile(file: BrowserObject): Promise<void> {
    if (isFolder.value) {
        await obStore.deleteFolder(file, filePath.value ? filePath.value + '/' : '', false);
    } else {
        await obStore.deleteObject(filePath.value ? filePath.value + '/' : '', file, false, false);
    }
}

watch(innerContent, comp => !comp && emit('contentRemoved'));
</script>
