// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-row align="center" class="mb-3">
        <v-col>
            <v-text-field
                v-model="search"
                label="Search"
                prepend-inner-icon="mdi-magnify"
                single-line
                variant="solo-filled"
                flat
                hide-details
                clearable
                density="compact"
                rounded="lg"
            />
        </v-col>
        <v-col cols="auto">
            <v-menu>
                <template #activator="{ props: sortProps }">
                    <v-btn
                        variant="outlined"
                        color="default"
                        prepend-icon="mdi-sort"
                        append-icon="mdi-chevron-down"
                        v-bind="sortProps"
                    >
                        <span class="text-body-2">Sort by</span> <span class="ml-1 text-capitalize">{{ sortKey }}</span>
                    </v-btn>
                </template>
                <v-list>
                    <v-list-item
                        v-for="(key, index) in sortKeys"
                        :key="index"
                        :title="key"
                        @click="() => sortKey = key.toLowerCase()"
                    />
                </v-list>
            </v-menu>
        </v-col>

        <v-col cols="auto">
            <v-btn-toggle
                v-model="sortOrder"
                density="comfortable"
                variant="outlined"
                color="default"
                rounded="xl"
                class="pa-1"
                border
                mandatory
            >
                <v-btn size="small" value="asc" title="Ascending" variant="text" rounded="xl">
                    <v-icon>mdi-sort-ascending</v-icon>
                </v-btn>
                <v-btn size="small" value="desc" title="Descending" variant="text" rounded="xl">
                    <v-icon>mdi-sort-descending</v-icon>
                </v-btn>
            </v-btn-toggle>
        </v-col>
    </v-row>

    <v-data-iterator
        :page="cursor.page"
        :items-per-page="cursor.limit"
        :items="browserFiles"
        :search="search"
        :sort-by="sortBy"
        :loading="isFetching"
    >
        <template #no-data>
            <div class="d-flex justify-center">No results found</div>
        </template>

        <template #default="fileProps">
            <v-row>
                <v-col v-for="item in fileProps.items" :key="item.raw.browserObject.Key" cols="12" sm="6" md="4" lg="3" xl="2">
                    <file-card
                        :item="item.raw"
                        class="h-100"
                        @preview-click="onFileClick(item.raw.browserObject)"
                        @delete-file-click="onDeleteFileClick(item.raw.browserObject)"
                        @share-click="onShareClick(item.raw.browserObject)"
                    />
                </v-col>
            </v-row>
        </template>

        <template #footer>
            <div class="d-flex align-center py-5">
                <v-menu>
                    <template #activator="{ props: limitProps }">
                        <span class="text-subtitle-2 mr-2">Items per page:</span>
                        <v-btn
                            variant="outlined"
                            color="default"
                            append-icon="mdi-chevron-down"
                            v-bind="limitProps"
                        >
                            {{ cursor.limit }}
                        </v-btn>
                    </template>
                    <v-list>
                        <v-list-item
                            v-for="(number, index) in tableSizeOptions(totalObjectCount, true)"
                            :key="index"
                            :title="number.title"
                            @click="() => onLimitChange(number.value)"
                        />
                    </v-list>
                </v-menu>

                <v-spacer />

                <span class="mr-4 text-medium-emphasis">
                    Page {{ cursor.page }} of {{ lastPage }}
                </span>
                <v-btn
                    icon
                    size="small"
                    variant="outlined"
                    color="default"
                    :disabled="cursor.page === 1"
                    @click="() => onPageChange(cursor.page - 1)"
                >
                    <v-icon>mdi-chevron-left</v-icon>
                </v-btn>
                <v-btn
                    icon
                    size="small"
                    variant="outlined"
                    color="default"
                    class="ml-2"
                    :disabled="cursor.page === lastPage"
                    @click="() => onPageChange(cursor.page + 1)"
                >
                    <v-icon>mdi-chevron-right</v-icon>
                </v-btn>
            </div>
        </template>
    </v-data-iterator>
    <file-preview-dialog v-model="previewDialog" />

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
import { computed, ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import {
    VBtn,
    VBtnToggle,
    VCol,
    VIcon,
    VList,
    VListItem,
    VMenu,
    VRow,
    VSpacer,
    VTextField,
    VDataIterator,
} from 'vuetify/components';

import {
    BrowserObject,
    MAX_KEY_COUNT,
    ObjectBrowserCursor,
    PreviewCache,
    useObjectBrowserStore,
} from '@/store/modules/objectBrowserStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { LocalData } from '@/utils/localData';
import { useAppStore } from '@/store/modules/appStore';
import { BrowserObjectTypeInfo, BrowserObjectWrapper, EXTENSION_INFOS, FILE_INFO, FOLDER_INFO } from '@/types/browser';
import { useLinksharing } from '@/composables/useLinksharing';
import { tableSizeOptions } from '@/types/common';

import FilePreviewDialog from '@poc/components/dialogs/FilePreviewDialog.vue';
import DeleteFileDialog from '@poc/components/dialogs/DeleteFileDialog.vue';
import ShareDialog from '@poc/components/dialogs/ShareDialog.vue';
import BrowserSnackbarComponent from '@poc/components/BrowserSnackbarComponent.vue';
import FileCard from '@poc/components/FileCard.vue';

type SortKey = 'name' | 'type' | 'size' | 'date';

const props = defineProps<{
    forceEmpty?: boolean;
}>();

const emit = defineEmits<{
    uploadClick: [];
    newFolderClick: [];
}>();

const config = useConfigStore();
const obStore = useObjectBrowserStore();
const projectsStore = useProjectsStore();
const bucketsStore = useBucketsStore();
const appStore = useAppStore();

const notify = useNotify();
const router = useRouter();

const { generateObjectPreviewAndMapURL } = useLinksharing();

const isFetching = ref<boolean>(false);
const search = ref<string>('');
const selected = ref([]);
const previewDialog = ref<boolean>(false);
const fileToDelete = ref<BrowserObject | null>(null);
const isDeleteFileDialogShown = ref<boolean>(false);
const fileToShare = ref<BrowserObject | null>(null);
const isShareDialogShown = ref<boolean>(false);
const routePageCache = new Map<string, number>();

const sortKey = ref<string>('name');
const sortOrder = ref<string>('asc');
const sortKeys = ['Name', 'Type', 'Size', 'Date'];
const collator = new Intl.Collator('en', { sensitivity: 'case' });

/**
 * Returns object preview URLs cache from store.
 */
const cachedObjectPreviewURLs = computed((): Map<string, PreviewCache> => {
    return obStore.state.cachedObjectPreviewURLs;
});

/**
 * Returns the name of the selected bucket.
 */
const bucketName = computed<string>(() => bucketsStore.state.fileComponentBucketName);

/**
 * Returns the current path within the selected bucket.
 */
const filePath = computed<string>(() => bucketsStore.state.fileComponentPath);

/**
 * Indicates whether objects upload modal should be shown.
 */
const isObjectsUploadModal = computed<boolean>(() => appStore.state.isUploadingModal);

/**
 * Returns total object count from store.
 */
const isPaginationEnabled = computed<boolean>(() => config.state.config.objectBrowserPaginationEnabled);

/**
 * Returns total object count from store.
 */
const totalObjectCount = computed<number>(() => isPaginationEnabled.value ? obStore.state.totalObjectCount : allFiles.value.length);

/**
 * Returns browser cursor from store.
 */
const cursor = computed<ObjectBrowserCursor>(() => obStore.state.cursor);

/**
 * Returns the last page of the file list.
 */
const lastPage = computed<number>(() => {
    const page = Math.ceil(totalObjectCount.value / cursor.value.limit);
    return page === 0 ? page + 1 : page;
});

/**
 * Returns every file under the current path.
 */
const allFiles = computed<BrowserObjectWrapper[]>(() => {
    if (props.forceEmpty) return [];

    const objects = obStore.state.files;
    return objects.map<BrowserObjectWrapper>(file => {
        const lowerName = file.Key.toLowerCase();
        const dotIdx = lowerName.lastIndexOf('.');
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
 * The sorting criteria to be used for the file list.
 */
const sortBy = computed(() => [{ key: sortKey.value, order: sortOrder.value }]);

/**
 * Returns the files to be displayed in the browser.
 */
const browserFiles = computed<BrowserObjectWrapper[]>(() => {
    const files = [...filteredFiles.value];

    if (sortBy.value.length) {
        const sort = sortBy.value[0];

    type CompareFunc = (a: BrowserObjectWrapper, b: BrowserObjectWrapper) => number;
    const compareFuncs: Record<SortKey, CompareFunc> = {
        name: (a, b) => collator.compare(a.browserObject.Key, b.browserObject.Key),
        type: (a, b) => collator.compare(a.typeInfo.title, b.typeInfo.title) || collator.compare(a.ext, b.ext),
        size: (a, b) => a.browserObject.Size - b.browserObject.Size,
        date: (a, b) => a.browserObject.LastModified.getTime() - b.browserObject.LastModified.getTime(),
    };

    files.sort((a, b) => {
        const objA = a.browserObject, objB = b.browserObject;
        if (sort.key !== 'type') {
            if (objA.type === 'folder') {
                if (objB.type !== 'folder') return -1;
                if (sort.key === 'size' || sort.key === 'date') return 0;
            } else if (objB.type === 'folder') {
                return 1;
            }
        }

        const cmp = compareFuncs[sort.key](a, b);
        return sort.order === 'asc' ? cmp : -cmp;
    });
    }

    if (cursor.value.limit === -1 || isPaginationEnabled.value) return files;

    return files.slice((cursor.value.page - 1) * cursor.value.limit, cursor.value.page * cursor.value.limit);
});

/**
 * Handles page change event.
 */
function onPageChange(page: number): void {
    if (page < 1) return;
    if (page > lastPage.value) return;
    const path = filePath.value ? filePath.value + '/' : '';
    routePageCache.set(path, page);
    obStore.setCursor({ page, limit: cursor.value?.limit ?? 10 });

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
    // if the new limit is large enough to cause the page index to be out of range
    // we calculate an appropriate new page index.
    const oldPage = cursor.value.page ?? 1;
    const maxPage = Math.ceil(totalObjectCount.value / newLimit);
    const page = oldPage > maxPage ? maxPage : oldPage;
    obStore.setCursor({ page, limit: newLimit });
}

/**
 * Returns the title and icon representing a file's type.
 */
function getFileTypeInfo(ext: string, type: BrowserObject['type']): BrowserObjectTypeInfo {
    if (!type) return FILE_INFO;
    if (type === 'folder') return FOLDER_INFO;

    ext = ext.toLowerCase();
    for (const [exts, info] of EXTENSION_INFOS.entries()) {
        if (exts.indexOf(ext) !== -1) return info;
    }
    return FILE_INFO;
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

    obStore.setObjectPathForModal(file.path + file.Key);
    previewDialog.value = true;
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

/**
 * Get the object preview url.
 */
async function fetchPreviewUrl(file: BrowserObject) {
    let url = '';
    try {
        url = await generateObjectPreviewAndMapURL(bucketsStore.state.fileComponentBucketName, file.path + file.Key);
    } catch (error) {
        error.message = `Unable to get file preview URL. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.FILE_BROWSER_ENTRY);
    }

    if (!url) {
        return;
    }
    const filePath = encodeURIComponent(`${bucketName.value}/${file.path}${file.Key}`);
    obStore.cacheObjectPreviewURL(filePath, { url, lastModified: file.LastModified.getTime() });
}

/**
 * Try to find current object's url in cache.
 */
function findCachedURL(file: BrowserObject): string | undefined {
    const filePath = encodeURIComponent(`${bucketName.value}/${file.path}${file.Key}`);
    const cache = cachedObjectPreviewURLs.value.get(filePath);

    if (!cache) return undefined;

    if (cache.lastModified !== file.LastModified.getTime()) {
        obStore.removeFromObjectPreviewCache(filePath);
        return undefined;
    }

    return cache.url;
}

/**
 * Loads object URL from cache or generates new URL for previewing
 * images on card items.
 */
async function processFilePath(file: BrowserObjectWrapper) {
    if (file.browserObject.type === 'folder') return;
    if (file.typeInfo.title !== 'Image') return;
    const url = findCachedURL(file.browserObject);
    if (!url) {
        await fetchPreviewUrl(file.browserObject);
    }
}

watch(filePath, fetchFiles, { immediate: true });
watch(() => props.forceEmpty, v => !v && fetchFiles());

watch(allFiles, async (files: BrowserObjectWrapper[]) => {
    for (const file of files) {
        await processFilePath(file);
    }
});
</script>