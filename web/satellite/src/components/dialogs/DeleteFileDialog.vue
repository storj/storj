// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        max-width="420px"
        transition="fade-transition"
        :persistent="false"
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
                        <component :is="Trash2" :size="18" />
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold text-capitalize">Delete {{ fileTypes }}</v-card-title>
                <template #append>
                    <v-btn
                        :icon="X"
                        variant="text"
                        size="small"
                        color="default"
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
                        <v-btn variant="outlined" color="default" block @click="model = false">
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn color="error" variant="flat" block @click="onDeleteClick">
                            Delete
                        </v-btn>
                    </v-col>
                </v-row>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
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
import { Trash2, X } from 'lucide-vue-next';

import { useBucketsStore } from '@/store/modules/bucketsStore';
import { BrowserObject, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';

const props = defineProps<{
    files: BrowserObject[],
}>();

const emit = defineEmits<{
    'contentRemoved': [],
}>();

const model = defineModel<boolean>({ required: true });

const obStore = useObjectBrowserStore();
const bucketsStore = useBucketsStore();

const innerContent = ref<VCard | null>(null);

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

function onDeleteClick(): void {
    let deleteRequest: Promise<number>;
    if (props.files.length === 1) {
        deleteRequest = deleteSingleFile(props.files[0]);
    } else if (props.files.length > 1) {
        // multiple files selected in the file browser.
        deleteRequest = obStore.deleteSelected();
    } else return;
    obStore.handleDeleteObjectRequest(deleteRequest);
    model.value = false;
}

async function deleteSingleFile(file: BrowserObject): Promise<number> {
    if (isFolder.value) {
        return await obStore.deleteFolder(filePath.value ? filePath.value + '/' : '', file);
    } else {
        return await obStore.deleteObject(filePath.value ? filePath.value + '/' : '', file);
    }
}

watch(innerContent, comp => !comp && emit('contentRemoved'));
</script>
