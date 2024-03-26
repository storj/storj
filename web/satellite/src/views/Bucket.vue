// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container
        class="bucket-view"
        @dragover.prevent="isDragging = true"
    >
        <dropzone-dialog v-model="isDragging" :bucket="bucketName" @file-drop="upload" />
        <page-title-component title="Browse Files" />

        <browser-breadcrumbs-component />
        <v-col>
            <v-row align="center" class="mt-2 mb-2 mb-sm-4">
                <v-menu v-model="menu" location="bottom" transition="scale-transition" offset="5">
                    <template #activator="{ props }">
                        <v-btn
                            color="primary"
                            min-width="120"
                            :disabled="!isInitialized"
                            v-bind="props"
                        >
                            <IconUpload class="mr-2" />
                            Upload
                        </v-btn>
                    </template>
                    <v-list class="pa-1">
                        <v-list-item rounded="lg" :disabled="!isInitialized" @click.stop="buttonFileUpload">
                            <template #prepend>
                                <IconFile size="16" />
                            </template>
                            <v-list-item-title class="text-body-2 ml-2">
                                Upload Files
                            </v-list-item-title>
                        </v-list-item>

                        <v-divider class="my-1" />

                        <v-list-item class="mt-1" rounded="lg" :disabled="!isInitialized" @click.stop="buttonFolderUpload">
                            <template #prepend>
                                <icon-folder size="16" bold />
                            </template>
                            <v-list-item-title class="text-body-2 ml-2">
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
                    @change="upload"
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
                    @change="upload"
                >
                <v-btn
                    variant="outlined"
                    color="default"
                    class="ml-2 ml-sm-4"
                    :disabled="!isInitialized"
                    @click="isNewFolderDialogOpen = true"
                >
                    <icon-folder class="mr-2" bold />
                    New Folder
                </v-btn>

                <v-menu v-model="settingsMenu" location="bottom" transition="scale-transition" offset="5">
                    <template #activator="{ props }">
                        <v-btn
                            variant="outlined"
                            color="default"
                            class="ml-2 ml-sm-4"
                            v-bind="props"
                        >
                            <template #append>
                                <IconDropdown />
                            </template>
                            <IconSettings size="16" />
                        </v-btn>
                    </template>
                    <v-list class="pa-2">
                        <v-list-item
                            v-if="versioningUIEnabled && bucket && bucket.versioning !== Versioning.NotSupported"
                            density="comfortable"
                            link
                            rounded="lg"
                            @click="onToggleVersioning"
                        >
                            <template #prepend>
                                <IconVersioning v-if="bucket.versioning !== Versioning.Enabled" />
                                <IconPause v-else />
                            </template>
                            <v-list-item-title
                                class="pl-2 text-body-2 font-weight-medium"
                            >
                                {{
                                    bucket.versioning !== Versioning.Enabled ? 'Enable Versioning' : 'Suspend Versioning'
                                }}
                            </v-list-item-title>
                        </v-list-item>
                        <v-list-item
                            density="comfortable"
                            link
                            rounded="lg"
                            @click="showShareBucketDialog"
                        >
                            <template #prepend>
                                <icon-share bold />
                            </template>
                            <v-list-item-title
                                class="pl-2 text-body-2 font-weight-medium"
                            >
                                Share Bucket
                            </v-list-item-title>
                        </v-list-item>
                        <v-divider class="my-2" />
                        <v-list-item
                            density="comfortable"
                            link rounded="lg"
                            base-color="error"
                            @click="isDeleteBucketDialogShown = true"
                        >
                            <template #prepend>
                                <icon-trash bold />
                            </template>
                            <v-list-item-title
                                class="pl-2 text-body-2 font-weight-medium"
                            >
                                Delete Bucket
                            </v-list-item-title>
                        </v-list-item>
                    </v-list>
                </v-menu>

                <v-spacer v-if="smAndUp" />

                <v-col class="pa-0 pt-5 pa-sm-0" cols="auto">
                    <v-btn-toggle
                        mandatory
                        border
                        inset
                        density="comfortable"
                        class="pa-1"
                    >
                        <v-tooltip location="top">
                            <template #activator="{ props }">
                                <v-btn
                                    size="small"
                                    rounded="xl"
                                    active-class="active"
                                    :active="isCardView"
                                    aria-label="Toggle Cards View"
                                    v-bind="props"
                                    @click="isCardView = true"
                                >
                                    <icon-card-view />
                                    Gallery
                                </v-btn>
                            </template>
                            Gallery view shows image previews using download bandwidth.
                        </v-tooltip>
                        <v-btn
                            size="small"
                            rounded="xl"
                            active-class="active"
                            :active="!isCardView"
                            aria-label="Toggle Table View"
                            @click="isCardView = false"
                        >
                            <icon-table-view />
                            List
                        </v-btn>
                    </v-btn-toggle>
                </v-col>
            </v-row>
        </v-col>

        <browser-card-view-component v-if="isCardView" :force-empty="!isInitialized" @new-folder-click="isNewFolderDialogOpen = true" @upload-click="menu = true" />
        <browser-table-component v-else :loading="isFetching" :force-empty="!isInitialized" />
    </v-container>

    <browser-new-folder-dialog v-model="isNewFolderDialogOpen" />
    <enter-bucket-passphrase-dialog v-model="isBucketPassphraseDialogOpen" @passphrase-entered="initObjectStore" />
    <share-dialog v-model="isShareBucketDialogShown" :bucket-name="bucketName" />
    <delete-bucket-dialog v-model="isDeleteBucketDialogShown" :bucket-name="bucketName" @deleted="onBucketDeleted" />
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import {
    VContainer,
    VCol,
    VRow,
    VBtn,
    VMenu,
    VList,
    VListItem,
    VListItemTitle,
    VSpacer,
    VDivider,
    VBtnToggle,
    VTooltip,
} from 'vuetify/components';
import { useDisplay } from 'vuetify';

import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { EdgeCredentials } from '@/types/accessGrants';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useAppStore } from '@/store/modules/appStore';
import { useConfigStore } from '@/store/modules/configStore';
import { ROUTES } from '@/router';
import { Versioning } from '@/types/versioning';
import { BucketMetadata } from '@/types/buckets';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useVersioning } from '@/composables/useVersioning.js';

import PageTitleComponent from '@/components/PageTitleComponent.vue';
import BrowserBreadcrumbsComponent from '@/components/BrowserBreadcrumbsComponent.vue';
import BrowserTableComponent from '@/components/BrowserTableComponent.vue';
import BrowserNewFolderDialog from '@/components/dialogs/BrowserNewFolderDialog.vue';
import IconUpload from '@/components/icons/IconUpload.vue';
import IconFolder from '@/components/icons/IconFolder.vue';
import IconFile from '@/components/icons/IconFile.vue';
import EnterBucketPassphraseDialog from '@/components/dialogs/EnterBucketPassphraseDialog.vue';
import DropzoneDialog from '@/components/dialogs/DropzoneDialog.vue';
import BrowserCardViewComponent from '@/components/BrowserCardViewComponent.vue';
import IconTableView from '@/components/icons/IconTableView.vue';
import IconCardView from '@/components/icons/IconCardView.vue';
import IconSettings from '@/components/icons/IconSettings.vue';
import IconDropdown from '@/components/icons/IconDropdown.vue';
import ShareDialog from '@/components/dialogs/ShareDialog.vue';
import DeleteBucketDialog from '@/components/dialogs/DeleteBucketDialog.vue';
import IconPause from '@/components/icons/IconPause.vue';
import IconVersioning from '@/components/icons/IconVersioning.vue';
import IconShare from '@/components/icons/IconShare.vue';
import IconTrash from '@/components/icons/IconTrash.vue';

const bucketsStore = useBucketsStore();
const obStore = useObjectBrowserStore();
const projectsStore = useProjectsStore();
const analyticsStore = useAnalyticsStore();
const config = useConfigStore();
const appStore = useAppStore();

const router = useRouter();
const route = useRoute();
const notify = useNotify();
const { smAndUp } = useDisplay();
const { toggleVersioning } = useVersioning();

const folderInput = ref<HTMLInputElement>();
const fileInput = ref<HTMLInputElement>();
const menu = ref<boolean>(false);
const settingsMenu = ref<boolean>(false);
const isFetching = ref<boolean>(true);
const isInitialized = ref<boolean>(false);
const isDragging = ref<boolean>(false);
const snackbar = ref<boolean>(false);
const isBucketPassphraseDialogOpen = ref<boolean>(false);
const isNewFolderDialogOpen = ref<boolean>(false);
const isShareBucketDialogShown = ref<boolean>(false);
const isDeleteBucketDialogShown = ref<boolean>(false);

let passphraseDialogCallback: () => void = () => {};

/**
 * Whether versioning has been enabled for current project.
 */
const versioningUIEnabled = computed(() => projectsStore.versioningUIEnabled);

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
 * Returns condition if user has to be prompt for passphrase from store.
 */
const promptForPassphrase = computed((): boolean => {
    return bucketsStore.state.promptForPassphrase;
});

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
    menu.value = false;
    const fileInputElement = fileInput.value as HTMLInputElement;
    fileInputElement.showPicker();
    analyticsStore.eventTriggered(AnalyticsEvent.UPLOAD_FILE_CLICKED);
}

/**
 * Open the operating system's file system for folder upload.
 */
async function buttonFolderUpload(): Promise<void> {
    menu.value = false;
    const folderInputElement = folderInput.value as HTMLInputElement;
    folderInputElement.showPicker();
    analyticsStore.eventTriggered(AnalyticsEvent.UPLOAD_FOLDER_CLICKED);
}

/**
 * Initializes object browser store.
 */
function initObjectStore(): void {
    obStore.init({
        endpoint: edgeCredentials.value.endpoint,
        accessKey: edgeCredentials.value.accessKeyId,
        secretKey: edgeCredentials.value.secretKey,
        bucket: bucketName.value,
        browserRoot: '', // unused
    });
    isInitialized.value = true;
}

/**
 * Upload the current selected or dragged-and-dropped file.
 */
async function upload(e: Event): Promise<void> {
    if (isDragging.value) {
        isDragging.value = false;
    }

    await obStore.upload({ e });
    analyticsStore.eventTriggered(AnalyticsEvent.OBJECT_UPLOADED);
    const target = e.target as HTMLInputElement;
    target.value = '';
}

/**
 * Toggles versioning for the bucket between Suspended and Enabled.
 */
async function onToggleVersioning() {
    if (!bucket.value) {
        return;
    }

    try {
        await toggleVersioning(bucket.value?.name, bucket.value?.versioning);
        notify.success(`Versioning ${bucket.value.versioning !== Versioning.Enabled ? 'enabled' : 'suspended'} for bucket ${bucket.value.name}.`);
        await bucketsStore.getAllBucketsMetadata(projectId.value);
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.BUCKET_TABLE);
        return;
    }
}

function onBucketDeleted() {
    router.push({
        name: ROUTES.Buckets.name,
        params: { id: projectUrlId.value },
    });
}

/**
 * Displays the Share Bucket dialog.
 */
function showShareBucketDialog(): void {
    if (promptForPassphrase.value) {
        bucketsStore.setFileComponentBucketName(bucketName.value);
        isBucketPassphraseDialogOpen.value = true;
        passphraseDialogCallback = () => isShareBucketDialogShown.value = true;
        return;
    }
    isShareBucketDialogShown.value = true;
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

    let bucketName = '', filePath = '';
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
    try {
        await bucketsStore.getAllBucketsMetadata(projectId.value);
    } catch (error) {
        error.message = `Error fetching bucket names and/or placements. ${error.message}`;
        notify.notifyError(error, AnalyticsErrorEventSource.UPLOAD_FILE_VIEW);
        return;
    }

    if (!bucket.value) {
        router.push({
            name: ROUTES.Buckets.name,
            params: { id: projectUrlId.value },
        });
        return;
    }

    if (isPromptForPassphrase.value) {
        isBucketPassphraseDialogOpen.value = true;
    } else {
        initObjectStore();
    }

    isFetching.value = false;
});
</script>

<style scoped lang="scss">
.bucket-view {
    height: 100%;
}
</style>
