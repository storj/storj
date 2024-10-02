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
                <p v-if="lockedVersions.size === 0" class="mb-3 font-weight-bold">
                    You are about to delete the following object version{{ props.files.length > 1 ? 's' : '' }}:
                </p>
                <p v-else class="mb-3 font-weight-bold">
                    Some versions are locked and cannot be deleted.
                </p>
                <v-treeview
                    item-value="title"
                    :open-all="files.length < 10"
                    :activatable="false"
                    :selectable="false"
                    :items="groupedFiles"
                    tile
                >
                    <template #item="{ props }">
                        <v-list-item :title="props.title" :class="{ 'text-medium-emphasis': lockedVersions.has(props.title) }">
                            <v-list-item-subtitle v-if="lockedVersions.has(props.title)" class="text-caption">
                                {{ lockedVersions.get(props.title) }}
                            </v-list-item-subtitle>
                        </v-list-item>
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
    VCol,
    VDialog,
    VDivider,
    VIcon,
    VListItem,
    VListItemSubtitle,
    VRow,
    VSheet,
} from 'vuetify/components';
import { VTreeview } from 'vuetify/labs/VTreeview';
import { Trash2 } from 'lucide-vue-next';

import { useBucketsStore } from '@/store/modules/bucketsStore';
import { BrowserObject, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { Time } from '@/utils/time';

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

const lockedVersions = ref(new Map<string, string>());

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
    let deleteRequest: Promise<number>;
    if (props.files.length === 1) {
        deleteRequest = obStore.deleteObject(filePath.value ? filePath.value + '/' : '', props.files[0]);
    } else if (props.files.length > 1) {
        // multiple files selected in the file browser.
        deleteRequest = obStore.deleteSelected();
    } else return;
    obStore.handleDeleteObjectRequest(deleteRequest, 'version');
    model.value = false;
}

function formatDate(date?: Date): string {
    if (!date) {
        return '-';
    }
    return Time.formattedDate(date, { day: 'numeric', month: 'numeric', year: 'numeric' });
}

async function checkLockedVersions() {
    const results = await Promise.allSettled(props.files.map(async file => {
        const lockStatus = await obStore.getObjectLockStatus(file);
        return { id: file.VersionId ?? '', lockStatus };
    }));
    for (const result of results) {
        if (result.status !== 'fulfilled') {
            continue;
        }
        const id = result.value.id;
        const lockStatus = result.value.lockStatus;
        if (!lockStatus.retention.active && !lockStatus.legalHold) {
            continue;
        }
        let untilText = '';
        if (lockStatus.retention.active) {
            untilText = formatDate(lockStatus.retention.retainUntil);
        }
        if (lockStatus.legalHold) {
            if (untilText) {
                untilText += ' and ';
            }
            untilText += 'Legal Hold removed';
        }
        lockedVersions.value.set(`Version ID: ${id}`, `Locked until: ${untilText}`);
    }
}

watch(innerContent, comp => {
    if (!comp) {
        emit('contentRemoved');
        lockedVersions.value.clear();
        return;
    }
    checkLockedVersions();
});
</script>
