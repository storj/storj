// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <tr
        scope="row"
        :class="{ 'selected-row': isFileSelected }"
        @click.stop="selectFile"
    >
        <td data-ls-disabled class="px-3">
            <span v-if="fileTypeIsFolder" class="folder-name">
                <svg
                    class="ml-2 mr-1"
                    width="21"
                    height="18"
                    viewBox="0 0 21 18"
                    fill="none"
                    xmlns="http://www.w3.org/2000/svg"
                >
                    <path
                        d="M0 2.57143C0 1.15127 1.15127 0 2.57143 0H9C9.71008 0 10.2857 0.575634 10.2857 1.28571C10.2857 1.99579 10.8613 2.57143 11.5714 2.57143H18C19.4202 2.57143 20.5714 3.7227 20.5714 5.14286V15.4286C20.5714 16.8487 19.4202 18 18 18H2.57143C1.15127 18 0 16.8487 0 15.4286V2.57143Z"
                        fill="#768394"
                    />
                </svg>

                <span @click="openBucket">
                    <a
                        href="javascript:null"
                        class="file-name"
                        aria-roledescription="folder"
                    >
                        {{ file.Key }}
                    </a>
                </span>
            </span>

            <span
                v-else
                class="file-name"
                aria-roledescription="file"
                @click.stop="openModal"
            >
                <svg
                    width="1.5em"
                    height="1.5em"
                    viewBox="0 0 16 16"
                    class="bi bi-file-earmark mx-1 flex-shrink-0"
                    fill="#768394"
                    xmlns="http://www.w3.org/2000/svg"
                >
                    <path
                        d="M4 0h5.5v1H4a1 1 0 0 0-1 1v12a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1V4.5h1V14a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V2a2 2 0 0 1 2-2z"
                    />
                    <path d="M9.5 3V0L14 4.5h-3A1.5 1.5 0 0 1 9.5 3z" />
                </svg>
                <middle-truncate :text="file.Key" />
            </span>
        </td>
        <td>
            <span v-if="fileTypeIsFile" aria-roledescription="file-size">{{ size }}</span>
        </td>
        <td>
            <span
                v-if="fileTypeIsFile"
                aria-roledescription="file-upload-date"
            >{{ uploadDate }}</span>
        </td>
        <td class="text-right">
            <div v-if="fileTypeIsFile" class="d-inline-flex">
                <div class="dropleft">
                    <div
                        v-if="loadingSpinner()"
                        class="spinner-border"
                        role="status"
                    />
                    <button
                        v-else
                        class="btn btn-white btn-actions"
                        type="button"
                        aria-haspopup="true"
                        aria-expanded="false"
                        aria-roledescription="dropdown"
                        @click.stop="toggleDropdown"
                    >
                        <svg
                            width="4"
                            height="16"
                            viewBox="0 0 4 16"
                            fill="none"
                            xmlns="http://www.w3.org/2000/svg"
                        >
                            <path
                                d="M3.2 1.6C3.2 2.48366 2.48366 3.2 1.6 3.2C0.716344 3.2 0 2.48366 0 1.6C0 0.716344 0.716344 0 1.6 0C2.48366 0 3.2 0.716344 3.2 1.6Z"
                                fill="#7C8794"
                            />
                            <path
                                d="M3.2 8C3.2 8.88366 2.48366 9.6 1.6 9.6C0.716344 9.6 0 8.88366 0 8C0 7.11634 0.716344 6.4 1.6 6.4C2.48366 6.4 3.2 7.11634 3.2 8Z"
                                fill="#7C8794"
                            />
                            <path
                                d="M1.6 16C2.48366 16 3.2 15.2837 3.2 14.4C3.2 13.5163 2.48366 12.8 1.6 12.8C0.716344 12.8 0 13.5163 0 14.4C0 15.2837 0.716344 16 1.6 16Z"
                                fill="#7C8794"
                            />
                        </svg>
                    </button>
                    <div v-if="dropdownOpen" class="dropdown-menu shadow show">
                        <button
                            type="button"
                            class="dropdown-item action p-3"
                            @click.stop="openModal"
                        >
                            <DetailsIcon />
                            Details
                        </button>
                        <button
                            type="button"
                            class="dropdown-item action p-3"
                            @click.stop="download"
                        >
                            <DownloadIcon />
                            Download
                        </button>
                        <button
                            type="button"
                            class="dropdown-item action p-3"
                            @click.stop="share"
                        >
                            <ShareIcon />
                            Share
                        </button>
                        <button
                            v-if="!deleteConfirmation"
                            type="button"
                            class="dropdown-item action p-3 delete"
                            @click.stop="confirmDeletion"
                        >
                            <DeleteIcon />
                            Delete
                        </button>
                        <div v-else>
                            <p class="deletion-confirmation mx-5 pt-3">
                                Are you sure?
                            </p>
                            <div class="d-flex">
                                <button
                                    type="button"
                                    class="dropdown-item trash p-3 action"
                                    @click.stop="finalDelete"
                                >
                                    <svg
                                        xmlns="http://www.w3.org/2000/svg"
                                        width="15"
                                        height="15"
                                        fill="red"
                                        class="bi bi-trash"
                                        viewBox="0 0 16 16"
                                    >
                                        <path
                                            d="M5.5 5.5A.5.5 0 0 1 6 6v6a.5.5 0 0 1-1 0V6a.5.5 0 0 1 .5-.5zm2.5 0a.5.5 0 0 1 .5.5v6a.5.5 0 0 1-1 0V6a.5.5 0 0 1 .5-.5zm3 .5a.5.5 0 0 0-1 0v6a.5.5 0 0 0 1 0V6z"
                                        />
                                        <path
                                            fill-rule="evenodd"
                                            d="M14.5 3a1 1 0 0 1-1 1H13v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V4h-.5a1 1 0 0 1-1-1V2a1 1 0 0 1 1-1H6a1 1 0 0 1 1-1h2a1 1 0 0 1 1 1h3.5a1 1 0 0 1 1 1v1zM4.118 4L4 4.059V13a1 1 0 0 0 1 1h6a1 1 0 0 0 1-1V4.059L11.882 4H4.118zM2.5 3V2h11v1h-11z"
                                        />
                                    </svg>
                                    Yes
                                </button>
                                <button
                                    type="button"
                                    class="dropdown-item p-3 action"
                                    @click.stop="cancelDeletion"
                                >
                                    <svg
                                        width="1.5em"
                                        height="1.5em"
                                        viewBox="0 0 16 16"
                                        class="bi bi-x mr-1"
                                        xmlns="http://www.w3.org/2000/svg"
                                    >
                                        <path
                                            fill-rule="evenodd"
                                            d="M4.646 4.646a.5.5 0 0 1 .708 0L8 7.293l2.646-2.647a.5.5 0 0 1 .708.708L8.707 8l2.647 2.646a.5.5 0 0 1-.708.708L8 8.707l-2.646 2.647a.5.5 0 0 1-.708-.708L7.293 8 4.646 5.354a.5.5 0 0 1 0-.708z"
                                        />
                                    </svg>
                                    No
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            </div>

            <div v-else class="d-inline-flex">
                <div class="dropleft">
                    <div
                        v-if="loadingSpinner()"
                        class="spinner-border"
                        role="status"
                    />
                    <button
                        v-else
                        class="btn btn-white btn-actions"
                        type="button"
                        aria-haspopup="true"
                        aria-expanded="false"
                        aria-roledescription="dropdown"
                        @click.stop="toggleDropdown"
                    >
                        <svg
                            width="4"
                            height="16"
                            viewBox="0 0 4 16"
                            fill="none"
                            xmlns="http://www.w3.org/2000/svg"
                        >
                            <path
                                d="M3.2 1.6C3.2 2.48366 2.48366 3.2 1.6 3.2C0.716344 3.2 0 2.48366 0 1.6C0 0.716344 0.716344 0 1.6 0C2.48366 0 3.2 0.716344 3.2 1.6Z"
                                fill="#7C8794"
                            />
                            <path
                                d="M3.2 8C3.2 8.88366 2.48366 9.6 1.6 9.6C0.716344 9.6 0 8.88366 0 8C0 7.11634 0.716344 6.4 1.6 6.4C2.48366 6.4 3.2 7.11634 3.2 8Z"
                                fill="#7C8794"
                            />
                            <path
                                d="M1.6 16C2.48366 16 3.2 15.2837 3.2 14.4C3.2 13.5163 2.48366 12.8 1.6 12.8C0.716344 12.8 0 13.5163 0 14.4C0 15.2837 0.716344 16 1.6 16Z"
                                fill="#7C8794"
                            />
                        </svg>
                    </button>
                    <div v-if="dropdownOpen" class="dropdown-menu shadow show">
                        <button
                            v-if="!deleteConfirmation"
                            type="button"
                            class="dropdown-item action p-3 "
                            @click.stop="confirmDeletion"
                        >
                            <DeleteIcon />
                            Delete
                        </button>
                        <div v-else>
                            <p class="deletion-confirmation mx-5 pt-3">
                                Are you sure?
                            </p>
                            <div class="d-flex">
                                <button
                                    type="button"
                                    class="dropdown-item trash p-3 action"
                                    @click.stop="finalDelete"
                                >
                                    <svg
                                        xmlns="http://www.w3.org/2000/svg"
                                        width="15"
                                        height="15"
                                        fill="red"
                                        class="bi bi-trash"
                                        viewBox="0 0 16 16"
                                    >
                                        <path
                                            d="M5.5 5.5A.5.5 0 0 1 6 6v6a.5.5 0 0 1-1 0V6a.5.5 0 0 1 .5-.5zm2.5 0a.5.5 0 0 1 .5.5v6a.5.5 0 0 1-1 0V6a.5.5 0 0 1 .5-.5zm3 .5a.5.5 0 0 0-1 0v6a.5.5 0 0 0 1 0V6z"
                                        />
                                        <path
                                            fill-rule="evenodd"
                                            d="M14.5 3a1 1 0 0 1-1 1H13v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V4h-.5a1 1 0 0 1-1-1V2a1 1 0 0 1 1-1H6a1 1 0 0 1 1-1h2a1 1 0 0 1 1 1h3.5a1 1 0 0 1 1 1v1zM4.118 4L4 4.059V13a1 1 0 0 0 1 1h6a1 1 0 0 0 1-1V4.059L11.882 4H4.118zM2.5 3V2h11v1h-11z"
                                        />
                                    </svg>
                                    Yes
                                </button>
                                <button
                                    type="button"
                                    class="dropdown-item p-3 action"
                                    @click.stop="cancelDeletion"
                                >
                                    <svg
                                        width="1.5em"
                                        height="1.5em"
                                        viewBox="0 0 16 16"
                                        class="bi bi-x mr-1"
                                        xmlns="http://www.w3.org/2000/svg"
                                    >
                                        <path
                                            fill-rule="evenodd"
                                            d="M4.646 4.646a.5.5 0 0 1 .708 0L8 7.293l2.646-2.647a.5.5 0 0 1 .708.708L8.707 8l2.647 2.646a.5.5 0 0 1-.708.708L8 8.707l-2.646 2.647a.5.5 0 0 1-.708-.708L7.293 8 4.646 5.354a.5.5 0 0 1 0-.708z"
                                        />
                                    </svg>
                                    No
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </td>
    </tr>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import prettyBytes from 'pretty-bytes';

import MiddleTruncate from './MiddleTruncate.vue';

import type { BrowserFile } from '@/types/browser';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { useNotify, useRouter, useStore } from '@/utils/hooks';

import DeleteIcon from '@/../static/images/objects/delete.svg';
import ShareIcon from '@/../static/images/objects/share.svg';
import DetailsIcon from '@/../static/images/objects/details.svg';
import DownloadIcon from '@/../static/images/objects/download.svg';

const store = useStore();
const notify = useNotify();
const router = useRouter();

const props = defineProps<{
    path: string,
    file: BrowserFile,
}>();

const emit = defineEmits(['onUpdate']);

const deleteConfirmation = ref(false);

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
    return store.state.files.openedDropdown === props.file.Key;
});

/**
 * Return a link to the current folder for navigation.
 */
const link = computed((): string => {
    const browserRoot = store.state.files.browserRoot;
    const pathAndKey = store.state.files.path + props.file.Key;
    return pathAndKey.length > 0
        ? browserRoot + pathAndKey + '/'
        : browserRoot;
});

/**
 * Return a flag signifying whether the current file/folder is selected.
 */
const isFileSelected = computed((): boolean => {
    return Boolean(
        store.state.files.selectedAnchorFile === props.file ||
        store.state.files.selectedFiles.find(
            (file) => file === props.file,
        ) ||
        store.state.files.shiftSelectedFiles.find(
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
 * Open the modal for the current file.
 */
function openModal(): void {
    store.commit('files/setObjectPathForModal', props.path + props.file.Key);
    store.commit(APP_STATE_MUTATIONS.TOGGLE_OBJECT_DETAILS_MODAL_SHOWN);
    store.dispatch('files/closeDropdown');
}

/**
 * Return a boolean signifying whether the current file/folder is in the process of being deleted, therefore a spinner shoud be shown.
 */
function loadingSpinner(): boolean {
    return Boolean(store.state.files.filesToBeDeleted.find(
        (file) => file === props.file,
    ));
}

/**
 * Select the current file/folder whether it be a click, click + shiftKey, click + metaKey or ctrlKey, or unselect the rest.
 */
function selectFile(event: KeyboardEvent): void {
    if (store.state.files.openedDropdown) {
        store.dispatch('files/closeDropdown');
    }

    if (event.shiftKey) {
        setShiftSelectedFiles();

        return;
    }

    const isSelectedFile = Boolean(event.metaKey || event.ctrlKey);

    setSelectedFile(isSelectedFile);
}

async function openBucket(): Promise<void> {
    await router.push(link.value);
    emit('onUpdate');
}

/**
 * Set the selected file/folder in the store.
 */
function setSelectedFile(command: boolean): void {
    /* this function is responsible for selecting and unselecting a file on file click or [CMD + click] AKA command. */
    const shiftSelectedFiles =
        store.state.files.shiftSelectedFiles;
    const selectedFiles = store.state.files.selectedFiles;

    const files = [
        ...selectedFiles,
        ...shiftSelectedFiles,
    ];

    const selectedAnchorFile =
        store.state.files.selectedAnchorFile;

    if (command && props.file === selectedAnchorFile) {
        /* if it's [CMD + click] and the file selected is the actual selectedAnchorFile, then unselect the file but store it under unselectedAnchorFile in case the user decides to do a [shift + click] right after this action. */

        store.commit('files/setUnselectedAnchorFile', props.file);
        store.commit('files/setSelectedAnchorFile', null);
    } else if (command && files.includes(props.file)) {
        /* if it's [CMD + click] and the file selected is a file that has already been selected in selectedFiles and shiftSelectedFiles, then unselect it by filtering it out. */

        store.dispatch(
            'files/updateSelectedFiles',
            selectedFiles.filter(
                (fileSelected) => fileSelected !== props.file,
            ),
        );

        store.dispatch(
            'files/updateShiftSelectedFiles',
            shiftSelectedFiles.filter(
                (fileSelected) => fileSelected !== props.file,
            ),
        );
    } else if (command && selectedAnchorFile) {
        /* if it's [CMD + click] and there is already a selectedAnchorFile, then add the selectedAnchorFile and shiftSelectedFiles into the array of selectedFiles, set selectedAnchorFile to the file that was clicked, set unselectedAnchorFile to null, and set shiftSelectedFiles to an empty array. */

        const filesSelected = [...selectedFiles];

        if (!filesSelected.includes(selectedAnchorFile)) {
            filesSelected.push(selectedAnchorFile);
        }

        store.dispatch('files/updateSelectedFiles', [
            ...filesSelected,
            ...shiftSelectedFiles.filter(
                (file) => !filesSelected.includes(file),
            ),
        ]);

        store.commit('files/setSelectedAnchorFile', props.file);
        store.commit('files/setUnselectedAnchorFile', null);
        store.dispatch('files/updateShiftSelectedFiles', []);
    } else if (command) {
        /* if it's [CMD + click] and it has not met any of the above conditions, then set selectedAnchorFile to file and set unselectedAnchorfile to null, update the selectedFiles, and update the shiftSelectedFiles */

        store.commit('files/setSelectedAnchorFile', props.file);
        store.commit('files/setUnselectedAnchorFile', null);

        store.dispatch('files/updateSelectedFiles', [
            ...selectedFiles,
            ...shiftSelectedFiles,
        ]);

        store.dispatch('files/updateShiftSelectedFiles', []);
    } else {
        /* if it's just a file click without any modifier, then set selectedAnchorFile to the file that was clicked, set shiftSelectedFiles and selectedFiles to an empty array. */

        store.commit('files/setSelectedAnchorFile', props.file);
        store.dispatch('files/updateShiftSelectedFiles', []);
        store.dispatch('files/updateSelectedFiles', []);
    }
}

/**
 * Set files/folders selected using shift key in the store.
 */
function setShiftSelectedFiles(): void {
    /* this function is responsible for selecting all files from selectedAnchorFile to the file that was selected with [shift + click] */

    const files = store.getters['files/sortedFiles'];
    const unselectedAnchorFile =
        store.state.files.unselectedAnchorFile;

    if (unselectedAnchorFile) {
        /* if there is an unselectedAnchorFile, meaning that in the previous action the user unselected the anchor file but is now chosing to do a [shift + click] on another file, then reset the selectedAnchorFile, the achor file, to unselectedAnchorFile. */
        store.commit(
            'files/setSelectedAnchorFile',
            unselectedAnchorFile,
        );

        store.commit('files/setUnselectedAnchorFile', null);
    }

    const selectedAnchorFile = store.state.files.selectedAnchorFile;

    if (!selectedAnchorFile) {
        store.commit('files/setSelectedAnchorFile', props.file);

        return;
    }

    const anchorIdx = files.findIndex(
        (file) => file === selectedAnchorFile,
    );
    const shiftIdx = files.findIndex((file) => file === props.file);

    const start = Math.min(anchorIdx, shiftIdx);
    const end = Math.max(anchorIdx, shiftIdx) + 1;

    store.dispatch(
        'files/updateShiftSelectedFiles',
        files
            .slice(start, end)
            .filter(
                (file) =>
                    !store.state.files.selectedFiles.includes(
                        file,
                    ) && file !== selectedAnchorFile,
            ),
    );
}

/**
 * Open the share modal for the current file.
 */
function share(): void {
    store.dispatch('files/closeDropdown');
    store.commit('files/setObjectPathForModal', this.path + this.file.Key);
    store.commit(APP_STATE_MUTATIONS.TOGGLE_SHARE_OBJECT_MODAL_SHOWN);
}

/**
 * Toggle the dropdown for the current file/folder.
 */
function toggleDropdown(): void {
    (store.state.files.openedDropdown === props.file.Key) ?
        store.dispatch('files/closeDropdown')
        : store.dispatch('files/openDropdown', props.file.Key);

    // remove the dropdown delete confirmation
    deleteConfirmation.value = false;
}

/**
 * Download the current file.
 */
function download(): void {
    try {
        store.dispatch('files/download', props.file);
        notify.warning('Do not share download link with other people. If you want to share this data better use "Share" option.');
    } catch (error) {
        notify.error('Can not download your file');
    }

    store.dispatch('files/closeDropdown');
    deleteConfirmation.value = false;
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
    store.dispatch('files/closeDropdown');
    store.dispatch('files/addFileToBeDeleted', props.file);

    const params = { ...props };

    (props.file.type === 'file') ? await store.dispatch('files/delete', params) : store.dispatch('files/deleteFolder', params);

    // refresh the files displayed
    await store.dispatch('files/list');
    store.dispatch('files/removeFileFromToBeDeleted', props.file);
    deleteConfirmation.value = false;
}

/**
 * Abort the deletion of the current file/folder.
 */
function cancelDeletion(): void {
    store.dispatch('files/closeDropdown');
    deleteConfirmation.value = false;
}
</script>

<style>
a {
    text-decoration: none !important;
}

* {
    margin: 0;
    padding: 0;
}

.table td,
.table th {
    padding: 16px 16px 16px 0 !important;
    white-space: nowrap;
    vertical-align: middle !important;
}

.table-hover tbody tr:hover {
    background-color: #f9f9f9;
}

.selected-row {
    background-color: #f4f5f7;
}

.btn-actions {
    padding-top: 0;
    padding-bottom: 0;
}

.dropdown-menu {
    padding: 0;
}

.dropdown-item.action.p-3.action {
    font-family: 'Inter', sans-serif;
}

.dropdown-item svg {
    color: #768394;
}

.dropdown-item:focus svg,
.dropdown-item:hover svg {
    fill: #0149ff;
}

.dropdown-item.action.p-3.action:focus,
.dropdown-item.action.p-3.action:hover {
    background-color: #f4f5f7;
    font-weight: bold;
    color: #0149ff;
}

.deletion-confirmation {
    font-size: 14px;
    font-weight: bold;
}

.bi-trash {
    cursor: pointer;
}

.file-name {
    position: relative;
    margin-left: 5px;
    cursor: pointer;
    color: #000;
    display: flex;
    align-items: center;
}

.file-name:hover {
    color: #0149ff;
}

.file-name:hover svg path {
    fill: #0149ff;
}

.folder-name:hover {
    color: #0149ff;
}

.folder-name:hover svg path {
    fill: #0149ff;
}

.file-browser .dropleft .dropdown-menu {
    top: 40px !important;
    right: 10px !important;
    border: none;
    width: 255px;
    box-shadow: 0 20px 34px rgb(10 27 44 / 28%);
    border-radius: 6px;
    padding: 0;
    overflow: hidden;
}

.file-browser .p-3.action {
    padding: 17px 21px !important;
}

.file-browser .p-3.delete {
    border-top: 1px solid #e5e7eb;
}

.file-browser .p-3 svg {
    margin-right: 10px;
}

.file-browser .p-3:hover svg path {
    fill: #0149ff;
}
</style>
