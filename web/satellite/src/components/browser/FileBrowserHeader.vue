// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <fragment>
        <th
            class="w-50"
            scope="col"
            @mouseover="mouseOverName"
            @mouseleave="mouseLeave"
            @click="sortByName"
        >
            Name
            <span v-if="showNameArrow">
                <a v-if="nameDesc" class="arrow">
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
            @mouseover="mouseOverSize"
            @mouseleave="mouseLeave"
            @click="sortBySize"
        >
            Size
            <span v-if="showSizeArrow">
                <a v-if="sizeDesc" class="arrow">
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
            @mouseover="mouseOverDate"
            @mouseleave="mouseLeave"
            @click="sortByDate"
        >
            Upload Date
            <span v-if="showDateArrow">
                <a v-if="dateDesc" class="arrow">
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
                    @click="deleteSelectedDropdown"
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
                    v-if="displayDropdown"
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

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import { Fragment } from 'vue-fragment';

// @vue/component
@Component({
    components: {
        Fragment,
    },
})
export default class FileBrowserHeader extends Vue {
    private hover = '';

    /**
     * Check if a heading is sorted in descending order.
     */
    private isDesc(heading: string): boolean {
        return this.headingSorted === heading && this.orderBy === 'desc';
    }

    /**
     * Check if sorting arrow should be displayed.
     */
    private showArrow(heading: string): boolean {
        return this.headingSorted === heading || this.hover === heading;
    }

    /**
     * Retreive a string property from the store.
     */
    private fromFilesStore(prop: string): string {
        return this.$store.state.files[prop];
    }

    /**
     * Set the heading of the current heading being hovered over.
     */
    private mouseOver(heading: string): void {
        this.hover = heading;
    }

    /**
     * Set the heading for files/folders to be sorted by in the store.
     */
    private sortBy(heading: string): void {
        this.$store.commit('files/sort', heading);
    }

    /**
     * Get the current heading being sorted from the store.
     */
    private get headingSorted(): string {
        return this.fromFilesStore('headingSorted');
    }

    /**
     * Get the current order being sorted from the store.
     */
    private get orderBy(): string {
        return this.fromFilesStore('orderBy');
    }

    /**
     * Check if the name heading is being sorted in descending order.
     */
    public get nameDesc(): boolean {
        return this.isDesc('name');
    }

    /**
     * Check if the size heading is being sorted in descending order.
     */
    public get sizeDesc(): boolean {
        return this.isDesc('size');
    }

    /**
     * Check if the date heading is being sorted in descending order.
     */
    public get dateDesc(): boolean {
        return this.isDesc('date');
    }

    /**
     * Check if the name heading's arrow should be displayed.
     */
    public get showNameArrow(): boolean {
        return this.showArrow('name');
    }

    /**
     * Check if the size heading's arrow should be displayed.
     */
    public get showSizeArrow(): boolean {
        return this.showArrow('size');
    }

    /**
     * Check if the date heading's arrow should be displayed.
     */
    public get showDateArrow(): boolean {
        return this.showArrow('date');
    }

    /**
     * Check if the trashcan to delete selected files/folder should be displayed.
     */
    public get filesToDelete(): boolean {
        return (
            !!this.$store.state.files.selectedAnchorFile ||
            !!(
                this.$store.state.files.unselectedAnchorFile &&
                (this.$store.state.files.selectedFiles.length > 0 ||
                    this.$store.state.files.shiftSelectedFiles.length > 0)
            )
        );
    }

    /**
     * Check if the files/folders deletion dropdown should be displayed.
     */
    public get displayDropdown(): boolean {
        return this.$store.state.files.openedDropdown === 'FileBrowser';
    }

    /**
     * Sort files/folder based on their name.
     */
    public sortByName(): void {
        this.sortBy('name');
    }

    /**
     * Sort files/folder based on their size.
     */
    public sortBySize(): void {
        this.sortBy('size');
    }

    /**
     * Sort files/folder based on their date.
     */
    public sortByDate(): void {
        this.sortBy('date');
    }

    /**
     * Change the hover property to the name heading on hover.
     */
    public mouseOverName(): void {
        this.mouseOver('name');
    }

    /**
     * Change the hover property to the size heading on hover.
     */
    public mouseOverSize(): void {
        this.mouseOver('size');
    }

    /**
     * Change the hover property to the date heading on hover.
     */
    public mouseOverDate(): void {
        this.mouseOver('date');
    }

    /**
     * Change the hover property to nothing on mouse leave.
     */
    public mouseLeave(): void {
        this.hover = '';
    }

    /**
     * Open the deletion of files/folders dropdown.
     */
    public deleteSelectedDropdown(event: Event): void {
        event.stopPropagation();
        this.$store.dispatch('files/openFileBrowserDropdown');
    }

    /**
     * Delete the selected files/folders.
     */
    public confirmDeleteSelection(): void {
        this.$store.dispatch('files/deleteSelected');
        this.$store.dispatch('files/closeDropdown');
    }

    /**
     * Abort files/folder selected for deletion.
     */
    public cancelDeleteSelection(): void {
        this.$store.dispatch('files/closeDropdown');
    }
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
