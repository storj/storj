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
            no-data-text="No results found"
            :page="cursor.page"
            hover
            must-sort
            :loading="isFetching || loading"
            :items-length="isPaginationEnabled ? totalObjectCount : allFiles.length"
            :items-per-page-options="isPaginationEnabled ? tableSizeOptions(totalObjectCount, true) : undefined"
            @update:page="onPageChange"
            @update:itemsPerPage="onLimitChange"
        >
            <template #item="{ props: rowProps }">
                <v-data-table-row v-bind="rowProps">
                    <template #item.name="{ item }: ItemSlotProps">
                        <v-btn
                            class="rounded-lg w-100 px-1 justify-start font-weight-bold"
                            variant="text"
                            height="40"
                            color="default"
                            block
                            @click="onFileClick(item.raw.browserObject)"
                        >
                            <img :src="item.raw.typeInfo.icon" :alt="item.raw.typeInfo.title + 'icon'" class="mr-3">
                            <v-tooltip
                                v-if="firstFile && item.raw.browserObject.Key === firstFile.Key"
                                :model-value="isFileGuideShown"
                                persistent
                                no-click-animation
                                location="bottom"
                                class="browser-table__file-guide"
                                content-class="py-2"
                                @update:model-value="() => {}"
                            >
                                Click on the file name to preview.
                                <template #activator="{ props: activatorProps }">
                                    <span v-bind="activatorProps">{{ item.raw.browserObject.Key }}</span>
                                </template>
                            </v-tooltip>
                            <template v-else>{{ item.raw.browserObject.Key }}</template>
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
                        <browser-row-actions
                            :file="item.raw.browserObject"
                            @preview-click="onFileClick(item.raw.browserObject)"
                            @delete-file-click="onDeleteFileClick(item.raw.browserObject)"
                            @share-click="onShareClick(item.raw.browserObject)"
                        />
                    </template>
                </v-data-table-row>
            </template>
        </v-data-table-server>

        <file-preview-dialog v-model="previewDialog" />
    </v-card>

    <delete-file-dialog
        v-if="fileToDelete"
        v-model="isDeleteFileDialogShown"
        :file="fileToDelete"
        @content-removed="fileToDelete = null"
    />
    <share-dialog
        v-model="isShareDialogShown"
        :bucket-name="bucketName"
        :file="fileToShare || undefined"
        @content-removed="fileToShare = null"
    />
    <browser-snackbar-component v-model="isObjectsUploadModal" @file-click="onFileClick" />
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { useRouter } from 'vue-router';
import { VCard, VTextField, VBtn, VTooltip } from 'vuetify/components';
import { VDataTableServer, VDataTableRow } from 'vuetify/labs/components';

import {
    BrowserObject,
    MAX_KEY_COUNT,
    ObjectBrowserCursor,
    useObjectBrowserStore,
} from '@/store/modules/objectBrowserStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useNotify } from '@/utils/hooks';
import { SHORT_MONTHS_NAMES } from '@/utils/constants/date';
import { Size } from '@/utils/bytesSize';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { tableSizeOptions } from '@/types/common';
import { LocalData } from '@/utils/localData';
import { useAppStore } from '@/store/modules/appStore';

import BrowserRowActions from '@poc/components/BrowserRowActions.vue';
import FilePreviewDialog from '@poc/components/dialogs/FilePreviewDialog.vue';
import DeleteFileDialog from '@poc/components/dialogs/DeleteFileDialog.vue';
import ShareDialog from '@poc/components/dialogs/ShareDialog.vue';
import BrowserSnackbarComponent from '@poc/components/BrowserSnackbarComponent.vue';

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

const config = useConfigStore();
const obStore = useObjectBrowserStore();
const projectsStore = useProjectsStore();
const bucketsStore = useBucketsStore();
const appStore = useAppStore();

const notify = useNotify();
const router = useRouter();

const isFetching = ref<boolean>(false);
const search = ref<string>('');
const selected = ref([]);
const previewDialog = ref<boolean>(false);
const options = ref<TableOptions>();
const fileToDelete = ref<BrowserObject | null>(null);
const isDeleteFileDialogShown = ref<boolean>(false);
const fileToShare = ref<BrowserObject | null>(null);
const isShareDialogShown = ref<boolean>(false);
const isFileGuideShown = ref<boolean>(false);
const routePageCache = new Map<string, number>();

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
    [['jpg', 'jpeg', 'png', 'gif', 'svg'], { title: 'Image', icon: imageIcon }],
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
 * Returns the current path within the selected bucket.
 */
const filePath = computed<string>(() => bucketsStore.state.fileComponentPath);

/**
 * Returns total object count from store.
 */
const isPaginationEnabled = computed<boolean>(() => config.state.config.objectBrowserPaginationEnabled);

/**
 * Returns total object count from store.
 */
const totalObjectCount = computed<number>(() => obStore.state.totalObjectCount);

/**
 * Returns table cursor from store.
 */
const cursor = computed<ObjectBrowserCursor>(() => obStore.state.cursor);

/**
 * Indicates whether objects upload modal should be shown.
 */
const isObjectsUploadModal = computed<boolean>(() => appStore.state.isUploadingModal);

/**
 * Returns every file under the current path.
 */
const allFiles = computed<BrowserObjectWrapper[]>(() => {
    if (props.forceEmpty) return [];

    const objects = isPaginationEnabled.value ? obStore.displayedObjects : obStore.state.files;
    return objects.map<BrowserObjectWrapper>(file => {
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

    if (opts.itemsPerPage === -1 || isPaginationEnabled.value) return files;

    return files.slice((opts.page - 1) * opts.itemsPerPage, opts.page * opts.itemsPerPage);
});

/**
 * Returns the first browser object in the table that is a file.
 */
const firstFile = computed<BrowserObject | null>(() => {
    return tableFiles.value.find(f => f.browserObject.type === 'file')?.browserObject || null;
});

/**
 * Handles page change event.
 */
function onPageChange(page: number): void {
    const path = filePath.value ? filePath.value + '/' : '';
    routePageCache.set(path, page);
    obStore.setCursor({ page, limit: options.value?.itemsPerPage ?? 10 });

    const lastObjectOnPage = page * cursor.value.limit;
    const activeRange = obStore.state.activeObjectsRange;

    if (lastObjectOnPage > activeRange.start && lastObjectOnPage <= activeRange.end) {
        return;
    }

    const tokenKey = Math.ceil(lastObjectOnPage / MAX_KEY_COUNT) * MAX_KEY_COUNT;

    const tokenToBeFetched = obStore.state.continuationTokens.get(tokenKey);
    if (!tokenToBeFetched) {
        obStore.initList(path);
        return;
    }

    obStore.listByToken(path, tokenKey, tokenToBeFetched);
}

/**
 * Handles items per page change event.
 */
function onLimitChange(newLimit: number): void {
    obStore.setCursor({ page: options.value?.page ?? 1, limit: newLimit });
}

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
        router.push(`/projects/${projectsStore.state.selectedProject.urlId}/buckets/${bucketName.value}/${pathAndKey}`);
        return;
    }

    obStore.setObjectPathForModal(file.path ?? '' + file.Key);
    previewDialog.value = true;
    isFileGuideShown.value = false;
    LocalData.setFileGuideHidden();
}

/**
 * Fetches all files in the current directory.
 */
async function fetchFiles(): Promise<void> {
    if (isFetching.value || props.forceEmpty) return;
    isFetching.value = true;

    try {
        const path = filePath.value ? filePath.value + '/' : '';

        if (isPaginationEnabled.value) {
            await obStore.initList(path);
        } else {
            await obStore.list(path);
        }

        selected.value = [];

        if (isPaginationEnabled.value) {
            const cachedPage = routePageCache.get(path);
            if (cachedPage !== undefined) {
                obStore.setCursor({ limit: cursor.value.limit, page: cachedPage });
            } else {
                obStore.setCursor({ limit: cursor.value.limit, page: 1 });
            }
        }
    } catch (err) {
        err.message = `Error fetching files. ${err.message}`;
        notify.notifyError(err, AnalyticsErrorEventSource.FILE_BROWSER_LIST_CALL);
    }

    isFetching.value = false;
}

/**
 * Handles delete button click event for files.
 */
function onDeleteFileClick(file: BrowserObject): void {
    fileToDelete.value = file;
    isDeleteFileDialogShown.value = true;
}

/**
 * Handles Share button click event.
 */
function onShareClick(file: BrowserObject): void {
    fileToShare.value = file;
    isShareDialogShown.value = true;
}

watch(filePath, fetchFiles, { immediate: true });
watch(() => props.forceEmpty, v => !v && fetchFiles());

if (!LocalData.getFileGuideHidden()) {
    const unwatch = watch(firstFile, () => {
        isFileGuideShown.value = true;
        LocalData.setFileGuideHidden();
        unwatch();
    });
}
</script>

<style scoped lang="scss">
.browser-table {

    &__loader-overlay :deep(.v-overlay__scrim) {
        opacity: 1;
        bottom: 0.8px;
    }

    &__file-guide :deep(.v-overlay__content) {
        color: var(--c-white) !important;
        background-color: rgb(var(--v-theme-primary)) !important;
    }
}
</style>
