// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card v-if="!isAltPagination" class="pa-2 mb-7" variant="flat" :loading="isFetching">
        <v-row align="center">
            <v-col>
                <v-text-field
                    v-model="search"
                    label="Search"
                    :prepend-inner-icon="Search"
                    single-line
                    variant="solo-filled"
                    flat
                    hide-details
                    clearable
                    density="comfortable"
                    rounded="lg"
                    @update:modelValue="analyticsStore.eventTriggered(AnalyticsEvent.SEARCH_BUCKETS)"
                />
            </v-col>
            <v-col cols="auto">
                <v-menu>
                    <template #activator="{ props: sortProps }">
                        <v-btn
                            variant="text"
                            color="default"
                            :prepend-icon="ArrowUpDown"
                            :append-icon="ChevronDown"
                            v-bind="sortProps"
                            class="mr-2 ml-n2"
                            title="Sort by"
                        >
                            <span class="text-body-2 hidden-xs">Sort by</span> <span class="ml-1 text-capitalize">{{ sortKey }}</span>
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
                <v-btn-toggle
                    v-model="sortOrder"
                    density="comfortable"
                    variant="outlined"
                    color="default"
                    rounded="xl"
                    class="pa-1"
                    mandatory
                >
                    <v-btn size="small" value="asc" title="Ascending" variant="text" rounded="xl">
                        <v-icon :icon="ArrowDownNarrowWide" />
                    </v-btn>
                    <v-btn size="small" value="desc" title="Descending" variant="text" rounded="xl">
                        <v-icon :icon="ArrowUpNarrowWide" />
                    </v-btn>
                </v-btn-toggle>
            </v-col>
        </v-row>
    </v-card>

    <v-data-iterator
        :page="cursor.page"
        :items-per-page="cursor.limit"
        :items="isAltPagination ? allFiles : browserFiles"
        :search="search"
        :sort-by="sortBy"
        :loading="isFetching"
    >
        <template #no-data>
            <div class="d-flex justify-center">
                <p class="text-body-2 cursor-pointer py-16 rounded-xlg w-100 text-center bg-light border" @click="emit('uploadClick')">
                    {{ search ? 'No data found' : 'Drag and drop files or folders here, or click to upload files.' }}
                </p>
            </div>
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
            <v-card class="pa-2 my-6" variant="flat">
                <div class="d-flex align-center">
                    <v-menu>
                        <template #activator="{ props: limitProps }">
                            <v-btn
                                variant="text"
                                color="default"
                                :append-icon="ChevronDown"
                                v-bind="limitProps"
                            >
                                <span class="text-caption text-medium-emphasis mr-2">Items per page:</span>
                                {{ cursor.limit }}
                            </v-btn>
                        </template>
                        <v-list>
                            <v-list-item
                                v-for="(number, index) in pageSizes"
                                :key="index"
                                :title="number.title"
                                @click="() => onLimitChange(number.value)"
                            />
                        </v-list>
                    </v-menu>

                    <v-spacer />

                    <span v-if="!isAltPagination" class="mr-4 text-caption text-medium-emphasis">
                        Page {{ cursor.page }} of {{ lastPage }}
                    </span>
                    <v-btn
                        :icon="ChevronLeft"
                        size="small"
                        rounded="md"
                        variant="text"
                        color="default"
                        :disabled="cursor.page <= 1"
                        @click="() => isAltPagination ? onPreviousPageClicked() : onPageChange(cursor.page - 1)"
                    />
                    <v-btn
                        :icon="ChevronRight"
                        size="small"
                        rounded="md"
                        variant="text"
                        color="default"
                        class="ml-2"
                        :disabled="isAltPagination ? !hasNextPage : cursor.page === lastPage"
                        @click="() => isAltPagination ? onNextPageClicked() : onPageChange(cursor.page + 1)"
                    />
                </div>
            </v-card>
        </template>
    </v-data-iterator>
    <file-preview-dialog
        v-model="previewDialog"
        v-model:current-file="fileToPreview"
        :showing-versions="!!fileToPreview?.VersionId"
        video-autoplay
    />

    <delete-file-dialog
        v-model="isDeleteFileDialogShown"
        :files="filesToDelete"
    />
    <share-dialog
        v-model="isShareDialogShown"
        :bucket-name="bucketName"
        :file="fileToShare || undefined"
        @content-removed="fileToShare = null"
    />
</template>

<script setup lang="ts">
import { computed, onBeforeMount, ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import {
    VBtn,
    VBtnToggle,
    VCol,
    VCard,
    VIcon,
    VList,
    VListItem,
    VMenu,
    VRow,
    VSpacer,
    VTextField,
    VDataIterator,
} from 'vuetify/components';
import { ChevronLeft, ChevronRight, Search, ChevronDown, ArrowDownNarrowWide, ArrowUpNarrowWide, ArrowUpDown } from 'lucide-vue-next';

import {
    BrowserObject,
    MAX_KEY_COUNT,
    ObjectBrowserCursor,
    PreviewCache,
    useObjectBrowserStore,
} from '@/store/modules/objectBrowserStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { BrowserObjectTypeInfo, BrowserObjectWrapper, EXTENSION_INFOS, FILE_INFO, FOLDER_INFO } from '@/types/browser';
import { useLinksharing } from '@/composables/useLinksharing';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { ROUTES } from '@/router';

import FilePreviewDialog from '@/components/dialogs/FilePreviewDialog.vue';
import DeleteFileDialog from '@/components/dialogs/DeleteFileDialog.vue';
import ShareDialog from '@/components/dialogs/ShareDialog.vue';
import FileCard from '@/components/FileCard.vue';

type SortKey = 'name' | 'type' | 'size' | 'date';

const props = defineProps<{
    forceEmpty?: boolean;
}>();

const emit = defineEmits<{
    uploadClick: [];
}>();

const analyticsStore = useAnalyticsStore();
const obStore = useObjectBrowserStore();
const projectsStore = useProjectsStore();
const bucketsStore = useBucketsStore();

const notify = useNotify();
const router = useRouter();

const { generateObjectPreviewAndMapURL } = useLinksharing();

const isFetching = ref<boolean>(false);
const search = ref<string>('');
const selected = ref([]);
const previewDialog = ref<boolean>(false);
const fileToPreview = ref<BrowserObject | null>(null);
const filesToDelete = ref<BrowserObject[]>([]);
const isDeleteFileDialogShown = ref<boolean>(false);
const fileToShare = ref<BrowserObject | null>(null);
const isShareDialogShown = ref<boolean>(false);
const routePageCache = new Map<string, number>();
let previewQueue: BrowserObjectWrapper[] = [];
let processingPreview = false;

const sortKey = ref<string>('name');
const sortOrder = ref<string>('asc');
const sortKeys = ['Name', 'Type', 'Size', 'Date'];
const pageSizes = [
    { title: '12', value: 12 },
    { title: '24', value: 24 },
    { title: '36', value: 36 },
    { title: '144', value: 144 },
];
const collator = new Intl.Collator('en', { sensitivity: 'case' });

/**
 * Indicates if alternative pagination has next page.
 */
const hasNextPage = computed<boolean>(() => {
    const nextToken = obStore.state.continuationTokens.get(cursor.value.page + 1);

    return nextToken !== undefined;
});

/**
 * Indicates if alternative pagination should be used.
 */
const isAltPagination = computed(() => obStore.isAltPagination);

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
 * Returns total object count from store.
 */
const totalObjectCount = computed<number>(() => obStore.state.totalObjectCount);

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

    const objects = isAltPagination.value ? obStore.sortedFiles : obStore.displayedObjects;

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
    if (isAltPagination.value) return [];
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
    if (isAltPagination.value) return [];

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

    return files;
});

/**
 * Handles previous page click for alternative pagination.
 */
function onPreviousPageClicked(): void {
    fetchFiles(cursor.value.page - 1, false);
}

/**
 * Handles next page click for alternative pagination.
 */
function onNextPageClicked(): void {
    fetchFiles(cursor.value.page + 1, true);
}

/**
 * Handles page change event.
 */
function onPageChange(page: number): void {
    if (isAltPagination.value || page < 1 || page > lastPage.value) return;

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
    if (isAltPagination.value) {
        obStore.setCursor({ page: 1, limit: newLimit });
        obStore.clearTokens();
        fetchFiles();
    } else {
        // if the new limit is large enough to cause the page index to be out of range
        // we calculate an appropriate new page index.
        const oldPage = cursor.value.page ?? 1;
        const maxPage = Math.ceil(totalObjectCount.value / newLimit);
        const page = oldPage > maxPage ? maxPage : oldPage;
        obStore.setCursor({ page, limit: newLimit });
    }
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
        router.push(`${ROUTES.Projects.path}/${projectsStore.state.selectedProject.urlId}/${ROUTES.Buckets.path}/${bucketName.value}/${pathAndKey}`);
        return;
    }

    obStore.setObjectPathForModal((file.path ?? '') + file.Key);
    fileToPreview.value = file;
    previewDialog.value = true;
}

/**
 * Fetches all files in the current directory.
 */
async function fetchFiles(page = 1, saveNextToken = true): Promise<void> {
    if (isFetching.value || props.forceEmpty) return;
    isFetching.value = true;

    try {
        const path = filePath.value ? filePath.value + '/' : '';

        if (isAltPagination.value) {
            await obStore.listCustom(path, page, saveNextToken);
            selected.value = [];
        } else {
            await obStore.initList(path);

            selected.value = [];

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
    filesToDelete.value = [file];
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
    const url = findCachedURL(file.browserObject);
    if (!url) {
        await fetchPreviewUrl(file.browserObject);
    }
}

/**
 * Adds image files to preview queue.
 */
function addToPreviewQueue(file: BrowserObjectWrapper) {
    if (file.browserObject.type === 'folder' || (file.typeInfo.title !== 'Image' && file.typeInfo.title !== 'Video')) return;

    previewQueue.push(file);
    if (!processingPreview) {
        processPreviewQueue();
    }
}

/**
 * Processes preview queue to get preview urls for each
 * image file in the queue sequentially.
 */
async function processPreviewQueue() {
    if (previewQueue.length > 0) {
        processingPreview = true;
        const files = [...previewQueue];
        const file = files.shift();
        previewQueue = files;
        if (file) {
            await processFilePath(file);
            processPreviewQueue();
        }
    } else {
        processingPreview = false;
    }
}

obStore.$onAction(({ name, after }) => {
    if (name === 'filesDeleted') {
        after((_) => {
            fetchFiles();
            filesToDelete.value = [];
            obStore.updateSelectedFiles([]);
        });
    }
});

watch(filePath, () => {
    obStore.clearTokens();
    fetchFiles();
}, { immediate: true });
watch(() => props.forceEmpty, v => !v && fetchFiles());

watch(allFiles, async (value, oldValue) => {
    // find new files for which we haven't fetched preview url yet.
    const newFiles = value.filter(file => {
        return !oldValue?.some(oldFile => {
            return oldFile.browserObject.Key === file.browserObject.Key
                && oldFile.browserObject.path === file.browserObject.path;
        });
    });
    for (const file of newFiles) {
        addToPreviewQueue(file);
    }
}, { immediate: true });

onBeforeMount(() => {
    obStore.setCursor({ page: 1, limit: pageSizes[0].value });
});
</script>