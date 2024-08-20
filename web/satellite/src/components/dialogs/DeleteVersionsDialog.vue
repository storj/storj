// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        max-width="500px"
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
                <v-card-title class="font-weight-bold text-capitalize">
                    Delete Version{{ props.files.length > 1 ? 's' : '' }}
                </v-card-title>
                <template #append>
                    <v-btn
                        icon="$close"
                        variant="text"
                        size="small"
                        color="default"
                        @click="model = false"
                    />
                </template>
            </v-card-item>

            <v-divider />

            <v-card-item class="pa-6">
                <p class="mb-3 font-weight-bold">
                    You are about to delete the following file version{{ props.files.length > 1 ? 's' : '' }}:
                </p>
                <v-treeview
                    item-value="title"
                    :open-all="files.length < 10"
                    :activatable="false"
                    :selectable="false"
                    :tile="true"
                    :items="groupedFiles"
                >
                    <template #title="{ item }">
                        <template v-if="!item.children?.length" title>
                            {{ item.title }}
                        </template>
                        <v-chip v-else title>
                            {{ item.title }}
                        </v-chip>
                    </template>
                </v-treeview>
            </v-card-item>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col>
                        <v-btn
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
import { Component, computed, ref, watch } from 'vue';
import {
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardTitle,
    VChip,
    VCol,
    VDialog,
    VDivider,
    VIcon,
    VRow,
    VSheet,
} from 'vuetify/components';
import { VTreeview } from 'vuetify/labs/VTreeview';
import { Trash2 } from 'lucide-vue-next';

import { useBucketsStore } from '@/store/modules/bucketsStore';
import { BrowserObject, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';

interface TreeItem {
    title: string;
    children?: TreeItem[];
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

const innerContent = ref<Component | null>(null);

const filePath = computed<string>(() => bucketsStore.state.fileComponentPath);

const groupedFiles = computed(() => {
    const uniqueFiles = new Map<string, TreeItem>(props.files.map(file => [file.path + file.Key, { title: file.path + file.Key, children: [] }]));

    for (const file of props.files) {
        const existingFile = uniqueFiles.get(file.path + file.Key);
        existingFile?.children?.push({ title: `Version ID: ${file.VersionId ?? ''}` });
    }

    return [...uniqueFiles.values()];
});

function onDeleteClick(): void {
    let deleteRequest: Promise<void>;
    if (props.files.length === 1) {
        deleteRequest = obStore.deleteObject(filePath.value ? filePath.value + '/' : '', props.files[0], false, false);
    } else if (props.files.length > 1) {
    // multiple files selected in the file browser.
        deleteRequest = obStore.deleteSelected();
    } else return;
    obStore.handleDeleteObjectRequest(props.files.length, `Version${ props.files.length > 1 ? 's' : '' }`, deleteRequest);
    model.value = false;
}

watch(innerContent, comp => !comp && emit('contentRemoved'));
</script>
