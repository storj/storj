// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="410px"
        transition="fade-transition"
        :persistent="isLoading"
    >
        <v-card ref="innerContent" rounded="xlg">
            <v-card-item class="pa-5 pl-7">
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

            <div class="pa-7">
                The following {{ fileTypes }}<template v-if="isFolder">, and all contained data</template> will be deleted. This action cannot be undone.
                <br><br>
                <p v-for="file of files" :key="file.path + file.Key" class="font-weight-bold">{{ file.path + file.Key }}</p>
            </div>

            <v-divider />

            <v-card-actions class="pa-7">
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
} from 'vuetify/components';

import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/utils/hooks';
import { BrowserObject, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';

import IconTrash from '@poc/components/icons/IconTrash.vue';

const props = defineProps<{
    modelValue: boolean,
    files: BrowserObject[],
}>();

const emit = defineEmits<{
    'update:modelValue': [value: boolean],
    'contentRemoved': [],
    'filesDeleted': [],
}>();

const model = computed<boolean>({
    get: () => props.modelValue,
    set: value => emit('update:modelValue', value),
});

const obStore = useObjectBrowserStore();
const bucketsStore = useBucketsStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const innerContent = ref<Component | null>(null);

const filePath = computed<string>(() => bucketsStore.state.fileComponentPath);

const fileCount = computed<number>(() => props.files.filter(file => file.type === 'file').length);

const folderCount = computed<number>(() => props.files.filter(file => file.type === 'folder').length);

/**
 * types of objects to be deleted.
 */
const fileTypes = computed<string>(() => {
    if (fileCount.value > 0 && folderCount.value > 0) {
        return `file${fileCount.value > 1 ? 's' : ''} and folder${folderCount.value > 1 ? 's' : ''}`;
    } else if (folderCount.value > 0) {
        return `folder${folderCount.value > 1 ? 's' : ''}`;
    } else {
        return `file${fileCount.value > 1 ? 's' : ''}`;
    }
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
        notify.success(`${fileCount.value + folderCount.value} ${fileTypes.value} deleted`);
        model.value = false;
    });
}

async function deleteSingleFile(file: BrowserObject): Promise<void> {
    if (isFolder.value) {
        await obStore.deleteFolder(file, filePath.value ? filePath.value + '/' : '');
    } else {
        await obStore.deleteObject(filePath.value ? filePath.value + '/' : '', file);
    }
}

watch(innerContent, comp => !comp && emit('contentRemoved'));
</script>
