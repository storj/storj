// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card>
        <v-text-field
            v-model="search"
            label="Search"
            :prepend-inner-icon="mdiMagnify"
            single-line
            variant="solo-filled"
            flat
            hide-details
            clearable
            density="comfortable"
            rounded="lg"
            class="mx-2 mt-2"
            @update:modelValue="analyticsStore.eventTriggered(AnalyticsEvent.SEARCH_BUCKETS)"
        />

        <v-data-table-server
            v-model="selectedFiles"
            v-model:options="options"
            v-model:expanded="expandedFiles"
            :sort-by="sortBy"
            :headers="headers"
            :items="tableFiles"
            :search="search"
            :item-value="(item: BrowserObjectWrapper) => item.browserObject"
            :page="cursor.page"
            hover
            must-sort
            select-strategy="page"
            show-select
            :loading="isFetching || loading"
            :items-length="isPaginationEnabled ? totalObjectCount : allFiles.length"
            :items-per-page-options="isPaginationEnabled ? tableSizeOptions(totalObjectCount, true) : undefined"
            :show-expand="showObjectVersions"
            @update:page="onPageChange"
            @update:itemsPerPage="onLimitChange"
        >
            <!-- the key of the row is defined by :item-value="(item: BrowserObjectWrapper) => item.browserObject" above -->
            <template #expanded-row="{ columns, internalItem: { key } }">
                <template v-if="!versionsCache.get(key.path + key.Key)?.length">
                    <tr>
                        <td :colspan="columns.length">
                            <p class="text-center">No older versions stored</p>
                        </td>
                    </tr>
                </template>
                <tr v-for="file in versionsCache.get(key.path + key.Key) as BrowserObject[]" v-else :key="file.VersionId" class="bg-altbg">
                    <td class="v-data-table__td v-data-table-column--no-padding v-data-table-column--align-start">
                        <v-checkbox-btn :model-value="selectedFiles.includes(file)" hide-details @update:modelValue="(selected) => toggleSelectObjectVersion(selected as boolean, file)" />
                    </td>
                    <td>
                        <v-list-item class="rounded-lg text-caption pl-1 ml-n1" link @click="() => onFileClick(file)">
                            <template #prepend>
                                <icon-curve-right />
                                <icon-versioning-clock class="ml-4 mr-3" size="32" dotted />
                            </template>
                            {{ file.Key }}
                        </v-list-item>
                    </td>
                    <td>
                        <p class="text-caption">
                            {{ getFormattedSize(file) }}
                        </p>
                    </td>
                    <td>
                        <p class="text-caption">
                            {{ getFileInfo(file).typeInfo.title }}
                        </p>
                    </td>
                    <td>
                        <p class="text-caption">
                            {{ getFormattedDate(file) }}
                        </p>
                    </td>
                    <td>
                        <browser-row-actions
                            :file="file"
                            is-version
                            align="right"
                            @preview-click="onFileClick(file)"
                            @delete-file-click="onDeleteFileClick(file)"
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
            <template #item="{ props: rowProps }">
                <v-data-table-row v-bind="rowProps">
                    <template v-if="rowProps.item.raw.browserObject.type === 'folder'" #item.data-table-expand />
                    <template #item.name="{ item }: ItemSlotProps">
                        <v-btn
                            class="rounded-lg w-100 px-1 ml-n1 justify-start font-weight-bold"
                            variant="text"
                            height="40"
                            color="default"
                            block
                            @click="onFileClick(item.browserObject)"
                        >
                            <img :src="item.typeInfo.icon" :alt="item.typeInfo.title + 'icon'" class="mr-3">
                            <v-tooltip
                                v-if="firstFile && item.browserObject.Key === firstFile.Key"
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
                                    <span v-bind="activatorProps">{{ item.browserObject.Key }}</span>
                                </template>
                            </v-tooltip>
                            <template v-else>{{ item.browserObject.Key }}</template>
                        </v-btn>
                    </template>

                    <template #item.type="{ item }: ItemSlotProps">
                        {{ item.typeInfo.title }}
                    </template>

                    <template #item.size="{ item }: ItemSlotProps">
                        {{ getFormattedSize(item.browserObject) }}
                    </template>

                    <template #item.date="{ item }: ItemSlotProps">
                        <span class="text-no-wrap">{{ getFormattedDate(item.browserObject) }}</span>
                    </template>

                    <template #item.actions="{ item }: ItemSlotProps">
                        <browser-row-actions
                            :file="item.browserObject"
                            align="right"
                            @preview-click="onFileClick(item.browserObject)"
                            @delete-file-click="onDeleteFileClick(item.browserObject)"
                            @share-click="onShareClick(item.browserObject)"
                        />
                    </template>
                </v-data-table-row>
            </template>
        </v-data-table-server>

        <file-preview-dialog
            v-model="previewDialog"
            v-model:current-file="fileToPreview"
            :showing-versions="!!fileToPreview?.VersionId"
        />
    </v-card>

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
                        color="default"
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

    <delete-file-dialog
        v-model="isDeleteFileDialogShown"
        :files="filesToDelete"
        @files-deleted="clearDeleteFiles"
        @content-removed="fileToDelete = null"
    />
    <share-dialog
        v-model="isShareDialogShown"
        :bucket-name="bucketName"
        :file="fileToShare || undefined"
        @content-removed="fileToShare = null"
    />
</template>

<script setup lang="ts">
import { computed, ref, watch, WritableComputedRef } from 'vue';
import { useRouter } from 'vue-router';
import {
    VBtn,
    VCard,
    VCheckboxBtn,
    VCol,
    VDataTableRow,
    VDataTableServer,
    VListItem,
    VRow,
    VSnackbar,
    VTextField,
    VTooltip,
} from 'vuetify/components';
import { mdiMagnify } from '@mdi/js';

import {
    BrowserObject,
    MAX_KEY_COUNT,
    ObjectBrowserCursor,
    useObjectBrowserStore,
} from '@/store/modules/objectBrowserStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useNotify } from '@/utils/hooks';
import { Size } from '@/utils/bytesSize';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { tableSizeOptions } from '@/types/common';
import { BrowserObjectTypeInfo, BrowserObjectWrapper, EXTENSION_INFOS, FILE_INFO, FOLDER_INFO } from '@/types/browser';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { ROUTES } from '@/router';
import { Time } from '@/utils/time';
import { Versioning } from '@/types/versioning';
import { BucketMetadata } from '@/types/buckets';

import BrowserRowActions from '@/components/BrowserRowActions.vue';
import FilePreviewDialog from '@/components/dialogs/FilePreviewDialog.vue';
import DeleteFileDialog from '@/components/dialogs/DeleteFileDialog.vue';
import ShareDialog from '@/components/dialogs/ShareDialog.vue';
import IconTrash from '@/components/icons/IconTrash.vue';
import IconCurveRight from '@/components/icons/IconCurveRight.vue';
import IconVersioningClock from '@/components/icons/IconVersioningClock.vue';

type SortKey = 'name' | 'type' | 'size' | 'date';

type TableOptions = {
    page: number;
    itemsPerPage: number;
    sortBy: {
        key: SortKey;
        order: 'asc' | 'desc';
    }[];
};

type ItemSlotProps = { item: BrowserObjectWrapper };

const props = defineProps<{
    forceEmpty?: boolean;
    loading?: boolean;
    bucket: BucketMetadata;
}>();

const emit = defineEmits<{
    uploadClick: [];
}>();

const analyticsStore = useAnalyticsStore();
const config = useConfigStore();
const obStore = useObjectBrowserStore();
const projectsStore = useProjectsStore();
const bucketsStore = useBucketsStore();
const userStore = useUsersStore();

const notify = useNotify();
const router = useRouter();

const isFetching = ref<boolean>(false);
const search = ref<string>('');
const previewDialog = ref<boolean>(false);
const options = ref<TableOptions>();
const fileToDelete = ref<BrowserObject | null>(null);
const fileToPreview = ref<BrowserObject | null>(null);
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

/**
 * Returns the name of the selected bucket.
 */
const bucketName = computed<string>(() => bucketsStore.state.fileComponentBucketName);

/**
 * Returns the current path within the selected bucket.
 */
const filePath = computed<string>(() => bucketsStore.state.fileComponentPath);

/**
 * Whether versioning has been enabled for current project and versions should be shown.
 */
const showObjectVersions = computed(() => {
    if (!projectsStore.versioningUIEnabled) {
        return false;
    }
    return obStore.state.showObjectVersions && props.bucket && props.bucket?.versioning !== Versioning.NotSupported;
});

const isPaginationEnabled = computed<boolean>(() => config.state.config.objectBrowserPaginationEnabled);

const versionsCache = computed<Map<string, BrowserObject[]>>(() => obStore.state.objectVersions);

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
 * Returns total object count from store.
 */
const totalObjectCount = computed<number>(() => obStore.state.totalObjectCount);

/**
 * Returns table cursor from store.
 */
const cursor = computed<ObjectBrowserCursor>(() => obStore.state.cursor);

/**
 * Returns every file under the current path.
 */
const allFiles = computed<BrowserObjectWrapper[]>(() => {
    if (props.forceEmpty) return [];

    const objects = isPaginationEnabled.value ? obStore.displayedObjects : obStore.state.files;
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
    return obStore.state.selectedFiles;
});

/**
 * Returns the first browser object in the table that is a file.
 */
const firstFile = computed<BrowserObject | null>(() => {
    return tableFiles.value.find(f => f.browserObject.type === 'file')?.browserObject || null;
});

function clearDeleteFiles(): void {
    fileToDelete.value = null;
    obStore.updateSelectedFiles([]);
}

/**
 * Handles page change event.
 */
function onPageChange(page: number): void {
    obStore.updateSelectedFiles([]);
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
    isFileGuideShown.value = false;
    dismissFileGuide();
}

/**
 * Fetches all files in the current directory.
 */
async function fetchFiles(): Promise<void> {
    if (isFetching.value || props.forceEmpty) return;

    obStore.updateSelectedFiles([]);
    obStore.updateVersionsExpandedKeys([]);
    isFetching.value = true;

    try {
        const path = filePath.value ? filePath.value + '/' : '';

        if (isPaginationEnabled.value) {
            await obStore.initList(path);
        } else {
            await obStore.list(path);
        }

        selectedFiles.value = [];

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

async function dismissFileGuide() {
    try {
        const noticeDismissal = { ...userStore.state.settings.noticeDismissal };
        noticeDismissal.fileGuide = true;
        await userStore.updateSettings({ noticeDismissal });
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.FILE_BROWSER);
    }
}

watch(filePath, fetchFiles, { immediate: true });
watch(() => props.forceEmpty, v => !v && fetchFiles());

// watch which table rows are expanded and fetch their versions.
watch(expandedFiles, (objects, oldObjects) => {
    const newObjects = objects.filter(obj => {
        return !oldObjects?.some(oldObj => {
            return oldObj.path + oldObj.Key === obj.path + obj.Key;
        });
    });
    newObjects.forEach(obj => {
        obStore.listVersions(obj.path + obj.Key);
    });
});

watch(() => obStore.state.showObjectVersions, showObjectVersions => {
    if (!showObjectVersions) {
        obStore.updateVersionsExpandedKeys([]);
    }
});

if (!userStore.noticeDismissal.fileGuide) {
    const unwatch = watch(firstFile, () => {
        isFileGuideShown.value = true;
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
