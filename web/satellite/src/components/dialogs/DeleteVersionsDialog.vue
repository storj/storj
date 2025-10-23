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
                <v-card-title class="font-weight-bold">
                    Delete Object{{ props.files.length > 1 ? 's' : '' }} {{ !!foldersCount ? 'and' : '' }} {{ foldersCount > 0 ? foldersCount > 1 ? 'Folders' : 'Folder' : '' }}
                </v-card-title>
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

            <v-card-item class="pa-6">
                <p class="mb-3 font-weight-bold">
                    You are about to delete the following objects:
                    <template v-if="subtitles.size - foldersCount > 0">
                        Some versions are locked and cannot be deleted.
                    </template>
                </p>

                <v-treeview
                    class="ml-n6"
                    item-value="title"
                    :open-all="files.length < 10"
                    :activatable="false"
                    :selectable="false"
                    :items="groupedFiles"
                    open-on-click
                    tile
                >
                    <template #prepend="{ item }">
                        <img :src="icons.get(item.title)" alt="icon" class="mr-3">
                    </template>
                    <template #item="{ props: itemProps }">
                        <v-list-item :title="itemProps.title" :class="{ 'text-medium-emphasis': subtitles.has(itemProps.title) }">
                            <v-list-item-subtitle v-if="subtitles.has(itemProps.title)" class="text-caption">
                                {{ subtitles.get(itemProps.title) }}
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
import { Trash2, X } from 'lucide-vue-next';

import { useBucketsStore } from '@/store/modules/bucketsStore';
import { BrowserObject, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { Time } from '@/utils/time';
import { EXTENSION_INFOS, FILE_INFO, FOLDER_INFO } from '@/types/browser';
import { ObjectLockStatus } from '@/types/objectLock';

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

const subtitles = ref(new Map<string, string>());

const filePath = computed<string>(() => bucketsStore.state.fileComponentPath);

const foldersCount = computed(() => props.files.filter(f => f.type === 'folder').length);

const groupedFiles = computed(() => {
    const uniqueFiles = new Map<string, TreeItem>(
        props.files.map(file => [
            `${file.path + file.Key}${file.type === 'folder' ? '/' : ''}`,
            {
                title: `${file.path + file.Key}${file.type === 'folder' ? '/' : ''}`,
                children: [],
            },
        ]),
    );

    for (const file of props.files) {
        const fileName = `${file.path + file.Key}${file.type === 'folder' ? '/' : ''}`;
        const existingFile = uniqueFiles.get(fileName);
        if (file.type === 'folder') {
            if (subtitles.value.has(fileName)) {
                existingFile?.children?.push({ title: subtitles.value.get(fileName) ?? 'May contain objects' });
            }
            continue;
        }
        existingFile?.children?.push({ title: `Version ID: ${file.VersionId ?? ''}` });
    }

    return [...uniqueFiles.values()];
});

const icons = computed<Map<string, string>>(() => {
    const iconMap = new Map<string, string>();
    for (const file of groupedFiles.value) {
        if (file.title.slice(-1) === '/') {
            iconMap.set(file.title, FOLDER_INFO.icon);
            continue;
        }

        const dotIdx = file.title.lastIndexOf('.');
        const ext = dotIdx === -1 ? '' : file.title.slice(dotIdx + 1).toLowerCase();
        for (const [exts, info] of EXTENSION_INFOS.entries()) {
            if (exts.indexOf(ext) !== -1) iconMap.set(file.title, info.icon);
        }
        if (!iconMap.has(file.title)) iconMap.set(file.title, FILE_INFO.icon);
    }
    return iconMap;
});

function onDeleteClick(): void {
    let deleteRequest: Promise<number>;
    if (props.files.length === 1) {
        deleteRequest = deleteSingleObject(props.files[0]);
    } else if (props.files.length > 1) {
        // multiple files selected in the file browser.
        deleteRequest = obStore.deleteSelectedVersions();
    } else return;
    obStore.handleDeleteObjectRequest(deleteRequest);
    model.value = false;
}

async function deleteSingleObject(file: BrowserObject): Promise<number> {
    if (file.type === 'folder') {
        return await obStore.deleteFolderWithVersions(filePath.value ? filePath.value + '/' : '', file);
    } else {
        return await obStore.deleteObject(filePath.value ? filePath.value + '/' : '', file);
    }
}

function formatDate(date?: Date): string {
    if (!date) {
        return '-';
    }
    return Time.formattedDate(date, { day: 'numeric', month: 'numeric', year: 'numeric' });
}

async function checkLockedVersions() {
    interface VersionsData { id: string, lockStatus?: ObjectLockStatus, contentCountTxt?: string }
    const results = await Promise.allSettled<Awaited<VersionsData>>(props.files.map(async file => {
        if (file.type === 'folder') {
            const count = await obStore.countVersions(file.path + file.Key + '/', 50);
            return {
                id: `${file.path + file.Key}/`,
                contentCountTxt: count,
            };
        }
        const lockStatus = await obStore.getObjectLockStatus(file);
        return { id: file.VersionId ?? '', lockStatus };
    }));
    for (const result of results) {
        if (result.status !== 'fulfilled') {
            continue;
        }
        if (result.value.contentCountTxt) {
            subtitles.value.set(result.value.id, `Contains ${result.value.contentCountTxt} object(s)`);
            continue;
        }
        const id = result.value.id;
        const lockStatus = result.value.lockStatus;
        if (!lockStatus?.retention.active && !lockStatus?.legalHold) {
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
        subtitles.value.set(`Version ID: ${id}`, `Locked until: ${untilText}`);
    }
}

watch(innerContent, comp => {
    if (!comp) {
        emit('contentRemoved');
        subtitles.value.clear();
        return;
    }
    checkLockedVersions();
});
</script>
