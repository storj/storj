// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <tr
        scope="row"
        :class="{ 'selected-row': isFileSelected }"
        @click.stop="selectFile"
    >
        <td class="w-50" data-ls-disabled>
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

                <span @click.stop="fileClick">
                    <router-link :to="link">
                        <a
                            href="javascript:null"
                            class="file-name"
                            aria-roledescription="folder"
                        >
                            {{ filename }}
                        </a>
                    </router-link>
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
                    class="bi bi-file-earmark ml-1 mr-1 mb-1"
                    fill="#768394"
                    xmlns="http://www.w3.org/2000/svg"
                >
                    <path
                        d="M4 0h5.5v1H4a1 1 0 0 0-1 1v12a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1V4.5h1V14a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V2a2 2 0 0 1 2-2z"
                    />
                    <path d="M9.5 3V0L14 4.5h-3A1.5 1.5 0 0 1 9.5 3z" />
                </svg>

                {{ filename }}
            </span>
        </td>
        <td class="w-25">
            <span v-if="fileTypeIsFile" aria-roledescription="file-size">{{
                size
            }}</span>
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
                            <svg
                                xmlns="http://www.w3.org/2000/svg"
                                width="1.2em"
                                height="1.2em"
                                fill="currentColor"
                                class="bi bi-eye mr-2 ml-1"
                                viewBox="0 0 16 16"
                            >
                                <path
                                    d="M16 8s-3-5.5-8-5.5S0 8 0 8s3 5.5 8 5.5S16 8 16 8zM1.173 8a13.133 13.133 0 0 1 1.66-2.043C4.12 4.668 5.88 3.5 8 3.5c2.12 0 3.879 1.168 5.168 2.457A13.133 13.133 0 0 1 14.828 8c-.058.087-.122.183-.195.288-.335.48-.83 1.12-1.465 1.755C11.879 11.332 10.119 12.5 8 12.5c-2.12 0-3.879-1.168-5.168-2.457A13.134 13.134 0 0 1 1.172 8z"
                                />
                                <path
                                    d="M8 5.5a2.5 2.5 0 1 0 0 5 2.5 2.5 0 0 0 0-5zM4.5 8a3.5 3.5 0 1 1 7 0 3.5 3.5 0 0 1-7 0z"
                                />
                            </svg>
                            Details
                        </button>
                        <button
                            type="button"
                            class="dropdown-item action p-3"
                            @click.stop="download"
                        >
                            <svg
                                width="1.2em"
                                height="1.2em"
                                viewBox="0 0 16 16"
                                class="bi bi-cloud-download mr-2 ml-1"
                                fill="currentColor"
                                xmlns="http://www.w3.org/2000/svg"
                            >
                                <path
                                    fill-rule="evenodd"
                                    d="M4.406 1.342A5.53 5.53 0 0 1 8 0c2.69 0 4.923 2 5.166 4.579C14.758 4.804 16 6.137 16 7.773 16 9.569 14.502 11 12.687 11H10a.5.5 0 0 1 0-1h2.688C13.979 10 15 8.988 15 7.773c0-1.216-1.02-2.228-2.313-2.228h-.5v-.5C12.188 2.825 10.328 1 8 1a4.53 4.53 0 0 0-2.941 1.1c-.757.652-1.153 1.438-1.153 2.055v.448l-.445.049C2.064 4.805 1 5.952 1 7.318 1 8.785 2.23 10 3.781 10H6a.5.5 0 0 1 0 1H3.781C1.708 11 0 9.366 0 7.318c0-1.763 1.266-3.223 2.942-3.593.143-.863.698-1.723 1.464-2.383z"
                                />
                                <path
                                    fill-rule="evenodd"
                                    d="M7.646 15.854a.5.5 0 0 0 .708 0l3-3a.5.5 0 0 0-.708-.708L8.5 14.293V5.5a.5.5 0 0 0-1 0v8.793l-2.146-2.147a.5.5 0 0 0-.708.708l3 3z"
                                />
                            </svg>
                            Download
                        </button>
                        <button
                            type="button"
                            class="dropdown-item action p-3"
                            @click.stop="share"
                        >
                            <svg
                                width="1.5em"
                                height="1.5em"
                                viewBox="0 0 16 16"
                                class="bi bi-link-45deg mr-1"
                                fill="currentColor"
                                xmlns="http://www.w3.org/2000/svg"
                            >
                                <path
                                    d="M4.715 6.542L3.343 7.914a3 3 0 1 0 4.243 4.243l1.828-1.829A3 3 0 0 0 8.586 5.5L8 6.086a1.001 1.001 0 0 0-.154.199 2 2 0 0 1 .861 3.337L6.88 11.45a2 2 0 1 1-2.83-2.83l.793-.792a4.018 4.018 0 0 1-.128-1.287z"
                                />
                                <path
                                    d="M6.586 4.672A3 3 0 0 0 7.414 9.5l.775-.776a2 2 0 0 1-.896-3.346L9.12 3.55a2 2 0 0 1 2.83 2.83l-.793.792c.112.42.155.855.128 1.287l1.372-1.372a3 3 0 0 0-4.243-4.243L6.586 4.672z"
                                />
                            </svg>
                            Share
                        </button>
                        <button
                            v-if="!deleteConfirmation"
                            type="button"
                            class="dropdown-item action p-3"
                            @click.stop="confirmDeletion"
                        >
                            <svg
                                width="1.5em"
                                height="1.5em"
                                viewBox="0 0 16 16"
                                class="bi bi-x mr-1"
                                fill="currentColor"
                                xmlns="http://www.w3.org/2000/svg"
                            >
                                <path
                                    fill-rule="evenodd"
                                    d="M4.646 4.646a.5.5 0 0 1 .708 0L8 7.293l2.646-2.647a.5.5 0 0 1 .708.708L8.707 8l2.647 2.646a.5.5 0 0 1-.708.708L8 8.707l-2.646 2.647a.5.5 0 0 1-.708-.708L7.293 8 4.646 5.354a.5.5 0 0 1 0-.708z"
                                />
                            </svg>
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
                            class="dropdown-item action p-3"
                            @click.stop="confirmDeletion"
                        >
                            <svg
                                width="1.5em"
                                height="1.5em"
                                viewBox="0 0 16 16"
                                class="bi bi-x mr-1"
                                fill="currentColor"
                                xmlns="http://www.w3.org/2000/svg"
                            >
                                <path
                                    fill-rule="evenodd"
                                    d="M4.646 4.646a.5.5 0 0 1 .708 0L8 7.293l2.646-2.647a.5.5 0 0 1 .708.708L8.707 8l2.647 2.646a.5.5 0 0 1-.708.708L8 8.707l-2.646 2.647a.5.5 0 0 1-.708-.708L7.293 8 4.646 5.354a.5.5 0 0 1 0-.708z"
                                />
                            </svg>
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

<script lang="ts">
import { Component, Prop, Vue } from "vue-property-decorator";
import type { BrowserFile } from "@/types/browser.ts";
import prettyBytes from "pretty-bytes";

// @vue/component
@Component
export default class FileEntry extends Vue {
    public deleteConfirmation = false;

    @Prop({default: ""})
    private readonly path: string;
    @Prop()
    private readonly file: BrowserFile;

    /**
     * Return the name of file/folder formatted.
     */
    public get filename(): string {
        return this.file.Key.length > 25
            ? this.file.Key.slice(0, 25) + "..."
            : this.file.Key;
    }

    /**
     * Return the size of the file formatted.
     */
    public get size(): string {
        return prettyBytes(this.file.Size);
    }

    /**
     * Return the upload date of the file formatted.
     */
    public get uploadDate(): string {
        return this.file.LastModified.toLocaleString().split(",")[0];
    }

    /**
     * Check with the store to see if the dropdown is open for the current file/folder.
     */
    public get dropdownOpen(): boolean {
        return this.$store.state.files.openedDropdown === this.file.Key;
    }

    /**
     * Return a link to the current folder for navigation.
     */
    public get link(): string {
        const browserRoot = this.$store.state.files.browserRoot;
        const pathAndKey = this.$store.state.files.path + this.file.Key;
        const url =
            pathAndKey.length > 0
                ? browserRoot + pathAndKey + "/"
                : browserRoot;
        return url;
    }

    /**
     * Return a boolean signifying whether the current file/folder is selected.
     */
    public get isFileSelected(): boolean {
        return !!(
            this.$store.state.files.selectedAnchorFile === this.file ||
            this.$store.state.files.selectedFiles.find(
                (file) => file === this.file
            ) ||
            this.$store.state.files.shiftSelectedFiles.find(
                (file) => file === this.file
            )
        );
    }

    /**
     * Return a boolean signifying whether the current file/folder is a folder.
     */
    public get fileTypeIsFolder(): boolean {
        return this.file.type === "folder";
    }

    /**
     * Return a boolean signifying whether the current file/folder is a folder.
     */
    public get fileTypeIsFile(): boolean {
        return this.file.type === "file";
    }

    /**
     * Open the modal for the current file.
     */
    public openModal(): void {
        this.$store.commit("files/openModal", this.path + this.file.Key);
        this.$store.dispatch("files/closeDropdown");
    }

    /**
     * Return a boolean signifying whether the current file/folder is in the process of being deleted, therefore a spinner shoud be shown.
     */
    public loadingSpinner(): boolean {
        return !!this.$store.state.files.filesToBeDeleted.find(
            (file) => file === this.file
        );
    }

    /**
     * Hide the folder creation input on navigation due to folder click.
     */
    public fileClick(): void {
        this.$store.dispatch("files/updateCreateFolderInputShow", false);
    }

    /**
     * Select the current file/folder whether it be a click, click + shiftKey, click + metaKey or ctrlKey, or unselect the rest.
     */
    public selectFile(event: KeyboardEvent): void {
        if (this.$store.state.files.openedDropdown) {
            this.$store.dispatch("files/closeDropdown");
        }

        if (event.shiftKey) {
            this.setShiftSelectedFiles();
        } else if (event.metaKey || event.ctrlKey) {
            this.setSelectedFile(true);
        } else {
            this.setSelectedFile(false);
        }
    }

    /**
     * Set the selected file/folder in the store.
     */
    private setSelectedFile(command: boolean): void {
        /* this function is responsible for selecting and unselecting a file on file click or [CMD + click] AKA command. */

        const files = [
            ...this.$store.state.files.selectedFiles,
            ...this.$store.state.files.shiftSelectedFiles
        ];

        const selectedAnchorFile =
            this.$store.state.files.selectedAnchorFile;
        const shiftSelectedFiles =
            this.$store.state.files.shiftSelectedFiles;
        const selectedFiles = this.$store.state.files.selectedFiles;

        if (command && this.file === selectedAnchorFile) {
            /* if it's [CMD + click] and the file selected is the actual selectedAnchorFile, then unselect the file but store it under unselectedAnchorFile in case the user decides to do a [shift + click] right after this action. */

            this.$store.commit("files/setUnselectedAnchorFile", this.file);
            this.$store.commit("files/setSelectedAnchorFile", null);
        } else if (command && files.includes(this.file)) {
            /* if it's [CMD + click] and the file selected is a file that has already been selected in selectedFiles and shiftSelectedFiles, then unselect it by filtering it out. */

            this.$store.dispatch(
                "files/updateSelectedFiles",
                selectedFiles.filter(
                    (fileSelected) => fileSelected !== this.file
                )
            );

            this.$store.dispatch(
                "files/updateShiftSelectedFiles",
                shiftSelectedFiles.filter(
                    (fileSelected) => fileSelected !== this.file
                )
            );
        } else if (command && selectedAnchorFile) {
            /* if it's [CMD + click] and there is already a selectedAnchorFile, then add the selectedAnchorFile and shiftSelectedFiles into the array of selectedFiles, set selectedAnchorFile to the file that was clicked, set unselectedAnchorFile to null, and set shiftSelectedFiles to an empty array. */

            const filesSelected = [...selectedFiles];

            if (!filesSelected.includes(selectedAnchorFile)) {
                filesSelected.push(selectedAnchorFile);
            }

            this.$store.dispatch("files/updateSelectedFiles", [
                ...filesSelected,
                ...shiftSelectedFiles.filter(
                    (file) => !filesSelected.includes(file)
                )
            ]);

            this.$store.commit("files/setSelectedAnchorFile", this.file);
            this.$store.commit("files/setUnselectedAnchorFile", null);
            this.$store.dispatch("files/updateShiftSelectedFiles", []);
        } else if (command) {
            /* if it's [CMD + click] and it has not met any of the above conditions, then set selectedAnchorFile to file and set unselectedAnchorfile to null, update the selectedFiles, and update the shiftSelectedFiles */

            this.$store.commit("files/setSelectedAnchorFile", this.file);
            this.$store.commit("files/setUnselectedAnchorFile", null);

            this.$store.dispatch("files/updateSelectedFiles", [
                ...selectedFiles,
                ...shiftSelectedFiles
            ]);

            this.$store.dispatch("files/updateShiftSelectedFiles", []);
        } else {
            /* if it's just a file click without any modifier, then set selectedAnchorFile to the file that was clicked, set shiftSelectedFiles and selectedFiles to an empty array. */

            this.$store.commit("files/setSelectedAnchorFile", this.file);
            this.$store.dispatch("files/updateShiftSelectedFiles", []);
            this.$store.dispatch("files/updateSelectedFiles", []);
        }
    }

    /**
     * Set files/folders selected using shift key in the store.
     */
    private setShiftSelectedFiles(): void {
        /* this function is responsible for selecting all files from selectedAnchorFile to the file that was selected with [shift + click] */

        const files = this.$store.getters["files/sortedFiles"];
        const unselectedAnchorFile =
            this.$store.state.files.unselectedAnchorFile;

        if (unselectedAnchorFile) {
            /* if there is an unselectedAnchorFile, meaning that in the previous action the user unselected the anchor file but is now chosing to do a [shift + click] on another file, then reset the selectedAnchorFile, the achor file, to unselectedAnchorFile. */

            this.$store.commit(
                "files/setSelectedAnchorFile",
                unselectedAnchorFile
            );
            this.$store.commit("files/setUnselectedAnchorFile", null);
        }

        const selectedAnchorFile =
            this.$store.state.files.selectedAnchorFile;

        if (!selectedAnchorFile) {
            this.$store.commit("files/setSelectedAnchorFile", this.file);
            return;
        }

        const anchorIdx = files.findIndex(
            (file) => file === selectedAnchorFile
        );
        const shiftIdx = files.findIndex((file) => file === this.file);

        const start = Math.min(anchorIdx, shiftIdx);
        const end = Math.max(anchorIdx, shiftIdx) + 1;

        this.$store.dispatch(
            "files/updateShiftSelectedFiles",
            files
                .slice(start, end)
                .filter(
                    (file) =>
                        !this.$store.state.files.selectedFiles.includes(
                            file
                        ) && file !== selectedAnchorFile
                )
        );
    }

    /**
     * Open the share modal for the current file.
     */
    public async share(): Promise<void> {
        this.$store.dispatch("files/closeDropdown");

        this.$store.commit(
            "files/setFileShareModal",
            this.path + this.file.Key
        );
    }

    /**
     * Toggle the dropdown for the current file/folder.
     */
    public toggleDropdown(): void {
        if (this.$store.state.files.openedDropdown === this.file.Key) {
            this.$store.dispatch("files/closeDropdown");
        } else {
            this.$store.dispatch("files/openDropdown", this.file.Key);
        }

        // remove the dropdown delete confirmation
        this.deleteConfirmation = false;
    }

    /**
     * Download the current file.
     */
    public download(): void {
        try {
            this.$store.dispatch("files/download", this.file);
            this.$notify.warning("Do not share download link with other people. If you want to share this data better use \"Share\" option.");
        } catch (error) {
            this.$notify.error("Can not download your file");
        }

        this.$store.dispatch("files/closeDropdown");
        this.deleteConfirmation = false;
    }

    /**
     * Set the data property deleteConfirmation to true, signifying that this user does in fact want the current selected file/folder.
     */
    public confirmDeletion(): void {
        this.deleteConfirmation = true;
    }

    /**
     * Delete the selected file/folder.
     */
    public async finalDelete(): Promise<void> {
        this.$store.dispatch("files/closeDropdown");
        this.$store.dispatch("files/addFileToBeDeleted", this.file);

        const params = {
            path: this.path,
            file: this.file
        };

        if (this.file.type === "file") {
            await this.$store.dispatch("files/delete", params);
        } else {
            this.$store.dispatch("files/deleteFolder", params);
        }

        // refresh the files displayed
        await this.$store.dispatch("files/list");
        this.$store.dispatch("files/removeFileFromToBeDeleted", this.file);
        this.deleteConfirmation = false;
    }

    /**
     * Abort the deletion of the current file/folder.
     */
    public cancelDeletion(): void {
        this.$store.dispatch("files/closeDropdown");
        this.deleteConfirmation = false;
    }
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

.dropdown-item {
    font-size: 14px;
    font-family: 'Inter', sans-serif;
}

.dropdown-item svg {
    color: #768394;
}

.dropdown-item:focus,
.dropdown-item:hover {
    color: #1b2533;
    background-color: #f4f5f7;
    font-weight: bold;
}

.dropdown-item:focus svg,
.dropdown-item:hover svg {
    color: #0068dc;
}

.deletion-confirmation {
    font-size: 14px;
    font-weight: bold;
}

.bi-trash {
    cursor: pointer;
}

.file-name {
    margin-left: 5px;
    cursor: pointer;
    color: #000;
}

.file-name:hover {
    color: #376fff;
}

.file-name:hover svg path {
    fill: #376fff;
}

.folder-name:hover {
    color: #376fff;
}

.folder-name:hover svg path {
    fill: #376fff;
}
</style>
