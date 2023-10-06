// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="file-browser">
        <div v-if="isInitialized" class="row white-background" @click="closeModalDropdown">
            <div class="col-sm-12">
                <div
                    v-cloak
                    class="div-responsive"
                    @drop.prevent="upload"
                    @dragover.prevent="showDropzone"
                >
                    <Dropzone v-if="isOver" :bucket="bucketName" :close="hideDropzone" />

                    <bread-crumbs @onUpdate="onRouteChange" @bucketClick="goToBuckets" />

                    <div class="tile-action-bar">
                        <h2 class="tile-action-bar__title">{{ bucketName }}</h2>
                        <div class="tile-action-bar__actions">
                            <div v-click-outside="closeUploadDropdown" class="position-relative">
                                <button
                                    type="button"
                                    class="btn btn-sm btn-primary btn-block upload-button"
                                    @click.stop="toggleUploadDropdown"
                                >
                                    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
                                        <path d="M8.32666 0.974108L8.34573 0.992444L11.7281 4.37479C11.9849 4.63164 11.9849 5.04808 11.7281 5.30494C11.4775 5.55552 11.075 5.56164 10.817 5.32327L10.7979 5.30494L8.5164 3.02335V10.4543C8.5164 10.8175 8.22193 11.112 7.85869 11.112C7.5043 11.112 7.21538 10.8317 7.2015 10.4808L7.20097 10.4543V3.06712L4.96339 5.30494C4.7128 5.55552 4.31031 5.56164 4.05232 5.32327L4.03324 5.30494C3.78266 5.05435 3.77654 4.65186 4.01491 4.39386L4.03324 4.37479L7.41559 0.992444C7.66618 0.741856 8.06866 0.735744 8.32666 0.974108ZM15.2008 13.8745C15.2008 14.2378 14.9063 14.5322 14.5431 14.5322H1.50841C1.14517 14.5322 0.850702 14.2378 0.850702 13.8745C0.850702 13.5113 1.14517 13.2168 1.50841 13.2168H14.5431C14.9063 13.2168 15.2008 13.5113 15.2008 13.8745ZM1.45849 14.4823C1.09525 14.4823 0.800781 14.1878 0.800781 13.8246V11.1937C0.800781 10.8305 1.09525 10.536 1.45849 10.536C1.82174 10.536 2.11621 10.8305 2.11621 11.1937V13.8246C2.11621 14.1878 1.82174 14.4823 1.45849 14.4823ZM14.4931 14.4823C14.1299 14.4823 13.8354 14.1878 13.8354 13.8246V11.1937C13.8354 10.8305 14.1299 10.536 14.4931 10.536C14.8564 10.536 15.1509 10.8305 15.1509 11.1937V13.8246C15.1509 14.1878 14.8564 14.4823 14.4931 14.4823Z" fill="white" />
                                    </svg>
                                    Upload
                                    <span class="upload-button__divider" />
                                    <BlackArrowExpand :class="{ active: isUploadDropDownShown }" class="arrow" />
                                </button>
                                <input
                                    ref="fileInput"
                                    type="file"
                                    aria-roledescription="file-upload"
                                    hidden
                                    multiple
                                    @change="upload"
                                >
                                <input
                                    ref="folderInput"
                                    type="file"
                                    aria-roledescription="folder-upload"
                                    hidden
                                    multiple
                                    webkitdirectory
                                    mozdirectory
                                    @change="upload"
                                >
                                <div v-if="isUploadDropDownShown" class="dropdown">
                                    <div class="dropdown__item">
                                        <div
                                            class="upload-option"
                                            @click="buttonFileUpload"
                                        >
                                            <svg class="btn-icon" width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
                                                <path d="M8.75652 0.799805C9.13527 0.799805 9.49851 0.950259 9.76632 1.21807L13.5258 4.97747C13.7937 5.24529 13.9441 5.60854 13.9441 5.9873V12.9387C13.9441 14.1875 12.9318 15.1998 11.683 15.1998H4.66153C3.41274 15.1998 2.40039 14.1875 2.40039 12.9387V3.06104C2.40039 1.81225 3.41274 0.799805 4.66153 0.799805H8.75652ZM8.17213 2.10889H4.66153C4.14568 2.10889 3.72559 2.51926 3.70993 3.03139L3.70947 3.06104V12.9387C3.70947 13.4545 4.11979 13.8746 4.63188 13.8903L4.66153 13.8907H11.683C12.1989 13.8907 12.6189 13.4804 12.6346 12.9683L12.635 12.9387V6.57167L8.82679 6.57176C8.47412 6.57176 8.18659 6.29284 8.17277 5.94355L8.17225 5.91722L8.17213 2.10889ZM11.9597 5.26259L9.48122 2.78425L9.48134 5.26268L11.9597 5.26259Z" fill="black" />
                                            </svg>
                                            Upload File
                                        </div>
                                    </div>
                                    <div class="dropdown__item">
                                        <div
                                            class="upload-option"
                                            @click="buttonFolderUpload"
                                        >
                                            <svg class="btn-icon" width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
                                                <path d="M6.30544 1.63444C6.45643 1.66268 6.59063 1.70959 6.72634 1.78156L6.76941 1.80506C6.88832 1.87193 7.00713 1.95699 7.28196 2.18186L9.17564 3.73123H11.997C13.111 3.73123 13.515 3.84722 13.9223 4.06503C14.3295 4.28284 14.6492 4.60247 14.867 5.00975L14.8903 5.0542C15.0931 5.44734 15.2008 5.8615 15.2008 6.93502V11.104C15.2008 12.4302 15.0627 12.9111 14.8034 13.396C14.5441 13.8808 14.1636 14.2613 13.6787 14.5206L13.6302 14.5461L13.5824 14.5704C13.1346 14.7936 12.6429 14.9137 11.4512 14.918H4.61483C3.2886 14.918 2.80768 14.7799 2.32283 14.5206C1.83798 14.2613 1.45747 13.8808 1.19817 13.396L1.17264 13.3475C0.934003 12.8862 0.805265 12.4029 0.800781 11.1696V4.36463C0.800781 3.69226 0.909136 3.24683 1.1133 2.86088C1.31746 2.47493 1.61743 2.17098 2.00065 1.96174C2.38387 1.75251 2.82783 1.63828 3.50014 1.62941L5.70273 1.60054L5.81497 1.6001C6.06913 1.60045 6.18205 1.61136 6.30544 1.63444ZM7.08876 6.29289C6.82188 6.49471 6.49897 6.60794 6.16527 6.6174L6.1197 6.61805H3.03986C2.70795 6.61805 2.39296 6.54561 2.10984 6.41568V11.1649L2.11118 11.3259L2.11278 11.4328C2.12691 12.1945 2.19366 12.4815 2.35254 12.7786C2.48984 13.0353 2.68348 13.2289 2.94019 13.3662L2.97747 13.3857L3.01739 13.4054C3.30474 13.543 3.62468 13.5996 4.40102 13.6078L4.61483 13.6089L11.4963 13.6086C12.4225 13.6041 12.7384 13.539 13.0614 13.3662C13.3181 13.2289 13.5117 13.0353 13.649 12.7786L13.6685 12.7413L13.6882 12.7014C13.8257 12.414 13.8824 12.0941 13.8906 11.3178L13.8917 11.104V6.93502C13.8917 6.11696 13.8448 5.87439 13.7126 5.62711C13.6168 5.44797 13.484 5.31521 13.3049 5.21941L13.274 5.20332C13.0416 5.08595 12.7921 5.04222 12.0474 5.04038L8.74506 5.04026L7.08876 6.29289ZM5.75624 2.90917L5.6387 2.9104L3.51741 2.93839L3.42225 2.94052C3.03877 2.95287 2.81488 3.00869 2.62798 3.11073C2.47022 3.19687 2.35451 3.31411 2.27046 3.473C2.16869 3.66538 2.11572 3.89577 2.11033 4.29509C2.11339 4.31637 2.1153 4.33777 2.11616 4.35945L2.11668 4.38577C2.11668 4.88583 2.51425 5.29302 3.01055 5.3085L3.03986 5.30896H6.1197C6.17638 5.30896 6.2317 5.29277 6.27926 5.26255L6.29915 5.24874L7.68339 4.20185L6.38268 3.13773C6.22518 3.00999 6.16925 2.96959 6.12489 2.94458L6.11301 2.93809C6.09421 2.92812 6.08564 2.92512 6.06472 2.92121L6.03713 2.91681L6.01827 2.91465C5.97123 2.91001 5.90046 2.90815 5.75624 2.90917Z" fill="black" />
                                            </svg>
                                            Upload Folder
                                        </div>
                                    </div>
                                </div>
                            </div>

                            <div class="position-relative">
                                <v-button
                                    icon="folder"
                                    label="New Folder"
                                    height="44px"
                                    :is-white="true"
                                    font-size="14px"
                                    width="130px"
                                    :on-press="toggleFolderCreationModal"
                                />
                            </div>
                            <bucket-settings-nav class="new-folder-button" :bucket-name="bucket" />
                        </div>
                    </div>

                    <div class="hr-divider" />

                    <MultiplePassphraseBanner
                        v-if="lockedFilesEntryDisplayed && isLockedBanner"
                        :locked-files-count="lockedFilesCount"
                        :on-close="closeLockedBanner"
                    />

                    <TooManyObjectsBanner
                        v-if="!isPaginationEnabled && files.length >= NUMBER_OF_DISPLAYED_OBJECTS && isTooManyObjectsBanner"
                        :on-close="closeTooManyObjectsBanner"
                    />

                    <v-table
                        items-label="objects"
                        selectable
                        :selected="allFilesSelected"
                        :limit="isPaginationEnabled ? cursor.limit : 0"
                        :total-page-count="isPaginationEnabled ? pageCount : 0"
                        :total-items-count="isPaginationEnabled ? fetchedObjectsCount : files.length"
                        show-select
                        :loading="isLoading"
                        class="file-browser-table"
                        :on-page-change="isPaginationEnabled ? changePageAndLimit : null"
                        :page-number="cursor.page"
                        @selectAllClicked="toggleSelectAllFiles"
                    >
                        <template #head>
                            <file-browser-header />
                        </template>
                        <template #body>
                            <template v-if="!isNewUploadingModal">
                                <tr
                                    v-for="(file, index) in formattedFilesUploading"
                                    :key="index"
                                >
                                    <!-- using <th> to comply with common Vtable.vue-->
                                    <th class="hide-mobile icon" />
                                    <th
                                        class="align-left"
                                        aria-roledescription="file-uploading"
                                    >
                                        <p class="file-name">
                                            <file-icon />
                                            <span>{{ filename(file) }}</span>
                                        </p>
                                    </th>
                                    <th aria-roledescription="progress-bar">
                                        <div class="progress">
                                            <div
                                                class="progress-bar"
                                                role="progressbar"
                                                :style="{
                                                    width: `${file.progress}%`
                                                }"
                                            >
                                                {{ file.progress }}%
                                            </div>
                                        </div>
                                    </th>
                                    <th>
                                        <v-button
                                            width="60px"
                                            font-size="14px"
                                            label="Cancel"
                                            is-deletion
                                            :on-press="() => cancelUpload(file.Key)"
                                        />
                                    </th>
                                    <th class="hide-mobile" />
                                </tr>

                                <tr v-if="filesUploading.length" class="files-uploading-count">
                                    <th class="hide-mobile files-uploading-count__content icon" />
                                    <th class="align-left files-uploading-count__content" aria-roledescription="files-uploading-count">
                                        {{ formattedFilesWaitingToBeUploaded }}
                                        waiting to be uploaded...
                                    </th>
                                    <th class="hide-mobile files-uploading-count__content" />
                                    <th class="hide-mobile files-uploading-count__content" />
                                    <th class="files-uploading-count__content" />
                                </tr>
                            </template>

                            <up-entry v-if="path.length > 0" :on-back="onBack" />

                            <locked-files-entry v-if="lockedFilesEntryDisplayed" />

                            <file-entry
                                v-for="file in folders"
                                :key="file.Key"
                                :path="path"
                                :file="file"
                                @onUpdate="onRouteChange"
                            />

                            <file-entry
                                v-for="file in singleFiles"
                                :key="file.Key"
                                :path="path"
                                :file="file"
                            />
                        </template>
                    </v-table>
                    <div
                        v-if="!isLoading"
                        class="upload-help"
                        @click="buttonFileUpload"
                    >
                        <UploadIcon />
                        <p class="drop-files-text mt-4 mb-0">
                            Drop Files Here to Upload
                        </p>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, onBeforeMount, onBeforeUnmount, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import FileBrowserHeader from './FileBrowserHeader.vue';
import FileEntry from './FileEntry.vue';
import LockedFilesEntry from './LockedFilesEntry.vue';
import BreadCrumbs from './BreadCrumbs.vue';

import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { RouteConfig } from '@/types/router';
import { useNotify } from '@/utils/hooks';
import { Bucket } from '@/types/buckets';
import { MODALS } from '@/utils/constants/appStatePopUps';
import {
    BrowserObject,
    MAX_KEY_COUNT,
    ObjectBrowserCursor,
    useObjectBrowserStore,
} from '@/store/modules/objectBrowserStore';
import { useAppStore } from '@/store/modules/appStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';
import { useLoading } from '@/composables/useLoading';

import VButton from '@/components/common/VButton.vue';
import BucketSettingsNav from '@/components/objects/BucketSettingsNav.vue';
import VTable from '@/components/common/VTable.vue';
import MultiplePassphraseBanner from '@/components/browser/MultiplePassphrasesBanner.vue';
import TooManyObjectsBanner from '@/components/browser/TooManyObjectsBanner.vue';
import UpEntry from '@/components/browser/UpEntry.vue';
import Dropzone from '@/components/browser/Dropzone.vue';

import FileIcon from '@/../static/images/objects/file.svg';
import BlackArrowExpand from '@/../static/images/common/BlackArrowExpand.svg';
import UploadIcon from '@/../static/images/browser/upload.svg';

const bucketsStore = useBucketsStore();
const appStore = useAppStore();
const obStore = useObjectBrowserStore();
const configStore = useConfigStore();
const analyticsStore = useAnalyticsStore();

const router = useRouter();
const route = useRoute();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const folderInput = ref<HTMLInputElement>();
const fileInput = ref<HTMLInputElement>();

const isUploadDropDownShown = ref<boolean>(false);
const isLockedBanner = ref<boolean>(true);
const isTooManyObjectsBanner = ref<boolean>(true);
const isOver = ref<boolean>(false);
/**
 * Retrieve the pathMatch from the current route.
 */
const routePath = ref(calculateRoutePath());

const NUMBER_OF_DISPLAYED_OBJECTS = 1000;
const routePageCache = new Map<string, number>();

/**
 * Calculates page count depending on object count and page limit.
 */
const pageCount = computed((): number => {
    return Math.ceil(fetchedObjectsCount.value / cursor.value.limit);
});

/**
 * Returns fetched object count from store.
 */
const fetchedObjectsCount = computed((): number => {
    return obStore.state.totalObjectCount;
});

/**
 * Returns table cursor from store.
 */
const cursor = computed((): ObjectBrowserCursor => {
    return obStore.state.cursor;
});

/**
 * Check if the s3 client has been initialized in the store.
 */
const isInitialized = computed((): boolean => {
    return obStore.isInitialized;
});

/**
 * Indicates if pagination should be used.
 */
const isPaginationEnabled = computed((): boolean => {
    return configStore.state.config.objectBrowserPaginationEnabled;
});

/**
 * Indicates if new objects uploading flow should be working.
 */
const isNewUploadingModal = computed((): boolean => {
    return configStore.state.config.newUploadModalEnabled;
});

/**
 * Retrieve the current path from the store.
 */
const path = computed((): string => {
    return obStore.state.path;
});

/**
 * Return files that are currently being uploaded from the store.
 */
const filesUploading = computed((): BrowserObject[] => {
    return obStore.state.uploading;
});

/**
 * Return file browser path from store.
 */
const currentPath = computed((): string => {
    return obStore.state.path;
});

/**
 * Return locked files number.
 */
const lockedFilesCount = computed((): number => {
    return objectsCount.value - obStore.state.objectsCount;
});

/**
 * Returns bucket objects count from store.
 */
const objectsCount = computed((): number => {
    const name: string = obStore.state.bucket;
    const data: Bucket | undefined = bucketsStore.state.page.buckets.find(bucket => bucket.name === name);

    return data?.objectCount || 0;
});

/**
 * Indicates if locked files entry is displayed.
 */
const lockedFilesEntryDisplayed = computed((): boolean => {
    return lockedFilesCount.value > 0 &&
        objectsCount.value <= NUMBER_OF_DISPLAYED_OBJECTS &&
        !isLoading.value &&
        !currentPath.value;
});

/**
 * Return up to five files currently being uploaded for display purposes.
 */
const formattedFilesUploading = computed((): BrowserObject[] => {
    if (filesUploading.value.length > 5) {
        return filesUploading.value.slice(0, 5);
    }

    return filesUploading.value;
});

/**
 * Return the text of how many files in total are being uploaded to be displayed to give users more context.
 */
const formattedFilesWaitingToBeUploaded = computed((): string => {
    let file = 'file';

    if (filesUploading.value.length > 1) {
        file = 'files';
    }

    return `${filesUploading.value.length} ${file}`;
});

const bucketName = computed((): string => {
    return obStore.state.bucket;
});

/**
 * Whether all files are selected.
 * */
const allFilesSelected = computed((): boolean => {
    if (files.value.length === 0) {
        return false;
    }
    const shiftSelectedFiles = obStore.state.shiftSelectedFiles;
    const selectedFiles = obStore.state.selectedFiles;
    const selectedAnchorFile = obStore.state.selectedAnchorFile;
    const allSelectedFiles = [
        ...selectedFiles,
        ...shiftSelectedFiles,
    ];

    if (selectedAnchorFile && !allSelectedFiles.includes(selectedAnchorFile)) {
        allSelectedFiles.push(selectedAnchorFile);
    }
    return allSelectedFiles.length === files.value.length;
});

const files = computed((): BrowserObject[] => {
    return isPaginationEnabled.value ? obStore.displayedObjects : obStore.sortedFiles;
});

/**
 * Return an array of BrowserFile type that are files and not folders.
 */
const singleFiles = computed((): BrowserObject[] => {
    return files.value.filter((f) => f.type === 'file');
});

/**
 * Return an array of BrowserFile type that are folders and not files.
 */
const folders = computed((): BrowserObject[] => {
    return files.value.filter((f) => f.type === 'folder');
});

/**
 * Returns bucket name from store.
 */
const bucket = computed((): string => {
    return bucketsStore.state.fileComponentBucketName;
});

/**
 * Changes table page and limit.
 */
async function changePageAndLimit(page: number, limit: number): Promise<void> {
    routePageCache.set(routePath.value, page);
    obStore.setCursor({ limit, page });

    const lastObjectOnPage = page * limit;
    const activeRange = obStore.state.activeObjectsRange;

    if (lastObjectOnPage > activeRange.start && lastObjectOnPage <= activeRange.end) {
        return;
    }

    await withLoading(async () => {
        const tokenKey = Math.ceil(lastObjectOnPage / MAX_KEY_COUNT) * MAX_KEY_COUNT;

        const tokenToBeFetched = obStore.state.continuationTokens.get(tokenKey);
        if (!tokenToBeFetched) {
            await obStore.initList(routePath.value);
            return;
        }

        await obStore.listByToken(routePath.value, tokenKey, tokenToBeFetched);
    });
}

/**
 * Closes multiple passphrase banner.
 */
function closeLockedBanner(): void {
    isLockedBanner.value = false;
}

/**
 * Closes too many objects banner.
 */
function closeTooManyObjectsBanner(): void {
    isTooManyObjectsBanner.value = false;
}

function calculateRoutePath(): string {
    let pathMatch = route.params.pathMatch;
    pathMatch = Array.isArray(pathMatch)
        ? pathMatch.join('/') + '/'
        : pathMatch;
    return pathMatch || '';
}

async function onBack(): Promise<void> {
    await router.push('../');
    await onRouteChange();
}

async function onRouteChange(): Promise<void> {
    routePath.value = calculateRoutePath();
    obStore.closeDropdown();

    await withLoading(async () => {
        if (isPaginationEnabled.value) {
            await obStore.initList(routePath.value);
        } else {
            await list(routePath.value);
        }
    });

    if (isPaginationEnabled.value) {
        const cachedPage = routePageCache.get(routePath.value);
        if (cachedPage !== undefined) {
            obStore.setCursor({ limit: cursor.value.limit, page: cachedPage });
        } else {
            obStore.setCursor({ limit: cursor.value.limit, page: 1 });
        }
    }
}

/**
 * Close modal, file share modal, dropdown, and remove all selected files from the store.
 */
function closeModalDropdown(): void {
    if (obStore.state.openedDropdown) {
        obStore.closeDropdown();
    }

    obStore.clearAllSelectedFiles();
}

/**
 * Toggle the folder creation modal in the store.
 */
function toggleFolderCreationModal(): void {
    appStore.updateActiveModal(MODALS.newFolder);
}

/**
 * Return the file name of the passed in file argument formatted.
 */
function filename(file: BrowserObject): string {
    return file.Key.length > 25
        ? file.Key.slice(0, 25) + '...'
        : file.Key;
}

/**
 * Upload the current selected or dragged-and-dropped file.
 */
async function upload(e: Event): Promise<void> {
    if (isOver.value) {
        isOver.value = false;
    }

    await obStore.upload({ e });
    analyticsStore.eventTriggered(AnalyticsEvent.OBJECT_UPLOADED);
    const target = e.target as HTMLInputElement;
    target.value = '';
}

/**
 * Cancel the upload of the current file that's passed in as an argument.
 */
function cancelUpload(fileName: string): void {
    obStore.cancelUpload(fileName);
}

/**
 * Call the list method from the store, which will trigger a re-render and fetch all files under the current path passed in as an argument.
 */
async function list(path: string): Promise<void> {
    try {
        await obStore.list(path);
    } catch (error) {
        notify.error(error.message, AnalyticsErrorEventSource.FILE_BROWSER_LIST_CALL);
    }
}

/**
 * Open the operating system's file system for file upload.
 */
async function buttonFileUpload(): Promise<void> {
    const fileInputElement = fileInput.value as HTMLInputElement;
    fileInputElement.showPicker();
    analyticsStore.eventTriggered(AnalyticsEvent.UPLOAD_FILE_CLICKED);
    closeUploadDropdown();
}

/**
 * Open the operating system's file system for folder upload.
 */
async function buttonFolderUpload(): Promise<void> {
    const folderInputElement = folderInput.value as HTMLInputElement;
    folderInputElement.showPicker();
    analyticsStore.eventTriggered(AnalyticsEvent.UPLOAD_FOLDER_CLICKED);
    closeUploadDropdown();
}

/**
 * Toggles upload options dropdown.
 */
function toggleUploadDropdown(): void {
    isUploadDropDownShown.value = !isUploadDropDownShown.value;
}

/**
 * Makes dropzone visible.
 */
function showDropzone(): void {
    isOver.value = true;
}

/**
 * Hides dropzone.
 */
function hideDropzone(): void {
    isOver.value = false;
}

/**
 * Closes upload options dropdown.
 */
function closeUploadDropdown(): void {
    isUploadDropDownShown.value = false;
}

/**
 * Redirects to buckets list view.
 */
async function goToBuckets(): Promise<void> {
    await router.push(RouteConfig.Buckets.with(RouteConfig.BucketsManagement).path).catch(_ => {});
    analyticsStore.pageVisit(RouteConfig.Buckets.with(RouteConfig.BucketsManagement).path);
    await onRouteChange();
}

/**
 * Toggles the selection of all files.
 * */
function toggleSelectAllFiles(): void {
    if (files.value.length === 0) {
        return;
    }

    if (allFilesSelected.value) {
        obStore.clearAllSelectedFiles();
    } else {
        obStore.clearAllSelectedFiles();
        obStore.setSelectedAnchorFile(files.value[0]);
        obStore.updateSelectedFiles(files.value.slice(1, files.value.length));
    }
}

/**
 * Set spinner state. If routePath is not present navigate away.
 * If there's some error then re-render the page with a call to list.
 */
onBeforeMount(async () => {
    if (!bucket.value) {
        const path = RouteConfig.Buckets.with(RouteConfig.BucketsManagement).path;

        analyticsStore.pageVisit(path);
        await router.push(path);

        return;
    }

    // clear previous file selections.
    obStore.clearAllSelectedFiles();

    await withLoading(async () => {
        try {
            if (isPaginationEnabled.value) {
                await Promise.all([
                    obStore.initList(''),
                    obStore.getObjectCount(),
                ]);
            } else {
                await Promise.all([
                    list(''),
                    obStore.getObjectCount(),
                ]);
            }
        } catch (error) {
            notify.error(error.message, AnalyticsErrorEventSource.FILE_BROWSER_LIST_CALL);
        }
    });
});

onBeforeUnmount(() => {
    obStore.setCursor({ limit: DEFAULT_PAGE_LIMIT, page: 1 });
});
</script>

<style scoped lang="scss">
.file-browser {
    min-height: 500px;
}

.hide-mobile {
    @media screen and (width <= 550px) {
        display: none;
    }
}

@media screen and (width <= 550px) {
    // hide size, upload date columns on mobile screens

    :deep(.data:not(:nth-child(2))) {
        display: none;
    }
}

.position-relative {
    position: relative;
}

.file-name {
    display: flex;
    align-items: center;
    gap: 10px;
}

.no-selection {
    user-select: none;
}

.path {
    font-size: 18px;
    font-weight: 700;
}

.file-browser-table {
    box-shadow: none;
}

.upload-help {
    font-size: 1.75rem;
    text-align: center;
    margin-top: 1.5rem;
    color: #93a1ae;
    border: 2px dashed #bec4cd;
    border-radius: 10px;
    padding: 80px 20px;
    background: var(--c-grey-1);
    cursor: pointer;

    svg {
        width: 300px;

        @media screen and (width <= 425px) {
            width: unset;
        }
    }
}

.metric {
    color: #444;
}

.div-responsive {
    min-height: 400px;
}

.folder-input:focus {
    color: #fe5d5d;
    box-shadow: 0 0 0 0.2rem rgb(254 93 93 / 50%) !important;
    border-color: #fe5d5d !important;
    outline: none !important;
}

.new-folder-row:hover {
    background: #fff;
}

.btn-primary {
    background: #376fff;
    border-color: #376fff;
}

.btn-primary:hover {
    background: #0047ff;
    border-color: #0047ff;
}

.btn-light {
    background: #e6e9ef;
    border-color: #e6e9ef;
}

.btn-primary.disabled,
.btn-primary:disabled {
    color: #fff;
    background-color: #001030;
    border-color: #001030;
}

.input-folder {
    height: 43px;
}

.drop-files-text {
    font-weight: bold;
    font-size: 18px;
}

.up-button {

    &__content {
        padding: 0.5rem 1.125rem;
    }
}

.files-uploading-count {

    &__content {
        color: #0d6efd;
        border-top: none;
        padding: 0 1.125rem 0.5rem;
    }
}

.arrow {
    margin: unset;
    transition-duration: 0.5s;

    &.active {
        transform: rotate(180deg) scaleX(-1);
    }

    :deep(path) {
        fill: white;
    }
}

.dropdown {
    position: absolute;
    margin-top: 10px;
    border-radius: 8px;
    box-shadow: 0 -2px 16px rgb(0 0 0 / 10%);
    width: 240px;
    height: auto;
    overflow: hidden;
    z-index: 999;
    background: white;

    &__item {
        display: flex;
        align-items: center;
        justify-content: flex-start;
        box-sizing: border-box;
        height: 56px;
        width: 100%;
        padding: 0 18px;
        font-family: 'font_regular', sans-serif;
        font-size: 14px;
        color: var(--c-grey-6);
        background-clip: padding-box;

        &:hover {
            background: var(--c-grey-1);
            color: var(--c-blue-3);
            font-family: 'font_medium', sans-serif;

            .btn-icon > path {
                fill: var(--c-blue-3);
            }
        }

        &:not(:first-of-type) {
            border-top: 1px solid var(--c-grey-2);
        }
    }
}

.upload-option {
    all: unset;
    width: 100%;
    height: 100%;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: flex-start;

    & .btn-icon {
        margin-right: 18px;
    }
}

.btn {
    display: flex;
    align-items: center;
    position: relative;
    border-radius: 8px;
    width: auto;
    padding: 0 17px;
    height: 44px;
    font-family: 'font_bold', sans-serif;
    line-height: 2.4;

    svg {
        margin-right: 8px;
    }
}

.new-folder-button {
    background: white;
    border: 1px solid var(--c-grey-3);
    color: var(--c-grey-6);
    border-radius: 8px;
}

.upload-button {
    color: white;
    display: flex;
    align-items: center;
    justify-content: space-between;
    background-color: var(--c-blue-3);
    border: 1px solid transparent;
    cursor: pointer;
    padding-right: 0;

    &__divider {
        height: 100%;
        width: 1px;
        background: var(--c-blue-4);
        margin: 0 7px;
    }

    &:hover {
        background-color: #0059d0;
    }
}

.tile-action-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin: 1.5em 0;

    @media screen and (width <= 768px) {
        flex-direction: column;
        justify-content: flex-start;
        align-items: flex-start;
    }

    &__title {
        margin: 0;
        font-size: 2rem;
        font-weight: 500;
        line-height: 1.2;
        word-break: break-all;

        @media screen and (width <= 768px) {
            margin-bottom: 0.5rem;
        }
    }

    &__actions {
        display: flex;
        justify-content: flex-start;
        flex-wrap: wrap;
        gap: 5px;
    }
}

.hr-divider {
    margin-bottom: 1.5em;
    border-bottom: 1px solid #dadfe7;
}

/* copied over from scoped-bootstrap.css */

.file-browser .progress {
    display: flex;
    height: 1rem;
    overflow: hidden;
    line-height: 0;
    font-size: 0.75rem;
    background-color: #e9ecef;
    border-radius: 0.25rem;
}

.file-browser .progress-bar {
    display: flex;
    flex-direction: column;
    justify-content: center;
    overflow: visible !important; /* hack to override double import */
    color: #fff;
    text-shadow: 0 1px #000 !important; /* make #fff visible on gray background */
    text-align: center;
    white-space: nowrap;
    background-color: #007bff;
    transition: width 0.6s ease;
}
</style>
