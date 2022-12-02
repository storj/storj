// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <fragment>
        <th
            class="w-50"
            scope="col"
            @mouseover="mouseOver('name')"
            @mouseleave="mouseLeave"
            @click="sortBy('name')"
        >
            Name
            <span v-if="showArrow('name')">
                <a v-if="isDesc('name')" class="arrow">
                    <svg
                        xmlns="http://www.w3.org/2000/svg"
                        width="20"
                        height="20"
                        fill="currentColor"
                        class="bi bi-arrow-down-short down-arrow"
                        viewBox="0 0 16 16"
                    >
                        <path
                            fill-rule="evenodd"
                            d="M8 4a.5.5 0 0 1 .5.5v5.793l2.146-2.147a.5.5 0 0 1 .708.708l-3 3a.5.5 0 0 1-.708 0l-3-3a.5.5 0 1 1 .708-.708L7.5 10.293V4.5A.5.5 0 0 1 8 4z"
                        />
                    </svg>
                </a>
                <a v-else class="arrow">
                    <svg
                        xmlns="http://www.w3.org/2000/svg"
                        width="20"
                        height="20"
                        fill="currentColor"
                        class="bi bi-arrow-up-short"
                        viewBox="0 0 16 16"
                    >
                        <path
                            fill-rule="evenodd"
                            d="M8 12a.5.5 0 0 0 .5-.5V5.707l2.146 2.147a.5.5 0 0 0 .708-.708l-3-3a.5.5 0 0 0-.708 0l-3 3a.5.5 0 1 0 .708.708L7.5 5.707V11.5a.5.5 0 0 0 .5.5z"
                        />
                    </svg>
                </a>
            </span>
        </th>
        <th
            class="file-browser-heading w-25"
            scope="col"
            @mouseover="mouseOver('size')"
            @mouseleave="mouseLeave"
            @click="sortBy('size')"
        >
            Size
            <span v-if="showArrow('size')">
                <a v-if="isDesc('size')" class="arrow">
                    <svg
                        xmlns="http://www.w3.org/2000/svg"
                        width="20"
                        height="20"
                        fill="currentColor"
                        class="bi bi-arrow-down-short down-arrow"
                        viewBox="0 0 16 16"
                    >
                        <path
                            fill-rule="evenodd"
                            d="M8 4a.5.5 0 0 1 .5.5v5.793l2.146-2.147a.5.5 0 0 1 .708.708l-3 3a.5.5 0 0 1-.708 0l-3-3a.5.5 0 1 1 .708-.708L7.5 10.293V4.5A.5.5 0 0 1 8 4z"
                        />
                    </svg>
                </a>
                <a v-else class="arrow">
                    <svg
                        xmlns="http://www.w3.org/2000/svg"
                        width="20"
                        height="20"
                        fill="currentColor"
                        class="bi bi-arrow-up-short"
                        viewBox="0 0 16 16"
                    >
                        <path
                            fill-rule="evenodd"
                            d="M8 12a.5.5 0 0 0 .5-.5V5.707l2.146 2.147a.5.5 0 0 0 .708-.708l-3-3a.5.5 0 0 0-.708 0l-3 3a.5.5 0 1 0 .708.708L7.5 5.707V11.5a.5.5 0 0 0 .5.5z"
                        />
                    </svg>
                </a>
            </span>
        </th>
        <th
            class="file-browser-heading"
            scope="col"
            @mouseover="mouseOver('date')"
            @mouseleave="mouseLeave"
            @click="sortBy('date')"
        >
            Upload Date
            <span v-if="showArrow('date')">
                <a v-if="isDesc('date')" class="arrow">
                    <svg
                        xmlns="http://www.w3.org/2000/svg"
                        width="20"
                        height="20"
                        fill="currentColor"
                        class="bi bi-arrow-down-short down-arrow"
                        viewBox="0 0 16 16"
                    >
                        <path
                            fill-rule="evenodd"
                            d="M8 4a.5.5 0 0 1 .5.5v5.793l2.146-2.147a.5.5 0 0 1 .708.708l-3 3a.5.5 0 0 1-.708 0l-3-3a.5.5 0 1 1 .708-.708L7.5 10.293V4.5A.5.5 0 0 1 8 4z"
                        />
                    </svg>
                </a>
                <a v-else class="arrow">
                    <svg
                        xmlns="http://www.w3.org/2000/svg"
                        width="20"
                        height="20"
                        fill="currentColor"
                        class="bi bi-arrow-up-short"
                        viewBox="0 0 16 16"
                    >
                        <path
                            fill-rule="evenodd"
                            d="M8 12a.5.5 0 0 0 .5-.5V5.707l2.146 2.147a.5.5 0 0 0 .708-.708l-3-3a.5.5 0 0 0-.708 0l-3 3a.5.5 0 1 0 .708.708L7.5 5.707V11.5a.5.5 0 0 0 .5.5z"
                        />
                    </svg>
                </a>
            </span>
        </th>
        <th scope="col" class="overflow-override">
            <div class="dropleft">
                <a
                    v-if="filesToDelete"
                    id="header-delete"
                    class="d-flex justify-content-end"
                    @click.stop="deleteSelectedDropdown"
                >
                    <svg
                        xmlns="http://www.w3.org/2000/svg"
                        width="24"
                        height="24"
                        fill="currentColor"
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
                </a>
                <div
                    v-if="isDropdownDisplayed"
                    class="dropdown-menu shadow show"
                >
                    <div>
                        <p class="deletion-confirmation px-5 pt-3">
                            Are you sure?
                        </p>
                        <div class="d-flex">
                            <button
                                class="dropdown-item trash p-3 action"
                                type="button"
                                @click="confirmDeleteSelection"
                            >
                                <svg
                                    xmlns="http://www.w3.org/2000/svg"
                                    width="16"
                                    height="16"
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
                                class="dropdown-item p-3 action"
                                type="button"
                                @click="cancelDeleteSelection"
                            >
                                <svg
                                    width="2em"
                                    height="2em"
                                    viewBox="0 0 16 16"
                                    class="bi bi-x mr-1"
                                    fill="green"
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
        </th>
    </fragment>
</template>

<script setup lang="ts">
import { Fragment } from 'vue-fragment';
import { computed, ref } from 'vue';

import { useStore } from '@/utils/hooks';

const store = useStore();

const hover = ref('');

function fromFilesStore (prop: string): string {
    return store.state.files[prop];
}

/**
 * Check if the trashcan to delete selected files/folder should be displayed.
 */
const filesToDelete = computed( (): string => {
    return (!!store.state.files.selectedAnchorFile || (
        store.state.files.unselectedAnchorFile &&
        (store.state.files.selectedFiles.length > 0 ||
            store.state.files.shiftSelectedFiles.length > 0)
    ));
});

/**
 * Check if the files/folders deletion dropdown should be displayed.
 */
const isDropdownDisplayed = computed( (): boolean => {
    return store.state.files.openedDropdown === 'FileBrowser';
});

/**
 * Check if a heading is sorted in descending order.
 */
function isDesc(heading: string): boolean {
    return fromFilesStore('headingSorted') === heading && fromFilesStore('orderBy') === 'desc';
}

/**
 * Check if sorting arrow should be displayed.
 */
function showArrow(heading: string): boolean {
    return fromFilesStore('headingSorted') === heading || hover.value === heading;
}

/**
 * Set the heading of the current heading being hovered over.
 */
function mouseOver(heading: string): void {
    hover.value = heading;
}

/**
 * Set the heading for files/folders to be sorted by in the store.
 */
function sortBy(heading: string): void {
    store.commit('files/sort', heading);
}

/**
 * Change the hover property to nothing on mouse leave.
 */
function mouseLeave(): void {
    hover.value = '';
}

/**
 * Open the deletion of files/folders dropdown.
 */
function deleteSelectedDropdown(): void {
    store.dispatch('files/openFileBrowserDropdown');
}

/**
 * Delete the selected files/folders.
 */
function confirmDeleteSelection(): void {
    store.dispatch('files/deleteSelected');
    store.dispatch('files/closeDropdown');
}

/**
 * Abort files/folder selected for deletion.
 */
function cancelDeleteSelection(): void {
    store.dispatch('files/closeDropdown');
}
</script>

<style scoped>
th {
    user-select: none;
}

.file-browser-heading {
    padding: 16px 0;
}

.arrow {
    cursor: pointer;
    color: #768394;
}

th.overflow-override {
    overflow: unset;
}

a {
    cursor: pointer;
}
</style>
