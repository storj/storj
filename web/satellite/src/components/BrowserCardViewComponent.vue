// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card class="pa-4 mb-6" :loading="isFetching">
        <v-row align="center">
            <v-col cols="12" sm class="flex-grow-1 flex-sm-grow-1">
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
                />
            </v-col>
            <v-col cols="12" sm="auto" class="d-flex align-center">
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
                            class="mx-2"
                            @click="() => sortKey = key.toLowerCase()"
                        />
                    </v-list>
                </v-menu>
                <v-btn-toggle
                    v-model="sortOrder"
                    variant="outlined"
                    color="default"
                    class="pa-1"
                    rounded="md"
                    mandatory
                >
                    <v-btn value="asc" title="Ascending" variant="text" rounded="md">
                        <v-icon :icon="ArrowDownNarrowWide" />
                    </v-btn>
                    <v-btn value="desc" title="Descending" variant="text" rounded="md">
                        <v-icon :icon="ArrowUpNarrowWide" />
                    </v-btn>
                </v-btn-toggle>
            </v-col>
        </v-row>

        <v-data-iterator
            :page="cursor.page"
            :items-per-page="cursor.limit"
            :items="allFiles"
            :search="search"
            :loading="isFetching"
            class="mt-1"
        >
            <template #no-data>
                <div class="d-flex justify-center">
                    <p class="text-body-2 cursor-pointer py-16 rounded-md w-100 text-center bg-light border mt-3" @click="emit('uploadClick')">
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
                            @lock-object-click="onLockObjectClick(item.raw.browserObject)"
                            @legal-hold-click="onLegalHoldClick(item.raw.browserObject)"
                            @download-folder-click="onDownloadFolder(item.raw.browserObject)"
                            @locked-object-delete="(fullObject) => onLockedObjectDelete(fullObject)"
                        />
                    </v-col>
                </v-row>
            </template>

            <template #footer>
                <div class="d-flex align-center mt-4 rounded-md border pa-1">
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

                    <span class="mr-4 text-body-2">{{ pageDisplayText }}</span>
                    <v-btn
                        :icon="ChevronLeft"
                        size="small"
                        rounded="md"
                        variant="text"
                        color="default"
                        :disabled="cursor.page <= 1"
                        @click="() => onPreviousPageClicked()"
                    />
                    <v-btn
                        :icon="ChevronRight"
                        size="small"
                        rounded="md"
                        variant="text"
                        color="default"
                        class="ml-2"
                        :disabled="!hasNextPage"
                        @click="() => onNextPageClicked()"
                    />
                </div>
            </template>
        </v-data-iterator>
    </v-card>

    <file-preview-dialog
        v-model="previewDialog"
        v-model:current-file="fileToPreview"
        video-autoplay
    />

    <delete-file-dialog
        v-if="!isBucketVersioned"
        v-model="isDeleteFileDialogShown"
        :files="filesToDelete"
        @content-removed="fileToDelete = null"
    />
    <delete-versioned-file-dialog
        v-else
        v-model="isDeleteFileDialogShown"
        :files="filesToDelete"
        @content-removed="fileToDelete = null"
    />
    <share-dialog
        v-model="isShareDialogShown"
        :bucket-name="bucketName"
        :file="fileToShare || undefined"
        @content-removed="fileToShare = null"
    />
    <lock-object-dialog
        v-model="isLockDialogShown"
        :file="lockActionFile"
        @content-removed="lockActionFile = null"
    />
    <legal-hold-object-dialog
        v-model="isLegalHoldDialogShown"
        :file="lockActionFile"
        @content-removed="lockActionFile = null"
    />
    <locked-delete-error-dialog
        v-model="isLockedObjectDeleteDialogShown"
        :file="lockActionFile"
        @content-removed="lockActionFile = null"
    />
    <download-prefix-dialog
        v-if="downloadPrefixEnabled"
        v-model="isDownloadPrefixDialogShown"
        :prefix-type="DownloadPrefixType.Folder"
        :bucket="bucketName"
        :prefix="folderToDownload"
    />
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import {
    VBtn,
    VBtnToggle,
    VCard,
    VCol,
    VDataIterator,
    VIcon,
    VList,
    VListItem,
    VMenu,
    VRow,
    VSpacer,
    VTextField,
} from 'vuetify/components';
import {
    ArrowDownNarrowWide,
    ArrowUpDown,
    ArrowUpNarrowWide,
    ChevronDown,
    ChevronLeft,
    ChevronRight,
    Search,
} from 'lucide-vue-next';

import {
    BrowserObject,
    FullBrowserObject,
    ObjectBrowserCursor,
    PreviewCache,
    useObjectBrowserStore,
} from '@/store/modules/objectBrowserStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import {
    BrowserObjectTypeInfo,
    BrowserObjectWrapper,
    DownloadPrefixType,
    EXTENSION_INFOS,
    FILE_INFO,
    FOLDER_INFO,
} from '@/types/browser';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { ROUTES } from '@/router';
import { BucketMetadata } from '@/types/buckets';
import { Versioning } from '@/types/versioning';
import { usePreCheck } from '@/composables/usePreCheck';
import { useLoading } from '@/composables/useLoading';
import { useConfigStore } from '@/store/modules/configStore';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';

import FilePreviewDialog from '@/components/dialogs/FilePreviewDialog.vue';
import DeleteFileDialog from '@/components/dialogs/DeleteFileDialog.vue';
import ShareDialog from '@/components/dialogs/ShareDialog.vue';
import FileCard from '@/components/FileCard.vue';
import DeleteVersionedFileDialog from '@/components/dialogs/DeleteVersionedFileDialog.vue';
import LockObjectDialog from '@/components/dialogs/LockObjectDialog.vue';
import LockedDeleteErrorDialog from '@/components/dialogs/LockedDeleteErrorDialog.vue';
import LegalHoldObjectDialog from '@/components/dialogs/LegalHoldObjectDialog.vue';
import DownloadPrefixDialog from '@/components/dialogs/DownloadPrefixDialog.vue';

const props = defineProps<{
    forceEmpty?: boolean;
    bucket?: BucketMetadata;
}>();

const emit = defineEmits<{
    uploadClick: [];
}>();

const analyticsStore = useAnalyticsStore();
const obStore = useObjectBrowserStore();
const projectsStore = useProjectsStore();
const bucketsStore = useBucketsStore();
const configStore = useConfigStore();

const notify = useNotify();
const router = useRouter();
const { withTrialCheck } = usePreCheck();
const { withLoading } = useLoading();

const isFetching = ref<boolean>(false);
const search = ref<string>('');
const selected = ref([]);
const previewDialog = ref<boolean>(false);
const lockActionFile = ref<FullBrowserObject | null>(null);
const fileToPreview = ref<BrowserObject | undefined>();
const fileToDelete = ref<BrowserObject | null>(null);
const isDeleteFileDialogShown = ref<boolean>(false);
const fileToShare = ref<BrowserObject | null>(null);
const isShareDialogShown = ref<boolean>(false);
const isLockDialogShown = ref<boolean>(false);
const isLegalHoldDialogShown = ref<boolean>(false);
const isLockedObjectDeleteDialogShown = ref<boolean>(false);
const isDownloadPrefixDialogShown = ref<boolean>(false);
const folderToDownload = ref<string>('');
const searchTimer = ref<NodeJS.Timeout>();

let previewQueue: BrowserObjectWrapper[] = [];
let processingPreview = false;

const sortKey = ref<string>('name');
const sortOrder = ref<'asc' | 'desc'>('asc');
const sortKeys = ['Name', 'Size', 'Date'];
const pageSizes = [
    { title: '12', value: 12 },
    { title: '24', value: 24 },
    { title: '36', value: 36 },
    { title: '144', value: 144 },
    { title: '500', value: 500 },
];

const downloadPrefixEnabled = computed<boolean>(() => configStore.state.config.downloadPrefixEnabled);

/**
 * Returns the selected files to the delete dialog.
 */
const filesToDelete = computed<BrowserObject[]>(() => {
    if (fileToDelete.value) return [fileToDelete.value];
    return obStore.state.selectedFiles;
});

/**
 * Indicates if there is a next page.
 */
const hasNextPage = computed<boolean>(() => obStore.state.pageTokens[cursor.value.page] !== undefined);

/**
 * Returns the page display text for simplified pagination (e.g., "Page 2 of 2+").
 */
const pageDisplayText = computed<string>(() => {
    const currentPage = cursor.value.page;
    const knownPages = obStore.state.pageTokens.length;
    const hasMore = hasNextPage.value;

    return `Page ${currentPage} of ${knownPages}${hasMore ? '+' : ''}`;
});

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
 * Returns browser cursor from store.
 */
const cursor = computed<ObjectBrowserCursor>(() => obStore.state.cursor);

/**
 * Whether this bucket is versioned/version-suspended.
 */
const isBucketVersioned = computed<boolean>(() => {
    return props.bucket?.versioning !== Versioning.NotSupported && props.bucket?.versioning !== Versioning.Unversioned;
});

/**
 * Returns every file under the current path.
 */
const allFiles = computed<BrowserObjectWrapper[]>(() => {
    if (props.forceEmpty) return [];

    return obStore.sortedFiles.map<BrowserObjectWrapper>(file => {
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
 * Handles items per page change event.
 */
function onLimitChange(newLimit: number): void {
    obStore.setCursor({ page: 1, limit: newLimit });
    obStore.clearPageTokens();
    fetchFiles();
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
 * Handles download bucket action.
 */
function onDownloadFolder(object: BrowserObject): void {
    withTrialCheck(() => {
        folderToDownload.value = `${object.path ?? ''}${object.Key}`;
        isDownloadPrefixDialogShown.value = true;
    });
}

/**
 * Handles file click.
 */
function onFileClick(file: BrowserObject): void {
    if (isFetching.value) return;

    withTrialCheck(() => {
        withLoading(async () => {
            if (!file.type) return;

            if (file.type === 'folder') {
                const uriParts = [file.Key];
                if (filePath.value) {
                    uriParts.unshift(...filePath.value.split('/'));
                }
                const pathAndKey = uriParts.map(part => encodeURIComponent(part)).join('/');
                await router.push(`${ROUTES.Projects.path}/${projectsStore.state.selectedProject.urlId}/${ROUTES.Buckets.path}/${bucketName.value}/${pathAndKey}`);
                return;
            }

            obStore.setObjectPathForModal((file.path ?? '') + file.Key);
            fileToPreview.value = file;
            previewDialog.value = true;

            analyticsStore.eventTriggered(AnalyticsEvent.GALLERY_VIEW_CLICKED);
        });
    });
}

/**
 * Fetches all files in the current directory.
 */
async function fetchFiles(page = 1, saveNextToken = true): Promise<void> {
    if (isFetching.value || props.forceEmpty) return;
    isFetching.value = true;

    try {
        const path = filePath.value ? filePath.value + '/' : '';

        await obStore.listSimplified(path, page, saveNextToken);
        selected.value = [];
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
    withTrialCheck(() => {
        fileToShare.value = file;
        isShareDialogShown.value = true;
    });
}

/**
 * Handles lock object button click event.
 */
function onLockObjectClick(file: BrowserObject): void {
    withTrialCheck(() => {
        lockActionFile.value = file;
        isLockDialogShown.value = true;
    });
}

/**
 * Handles legal hold button click event.
 */
function onLegalHoldClick(file: BrowserObject): void {
    withTrialCheck(() => {
        lockActionFile.value = file;
        isLegalHoldDialogShown.value = true;
    });
}

/**
 * Handles locked object delete error.
 */
function onLockedObjectDelete(file: FullBrowserObject): void {
    lockActionFile.value = file;
    isLockedObjectDeleteDialogShown.value = true;
}

/**
 * Get the object preview url.
 */
async function fetchPreviewUrl(file: BrowserObject): Promise<void> {
    let url = '';
    try {
        url = await obStore.getDownloadLink(file);
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
            fileToDelete.value = null;
            obStore.updateSelectedFiles([]);
        });
    }
});

watch(filePath, () => {
    // Ensure cursor has valid page size for card view before fetching
    const validSizes = pageSizes.map(s => s.value);
    if (!validSizes.includes(cursor.value.limit)) {
        obStore.setCursor({ page: 1, limit: pageSizes[0].value });
    }

    obStore.clearTokens();
    obStore.clearPageTokens();
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

watch([sortKey, sortOrder], ([key, order]) => {
    obStore.setSort(key, order);
});

watch(() => search.value, () => {
    clearTimeout(searchTimer.value);

    searchTimer.value = setTimeout(() => {
        analyticsStore.eventTriggered(AnalyticsEvent.SEARCH_BUCKETS);
    }, 500); // 500ms delay for every new call.
});

onBeforeUnmount(() => {
    obStore.setCursor({ page: 1, limit: DEFAULT_PAGE_LIMIT });
});

defineExpose({
    refresh: async () => {
        await fetchFiles(cursor.value.page);
    },
});
</script>
