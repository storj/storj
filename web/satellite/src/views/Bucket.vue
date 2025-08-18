// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container
        class="bucket-view"
        @dragover.prevent="isDragging = true"
    >
        <dropzone-dialog v-model="isDragging" :bucket="bucketName" @file-drop="onUpload" />
        <page-title-component title="Browse" />

        <browser-breadcrumbs-component />
        <v-col>
            <v-row align="center" class="mt-1 mb-2">
                <div class="d-flex ga-2 flex-wrap">
                    <v-menu v-model="menu" location="bottom" transition="scale-transition" offset="5">
                        <template #activator="{ props }">
                            <v-btn
                                color="primary"
                                :disabled="!isInitialized"
                                v-bind="props"
                                min-width="120"
                                :prepend-icon="Upload"
                            >
                                Upload
                            </v-btn>
                        </template>
                        <v-list class="pa-1">
                            <v-list-item :disabled="!isInitialized" @click.stop="buttonFileUpload">
                                <template #prepend>
                                    <component :is="FileUp" :size="18" />
                                </template>
                                <v-list-item-title class="text-body-2 ml-3">
                                    Upload Files
                                </v-list-item-title>
                            </v-list-item>

                            <v-divider class="my-1" />

                            <v-list-item class="mt-1" :disabled="!isInitialized" @click.stop="buttonFolderUpload">
                                <template #prepend>
                                    <component :is="FolderUp" :size="18" />
                                </template>
                                <v-list-item-title class="text-body-2 ml-3">
                                    Upload Folders
                                </v-list-item-title>
                            </v-list-item>
                        </v-list>
                    </v-menu>

                    <input
                        id="File Input"
                        ref="fileInput"
                        type="file"
                        aria-roledescription="file-upload"
                        hidden
                        multiple
                        @change="onUpload"
                    >
                    <input
                        id="Folder Input"
                        ref="folderInput"
                        type="file"
                        aria-roledescription="folder-upload"
                        hidden
                        multiple
                        webkitdirectory
                        mozdirectory
                        @change="onUpload"
                    >
                    <v-btn
                        variant="outlined"
                        color="default"
                        :disabled="!isInitialized"
                        @click="onNewFolderClick"
                    >
                        <icon-folder class="mr-2" bold />
                        New Folder
                    </v-btn>

                    <v-btn
                        variant="outlined"
                        color="default"
                        :disabled="!isInitialized || isLoading"
                        @click="refreshFiles"
                    >
                        <v-tooltip text="Refresh" location="top" activator="parent" />
                        <component
                            :is="RefreshCcw"
                            :class="{ 'rotate-animation': isLoading }" :size="18"
                        />
                    </v-btn>

                    <v-menu v-model="settingsMenu" location="bottom" transition="scale-transition" offset="5">
                        <template #activator="{ props }">
                            <v-btn
                                variant="outlined"
                                color="default"
                                v-bind="props"
                                :prepend-icon="Settings"
                                :append-icon="ChevronDown"
                                aria-label="Bucket Options"
                            />
                        </template>
                        <v-list class="pa-1">
                            <div>
                                <v-list-item
                                    v-if="versioningUIEnabled"
                                    density="comfortable"
                                    link
                                    :disabled="bucket?.versioning === Versioning.Enabled && bucket?.objectLockEnabled"
                                    @click="onToggleVersioning"
                                >
                                    <template #prepend>
                                        <component :is="History" v-if="bucket?.versioning !== Versioning.Enabled" :size="18" />
                                        <component :is="CirclePause" v-else :size="18" />
                                    </template>
                                    <v-list-item-title
                                        class="ml-3 text-body-2 font-weight-medium"
                                    >
                                        {{
                                            bucket?.versioning !== Versioning.Enabled ? 'Enable Versioning' : 'Suspend Versioning'
                                        }}
                                    </v-list-item-title>
                                </v-list-item>
                                <v-tooltip
                                    v-if="bucket?.versioning === Versioning.Enabled && bucket?.objectLockEnabled"
                                    activator="parent"
                                    location="left"
                                    max-width="300"
                                >
                                    Versioning cannot be suspended on a bucket with object lock enabled
                                </v-tooltip>
                            </div>

                            <v-list-item
                                v-if="versioningUIEnabled"
                                density="comfortable"
                                link
                                @click="obStore.toggleShowObjectVersions()"
                            >
                                <template #prepend>
                                    <component :is="showObjectVersions ? EyeOff : Eye" :size="18" />
                                </template>
                                <v-list-item-title
                                    class="ml-3 text-body-2 font-weight-medium"
                                >
                                    {{ showObjectVersions ? "Hide" : "Show" }} Versions
                                </v-list-item-title>
                            </v-list-item>

                            <v-list-item
                                density="comfortable"
                                link
                                @click="isShareBucketDialogShown = true"
                            >
                                <template #prepend>
                                    <component :is="Share2" :size="18" />
                                </template>
                                <v-list-item-title
                                    class="ml-3 text-body-2 font-weight-medium"
                                >
                                    Share Bucket
                                </v-list-item-title>
                            </v-list-item>
                            <v-list-item
                                v-if="downloadPrefixEnabled"
                                density="comfortable"
                                link
                                @click="onDownloadBucket"
                            >
                                <template #prepend>
                                    <component :is="DownloadIcon" :size="18" />
                                </template>
                                <v-list-item-title
                                    class="ml-3 text-body-2 font-weight-medium"
                                >
                                    Download Bucket
                                </v-list-item-title>
                            </v-list-item>
                            <v-list-item
                                density="comfortable"
                                link
                                @click="isBucketDetailsDialogShown = true"
                            >
                                <template #prepend>
                                    <component :is="ReceiptText" :size="18" />
                                </template>
                                <v-list-item-title
                                    class="ml-3 text-body-2 font-weight-medium"
                                >
                                    Bucket Details
                                </v-list-item-title>
                            </v-list-item>
                            <v-divider class="my-1" />
                            <v-list-item
                                density="comfortable"
                                link
                                base-color="error"
                                @click="isDeleteBucketDialogShown = true"
                            >
                                <template #prepend>
                                    <component :is="Trash2" :size="18" />
                                </template>
                                <v-list-item-title
                                    class="ml-3 text-body-2 font-weight-medium"
                                >
                                    Delete Bucket
                                </v-list-item-title>
                            </v-list-item>
                        </v-list>
                    </v-menu>
                </div>

                <v-spacer v-if="smAndUp" />

                <div class="d-flex ga-2 flex-wrap pa-0 pt-5 pa-sm-0 justify-sm-end text-sm-right">
                    <v-btn
                        v-if="versioningUIEnabled"
                        variant="outlined"
                        color="default"
                        @click="obStore.toggleShowObjectVersions()"
                    >
                        <template #prepend>
                            <component :is="showObjectVersions ? EyeOff : Eye" :size="18" />
                        </template>
                        {{ showObjectVersions ? "Hide" : "Show" }} Versions
                    </v-btn>
                    <v-btn-toggle
                        mandatory
                        border
                        inset
                        rounded="lg"
                        class="pa-1 bg-surface"
                    >
                        <v-tooltip v-if="showObjectVersions" location="top" activator="parent">
                            Please hide versions to toggle the view.
                        </v-tooltip>
                        <v-tooltip :disabled="showObjectVersions || $vuetify.display.smAndDown" location="top">
                            <template #activator="{ props }">
                                <v-btn
                                    :disabled="showObjectVersions"
                                    size="small"
                                    rounded="md"
                                    active-class="active"
                                    :active="isCardView"
                                    aria-label="Toggle Card View"
                                    :title="$vuetify.display.smAndDown ? 'Card view shows image previews using download bandwidth.' : undefined"
                                    v-bind="props"
                                    @click="isCardView = true"
                                >
                                    <component :is="Grid2X2" :size="14" class="mr-1" />
                                    Cards
                                </v-btn>
                            </template>
                            Card view shows image previews using download bandwidth.
                        </v-tooltip>
                        <v-btn
                            :disabled="showObjectVersions"
                            size="small"
                            rounded="md"
                            active-class="active"
                            :active="!isCardView"
                            aria-label="Toggle Table View"
                            @click="isCardView = false"
                        >
                            <component :is="List" :size="14" class="mr-1" />
                            Table
                        </v-btn>
                    </v-btn-toggle>
                </div>
            </v-row>
        </v-col>

        <v-card v-if="isFetching">
            <v-card-item>
                <v-skeleton-loader type="card" />
            </v-card-item>
        </v-card>
        <template v-else>
            <browser-versions-table-component v-if="showObjectVersions" ref="filesListRef" :loading="isFetching" :force-empty="!isInitialized" @upload-click="buttonFileUpload" />
            <browser-card-view-component v-else-if="isCardView" ref="filesListRef" :bucket="bucket" :force-empty="!isInitialized" @upload-click="buttonFileUpload" />
            <browser-table-component v-else ref="filesListRef" :bucket="bucket" :loading="isFetching" :force-empty="!isInitialized" @upload-click="buttonFileUpload" />
        </template>
    </v-container>

    <browser-new-folder-dialog v-model="isNewFolderDialogOpen" />
    <enter-bucket-passphrase-dialog v-model="isBucketPassphraseDialogOpen" @passphrase-entered="initObjectStore" />
    <share-dialog v-model="isShareBucketDialogShown" :bucket-name="bucketName" />
    <bucket-details-dialog v-model="isBucketDetailsDialogShown" :bucket-name="bucketName" />
    <delete-bucket-dialog v-model="isDeleteBucketDialogShown" :bucket-name="bucketName" @deleted="onBucketDeleted" />
    <toggle-versioning-dialog v-model="bucketToToggleVersioning" @toggle="() => bucketsStore.getAllBucketsMetadata(projectId)" />
    <upload-overwrite-warning-dialog
        v-model="isDuplicateUploadDialogShown"
        :filenames="duplicateFiles"
        @proceed="upload(true)"
        @cancel="clearUpload"
    />
    <download-prefix-dialog v-if="downloadPrefixEnabled" v-model="isDownloadPrefixDialogShown" :prefix-type="DownloadPrefixType.Bucket" :bucket="bucketToDownload" />
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import {
    VCard,
    VCardItem,
    VContainer,
    VCol,
    VRow,
    VBtn,
    VMenu,
    VList,
    VListItem,
    VListItemTitle,
    VSkeletonLoader,
    VSpacer,
    VDivider,
    VBtnToggle,
    VTooltip,
} from 'vuetify/components';
import { useDisplay } from 'vuetify';
import {
    FileUp,
    FolderUp,
    ChevronDown,
    Settings,
    Upload,
    Share2,
    ReceiptText,
    Trash2,
    RefreshCcw,
    History,
    CirclePause,
    List,
    Eye,
    EyeOff,
    Grid2X2,
    DownloadIcon,
} from 'lucide-vue-next';

import { useBucketsStore } from '@/store/modules/bucketsStore';
import { FileToUpload, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { EdgeCredentials } from '@/types/accessGrants';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useAppStore } from '@/store/modules/appStore';
import { ROUTES } from '@/router';
import { Versioning } from '@/types/versioning';
import { BucketMetadata } from '@/types/buckets';
import { usePreCheck } from '@/composables/usePreCheck';
import { useUsersStore } from '@/store/modules/usersStore';
import { useConfigStore } from '@/store/modules/configStore';
import { DownloadPrefixType } from '@/types/browser';
import { useLoading } from '@/composables/useLoading';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import BrowserBreadcrumbsComponent from '@/components/BrowserBreadcrumbsComponent.vue';
import BrowserTableComponent from '@/components/BrowserTableComponent.vue';
import BrowserNewFolderDialog from '@/components/dialogs/BrowserNewFolderDialog.vue';
import EnterBucketPassphraseDialog from '@/components/dialogs/EnterBucketPassphraseDialog.vue';
import DropzoneDialog from '@/components/dialogs/DropzoneDialog.vue';
import BrowserCardViewComponent from '@/components/BrowserCardViewComponent.vue';
import IconFolder from '@/components/icons/IconFolder.vue';
import ShareDialog from '@/components/dialogs/ShareDialog.vue';
import BucketDetailsDialog from '@/components/dialogs/BucketDetailsDialog.vue';
import DeleteBucketDialog from '@/components/dialogs/DeleteBucketDialog.vue';
import ToggleVersioningDialog from '@/components/dialogs/ToggleVersioningDialog.vue';
import UploadOverwriteWarningDialog from '@/components/dialogs/UploadOverwriteWarningDialog.vue';
import BrowserVersionsTableComponent from '@/components/BrowserVersionsTableComponent.vue';
import DownloadPrefixDialog from '@/components/dialogs/DownloadPrefixDialog.vue';

const bucketsStore = useBucketsStore();
const obStore = useObjectBrowserStore();
const projectsStore = useProjectsStore();
const analyticsStore = useAnalyticsStore();
const appStore = useAppStore();
const userStore = useUsersStore();
const configStore = useConfigStore();

const router = useRouter();
const route = useRoute();
const notify = useNotify();
const { smAndUp } = useDisplay();
const { withTrialCheck } = usePreCheck();
const { isLoading, withLoading } = useLoading();

const filesListRef = ref<{ refresh: () => Promise<void> } | null>(null);

const folderInput = ref<HTMLInputElement>();
const fileInput = ref<HTMLInputElement>();
const menu = ref<boolean>(false);
const settingsMenu = ref<boolean>(false);
const isFetching = ref<boolean>(true);
const isInitialized = ref<boolean>(false);
const isDragging = ref<boolean>(false);
const isBucketPassphraseDialogOpen = ref<boolean>(false);
const isNewFolderDialogOpen = ref<boolean>(false);
const isShareBucketDialogShown = ref<boolean>(false);
const isBucketDetailsDialogShown = ref<boolean>(false);
const isDeleteBucketDialogShown = ref<boolean>(false);
const isDuplicateUploadDialogShown = ref<boolean>(false);
const bucketToToggleVersioning = ref<BucketMetadata | null>(null);
const isDownloadPrefixDialogShown = ref<boolean>(false);
const bucketToDownload = ref<string>('');

const duplicateFiles = ref<string[]>([]);

/**
 * Whether versioning has been enabled for current project and allowed for this bucket specifically.
 */
const versioningUIEnabled = computed(() => {
    return configStore.state.config.versioningUIEnabled
      && bucket.value
      && bucket.value.versioning !== Versioning.NotSupported
      && bucket.value.versioning !== Versioning.Unversioned;
});

const downloadPrefixEnabled = computed<boolean>(() => configStore.state.config.downloadPrefixEnabled);

/**
 * Whether the user should be warned when uploading duplicate files.
 */
const ignoreDuplicateUploads = computed<boolean>(() => {
    const duplicateWarningDismissed = !!userStore.state.settings.noticeDismissal?.uploadOverwriteWarning;
    const versioningEnabled = configStore.state.config.versioningUIEnabled && bucket.value && bucket.value.versioning === Versioning.Enabled;
    return versioningEnabled || duplicateWarningDismissed;
});

/**
 * Whether object versions should be shown.
 */
const showObjectVersions = computed(() =>  versioningUIEnabled.value && obStore.state.showObjectVersions.value);

/**
 * Returns the name of the selected bucket.
 */
const bucketName = computed<string>(() => bucketsStore.state.fileComponentBucketName);

/**
 * Returns metadata of the current bucket.
 */
const bucket = computed<BucketMetadata | undefined>(() => {
    return bucketsStore.state.allBucketMetadata.find(b => b.name === bucketName.value);
});

/**
 * Returns edge credentials from store.
 */
const edgeCredentials = computed((): EdgeCredentials => bucketsStore.state.edgeCredentials);

/**
 * Returns ID of selected project from store.
 */
const projectId = computed<string>(() => projectsStore.state.selectedProject.id);

/**
 * Returns urlID of selected project from store.
 */
const projectUrlId = computed<string>(() => projectsStore.state.selectedProject.urlId);

/**
 * Returns whether the user should be prompted to enter the passphrase.
 */
const isPromptForPassphrase = computed<boolean>(() => bucketsStore.state.promptForPassphrase);

/**
 * Returns whether to use the card view.
 */
const isCardView = computed<boolean>({
    get: () => appStore.state.isBrowserCardViewEnabled,
    set: value => {
        appStore.toggleBrowserCardViewEnabled(value);
        obStore.updateSelectedFiles([]);
    },
});

/**
 * Open the operating system's file system for file upload.
 */
async function buttonFileUpload(): Promise<void> {
    withTrialCheck(() => {
        menu.value = false;
        const fileInputElement = fileInput.value as HTMLInputElement;
        fileInputElement.showPicker();
        analyticsStore.eventTriggered(AnalyticsEvent.UPLOAD_FILE_CLICKED, { project_id: projectId.value });
    });
}

function onNewFolderClick(): void {
    withTrialCheck(() => {
        isNewFolderDialogOpen.value = true;
    });
}

/**
 * Handles download bucket action.
 */
function onDownloadBucket(): void {
    withTrialCheck(() => {
        bucketToDownload.value = bucketName.value;
        isDownloadPrefixDialogShown.value = true;
    });
}

/**
 * Open the operating system's file system for folder upload.
 */
async function buttonFolderUpload(): Promise<void> {
    withTrialCheck(() => {
        menu.value = false;
        const folderInputElement = folderInput.value as HTMLInputElement;
        folderInputElement.showPicker();
        analyticsStore.eventTriggered(AnalyticsEvent.UPLOAD_FOLDER_CLICKED, { project_id: projectId.value });
    });
}

/**
 * Initializes object browser store.
 */
async function initObjectStore() {
    if (!edgeCredentials.value.accessKeyId) {
        try {
            await bucketsStore.setS3Client(projectsStore.state.selectedProject.id);
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.UPLOAD_FILE_VIEW);
            router.push({
                name: ROUTES.Buckets.name,
                params: { id: projectUrlId.value },
            });
            return;
        }
    }
    obStore.init({
        endpoint: edgeCredentials.value.endpoint,
        accessKey: edgeCredentials.value.accessKeyId,
        secretKey: edgeCredentials.value.secretKey,
        bucket: bucketName.value,
        browserRoot: '', // unused
    });
    isInitialized.value = true;
}

const uploadEvent = ref<Event>();
const filesToUpload = ref<FileToUpload[]>();

function clearUpload() {
    if (!uploadEvent.value) {
        return;
    }
    const target = uploadEvent.value.target as HTMLInputElement;
    target.value = '';
    uploadEvent.value = undefined;
    filesToUpload.value = undefined;
    duplicateFiles.value = [];
}

function onUpload(e: Event | undefined) {
    if (isDragging.value) {
        isDragging.value = false;
    }

    uploadEvent.value = e;
    duplicateFiles.value = [];

    upload(ignoreDuplicateUploads.value);
}

/**
 * Upload the current selected or dragged-and-dropped file.
 */
async function upload(ignoreDuplicate: boolean): Promise<void> {
    if (!uploadEvent.value) {
        return;
    }

    if (!filesToUpload.value) {
        try {
            filesToUpload.value = await obStore.getFilesToUpload({ e: uploadEvent.value });
            if (!ignoreDuplicate) {
                duplicateFiles.value = obStore.lazyDuplicateCheck(filesToUpload.value);
                if (duplicateFiles.value.length > 0) {
                    isDuplicateUploadDialogShown.value = true;
                    return;
                }
            }
        } catch (error) {
            notify.notifyError(error);
            return;
        }
    }
    await obStore.upload(filesToUpload.value);
    clearUpload();
    analyticsStore.eventTriggered(AnalyticsEvent.OBJECT_UPLOADED, { project_id: projectId.value });
}

/**
 * Toggles versioning for the bucket between Suspended and Enabled.
 */
async function onToggleVersioning() {
    withTrialCheck(() => {
        if (!bucket.value) {
            return;
        }
        bucketToToggleVersioning.value = bucket.value;
    });
}

function onBucketDeleted() {
    router.push({
        name: ROUTES.Buckets.name,
        params: { id: projectUrlId.value },
    });
}

function refreshFiles() {
    withLoading(async () => await filesListRef.value?.refresh());
}

watch(isBucketPassphraseDialogOpen, isOpen => {
    if (isOpen || !isPromptForPassphrase.value) return;
    router.push({
        name: ROUTES.Buckets.name,
        params: { id: projectUrlId.value },
    });
});

watch(() => route.params.browserPath, browserPath => {
    if (browserPath === undefined) return;

    let bucketName: string, filePath = '';
    if (typeof browserPath === 'string') {
        bucketName = browserPath;
    } else {
        bucketName = browserPath[0];
        filePath = browserPath.slice(1).join('/');
    }

    bucketsStore.setFileComponentBucketName(bucketName);
    bucketsStore.setFileComponentPath(filePath);
}, { immediate: true });

watch(() => bucketsStore.state.passphrase, async newPass => {
    if (isBucketPassphraseDialogOpen.value) return;

    const bucketsURL = `${ROUTES.Projects.path}/${projectUrlId.value}/${ROUTES.Buckets.path}`;

    if (!newPass) {
        router.push(bucketsURL);
        return;
    }

    isInitialized.value = false;
    try {
        await bucketsStore.setS3Client(projectId.value);
        obStore.reinit({
            endpoint: edgeCredentials.value.endpoint,
            accessKey: edgeCredentials.value.accessKeyId,
            secretKey: edgeCredentials.value.secretKey,
        });
        await router.push(`${bucketsURL}/${bucketsStore.state.fileComponentBucketName}`);
        isInitialized.value = true;
    } catch (error) {
        error.message = `Error setting S3 client. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.UPLOAD_FILE_VIEW);
        router.push(bucketsURL);
    }
});

/**
 * Initializes file browser.
 */
onMounted(async () => {
    function goToBuckets() {
        router.push({
            name: ROUTES.Buckets.name,
            params: { id: projectUrlId.value },
        });
    }

    if (appStore.state.managedPassphraseNotRetrievable) {
        goToBuckets();
        return;
    }

    try {
        await bucketsStore.getAllBucketsMetadata(projectId.value);
    } catch (error) {
        error.message = `Error fetching bucket names and/or placements. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.UPLOAD_FILE_VIEW);
        goToBuckets();
        return;
    }

    if (!bucket.value) {
        goToBuckets();
        return;
    }

    if (versioningUIEnabled.value && !obStore.state.showObjectVersions.userModified) {
        // only toggle this view as default if the user hasn't already changed it
        obStore.toggleShowObjectVersions(true, false);
    }

    if (isPromptForPassphrase.value) {
        isBucketPassphraseDialogOpen.value = true;
    } else {
        await initObjectStore();
    }

    isFetching.value = false;
});
</script>

<style scoped lang="scss">
.bucket-view {
    height: 100%;
}

.rotate-animation {
    animation: spin 1s linear infinite;
}

@keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
}
</style>
