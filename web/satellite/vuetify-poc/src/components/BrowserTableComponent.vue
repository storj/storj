// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card variant="flat" :border="true" rounded="xlg">
        <v-text-field
            v-model="search"
            label="Search"
            prepend-inner-icon="mdi-magnify"
            single-line
            variant="solo-filled"
            flat
            hide-details
            clearable
            density="comfortable"
            rounded="lg"
            class="mx-2 mt-2"
        />

        <v-data-table-server
            v-model="selected"
            v-model:options="options"
            :sort-by="sortBy"
            :headers="headers"
            :items="tableFiles"
            :search="search"
            class="elevation-1"
            :item-value="(item: BrowserObjectWrapper) => item.browserObject.Key"
            show-select
            hover
            must-sort
            :loading="isFetching || loading"
            :items-length="allFiles.length"
        >
            <template #item.name="{ item }: ItemSlotProps">
                <v-btn
                    class="rounded-lg w-100 pl-1 pr-4 justify-start font-weight-bold"
                    variant="text"
                    height="40"
                    color="default"
                    block
                    @click="onFileClick(item.raw.browserObject)"
                >
                    <img :src="item.raw.typeInfo.icon" :alt="item.raw.typeInfo.title + 'icon'" class="mr-3">
                    {{ item.raw.browserObject.Key }}
                </v-btn>
            </template>

            <template #item.type="{ item }: ItemSlotProps">
                {{ item.raw.typeInfo.title }}
            </template>

            <template #item.size="{ item }: ItemSlotProps">
                {{ getFormattedSize(item.raw.browserObject) }}
            </template>

            <template #item.date="{ item }: ItemSlotProps">
                {{ getFormattedDate(item.raw.browserObject) }}
            </template>

            <template #item.actions="{ item }: ItemSlotProps">
                <browser-row-actions :file="item.raw.browserObject" />
            </template>
        </v-data-table-server>

        <file-preview-dialog v-model="previewDialog" />
    </v-card>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { useRouter } from 'vue-router';
import {
    VCard,
    VTextField,
    VBtn,
} from 'vuetify/components';
import { VDataTableServer } from 'vuetify/labs/components';

import { BrowserObject, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useNotify } from '@/utils/hooks';
import { SHORT_MONTHS_NAMES } from '@/utils/constants/date';
import { Size } from '@/utils/bytesSize';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useBucketsStore } from '@/store/modules/bucketsStore';

import BrowserRowActions from '@poc/components/BrowserRowActions.vue';
import FilePreviewDialog from '@poc/components/dialogs/FilePreviewDialog.vue';

import folderIcon from '@poc/assets/icon-folder-tonal.svg';
import pdfIcon from '@poc/assets/icon-pdf-tonal.svg';
import imageIcon from '@poc/assets/icon-image-tonal.svg';
import videoIcon from '@poc/assets/icon-video-tonal.svg';
import audioIcon from '@poc/assets/icon-audio-tonal.svg';
import textIcon from '@poc/assets/icon-text-tonal.svg';
import zipIcon from '@poc/assets/icon-zip-tonal.svg';
import spreadsheetIcon from '@poc/assets/icon-spreadsheet-tonal.svg';
import fileIcon from '@poc/assets/icon-file-tonal.svg';

type SortKey = 'name' | 'type' | 'size' | 'date';

type TableOptions = {
    page: number;
    itemsPerPage: number;
    sortBy: {
        key: SortKey;
        order: 'asc' | 'desc';
    }[];
};

type BrowserObjectTypeInfo = {
    title: string;
    icon: string;
};

/**
 * Contains extra information to aid in the display, filtering, and sorting of browser objects.
 */
type BrowserObjectWrapper = {
    browserObject: BrowserObject;
    typeInfo: BrowserObjectTypeInfo;
    lowerName: string;
    ext: string;
};

type ItemSlotProps = { item: { raw: BrowserObjectWrapper } };

const props = defineProps<{
    forceEmpty?: boolean;
    loading?: boolean;
}>();

const obStore = useObjectBrowserStore();
const projectsStore = useProjectsStore();
const bucketsStore = useBucketsStore();

const notify = useNotify();
const router = useRouter();

const isFetching = ref<boolean>(false);
const search = ref<string>('');
const selected = ref([]);
const previewDialog = ref<boolean>(false);
const options = ref<TableOptions>();

const sortBy = [{ key: 'name', order: 'asc' }];
const headers = [
    {
        title: 'Name',
        align: 'start',
        key: 'name',
    },
    { title: 'Type', key: 'type' },
    { title: 'Size', key: 'size' },
    { title: 'Date', key: 'date' },
    { title: '', key: 'actions', sortable: false, width: 0 },
];
const collator = new Intl.Collator('en', { sensitivity: 'case' });

const extensionInfos: Map<string[], BrowserObjectTypeInfo> = new Map([
    [['jpg', 'jpeg', 'png', 'gif'], { title: 'Image', icon: imageIcon }],
    [['mp4', 'mkv', 'mov'], { title: 'Video', icon: videoIcon }],
    [['mp3', 'aac', 'wav', 'm4a'], { title: 'Audio', icon: audioIcon }],
    [['txt', 'docx', 'doc', 'pages'], { title: 'Text', icon: textIcon }],
    [['pdf'], { title: 'PDF', icon: pdfIcon }],
    [['zip'], { title: 'ZIP', icon: zipIcon }],
    [['xls', 'numbers', 'csv', 'xlsx', 'tsv'], { title: 'Spreadsheet', icon: spreadsheetIcon }],
]);
const folderInfo: BrowserObjectTypeInfo = { title: 'Folder', icon: folderIcon };
const fileInfo: BrowserObjectTypeInfo = { title: 'File', icon: fileIcon };

/**
 * Returns the name of the selected bucket.
 */
const bucketName = computed<string>(() => bucketsStore.state.fileComponentBucketName);

/**
 * Returns the name of the current path within the selected bucket.
 */
const filePath = computed<string>(() => bucketsStore.state.fileComponentPath);

/**
 * Returns every file under the current path.
 */
const allFiles = computed<BrowserObjectWrapper[]>(() => {
    if (props.forceEmpty) return [];
    return obStore.state.files.map<BrowserObjectWrapper>(file => {
        const lowerName = file.Key.toLowerCase();
        const dotIdx = lowerName.indexOf('.');
        const ext = dotIdx === -1 ? '' : file.Key.slice(dotIdx + 1);
        return {
            browserObject: file,
            typeInfo: getFileTypeInfo(ext, file.type),
            lowerName,
            ext,
        };
    });
});

/**
 * Returns every file under the current path that matchs the search query.
 */
const filteredFiles = computed<BrowserObjectWrapper[]>(() => {
    if (!search.value) return allFiles.value;
    const searchLower = search.value.toLowerCase();
    return allFiles.value.filter(file => file.lowerName.includes(searchLower));
});

/**
 * Returns the files to be displayed in the table.
 */
const tableFiles = computed<BrowserObjectWrapper[]>(() => {
    const opts = options.value;
    if (!opts) return [];

    const files = [...filteredFiles.value];

    if (opts.sortBy.length) {
        const sortBy = opts.sortBy[0];

        type CompareFunc = (a: BrowserObjectWrapper, b: BrowserObjectWrapper) => number;
        const compareFuncs: Record<SortKey, CompareFunc> = {
            name: (a, b) => collator.compare(a.browserObject.Key, b.browserObject.Key),
            type: (a, b) => collator.compare(a.typeInfo.title, b.typeInfo.title) || collator.compare(a.ext, b.ext),
            size: (a, b) => a.browserObject.Size - b.browserObject.Size,
            date: (a, b) => a.browserObject.LastModified.getTime() - b.browserObject.LastModified.getTime(),
        };

        files.sort((a, b) => {
            const objA = a.browserObject, objB = b.browserObject;
            if (sortBy.key !== 'type') {
                if (objA.type === 'folder') {
                    if (objB.type !== 'folder') return -1;
                    if (sortBy.key === 'size' || sortBy.key === 'date') return 0;
                } else if (objB.type === 'folder') {
                    return 1;
                }
            }

            const cmp = compareFuncs[sortBy.key](a, b);
            return sortBy.order === 'asc' ? cmp : -cmp;
        });
    }

    if (opts.itemsPerPage === -1) return files;

    return files.slice((opts.page - 1) * opts.itemsPerPage, opts.page * opts.itemsPerPage);
});

/**
 * Returns the string form of the file's last modified date.
 */
function getFormattedDate(file: BrowserObject): string {
    if (file.type === 'folder') return '';
    const date = file.LastModified;
    return `${date.getDate()} ${SHORT_MONTHS_NAMES[date.getMonth()]} ${date.getFullYear()}`;
}

/**
 * Returns the string form of the file's size.
 */
function getFormattedSize(file: BrowserObject): string {
    if (file.type === 'folder') return '';
    const size = new Size(file.Size);
    return `${size.formattedBytes} ${size.label}`;
}

/**
 * Returns the title and icon representing a file's type.
 */
function getFileTypeInfo(ext: string, type: BrowserObject['type']): BrowserObjectTypeInfo {
    if (!type) return fileInfo;
    if (type === 'folder') return folderInfo;

    ext = ext.toLowerCase();
    for (const [exts, info] of extensionInfos.entries()) {
        if (exts.indexOf(ext) !== -1) return info;
    }
    return fileInfo;
}

/**
 * Handles file click.
 */
function onFileClick(file: BrowserObject): void {
    if (!file.type) return;

    if (file.type === 'folder') {
        const uriParts = [file.Key];
        if (filePath.value) {
            uriParts.unshift(...filePath.value.split('/'));
        }
        const pathAndKey = uriParts.map(part => encodeURIComponent(part)).join('/');
        router.push(`/projects/${projectsStore.state.selectedProject.id}/buckets/${bucketName.value}/${pathAndKey}`);
        return;
    }

    obStore.setObjectPathForModal(obStore.state.path + file.Key);
    previewDialog.value = true;
}

/**
 * Fetches all files in the current directory.
 */
async function fetchFiles(): Promise<void> {
    if (isFetching.value || props.forceEmpty) return;
    isFetching.value = true;

    try {
        await obStore.list(filePath.value ? filePath.value + '/' : '');
        selected.value = [];
    } catch (err) {
        err.message = `Error fetching files. ${err.message}`;
        notify.notifyError(err, AnalyticsErrorEventSource.FILE_BROWSER_LIST_CALL);
    }

    isFetching.value = false;
}

watch(filePath, fetchFiles, { immediate: true });
watch(() => props.forceEmpty, v => !v && fetchFiles());
</script>
