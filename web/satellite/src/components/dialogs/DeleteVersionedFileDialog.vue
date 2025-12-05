// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        max-width="420px"
        transition="fade-transition"
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
                        <v-icon :size="18" :icon="Trash2" />
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold text-capitalize">Delete {{ fileTypes }}</v-card-title>
                <template #append>
                    <v-btn
                        :disabled="isLoading"
                        :icon="X"
                        variant="text"
                        size="small"
                        color="default"
                        @click="model = false"
                    />
                </template>
            </v-card-item>

            <v-divider />

            <v-card-item v-if="onlyFolders || onlyOneFolder" class="pa-6">
                <p class="mb-3 font-weight-bold">
                    You are about to delete {{ onlyOneFolder ? 'a' : '' }} folder{{ !onlyOneFolder ? 's' : '' }}
                    in a versioned bucket. This action will affect all objects and subfolders within
                    {{ !onlyOneFolder ? 'these' : 'this' }} folder{{ !onlyOneFolder ? 's' : '' }}.
                </p>

                <p class="mt-2">
                    <v-chip-group column direction="horizontal" class="mt-2">
                        <v-chip v-for="file of files" :key="file.path + file.Key" :title="file.path + file.Key">
                            {{ file.path + file.Key }}
                        </v-chip>
                        <v-chip v-if="onlyOneFolder && folderContentCount">
                            {{ folderContentCount }} file{{ folderContentCount === '1' ? '' : 's' }}
                        </v-chip>
                    </v-chip-group>
                </p>

                <v-radio-group v-model="selectedOption" hide-details>
                    <v-sheet
                        :class="[
                            'pa-1 border rounded-md mt-4 cursor-pointer',
                            selectedOption === DeleteOption.CreateMarker ? 'border-error border-opacity-100' : 'border'
                        ]"
                        @click="() => selectedOption = DeleteOption.CreateMarker"
                    >
                        <v-radio
                            :value="DeleteOption.CreateMarker"
                            color="error"
                            class="pl-1"
                            hide-details
                        >
                            <template #label>
                                <div>
                                    <p class="text-body-2 pt-4 pb-1">
                                        <strong>
                                            Create Delete Markers
                                        </strong>
                                    </p>
                                    <p class="text-body-2 pb-4">
                                        Create delete markers for all objects in {{ !onlyOneFolder ? 'these' : 'this' }}
                                        folder{{ !onlyOneFolder ? 's' : '' }}. Makes the objects appear deleted for most
                                        operations, but preserving previous versions. The folder{{ !onlyOneFolder ? 's' : '' }}
                                        and {{ !onlyOneFolder ? 'their' : 'its' }} contents can be restored later.
                                    </p>
                                </div>
                            </template>
                        </v-radio>
                    </v-sheet>
                    <v-sheet
                        :class="[
                            'pa-1 border rounded-md mt-4 cursor-pointer',
                            selectedOption === DeleteOption.DeleteAll ? 'border-error border-opacity-100' : 'border'
                        ]"
                        @click="() => selectedOption = DeleteOption.DeleteAll"
                    >
                        <v-radio
                            :value="DeleteOption.DeleteAll"
                            color="error"
                            class="pl-1"
                            hide-details
                        >
                            <template #label>
                                <div>
                                    <p class="text-body-2 pt-4 pb-1">
                                        <strong>
                                            Delete All Versions
                                        </strong>
                                    </p>
                                    <p class="text-body-2 pb-4">
                                        Permanently delete all versions of all objects in
                                        {{ !onlyOneFolder ? 'these' : 'this' }} folder{{ !onlyOneFolder ? 's' : '' }}
                                        and {{ !onlyOneFolder ? 'their' : 'its' }} subfolders. This action cannot be undone.
                                    </p>
                                </div>
                            </template>
                        </v-radio>
                    </v-sheet>
                </v-radio-group>
            </v-card-item>
            <v-card-item v-else-if="onlyFiles || onlyOneFile" class="pa-6">
                <p class="mb-3 font-weight-bold">
                    You are about to delete {{ onlyOneFile ? 'a' : '' }} object{{ !onlyOneFile ? 's' : '' }}
                    in a versioned bucket. Please choose how you want to handle this deletion:
                </p>

                <p class="mt-2">
                    <v-chip-group column direction="horizontal" class="mt-2">
                        <v-chip v-for="file of files" :key="file.path + file.Key" :title="file.path + file.Key">
                            {{ file.path + file.Key }}
                        </v-chip>
                        <v-chip v-if="onlyOneFile && fileVersionsCount">
                            {{ fileVersionsCount }} version{{ fileVersionsCount === '1' ? '' : 's' }}
                        </v-chip>
                    </v-chip-group>
                </p>

                <v-radio-group v-model="selectedOption" hide-details>
                    <v-sheet
                        :class="[
                            'pa-1 border rounded-md mt-4 cursor-pointer',
                            selectedOption === DeleteOption.CreateMarker ? 'border-error border-opacity-100' : 'border'
                        ]"
                        @click="() => selectedOption = DeleteOption.CreateMarker"
                    >
                        <v-radio
                            :value="DeleteOption.CreateMarker"
                            color="error"
                            class="pl-1"
                            hide-details
                        >
                            <template #label>
                                <div>
                                    <p class="text-body-2 pt-4 pb-1">
                                        <strong>
                                            Create Delete Marker{{ !onlyOneFile ? 's' : '' }}
                                        </strong>
                                    </p>
                                    <p class="text-body-2 pb-4">
                                        Make the object{{ !onlyOneFile ? 's' : '' }} appear deleted for most operations,
                                        but preserve previous versions. The object{{ !onlyOneFile ? 's' : '' }} can be restored later.
                                    </p>
                                </div>
                            </template>
                        </v-radio>
                    </v-sheet>
                    <v-sheet
                        :class="[
                            'pa-1 border rounded-md mt-4 cursor-pointer',
                            selectedOption === DeleteOption.DeleteAll ? 'border-error border-opacity-100' : 'border'
                        ]"
                        @click="() => selectedOption = DeleteOption.DeleteAll"
                    >
                        <v-radio
                            :value="DeleteOption.DeleteAll"
                            color="error"
                            class="pl-1"
                            hide-details
                        >
                            <template #label>
                                <div>
                                    <p class="text-body-2 pt-4 pb-1">
                                        <strong>
                                            Delete All Versions
                                        </strong>
                                    </p>
                                    <p class="text-body-2 pb-4">
                                        Permanently delete all versions of {{ !onlyOneFile ? 'these' : 'this' }}
                                        object{{ !onlyOneFile ? 's' : '' }}. This action cannot be undone.
                                    </p>
                                </div>
                            </template>
                        </v-radio>
                    </v-sheet>
                </v-radio-group>
            </v-card-item>
            <v-card-item v-else class="pa-6">
                <p class="mb-3 font-weight-bold">
                    You are about to delete objects and/or folders in a versioned bucket.
                    This action will affect all objects and subfolders within the folders.
                </p>
                <v-chip-group column direction="horizontal" class="mt-2">
                    <v-chip v-for="file of files" :key="file.path + file.Key" :title="file.path + file.Key">
                        {{ file.path + file.Key }}
                    </v-chip>
                </v-chip-group>

                <v-radio-group v-model="selectedOption" hide-details>
                    <v-sheet
                        :class="[
                            'pa-1 border rounded-md mt-4 cursor-pointer',
                            selectedOption === DeleteOption.CreateMarker ? 'border-error border-opacity-100' : 'border'
                        ]"
                        @click="() => selectedOption = DeleteOption.CreateMarker"
                    >
                        <v-radio
                            :value="DeleteOption.CreateMarker"
                            color="error"
                            class="pl-1"
                            hide-details
                        >
                            <template #label>
                                <div>
                                    <p class="text-body-2 pt-4 pb-1">
                                        <strong>
                                            Create Delete Markers
                                        </strong>
                                    </p>
                                    <p class="text-body-2 pb-4">
                                        Create delete markers for all selected objects and objects in selected folders.
                                        Makes the objects appear deleted for most operations, but preserving previous versions.
                                        The folder{{ folderCount > 1 ? 's': '' }} and {{ folderCount > 1 ? 'their' : 'its' }}
                                        contents can be restored later.
                                    </p>
                                </div>
                            </template>
                        </v-radio>
                    </v-sheet>
                    <v-sheet
                        :class="[
                            'pa-1 border rounded-md mt-4 cursor-pointer',
                            selectedOption === DeleteOption.DeleteAll ? 'border-error border-opacity-100' : 'border'
                        ]"
                        @click="() => selectedOption = DeleteOption.DeleteAll"
                    >
                        <v-radio
                            :value="DeleteOption.DeleteAll"
                            color="error"
                            class="pl-1"
                            hide-details
                        >
                            <template #label>
                                <div>
                                    <p class="text-body-2 pt-4 pb-1">
                                        <strong>
                                            Delete All Versions
                                        </strong>
                                    </p>
                                    <p class="text-body-2 pb-4">
                                        Permanently delete all versions of all selected objects and objects in selected folders
                                        and their subfolders. This action cannot be undone.
                                    </p>
                                </div>
                            </template>
                        </v-radio>
                    </v-sheet>
                </v-radio-group>
            </v-card-item>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
                            :disabled="isLoading"
                            variant="outlined"
                            color="default"
                            block
                            @click="model = false"
                        >
                            Cancel
                        </v-btn>
                    </v-col>
                    <v-col>
                        <v-btn
                            :loading="isLoading"
                            color="error"
                            variant="flat"
                            block
                            @click="onDeleteClick"
                        >
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
    VChipGroup,
    VRadio,
    VRadioGroup,
    VIcon,
} from 'vuetify/components';
import { Trash2, X } from 'lucide-vue-next';

import { useBucketsStore } from '@/store/modules/bucketsStore';
import { BrowserObject, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useLoading } from '@/composables/useLoading';

enum DeleteOption {
    CreateMarker = 'create_marker',
    DeleteAll = 'delete_all',
}

const props = defineProps<{
    files: BrowserObject[],
}>();

const emit = defineEmits<{
    'contentRemoved': [],
}>();

const model = defineModel<boolean>({ required: true });

const obStore = useObjectBrowserStore();
const bucketsStore = useBucketsStore();

const { withLoading, isLoading } = useLoading();

const fileVersionsCount = ref('');
const folderContentCount = ref('');
const selectedOption = ref<DeleteOption>(DeleteOption.CreateMarker);
const innerContent = ref<Component | null>(null);

const filePath = computed<string>(() => bucketsStore.state.fileComponentPath);

const onlyFiles = computed<boolean>(() => fileCount.value === props.files.length);
const onlyOneFile = computed<boolean>(() => onlyFiles.value && fileCount.value === 1);

const onlyFolders = computed<boolean>(() => folderCount.value === props.files.length);
const onlyOneFolder = computed<boolean>(() => onlyFolders.value && folderCount.value === 1);

const fileCount = computed<number>(() => props.files.filter(file => file.type === 'file' && !file.VersionId).length);

const folderCount = computed<number>(() => props.files.filter(file => file.type === 'folder').length);

/**
 * types of objects to be deleted.
 */
const fileTypes = computed<string>(() => {
    let result = '';

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
        deleteRequest = obStore.deleteSelected(selectedOption.value === DeleteOption.DeleteAll);
    } else return;
    obStore.handleDeleteObjectRequest(deleteRequest, selectedOption.value === DeleteOption.DeleteAll ? 'version' : 'file');
    model.value = false;
}

async function deleteSingleFile(file: BrowserObject): Promise<number> {
    if (selectedOption.value === DeleteOption.CreateMarker) {
        if (isFolder.value) {
            return await obStore.deleteFolder(filePath.value ? filePath.value + '/' : '', file);
        } else {
            return await obStore.deleteObject(filePath.value ? filePath.value + '/' : '', file);
        }
    }
    if (isFolder.value) {
        return await obStore.deleteFolderWithVersions(filePath.value ? filePath.value + '/' : '', file);
    } else {
        return await obStore.deleteObjectWithVersions(filePath.value ? filePath.value + '/' : '', file);
    }
}

function initDialog() {
    if (!onlyOneFile.value && !onlyOneFolder.value) {
        return;
    }
    withLoading(async () => {
        try {
            if (onlyOneFolder.value) {
                folderContentCount.value = (await obStore.countVersions(props.files[0].path + props.files[0].Key + '/'));
            } else {
                fileVersionsCount.value = (await obStore.countVersions(props.files[0].path + props.files[0].Key));
            }
        } catch { /* empty */ }
    });
}

watch(innerContent, comp => {
    if (!comp) {
        emit('contentRemoved');
        fileVersionsCount.value = '';
        folderContentCount.value = '';
        selectedOption.value = DeleteOption.CreateMarker;
        return;
    }
    initDialog();
});
</script>
