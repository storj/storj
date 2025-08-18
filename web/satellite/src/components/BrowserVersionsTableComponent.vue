// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <!-- We cast expandedFiles to type 'any' because of the weird Vuetify limitation/bug -->
    <!-- https://github.com/vuetifyjs/vuetify/issues/20006 -->
    <v-data-table-server
        v-model="selectedFiles"
        v-model:expanded="expandedFiles as any"
        :loading="isFetching || loading"
        :headers="headers"
        :items="filesAndVersions"
        select-strategy="page"
        show-select
        show-expand
        :item-value="(item: BrowserObjectWrapper) => item.browserObject"
        :items-length="allFiles.length"
        :item-selectable="(item: BrowserObjectWrapper) => !item.browserObject.Versions?.length"
        :must-sort="false"
        hover
    >
        <!-- the key of the row is defined by :item-value="(item: BrowserObjectWrapper) => item.browserObject" above -->
        <template #expanded-row="{ columns, item }">
            <template v-if="!item.browserObject.Versions?.length">
                <tr>
                    <td :colspan="columns.length">
                        <p class="text-center">No older versions stored</p>
                    </td>
                </tr>
            </template>
            <tr v-for="(file) in item.browserObject.Versions" v-else :key="file.VersionId" class="bg-altbg">
                <td class="v-data-table__td v-data-table-column--no-padding v-data-table-column--align-start">
                    <v-checkbox-btn
                        :model-value="selectedFiles.includes(file)"
                        hide-details
                        @update:model-value="(selected) => toggleSelectObjectVersion(selected as boolean, file)"
                    />
                </td>
                <td>
                    <v-btn
                        class="text-caption pl-1 pr-3 ml-n1 justify-start rounded-lg w-100"
                        variant="text"
                        color="default"
                        block
                        :disabled="filesBeingDeleted.has(file.path + file.Key + file.VersionId)"
                        @click="() => onFileClick(file)"
                    >
                        <template #prepend>
                            <icon-curve-right />
                            <icon-versioning-clock class="ml-4 mr-3" size="32" dotted />
                        </template>
                        {{ file.Key }}
                        <v-chip v-if="file.isLatest" class="ml-2" size="small" variant="tonal" color="primary">LATEST</v-chip>
                    </v-btn>
                </td>
                <td>
                    <p class="text-caption">
                        <v-chip v-if="file.isDeleteMarker" size="small" variant="tonal" color="warning">Delete Marker</v-chip>
                        <template v-else>
                            {{ getFileInfo(file).typeInfo.title }}
                        </template>
                    </p>
                </td>
                <td>
                    <span class="text-caption text-no-wrap">{{ getFormattedSize(file) }}</span>
                </td>
                <td>
                    <span class="text-caption text-no-wrap">{{ getFormattedDate(file) }}</span>
                </td>
                <td>
                    <p class="text-caption">
                        <v-hover v-if="file.VersionId">
                            <template #default="{ isHovering, props }">
                                <v-chip
                                    v-bind="props"
                                    size="small"
                                    :variant="isHovering ? 'tonal' : 'text'"
                                    class="cursor-pointer"
                                    @click="() => copyToClipboard(file.VersionId)"
                                >
                                    <template #append>
                                        <v-icon class="ml-2" :class="{ 'invisible': !isHovering }" :icon="Copy" />
                                    </template>
                                    <template #default>
                                        <span>{{ '...' + file.VersionId.slice(-9) }}</span>
                                    </template>
                                </v-chip>
                            </template>
                        </v-hover>
                    </p>
                </td>
                <td>
                    <browser-row-actions
                        :deleting="isBeingDeleted(file)"
                        :file="file"
                        is-version
                        :is-file-deleted="item.browserObject.isDeleteMarker"
                        align="right"
                        @share-click="onShareClick(file)"
                        @preview-click="onFileClick(file)"
                        @delete-file-click="onDeleteFileClick(file)"
                        @restore-object-click="onRestoreObjectClick(file)"
                        @lock-object-click="onLockObjectClick(file)"
                        @legal-hold-click="onLegalHoldClick(file)"
                        @locked-object-delete="(fullObject) => onLockedObjectDelete(fullObject)"
                    />
                </td>
                <td />
            </tr>
        </template>

        <template #no-data>
            <p class="text-body-2 cursor-pointer py-14 rounded-xlg my-4" @click="emit('uploadClick')">
                {{ search ? 'No data found' : 'Drag and drop files or folders here, or click to upload files.' }}
            </p>
        </template>

        <template #item="{ props: { item } }">
            <tr v-if="shouldRenderRow(item.raw.browserObject)">
                <td class="v-data-table__td v-data-table-column--no-padding v-data-table-column--align-start">
                    <v-checkbox-btn
                        :model-value="areAllVersionsSelected(item.raw.browserObject)"
                        :indeterminate="areSomeVersionsSelected(item.raw.browserObject)"
                        hide-details
                        @update:model-value="(selected) => updateSelectedVersions(item.raw.browserObject, selected)"
                    />
                </td>
                <td>
                    <v-btn
                        class="rounded-lg w-100 pl-1 pr-3 ml-n1 justify-start font-weight-bold"
                        variant="text"
                        height="40"
                        color="default"
                        block
                        :disabled="filesBeingDeleted.has(item.raw.browserObject.path + item.raw.browserObject.Key)"
                        @click="onFileClick(item.raw.browserObject)"
                    >
                        <img :src="item.raw.typeInfo.icon" :alt="item.raw.typeInfo.title + 'icon'" class="mr-3">
                        {{ item.raw.browserObject.Key }}
                    </v-btn>
                </td>
                <td>
                    <p class="text-caption">
                        <v-chip v-if="item.raw.browserObject.isDeleteMarker" size="small" variant="tonal" color="warning">Delete Marker</v-chip>
                        <template v-else>
                            {{ item.raw.typeInfo.title }}
                        </template>
                    </p>
                </td>
                <td />
                <td />
                <td />
                <td />
                <td class="text-right">
                    <VBtn
                        v-if="!isFolder(item.raw.browserObject)"
                        :icon="getExpandOrCollapseIcon(item.raw.browserObject)"
                        size="small"
                        variant="text"
                        @click="toggleFileExpanded(item.raw.browserObject)"
                    />
                    <browser-row-actions
                        v-else
                        :deleting="isBeingDeleted(item.raw.browserObject)"
                        :file="item.raw.browserObject"
                        align="right"
                        @preview-click="onFileClick(item.raw.browserObject)"
                        @share-click="onShareClick(item.raw.browserObject)"
                        @delete-file-click="onDeleteFileClick(item.raw.browserObject)"
                        @download-folder-click="onDownloadFolder(item.raw.browserObject)"
                    />
                </td>
            </tr>
        </template>

        <template #bottom>
            <div class="v-data-table-footer">
                <v-row justify="end" align="center">
                    <v-col cols="auto">
                        <span class="caption">Items per page:</span>
                    </v-col>
                    <v-col cols="auto">
                        <v-select
                            v-model="cursor.limit"
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
        :versions="fileVersionsToPreview"
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
                            <icon-trash />
                        </template>
                        Delete
                    </v-btn>
                </div>
            </v-col>
        </v-row>
    </v-snackbar>

    <delete-versions-dialog
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
    <restore-version-dialog
        v-model="isRestoreDialogShown"
        :file="fileToRestore || undefined"
        @file-restored="refreshPage"
        @content-removed="fileToRestore = null"
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
    VCheckboxBtn,
    VChip,
    VCol,
    VDataTableServer,
    VRow,
    VSelect,
    VSnackbar,
    VHover,
    VIcon,
} from 'vuetify/components';
import { ChevronLeft, ChevronRight, Copy } from 'lucide-vue-next';

import {
    BrowserObject,
    FullBrowserObject,
    ObjectBrowserCursor,
    useObjectBrowserStore,
} from '@/store/modules/objectBrowserStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useNotify } from '@/composables/useNotify';
import { Size } from '@/utils/bytesSize';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import {
    BrowserObjectTypeInfo,
    BrowserObjectWrapper,
    DownloadPrefixType,
    EXTENSION_INFOS,
    FILE_INFO,
    FOLDER_INFO,
} from '@/types/browser';
import { ROUTES } from '@/router';
import { Time } from '@/utils/time';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';
import { DataTableHeader } from '@/types/common';
import { usePreCheck } from '@/composables/usePreCheck';
import { useConfigStore } from '@/store/modules/configStore';
import { useLoading } from '@/composables/useLoading';

import BrowserRowActions from '@/components/BrowserRowActions.vue';
import FilePreviewDialog from '@/components/dialogs/FilePreviewDialog.vue';
import ShareDialog from '@/components/dialogs/ShareDialog.vue';
import RestoreVersionDialog from '@/components/dialogs/RestoreVersionDialog.vue';
import IconTrash from '@/components/icons/IconTrash.vue';
import IconCurveRight from '@/components/icons/IconCurveRight.vue';
import IconVersioningClock from '@/components/icons/IconVersioningClock.vue';
import DeleteVersionsDialog from '@/components/dialogs/DeleteVersionsDialog.vue';
import LockObjectDialog from '@/components/dialogs/LockObjectDialog.vue';
import LockedDeleteErrorDialog from '@/components/dialogs/LockedDeleteErrorDialog.vue';
import LegalHoldObjectDialog from '@/components/dialogs/LegalHoldObjectDialog.vue';
import DownloadPrefixDialog from '@/components/dialogs/DownloadPrefixDialog.vue';

const compProps = defineProps<{
    forceEmpty?: boolean;
    loading?: boolean;
}>();

const emit = defineEmits<{
    uploadClick: [];
}>();

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
const fileToDelete = ref<BrowserObject | null>(null);
const fileToRestore = ref<BrowserObject | null>(null);
const fileToPreview = ref<BrowserObject | undefined>(undefined);
const lockActionFile = ref<BrowserObject | null>(null);
const fileVersionsToPreview = ref<BrowserObject[]>();
const isDeleteFileDialogShown = ref<boolean>(false);
const fileToShare = ref<BrowserObject | null>(null);
const isShareDialogShown = ref<boolean>(false);
const isRestoreDialogShown = ref<boolean>(false);
const isLockDialogShown = ref<boolean>(false);
const isLegalHoldDialogShown = ref<boolean>(false);
const isLockedObjectDeleteDialogShown = ref<boolean>(false);
const isDownloadPrefixDialogShown = ref<boolean>(false);
const folderToDownload = ref<string>('');

const pageSizes = [DEFAULT_PAGE_LIMIT, 25, 50, 100];

/**
 * Returns table headers.
 */
const headers = computed<DataTableHeader[]>(() => {
    return [
        { title: 'Name', align: 'start', key: 'name', sortable: false },
        { title: 'Type', key: 'type', sortable: false },
        { title: 'Size', key: 'size', sortable: false },
        { title: 'Date', key: 'date', sortable: false },
        { title: 'Version ID', key: 'versionId', sortable: false },
        { title: '', key: 'actions', sortable: false, width: 0 },
    ];
});

const downloadPrefixEnabled = computed<boolean>(() => configStore.state.config.downloadPrefixEnabled);

/**
 * Returns the name of the selected bucket.
 */
const bucketName = computed<string>(() => bucketsStore.state.fileComponentBucketName);

/**
 * Returns the current path within the selected bucket.
 */
const filePath = computed<string>(() => bucketsStore.state.fileComponentPath);

const expandedFiles = computed<BrowserObject[]>({
    get: () => {
        const files = obStore.state.versionsExpandedKeys.map(name => {
            const parts = name.split('/');
            const key = parts.pop();
            const path = parts.join('/') + (parts.length ? '/' : '');
            return allFiles.value.find(f => f.browserObject.Key === key && f.browserObject.path === path)?.browserObject;
        });
        return files.filter(f => f !== undefined) as BrowserObject[];
    },
    set: (files: BrowserObject[]) => obStore.updateVersionsExpandedKeys(files.map(f => f.path + f.Key)),
});

/**
 * Returns files being deleted from store.
 */
const filesBeingDeleted = computed((): Set<string> => obStore.state.filesToBeDeleted);

/**
 * Returns table cursor from store.
 */
const cursor = computed<ObjectBrowserCursor>(() => obStore.state.cursor);

/**
 * Returns every file under the current path.
 */
const allFiles = computed<BrowserObjectWrapper[]>(() => {
    if (compProps.forceEmpty) return [];

    return obStore.state.files.map<BrowserObjectWrapper>(file => {
        const { name, ext, typeInfo } = getFileInfo(file);
        return {
            browserObject: file,
            typeInfo,
            lowerName: name,
            ext,
        };
    });
});

const filesAndVersions = computed<BrowserObjectWrapper[]>(() => {
    if (compProps.forceEmpty) return [];

    const versions = allFiles.value.flatMap(parent => {
        return parent.browserObject.Versions?.map<BrowserObjectWrapper>(version => {
            const { name, ext, typeInfo } = getFileInfo(version);
            return {
                browserObject: version,
                typeInfo,
                lowerName: name,
                ext,
            };
        }) ?? [];
    });
    return [...allFiles.value, ...versions];
});

/**
 * Returns a list of path+keys for selected files in the table.
 */
const selectedFiles: WritableComputedRef<BrowserObject[]> = computed({
    get: () => obStore.state.selectedFiles,
    set: obStore.updateSelectedFiles,
});

/**
 * Returns the selected files to the delete dialog.
 */
const filesToDelete = computed<BrowserObject[]>(() => {
    if (fileToDelete.value) return [fileToDelete.value];
    return selectedFiles.value;
});

const continuationTokens = computed(() => obStore.state.continuationTokens);

const hasNextPage = computed(() => !!continuationTokens.value.get(cursor.value.page + 1));

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
    fetchFiles(cursor.value.page + 1);
}

function refreshPage(): void {
    fetchFiles(cursor.value.page, false);
    obStore.updateSelectedFiles([]);
}

/**
 * Handles items per page change event.
 */
function onLimitChange(newLimit: number): void {
    obStore.setCursor({ page: 1, limit: newLimit });
    obStore.clearTokens();
    fetchFiles();
}

function toggleSelectObjectVersion(isSelected: boolean, version: BrowserObject) {
    const selected = obStore.state.selectedFiles;
    if (isSelected) {
        obStore.updateSelectedFiles([...selected, version]);
    } else {
        obStore.updateSelectedFiles(selected.filter(f => f.VersionId !== version.VersionId));
    }
}

/**
 * Returns the string form of the file's last modified date.
 */
function getFormattedDate(file: BrowserObject): string {
    if (file.type === 'folder') return '';

    if (file.VersionId) {
        return Time.formattedDate(file.LastModified, { day: 'numeric', month: 'short', year: 'numeric', hour: 'numeric', minute: 'numeric' });
    }
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
 * Returns whether a file is a folder.
 */
function isFolder(file: BrowserObject): boolean {
    return file.type === 'folder';
}

/**
 * Returns whether a file row should be rendered.
 */
function shouldRenderRow(file: BrowserObject): boolean {
    return (file.Versions?.length ?? 0) > 0 || isFolder(file);
}

/**
 * Returns whether a files versions are all selected.
 */
function areAllVersionsSelected(file: BrowserObject): boolean {
    if (file.type === 'folder') {
        return selectedFiles.value.includes(file);
    }
    return !!file.Versions?.every(v => selectedFiles.value.includes(v));
}

/**
 * Returns whether some of a files versions are selected.
 */
function areSomeVersionsSelected(file: BrowserObject): boolean {
    return !!file.Versions?.some(v => selectedFiles.value.includes(v)) && !areAllVersionsSelected(file);
}

/**
 * toggles whether a file row is expanded.
 */
function toggleFileExpanded(file: BrowserObject): void {
    if (expandedFiles.value.includes(file)) {
        expandedFiles.value = expandedFiles.value.filter(f => f !== file);
    } else {
        expandedFiles.value = [...expandedFiles.value, file];
    }
}

/**
 * Handles version selection.
 */
function updateSelectedVersions(file: BrowserObject, selected: boolean): void {
    if (file.type === 'folder') {
        if (selected) {
            obStore.updateSelectedFiles([...selectedFiles.value, file]);
        } else {
            obStore.updateSelectedFiles(selectedFiles.value.filter(f => f !== file));
        }
        return;
    }
    if (selected) {
        obStore.updateSelectedFiles([...selectedFiles.value, ...file.Versions ?? []]);
    } else {
        obStore.updateSelectedFiles(selectedFiles.value.filter(f => !file.Versions?.includes(f)));
    }
}

function getExpandOrCollapseIcon(file: BrowserObject): string {
    return expandedFiles.value.includes(file) ? '$collapse' : '$expand';
}

function isBeingDeleted(file: BrowserObject): boolean {
    return filesBeingDeleted.value.has(file.path + file.Key) || filesBeingDeleted.value.has(file.path + file.Key + file.VersionId);
}

/**
 * Handles file click.
 */
function onFileClick(file: BrowserObject): void {
    if (compProps.loading || isFetching.value) return;

    withTrialCheck(() => {
        withLoading(async () => {
            if (!file.type) return;
            if (file.isDeleteMarker) return;

            if (file.type === 'folder') {
                const uriParts = [file.Key];
                if (filePath.value) {
                    uriParts.unshift(...filePath.value.split('/'));
                }
                const pathAndKey = uriParts.map(part => encodeURIComponent(part)).join('/');
                await router.push(`${ROUTES.Projects.path}/${projectsStore.state.selectedProject.urlId}/${ROUTES.Buckets.path}/${bucketName.value}/${pathAndKey}`);
                return;
            }
            if (!file.VersionId) {
                return;
            }

            obStore.setObjectPathForModal((file.path ?? '') + file.Key);
            fileToPreview.value = file;
            const parentFile = allFiles.value.find(f => f.browserObject.Key === file.Key && f.browserObject.path === file.path);
            fileVersionsToPreview.value = parentFile?.browserObject?.Versions?.filter(v => !v.isDeleteMarker);
            previewDialog.value = true;
        });
    });
}

/**
 * Copies the version ID to the clipboard.
 */
function copyToClipboard(versionId?: string): void {
    if (!versionId) return;
    navigator.clipboard.writeText(versionId).then(() => {
        notify.success('Version ID copied to clipboard');
    }).catch(err => {
        notify.notifyError(err, AnalyticsErrorEventSource.FILE_BROWSER);
    });
}

async function fetchFiles(page = 1, saveNextToken = true): Promise<void> {
    if (isFetching.value || compProps.forceEmpty) return;

    obStore.updateSelectedFiles([]);
    obStore.updateVersionsExpandedKeys([]);
    isFetching.value = true;

    try {
        const path = filePath.value ? filePath.value + '/' : '';
        await obStore.listAllVersions(path, page, saveNextToken);
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
 * Handles restore button click event.
 */
function onRestoreObjectClick(file: BrowserObject): void {
    withTrialCheck(() => {
        fileToRestore.value = file;
        isRestoreDialogShown.value = true;
    });
}

/**
 * Handles lock button click event.
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
            fetchFiles(cursor.value.page);
            fileToDelete.value = null;
            obStore.updateSelectedFiles([]);
        });
    }
});

watch(filePath, () => {
    obStore.clearTokens();
    fetchFiles();
}, { immediate: true });
watch(() => compProps.forceEmpty, v => !v && fetchFiles());

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

    &__file-guide :deep(.v-overlay__content) {
        color: #fff !important;
        background-color: rgb(var(--v-theme-primary)) !important;
    }
}
</style>
