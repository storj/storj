// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <table-item
        v-if="fileTypeIsFile"
        selectable
        :selected="isFileSelected"
        :on-click="openModal"
        :on-primary-click="openModal"
        :item="{'fileName': file.Key, 'size': size, 'date': uploadDate}"
        :item-type="fileType"
        @selectClicked="selectFile"
    >
        <template #options>
            <th v-click-outside="closeDropdown" class="file-entry__functional options overflow-visible" @click.stop="openDropdown">
                <div
                    v-if="loadingSpinner"
                    class="spinner-border"
                    role="status"
                />
                <dots-icon v-else />
                <div v-if="dropdownOpen" class="file-entry__functional__dropdown">
                    <div class="file-entry__functional__dropdown__item" @click.stop="openModal">
                        <preview-icon />
                        <p class="file-entry__functional__dropdown__item__label">Preview</p>
                    </div>

                    <div class="file-entry__functional__dropdown__item" @click.stop="download">
                        <download-icon />
                        <p class="file-entry__functional__dropdown__item__label">Download</p>
                    </div>

                    <div class="file-entry__functional__dropdown__item" @click.stop="share">
                        <share-icon />
                        <p class="file-entry__functional__dropdown__item__label">Share</p>
                    </div>

                    <div v-if="!deleteConfirmation" class="file-entry__functional__dropdown__item" @click.stop="confirmDeletion">
                        <delete-icon />
                        <p class="file-entry__functional__dropdown__item__label">Delete</p>
                    </div>
                    <div v-else class="file-entry__functional__dropdown__item confirmation">
                        <div class="delete-confirmation">
                            <p class="delete-confirmation__text">
                                Are you sure?
                            </p>
                            <div class="delete-confirmation__options">
                                <span class="delete-confirmation__options__item yes" @click.stop="finalDelete">
                                    <span><delete-icon /></span>
                                    <span>Yes</span>
                                </span>

                                <span class="delete-confirmation__options__item no" @click.stop="cancelDeletion">
                                    <span><close-icon /></span>
                                    <span>No</span>
                                </span>
                            </div>
                        </div>
                    </div>
                </div>
            </th>
        </template>
    </table-item>
    <table-item
        v-else-if="fileTypeIsFolder"
        :item="{'name': file.Key, 'size': '', 'date': ''}"
        selectable
        :selected="isFileSelected"
        :on-click="openFolder"
        :on-primary-click="openFolder"
        item-type="folder"
        @selectClicked="selectFile"
    >
        <template #options>
            <th v-click-outside="closeDropdown" class="file-entry__functional options overflow-visible" @click.stop="openDropdown">
                <div
                    v-if="loadingSpinner"
                    class="spinner-border"
                    role="status"
                />
                <dots-icon v-else />
                <div v-if="dropdownOpen" class="file-entry__functional__dropdown">
                    <div
                        v-if="!deleteConfirmation" class="file-entry__functional__dropdown__item"
                        @click.stop="confirmDeletion"
                    >
                        <delete-icon />
                        <p class="file-entry__functional__dropdown__item__label">Delete</p>
                    </div>
                    <div v-else class="file-entry__functional__dropdown__item confirmation">
                        <div class="delete-confirmation">
                            <p class="delete-confirmation__text">
                                Are you sure?
                            </p>
                            <div class="delete-confirmation__options">
                                <span class="delete-confirmation__options__item yes" @click.stop="finalDelete">
                                    <span><delete-icon /></span>
                                    <span>Yes</span>
                                </span>

                                <span class="delete-confirmation__options__item no" @click.stop="cancelDeletion">
                                    <span><close-icon /></span>
                                    <span>No</span>
                                </span>
                            </div>
                        </div>
                    </div>
                </div>
            </th>
        </template>
    </table-item>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { useRouter } from 'vue-router';
import prettyBytes from 'pretty-bytes';

import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { BrowserObject, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';
import { useAppStore } from '@/store/modules/appStore';
import { useConfigStore } from '@/store/modules/configStore';
import { AnalyticsHttpApi } from '@/api/analytics';
import { ObjectType } from '@/utils/objectIcon';

import TableItem from '@/components/common/TableItem.vue';

import PreviewIcon from '@/../static/images/objects/preview.svg';
import DeleteIcon from '@/../static/images/objects/delete.svg';
import ShareIcon from '@/../static/images/objects/share.svg';
import DownloadIcon from '@/../static/images/objects/download.svg';
import DotsIcon from '@/../static/images/objects/dots.svg';
import CloseIcon from '@/../static/images/common/closeCross.svg';

const appStore = useAppStore();
const obStore = useObjectBrowserStore();
const config = useConfigStore();
const notify = useNotify();
const router = useRouter();

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

const props = defineProps<{
  path: string,
  file: BrowserObject,
}>();

const emit = defineEmits(['onUpdate']);

const deleteConfirmation = ref(false);

/**
 * Return the type of the file.
 */
const fileType = computed((): string => ObjectType.findType(props.file.Key));

/**
 * Return the size of the file formatted.
 */
const size = computed((): string => {
    return prettyBytes(props.file.Size);
});

/**
 * Return the upload date of the file formatted.
 */
const uploadDate = computed((): string => {
    return props.file.LastModified.toLocaleString().split(',')[0];
});

/**
 * Check with the store to see if the dropdown is open for the current file/folder.
 */
const dropdownOpen = computed((): boolean => {
    return obStore.state.openedDropdown === props.file.Key;
});

/**
 * Return a link to the current folder for navigation.
 */
const link = computed((): string => {
    const browserRoot = obStore.state.browserRoot;
    const uriParts = (obStore.state.path + props.file.Key).split('/');
    const pathAndKey = uriParts.map(part => encodeURIComponent(part)).join('/');
    return pathAndKey.length > 0
        ? browserRoot + pathAndKey + '/'
        : browserRoot;
});

/**
 * Return a flag signifying whether the current file/folder is selected.
 */
const isFileSelected = computed((): boolean => {
    return Boolean(
        obStore.state.selectedAnchorFile === props.file ||
        obStore.state.selectedFiles.find(
            (file) => file === props.file,
        ) ||
        obStore.state.shiftSelectedFiles.find(
            (file) => file === props.file,
        ),
    );
});

/**
 * Return a boolean signifying whether the current file/folder is a folder.
 */
const fileTypeIsFolder = computed((): boolean => {
    return props.file.type === 'folder';
});

/**
 * Return a boolean signifying whether the current file/folder is a file.
 */
const fileTypeIsFile = computed((): boolean => {
    return props.file.type === 'file';
});

/**
 * Return a boolean signifying whether the current file/folder is in the process of being deleted, therefore a spinner shoud be shown.
 */
const loadingSpinner = computed((): boolean => {
    return obStore.state.filesToBeDeleted.some(
        (file) => file.Key === props.file.Key,
    );
});

/**
 * Open the modal for the current file.
 */
function openModal(): void {
    obStore.setObjectPathForModal(props.path + props.file.Key);

    if (config.state.config.galleryViewEnabled) {
        appStore.setGalleryView(true);
        analytics.eventTriggered(AnalyticsEvent.GALLERY_VIEW_CLICKED);
    } else {
        appStore.updateActiveModal(MODALS.objectDetails);
    }

    obStore.closeDropdown();
}

/**
 * Select the current file/folder whether it be a click, click + shiftKey, click + metaKey or ctrlKey, or unselect the rest.
 */
function selectFile(event: KeyboardEvent): void {
    if (obStore.state.openedDropdown) {
        obStore.closeDropdown();
    }

    if (event.shiftKey) {
        setShiftSelectedFiles();

        return;
    }

    const isSelectedFile = Boolean(event.metaKey || event.ctrlKey);

    setSelectedFile(isSelectedFile);
}

async function openFolder(): Promise<void> {
    await router.push(link.value);
    obStore.clearAllSelectedFiles();
    emit('onUpdate');
}

/**
 * Set the selected file/folder in the store.
 */
function setSelectedFile(command: boolean): void {
    /* this function is responsible for selecting and unselecting a file on file click or [CMD + click] AKA command. */
    const shiftSelectedFiles = obStore.state.shiftSelectedFiles;
    const selectedFiles = obStore.state.selectedFiles;

    const files = [
        ...selectedFiles,
        ...shiftSelectedFiles,
    ];

    const selectedAnchorFile = obStore.state.selectedAnchorFile;

    if (command && props.file === selectedAnchorFile) {
        /* if it's [CMD + click] and the file selected is the actual selectedAnchorFile, then unselect the file but store it under unselectedAnchorFile in case the user decides to do a [shift + click] right after this action. */
        obStore.setUnselectedAnchorFile(props.file);
        obStore.setSelectedAnchorFile(null);
    } else if (command && files.includes(props.file)) {
        /* if it's [CMD + click] and the file selected is a file that has already been selected in selectedFiles and shiftSelectedFiles, then unselect it by filtering it out. */
        obStore.updateSelectedFiles(selectedFiles.filter((fileSelected) => fileSelected !== props.file));
        obStore.updateShiftSelectedFiles(shiftSelectedFiles.filter((fileSelected) => fileSelected !== props.file));
    } else if (command && selectedAnchorFile) {
        /* if it's [CMD + click] and there is already a selectedAnchorFile, then add the selectedAnchorFile and shiftSelectedFiles into the array of selectedFiles, set selectedAnchorFile to the file that was clicked, set unselectedAnchorFile to null, and set shiftSelectedFiles to an empty array. */
        const filesSelected = [...selectedFiles];

        if (!filesSelected.includes(selectedAnchorFile)) {
            filesSelected.push(selectedAnchorFile);
        }

        obStore.updateSelectedFiles([
            ...filesSelected,
            ...shiftSelectedFiles.filter(
                (file) => !filesSelected.includes(file),
            ),
        ]);

        obStore.setSelectedAnchorFile(props.file);
        obStore.setUnselectedAnchorFile(null);
        obStore.updateShiftSelectedFiles([]);
    } else if (command) {
        /* if it's [CMD + click] and it has not met any of the above conditions, then set selectedAnchorFile to file and set unselectedAnchorfile to null, update the selectedFiles, and update the shiftSelectedFiles */
        obStore.setSelectedAnchorFile(props.file);
        obStore.setUnselectedAnchorFile(null);
        obStore.updateSelectedFiles([
            ...selectedFiles,
            ...shiftSelectedFiles,
        ]);
        obStore.updateShiftSelectedFiles([]);
    } else {
        /* if it's just a file click without any modifier ... */
        const newSelection = [...files];
        const fileIdx = newSelection.findIndex((file) => file === props.file);
        switch (true) {
        case fileIdx !== -1:
            // this file is already selected, deselect.
            newSelection.splice(fileIdx, 1);
            break;
        case selectedAnchorFile === props.file:
            // this file is already selected, deselect.
            obStore.setSelectedAnchorFile(null);
            obStore.setUnselectedAnchorFile(props.file);
            break;
        case !!selectedAnchorFile:
            // there's an anchor file, but not this file.
            // add the anchor file to the selection arr and make this file the anchor file.
            newSelection.push(selectedAnchorFile as BrowserObject);
            obStore.setSelectedAnchorFile(props.file);
            break;
        default:
            obStore.setSelectedAnchorFile(props.file);
        }

        obStore.updateShiftSelectedFiles([]);
        obStore.updateSelectedFiles(newSelection);
    }
}

/**
 * Set files/folders selected using shift key in the store.
 */
function setShiftSelectedFiles(): void {
    /* this function is responsible for selecting all files from selectedAnchorFile to the file that was selected with [shift + click] */
    const files = obStore.sortedFiles;
    const unselectedAnchorFile = obStore.state.unselectedAnchorFile;

    if (unselectedAnchorFile) {
        /* if there is an unselectedAnchorFile, meaning that in the previous action the user unselected the anchor file but is now chosing to do a [shift + click] on another file, then reset the selectedAnchorFile, the achor file, to unselectedAnchorFile. */
        obStore.setSelectedAnchorFile(unselectedAnchorFile);
        obStore.setUnselectedAnchorFile(null);
    }

    const selectedAnchorFile = obStore.state.selectedAnchorFile;
    if (!selectedAnchorFile) {
        obStore.setSelectedAnchorFile(props.file);

        return;
    }

    const anchorIdx = files.findIndex(
        (file) => file === selectedAnchorFile,
    );
    const shiftIdx = files.findIndex((file) => file === props.file);

    const start = Math.min(anchorIdx, shiftIdx);
    const end = Math.max(anchorIdx, shiftIdx) + 1;

    obStore.updateShiftSelectedFiles(
        files
            .slice(start, end)
            .filter((file) => !obStore.state.selectedFiles.includes(file) && file !== selectedAnchorFile),
    );
}

/**
 * Open the share modal for the current file.
 */
function share(): void {
    obStore.closeDropdown();
    obStore.setObjectPathForModal(props.path + props.file.Key);
    appStore.updateActiveModal(MODALS.shareObject);
}

/**
 * Close the dropdown.
 */
function closeDropdown(): void {
    obStore.closeDropdown();

    // remove the dropdown delete confirmation
    deleteConfirmation.value = false;
}

/**
 * Open the dropdown for the current file/folder.
 */
function openDropdown(): void {
    obStore.openDropdown(props.file.Key);

    // remove the dropdown delete confirmation
    deleteConfirmation.value = false;
}

/**
 * Download the current file.
 */
async function download(): Promise<void> {
    try {
        await obStore.download(props.file);
        const message = `
            <p class="message-title">Downloading...</p>
            <p class="message-info">
                Keep this download link private.<br>If you want to share, use the Share option.
            </p>
        `;
        notify.success('', message);
    } catch (error) {
        notify.error('Can not download your file', AnalyticsErrorEventSource.FILE_BROWSER_ENTRY);
    }

    closeDropdown();
}

/**
 * Set the data property deleteConfirmation to true, signifying that this user does in fact want the current selected file/folder.
 */
function confirmDeletion(): void {
    deleteConfirmation.value = true;
}

/**
 * Delete the selected file/folder.
 */
async function finalDelete(): Promise<void> {
    obStore.closeDropdown();
    obStore.addFileToBeDeleted(props.file);

    props.file.type === 'file' ?
        await obStore.deleteObject(props.path, props.file) :
        await obStore.deleteFolder(props.file, props.path);

    // refresh the files displayed
    try {
        await obStore.list();
    } catch (error) {
        notify.error(error.message, AnalyticsErrorEventSource.FILE_BROWSER_ENTRY);
    }

    obStore.removeFileFromToBeDeleted(props.file);
    deleteConfirmation.value = false;
}

/**
 * Abort the deletion of the current file/folder.
 */
function cancelDeletion(): void {
    obStore.closeDropdown();
    deleteConfirmation.value = false;
}
</script>

<style scoped lang="scss">
.file-entry {

    &__functional {
        padding: 0;
        width: 50px;
        position: relative;
        cursor: pointer;

        @media screen and (width <= 550px) {
            padding: 0 10px;
            width: unset;
        }

        &__dropdown {
            position: absolute;
            top: 25px;
            right: 15px;
            background: #fff;
            box-shadow: 0 20px 34px rgb(10 27 44 / 28%);
            border-radius: 6px;
            width: 255px;
            z-index: 100;

            &__item {
                display: flex;
                align-items: center;
                padding: 20px 25px;
                width: calc(100% - 50px);

                & > svg {
                    width: 20px;
                }

                .dropdown-item.action.p-3.action {
                    font-family: 'font_regular', sans-serif;
                }

                &:first-of-type {
                    border-radius: 6px 6px 0 0;
                }

                &:last-of-type {
                    border-radius: 0 0 6px 6px;
                }

                &__label {
                    margin: 0 0 0 10px;
                }

                &:not(.confirmation):hover {
                    background-color: #f4f5f7;
                    font-family: 'font_medium', sans-serif;
                    color: var(--c-blue-3);

                    svg :deep(path) {
                        fill: var(--c-blue-3);
                    }
                }
            }
        }
    }
}

.delete-confirmation {
    display: flex;
    flex-direction: column;
    gap: 5px;
    align-items: flex-start;
    width: 100%;

    &__options {
        display: flex;
        gap: 20px;
        align-items: center;

        &__item {
            display: flex;
            gap: 5px;
            align-items: center;

            &.yes:hover {
                color: var(--c-red-2);

                svg :deep(path) {
                    fill: var(--c-red-2);
                    stroke: var(--c-red-2);
                }
            }

            &.no:hover {
                color: var(--c-blue-3);

                svg :deep(path) {
                    fill: var(--c-blue-3);
                    stroke: var(--c-blue-3);
                }
            }
        }
    }
}

@media screen and (width <= 550px) {
    // hide size, upload date columns on mobile screens

    :deep(.data:not(:nth-child(2))) {
        display: none;
    }
}

@keyframes spinner-border {

    to {
        transform: rotate(360deg);
    }
}

.spinner-border {
    display: inline-block;
    width: 2rem;
    height: 2rem;
    vertical-align: text-bottom;
    border: 0.25em solid currentcolor;
    border-right-color: transparent;
    border-radius: 50%;
    animation: 0.75s linear infinite spinner-border;
}

:deep(.primary) {
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
}

:deep(th) {
    max-width: 26rem;
}

@media screen and (width <= 940px) {

    :deep(th) {
        max-width: 15rem;
    }
}

@media screen and (width <= 650px) {

    :deep(th) {
        max-width: 10rem;
    }
}
</style>
