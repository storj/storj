// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
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
        class="mb-5"
    />

    <v-data-table-server
        v-model="selectedFiles"
        v-model:options="options"
        :sort-by="sortBy"
        :headers="headers"
        :items="filteredFiles"
        :item-value="(item: BrowserObjectWrapper) => item.browserObject.path + item.browserObject.Key"
        :page="cursor.page"
        hover
        select-strategy="page"
        show-select
        :loading="isFetching || loading"
        :items-length="cursor.limit"
        elevation="1"
        @update:items-per-page="onLimitChange"
        @update:sort-by="onSortByChange"
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

        <template #bottom>
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
                        <span class="text-body-2">{{ pageDisplayText }}</span>
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
    <template v-if="configStore.isDefaultBrand">
        <share-dialog
            v-model="isShareDialogShown"
            :bucket-name="bucketName"
            :file="fileToShare || undefined"
            @content-removed="fileToShare = null"
        />
        <download-prefix-dialog
            v-if="downloadPrefixEnabled"
            v-model="isDownloadPrefixDialogShown"
            :prefix-type="DownloadPrefixType.Folder"
            :bucket="bucketName"
            :prefix="folderToDownload"
        />
    </template>
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
    ObjectBrowserCursor,
    useObjectBrowserStore,
} from '@/store/modules/objectBrowserStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useNotify } from '@/composables/useNotify';
import { Size } from '@/utils/bytesSize';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { DataTableHeader, SortItem } from '@/types/common';
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

type SortKey = 'name' | 'size' | 'date';

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
const isDownloadPrefixDialogShown = ref<boolean>(false);
const folderToDownload = ref<string>('');
const searchTimer = ref<NodeJS.Timeout>();

const pageSizes = [DEFAULT_PAGE_LIMIT, 25, 50, 100, 500];

const sortBy = computed<SortItem[]>(() => [{ key: obStore.state.headingSorted, order: obStore.state.orderBy }]);

const downloadPrefixEnabled = computed<boolean>(() => configStore.state.config.downloadPrefixEnabled);

/**
 * Returns table headers.
 */
const headers = computed<DataTableHeader[]>(() => {
    return [
        { title: 'Name', align: 'start', key: 'name', sortable: true },
        { title: 'Type', key: 'type', sortable: false },
        { title: 'Size', key: 'size', sortable: true },
        { title: 'Date', key: 'date', sortable: true },
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
 * Returns table cursor from store.
 */
const cursor = computed<ObjectBrowserCursor>(() => obStore.state.cursor);

/**
 * Check if we have a token for the next page.
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
 * Returns files filtered by the current search term.
 */
const filteredFiles = computed<BrowserObjectWrapper[]>(() => {
    if (!search.value) return allFiles.value;
    const query = search.value.toLowerCase();
    return allFiles.value.filter(f =>
        f.browserObject.Key.toLowerCase().includes(query) ||
        f.typeInfo.title.toLowerCase().includes(query),
    );
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
 * Handles items per page change event.
 */
function onLimitChange(newLimit: number): void {
    obStore.setCursor({ page: 1, limit: newLimit });
    obStore.clearPageTokens();
    fetchFiles();
}

function onSortByChange(val: SortItem[]): void {
    if (!val.length) {
        obStore.setSort('name', 'asc');
        return;
    }

    obStore.setSort(val[0].key as SortKey, val[0].order as 'asc' | 'desc');
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

            analyticsStore.eventTriggered(AnalyticsEvent.GALLERY_VIEW_CLICKED);
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

        await obStore.listSimplified(path, page, saveNextToken);
        selectedFiles.value = [];
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
    obStore.clearPageTokens();
    fetchFiles();
}, { immediate: true });

watch(() => props.forceEmpty, v => !v && fetchFiles());

watch(() => search.value, () => {
    clearTimeout(searchTimer.value);

    searchTimer.value = setTimeout(() => {
        analyticsStore.eventTriggered(AnalyticsEvent.SEARCH_BUCKETS);
    }, 500); // 500ms delay for every new call.
});

defineExpose({
    refresh: async () => {
        await fetchFiles(cursor.value.page);
    },
});
</script>
