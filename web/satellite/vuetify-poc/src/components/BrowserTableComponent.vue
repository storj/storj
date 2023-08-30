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

        <v-dialog v-model="previewDialog" transition="fade-transition" class="preview-dialog" fullscreen theme="dark">
            <v-card class="preview-card">
                <v-carousel hide-delimiters show-arrows="hover" height="100vh">
                    <template #prev="{ props: slotProps }">
                        <v-btn
                            color="default"
                            class="rounded-circle"
                            icon
                            @click="slotProps.onClick"
                        >
                            <svg width="10" height="17" viewBox="0 0 10 17" fill="none" xmlns="http://www.w3.org/2000/svg">
                                <g clip-path="url(#clip0_24843_332342)">
                                    <path fill-rule="evenodd" clip-rule="evenodd" d="M0.30725 8.23141C0.276805 7.67914 0.528501 7.04398 1.03164 6.54085L6.84563 0.726856C7.64837 -0.0758889 8.78719 -0.238577 9.38925 0.363481C9.99131 0.96554 9.82862 2.10436 9.02587 2.9071L3.71149 8.22148L9.02681 13.5368C9.82955 14.3395 9.99224 15.4784 9.39018 16.0804C8.78812 16.6825 7.6493 16.5198 6.84656 15.717L1.03257 9.90305C0.535173 9.40565 0.283513 8.77923 0.30725 8.23141Z" fill="white" />
                                </g>
                                <defs>
                                    <clipPath id="clip0_24843_332342">
                                        <rect width="17.0002" height="10" fill="white" transform="translate(10) rotate(90)" />
                                    </clipPath>
                                </defs>
                            </svg>
                        </v-btn>
                    </template>
                    <template #next="{ props: slotProps }">
                        <v-btn
                            color="default"
                            class="rounded-circle"
                            icon
                            @click="slotProps.onClick"
                        >
                            <svg width="10" height="17" viewBox="0 0 10 17" fill="none" xmlns="http://www.w3.org/2000/svg">
                                <g clip-path="url(#clip0_24843_332338)">
                                    <path fill-rule="evenodd" clip-rule="evenodd" d="M9.69263 8.23141C9.72307 7.67914 9.47138 7.04398 8.96824 6.54085L3.15425 0.726856C2.35151 -0.0758889 1.21269 -0.238577 0.61063 0.363481C0.00857207 0.96554 0.17126 2.10436 0.974005 2.9071L6.28838 8.22148L0.973072 13.5368C0.170328 14.3395 0.00763934 15.4784 0.609698 16.0804C1.21176 16.6825 2.35057 16.5198 3.15332 15.717L8.96731 9.90305C9.46471 9.40565 9.71637 8.77923 9.69263 8.23141Z" fill="white" />
                                </g>
                                <defs>
                                    <clipPath id="clip0_24843_332338">
                                        <rect width="17.0002" height="10" fill="white" transform="matrix(4.37114e-08 1 1 -4.37114e-08 0 0)" />
                                    </clipPath>
                                </defs>
                            </svg>
                        </v-btn>
                    </template>
                    <v-toolbar
                        color="rgba(0, 0, 0, 0.3)"
                        theme="dark"
                    >
                        <!-- <v-img src="../assets/logo-white.svg" height="30" width="160" class="ml-3" alt="Storj Logo"/> -->
                        <v-toolbar-title>
                            Image.jpg
                        </v-toolbar-title>
                        <template #append>
                            <v-btn icon size="small" color="white">
                                <img src="@poc/assets/icon-download.svg" width="22" alt="Download">
                                <v-tooltip
                                    activator="parent"
                                    location="bottom"
                                >
                                    Download
                                </v-tooltip>
                            </v-btn>
                            <v-btn icon size="small" color="white">
                                <icon-share size="22" />
                                <v-tooltip
                                    activator="parent"
                                    location="bottom"
                                >
                                    Share
                                </v-tooltip>
                            </v-btn>
                            <v-btn icon size="small" color="white">
                                <img src="@poc/assets/icon-geo-distribution.svg" width="22" alt="Geographic Distribution">
                                <v-tooltip
                                    activator="parent"
                                    location="bottom"
                                >
                                    Geographic Distribution
                                </v-tooltip>
                            </v-btn>
                            <v-btn icon size="small" color="white">
                                <img src="@poc/assets/icon-more.svg" width="22" alt="More">
                                <v-tooltip
                                    activator="parent"
                                    location="bottom"
                                >
                                    More
                                </v-tooltip>
                            </v-btn>
                            <v-btn icon size="small" color="white" @click="previewDialog = false">
                                <img src="@poc/assets/icon-close.svg" width="18" alt="Close">
                                <v-tooltip
                                    activator="parent"
                                    location="bottom"
                                >
                                    Close
                                </v-tooltip>
                            </v-btn>
                            <!-- <v-btn icon="$close" color="white" size="small" @click="previewDialog = false"></v-btn> -->
                        </template>
                    </v-toolbar>
                    <v-carousel-item
                        v-for="(item,i) in items"
                        :key="i"
                        :src="item.src"
                    />
                </v-carousel>
            </v-card>
        </v-dialog>
    </v-card>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { useRouter } from 'vue-router';
import {
    VCard,
    VTextField,
    VDialog,
    VCarousel,
    VBtn,
    VToolbar,
    VToolbarTitle,
    VTooltip,
    VCarouselItem,
} from 'vuetify/components';
import { VDataTableServer } from 'vuetify/labs/components';

import { BrowserObject, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useNotify } from '@/utils/hooks';
import { SHORT_MONTHS_NAMES } from '@/utils/constants/date';
import { Size } from '@/utils/bytesSize';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useBucketsStore } from '@/store/modules/bucketsStore';

import IconShare from '@poc/components/icons/IconShare.vue';
import BrowserRowActions from '@poc/components/BrowserRowActions.vue';

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

const items = [
    { src: 'https://cdn.vuetifyjs.com/images/carousel/squirrel.jpg' },
    { src: 'https://cdn.vuetifyjs.com/images/carousel/sky.jpg' },
    { src: 'https://cdn.vuetifyjs.com/images/carousel/bird.jpg' },
    { src: 'https://cdn.vuetifyjs.com/images/carousel/planet.jpg' },
];
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

    // Implement logic to fetch the file content for preview or generate a URL for preview
    // Then, open the preview dialog
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
