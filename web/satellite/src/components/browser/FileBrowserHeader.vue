// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <fragment>
        <th
            class="align-left"
            @mouseover="mouseOver('name')"
            @mouseleave="mouseLeave"
            @click="sortBy('name')"
        >
            <span class="header__item">
                <span>Name</span>
                <span :class="{ invisible: nameSortData.isHidden }">
                    <a v-if="nameSortData.isDesc" class="arrow">
                        <desc-icon />
                    </a>
                    <a v-else class="arrow">
                        <asc-icon />
                    </a>
                </span>
            </span>
        </th>
        <th
            class="align-left"
            @mouseover="mouseOver('size')"
            @mouseleave="mouseLeave"
            @click="sortBy('size')"
        >
            <span class="header__item">
                <span>Size</span>
                <span :class="{ invisible: sizeSortData.isHidden }">
                    <a v-if="sizeSortData.isDesc" class="arrow">
                        <desc-icon />
                    </a>
                    <a v-else class="arrow">
                        <asc-icon />
                    </a>
                </span>
            </span>
        </th>
        <th
            class="align-left"
            @mouseover="mouseOver('date')"
            @mouseleave="mouseLeave"
            @click="sortBy('date')"
        >
            <span class="header__item">
                <span>Upload Date</span>
                <span :class="{ invisible: dateSortData.isHidden }">
                    <a v-if="dateSortData.isDesc" class="arrow">
                        <desc-icon />
                    </a>
                    <a v-else class="arrow">
                        <asc-icon />
                    </a>
                </span>
            </span>
        </th>
        <th class="header__functional overflow-visible" @click.stop="deleteSelectedDropdown">
            <delete-icon v-if="filesToDelete" />
            <div v-if="isDropdownDisplayed" class="header__functional__dropdown">
                <div class="header__functional__dropdown__item">
                    <div class="delete-confirmation">
                        <p class="delete-confirmation__text">
                            Are you sure?
                        </p>
                        <div class="delete-confirmation__options">
                            <span
                                class="delete-confirmation__options__item yes"
                                @click.stop="confirmDeleteSelection"
                            >
                                <span><delete-icon /></span>
                                <span>Yes</span>
                            </span>

                            <span
                                class="delete-confirmation__options__item no"
                                @click.stop="cancelDeleteSelection"
                            >
                                <span><close-icon /></span>
                                <span>No</span>
                            </span>
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

import AscIcon from '@/../static/images/objects/asc.svg';
import CloseIcon from '@/../static/images/common/closeCross.svg';
import DescIcon from '@/../static/images/objects/desc.svg';
import DeleteIcon from '@/../static/images/objects/delete.svg';

const store = useStore();

const hover = ref('');

function fromFilesStore(prop: string): string {
    return store.state.files[prop];
}

/**
 * Check if the trashcan to delete selected files/folder should be displayed.
 */
const filesToDelete = computed((): boolean => {
    return (!!store.state.files.selectedAnchorFile || (
        !!store.state.files.unselectedAnchorFile &&
      (store.state.files.selectedFiles.length > 0 ||
          store.state.files.shiftSelectedFiles.length > 0)
    ));
});

/**
 * Check if the files/folders deletion dropdown should be displayed.
 */
const isDropdownDisplayed = computed((): boolean => {
    return store.state.files.openedDropdown === 'FileBrowser';
});

const nameSortData = computed((): { isHidden: boolean, isDesc: boolean } => {
    return {
        isHidden: !showArrow('name'),
        isDesc: isDesc('name'),
    };
});

const sizeSortData = computed((): { isHidden: boolean, isDesc: boolean } => {
    return {
        isHidden: !showArrow('size'),
        isDesc: isDesc('size'),
    };
});

const dateSortData = computed((): { isHidden: boolean, isDesc: boolean } => {
    return {
        isHidden: !showArrow('date'),
        isDesc: isDesc('date'),
    };
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

<style scoped lang="scss">
.header {

    &__item {
        display: flex;
        align-items: center;
        gap: 5px;

        & > .invisible {
            visibility: hidden;
        }
    }

    &__functional {
        padding: 0;
        width: 50px;
        position: relative;
        cursor: pointer;

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

                &:hover {
                    background-color: #f4f5f7;
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

</style>
