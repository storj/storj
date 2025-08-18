// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-text-field
        v-if="!isAltPagination"
        v-model="search"
        label="Search"
        :prepend-inner-icon="Search"
        single-line
        variant="solo-filled"
        flat
        hide-details
        clearable
        density="comfortable"
        class="mb-5"
        @update:model-value="analyticsStore.eventTriggered(AnalyticsEvent.SEARCH_BUCKETS)"
    />

    <v-data-table-server
        v-model="selectedFiles"
        v-model:options="options"
        :sort-by="sortBy"
        :headers="headers"
        :items="isAltPagination ? allFiles : tableFiles"
        :search="search"
        :item-value="(item: BrowserObjectWrapper) => item.browserObject.path + item.browserObject.Key"
        :page="cursor.page"
        hover
        :must-sort="!isAltPagination"
        :disable-sort="isAltPagination"
        select-strategy="page"
        show-select
        :loading="isFetching || loading"
        :items-length="isAltPagination ? cursor.limit : totalObjectCount"
        :items-per-page-options="isAltPagination ? [] : tableSizeOptions(totalObjectCount, true)"
        elevation="1"
        @update:page="onPageChange"
        @update:items-per-page="onLimitChange"
    >
        <template #no-data>
            <p class="text-body-2 cursor-pointer py-14 rounded-xlg my-4" @click="emit('uploadClick')">
                {{ search ? 'No data found' : 'Drag and drop files or folders here, or click to upload files.' }}
            </p>
        </template>
        <template #item="{ props: rowProps }">
            <v-data-table-row v-bind="rowProps">
                <template #item.name="{ item }">
                    <v-btn
                        class="rounded-lg w-100 pl-1 pr-3 ml-n1 justify-start font-weight-bold"
                        variant="text"
                        height="40"
                        color="default"
                        block
                        :disabled="filesBeingDeleted.has((item as BrowserObjectWrapper).browserObject.path + (item as BrowserObjectWrapper).browserObject.Key)"
                        @click="onFileClick((item as BrowserObjectWrapper).browserObject)"
                    >
                        <img :src="(item as BrowserObjectWrapper).typeInfo.icon" :alt="(item as BrowserObjectWrapper).typeInfo.title + 'icon'" class="mr-3">
                        {{ (item as BrowserObjectWrapper).browserObject.Key }}
                    </v-btn>
                </template>

                <template #item.type="{ item }">
                    {{ (item as BrowserObjectWrapper).typeInfo.title }}
                </template>

                <template #item.size="{ item }">
                    <span class="text-no-wrap">{{ getFormattedSize((item as BrowserObjectWrapper).browserObject) }}</span>
                </template>

                <template #item.date="{ item }">
                    <span class="text-no-wrap">{{ getFormattedDate((item as BrowserObjectWrapper).browserObject) }}</span>
                </template>

                <template #item.actions="{ item }">
                    <browser-row-actions
                        :deleting="filesBeingDeleted.has((item as BrowserObjectWrapper).browserObject.path + (item as BrowserObjectWrapper).browserObject.Key)"
                        :file="(item as BrowserObjectWrapper).browserObject"
                        align="right"
                        @preview-click="onFileClick((item as BrowserObjectWrapper).browserObject)"
                        @delete-file-click="onDeleteFileClick((item as BrowserObjectWrapper).browserObject)"
                        @share-click="onShareClick((item as BrowserObjectWrapper).browserObject)"
                        @lock-object-click="onLockObjectClick((item as BrowserObjectWrapper).browserObject)"
                        @legal-hold-click="onLegalHoldClick((item as BrowserObjectWrapper).browserObject)"
                        @locked-object-delete="(fullObject) => onLockedObjectDelete(fullObject)"
                        @download-folder-click="onDownloadFolder((item as BrowserObjectWrapper).browserObject)"
                    />
                </template>
            </v-data-table-row>
        </template>

        <template v-if="isAltPagination" #bottom>
            <div class="v-data-table-footer">
                <v-row justify="end" align="center" class="pa-2">
                    <v-col cols="auto">
                        <span class="caption">Items per page:</span>
                    </v-col>
                    <v-col cols="auto">
                        <v-select
                            :model-value="cursor.limit"
                            density="compact"
                            :items="pageSizes"
                            variant="outlined"
                            hide-details
                            @update:model-value="onLimitChange"
                        />
                    </v-col>
                    <v-col cols="auto">
                        <v-btn-group density="compact">
                            <v-btn :disabled="cursor.page <= 1" :icon="ChevronLeft" @click="onPreviousPageClick" />
                            <v-btn :disabled="!hasNextPage" :icon="ChevronRight" @click="onNextPageClick" />
                        </v-btn-group>
                    </v-col>
                </v-row>
            </div>
        </template>
    </v-data-table-server>

    <file-preview-dialog
        v-model="previewDialog"
        v-model:current-file="fileToPreview"
    />

    <v-snackbar
        rounded="lg"
        variant="elevated"
        color="surface"
        :model-value="!!selectedFiles.length"
        :timeout="-1"
        class="snackbar-multiple"
    >
        <v-row align="center" justify="space-between">
            <v-col>
                {{ selectedFiles.length }} items selected
            </v-col>
            <v-col>
                <div class="d-flex justify-end">
                    <v-btn
                        color="error"
                        density="comfortable"
                        variant="outlined"
                        @click="isDeleteFileDialogShown = true"
                    >
                        <template #prepend>
                            <component :is="Trash2" :size="18" />
                        </template>
                        Delete
                    </v-btn>
                </div>
            </v-col>
        </v-row>
    </v-snackbar>

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
import { computed, ref, watch, WritableComputedRef } from 'vue';
import { useRouter } from 'vue-router';
import {
    VBtn,
    VBtnGroup,
    VCol,
    VDataTableRow,
    VDataTableServer,
    VRow,
    VSelect,
    VSnackbar,
    VTextField,
} from 'vuetify/components';
import { ChevronLeft, ChevronRight, Search, Trash2 } from 'lucide-vue-next';

import {
    BrowserObject,
    FullBrowserObject,
    MAX_KEY_COUNT,
    ObjectBrowserCursor,
    useObjectBrowserStore,
} from '@/store/modules/objectBrowserStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useNotify } from '@/composables/useNotify';
import { Size } from '@/utils/bytesSize';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { DataTableHeader, SortItem, tableSizeOptions } from '@/types/common';
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
import { Time } from '@/utils/time';
import { BucketMetadata } from '@/types/buckets';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';
import { Versioning } from '@/types/versioning';
import { usePreCheck } from '@/composables/usePreCheck';
import { useConfigStore } from '@/store/modules/configStore';
import { useLoading } from '@/composables/useLoading';

import BrowserRowActions from '@/components/BrowserRowActions.vue';
import FilePreviewDialog from '@/components/dialogs/FilePreviewDialog.vue';
import DeleteFileDialog from '@/components/dialogs/DeleteFileDialog.vue';
import ShareDialog from '@/components/dialogs/ShareDialog.vue';
import DeleteVersionedFileDialog from '@/components/dialogs/DeleteVersionedFileDialog.vue';
import LockObjectDialog from '@/components/dialogs/LockObjectDialog.vue';
import LockedDeleteErrorDialog from '@/components/dialogs/LockedDeleteErrorDialog.vue';
import LegalHoldObjectDialog from '@/components/dialogs/LegalHoldObjectDialog.vue';
import DownloadPrefixDialog from '@/components/dialogs/DownloadPrefixDialog.vue';

type SortKey = 'name' | 'type' | 'size' | 'date';

type TableOptions = {
    page: number;
    itemsPerPage: number;
    sortBy: {
        key: SortKey;
        order: 'asc' | 'desc';
    }[];
};

const props = defineProps<{
    forceEmpty?: boolean;
    loading?: boolean;
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
const previewDialog = ref<boolean>(false);
const options = ref<TableOptions>();
const fileToDelete = ref<BrowserObject | null>(null);
const lockActionFile = ref<FullBrowserObject | null>(null);
const fileToPreview = ref<BrowserObject | undefined>();
const isDeleteFileDialogShown = ref<boolean>(false);
const fileToShare = ref<BrowserObject | null>(null);
const isShareDialogShown = ref<boolean>(false);
const isLockDialogShown = ref<boolean>(false);
const isLegalHoldDialogShown = ref<boolean>(false);
const isLockedObjectDeleteDialogShown = ref<boolean>(false);
const routePageCache = new Map<string, number>();
const isDownloadPrefixDialogShown = ref<boolean>(false);
const folderToDownload = ref<string>('');

const pageSizes = [DEFAULT_PAGE_LIMIT, 25, 50, 100];
const sortBy: SortItem[] = [{ key: 'name', order: 'asc' }];
const collator = new Intl.Collator('en', { sensitivity: 'case' });

const downloadPrefixEnabled = computed<boolean>(() => configStore.state.config.downloadPrefixEnabled);

/**
 * Indicates if alternative pagination should be used.
 */
const isAltPagination = computed(() => obStore.isAltPagination);

/**
 * Returns table headers.
 */
const headers = computed<DataTableHeader[]>(() => {
    return [
        { title: 'Name', align: 'start', key: 'name', sortable: !isAltPagination.value },
        { title: 'Type', key: 'type', sortable: !isAltPagination.value },
        { title: 'Size', key: 'size', sortable: !isAltPagination.value },
        { title: 'Date', key: 'date', sortable: !isAltPagination.value },
        { title: '', key: 'actions', sortable: false, width: 0 },
    ];
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
 * Returns files being deleted from store.
 */
const filesBeingDeleted = computed((): Set<string> => obStore.state.filesToBeDeleted);

/**
 * Returns total object count from store.
 */
const totalObjectCount = computed<number>(() => obStore.state.totalObjectCount);

/**
 * Returns table cursor from store.
 */
const cursor = computed<ObjectBrowserCursor>(() => obStore.state.cursor);

/**
 * Indicates if alternative pagination has next page.
 */
const hasNextPage = computed<boolean>(() => {
    const nextToken = obStore.state.continuationTokens.get(cursor.value.page + 1);

    return nextToken !== undefined;
});

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

    const objects = isAltPagination.value ? obStore.sortedFiles : obStore.displayedObjects;

    return objects.map<BrowserObjectWrapper>(file => {
        const { name, ext, typeInfo } = getFileInfo(file);
        return {
            browserObject: file,
            typeInfo,
            lowerName : name,
            ext,
        };
    });
});

/**
 * Returns every file under the current path that matches the search query.
 */
const filteredFiles = computed<BrowserObjectWrapper[]>(() => {
    if (isAltPagination.value) return [];
    if (!search.value) return allFiles.value;
    const searchLower = search.value.toLowerCase();
    return allFiles.value.filter(file => file.lowerName.includes(searchLower));
});

/**
 * Returns the files to be displayed in the table.
 */
const tableFiles = computed<BrowserObjectWrapper[]>(() => {
    const opts = options.value;
    if (!opts || isAltPagination.value) return [];

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

    return files;
});

/**
 * Returns a list of path+keys for selected files in the table.
 */
const selectedFiles: WritableComputedRef<string[]> = computed({
    get: () => obStore.state.selectedFiles.map(f => {
        return f.path + f.Key;
    }),
    set: (names: string[]) => {
        const files = names.map(name => {
            const parts = name.split('/');
            const key = parts.pop();
            const path = parts.join('/') + (parts.length ? '/' : '');
            return allFiles.value.find(f => f.browserObject.Key === key && f.browserObject.path === path)?.browserObject;
        });
        obStore.updateSelectedFiles(files.filter(f => f !== undefined) as BrowserObject[]);
    },
});

/**
 * Returns the selected files to the delete dialog.
 */
const filesToDelete = computed<BrowserObject[]>(() => {
    if (fileToDelete.value) return [fileToDelete.value];
    return obStore.state.selectedFiles;
});

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
 * Handles previous page click for alternative pagination.
 */
function onPreviousPageClick(): void {
    fetchFiles(cursor.value.page - 1, false);
}

/**
 * Handles next page click for alternative pagination.
 */
function onNextPageClick(): void {
    fetchFiles(cursor.value.page + 1, true);
}

/**
 * Handles page change event.
 */
function onPageChange(page: number): void {
    if (isAltPagination.value) return;

    obStore.updateSelectedFiles([]);
    const path = filePath.value ? filePath.value + '/' : '';
    routePageCache.set(path, page);
    obStore.setCursor({ page, limit: cursor.value.limit });

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
        obStore.setCursor({ page: options.value?.page ?? 1, limit: newLimit });
    }
}

/**
 * Returns the string form of the file's last modified date.
 */
function getFormattedDate(file: BrowserObject): string {
    if (file.type === 'folder') return '';

    return Time.formattedDate(file.LastModified);
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
function getFileInfo(file: BrowserObject): { name: string; ext: string; typeInfo: BrowserObjectTypeInfo } {
    const name = file.Key.toLowerCase();
    if (!file.type) return { name, ext: '', typeInfo: FILE_INFO };
    if (file.type === 'folder') return { name, ext: '', typeInfo: FOLDER_INFO };

    const dotIdx = name.lastIndexOf('.');
    const ext = dotIdx === -1 ? '' : file.Key.slice(dotIdx + 1).toLowerCase();
    for (const [exts, info] of EXTENSION_INFOS.entries()) {
        if (exts.indexOf(ext) !== -1) return { name, ext, typeInfo: info };
    }
    return { name, ext, typeInfo: FILE_INFO };
}

/**
 * Handles file click.
 */
function onFileClick(file: BrowserObject): void {
    if (props.loading || isFetching.value) return;

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
        });
    });
}

/**
 * Fetches all files in the current directory.
 */
async function fetchFiles(page = 1, saveNextToken = true): Promise<void> {
    if (isFetching.value || props.forceEmpty) return;

    obStore.updateSelectedFiles([]);
    obStore.updateVersionsExpandedKeys([]);
    isFetching.value = true;

    try {
        const path = filePath.value ? filePath.value + '/' : '';

        if (isAltPagination.value) {
            await obStore.listCustom(path, page, saveNextToken);
            selectedFiles.value = [];
        } else {
            await obStore.initList(path);

            selectedFiles.value = [];

            const cachedPage = routePageCache.get(path);
            if (cachedPage !== undefined) {
                obStore.setCursor({ limit: cursor.value.limit, page: cachedPage });
            } else {
                obStore.setCursor({ limit: cursor.value.limit, page: 1 });
            }
        }
    } catch (err) {
        err.message = `Error fetching objects. ${err.message}`;
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
    obStore.clearTokens();
    fetchFiles();
}, { immediate: true });
watch(() => props.forceEmpty, v => !v && fetchFiles());

defineExpose({
    refresh: async () => {
        await fetchFiles(cursor.value.page);
    },
});
</script>

<style scoped lang="scss">
.browser-table {

    &__loader-overlay :deep(.v-overlay__scrim) {
        opacity: 1;
        bottom: 0.8px;
    }
}
</style>
