// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="file-browser">
        <div v-if="isInitialized" class="row white-background p-4 p-lg-5" @click="closeModalDropdown">
            <div class="col-sm-12">
                <div
                    v-cloak
                    class="div-responsive"
                    @drop.prevent="upload"
                    @dragover.prevent
                >
                    <div class="row mb-2 d-flex justify-content-end">
                        <div class="col-sm-12 col-md-4 col-lg-3 mb-3">
                            <button
                                type="button"
                                class="btn btn-sm btn-light btn-block"
                                style="margin-right: 15px;"
                                @click="toggleFolderCreationInput"
                            >
                                <svg
                                    width="17"
                                    height="16"
                                    viewBox="0 0 17 16"
                                    fill="none"
                                    class="mr-2"
                                    xmlns="http://www.w3.org/2000/svg"
                                >
                                    <path
                                        d="M0.912109 1.92481C0.912109 1.1558 1.56545 0.5 2.41211 0.5H7.41211C7.70648 0.5 7.91211 0.724912 7.91211 0.962406C7.91211 1.78796 8.60191 2.42481 9.41211 2.42481H14.4121C15.2588 2.42481 15.9121 3.08061 15.9121 3.84962V14.0752C15.9121 14.8442 15.2588 15.5 14.4121 15.5H2.41211C1.56545 15.5 0.912109 14.8442 0.912109 14.0752V1.92481Z"
                                        fill="#768394"
                                        stroke="#7C8794"
                                    />
                                    <path
                                        d="M11.182 8.59043H5.79067M8.48633 5.89478V11.2861"
                                        stroke="white"
                                        stroke-linecap="round"
                                        stroke-linejoin="round"
                                    />
                                </svg>
                                New Folder
                            </button>
                        </div>

                        <div class="col-sm-12 col-md-4 col-lg-3 mb-3">
                            <input
                                ref="fileInput"
                                type="file"
                                aria-roledescription="file-upload"
                                hidden
                                multiple
                                @change="upload"
                            >
                            <button
                                type="button"
                                class="btn btn-sm btn-primary btn-block"
                                @click="buttonFileUpload"
                            >
                                <svg
                                    width="13"
                                    height="16"
                                    viewBox="0 0 13 16"
                                    class="mr-2"
                                    xmlns="http://www.w3.org/2000/svg"
                                >
                                    <path
                                        d="m2 0c-1.10799992 0-2 0.89199996-2 2v12c0 1.108 0.89200008 2 1.9999999 2h9.0000011c1.108 0 2-0.892 2-2v-9.0000001h-3c-1.108001 0-2.0000013-0.8919999-2.0000013-1.9999998v-3.0000001zm7 0v3.0000001c0 0.5539999 0.4459999 0.9999999 1.000001 0.9999999h3zm-2.4999997 5.3964843 3.3535156 3.3535155c0.1952141 0.1952542 0.1952141 0.5117779 0 0.7070317-0.195254 0.195212-0.5117775 0.195212-0.7070315 0l-2.1464843-2.1464848v4.7929693c0 0.276144-0.2238577 0.5-0.4999998 0.5-0.2761437 0-0.5286517-0.225348-0.5000002-0.5v-4.7929693l-2.1464841 2.1464848c-0.1952542 0.195212-0.5117779 0.195212-0.7070321 0-0.1952126-0.1952545-0.1952126-0.5117779 0-0.7070317z"
                                        fill="white"
                                    />
                                </svg>
                                Upload File
                            </button>
                        </div>

                        <div class="col-sm-12 col-md-4 col-lg-3 mb-3">
                            <input
                                ref="folderInput"
                                type="file"
                                aria-roledescription="folder-upload"
                                hidden
                                webkitdirectory
                                mozdirectory
                                multiple
                                @change="upload"
                            >
                            <button
                                type="button"
                                class="btn btn-sm btn-primary btn-block"
                                @click="buttonFolderUpload"
                            >
                                <svg
                                    width="16"
                                    height="16"
                                    viewBox="0 0 16 16"
                                    class="mr-2"
                                    xmlns="http://www.w3.org/2000/svg"
                                >
                                    <path
                                        d="m2 0c-1.1045695 0-2 0.8954305-2 2v12c0 1.104569 0.8954305 2 2 2h12c1.104569 0 2-0.895431 2-2v-10c0-1.1045695-0.895431-2-2-2h-5c-0.5522847 0-1-0.4477153-1-1 0-0.55228475-0.4477153-1-1-1zm6.5 5.3964844 3.353516 3.3535156c0.195212 0.1952535 0.195212 0.5117777 0 0.7070312-0.195254 0.1952123-0.511778 0.1952123-0.707032 0l-2.146484-2.1464843v4.7929691c0 0.276142-0.2238576 0.5-0.5 0.5s-0.5-0.223858-0.5-0.5v-4.7929691l-2.146484 2.1464843c-0.1952536 0.1952123-0.5117784 0.1952123-0.707032 0-0.1952118-0.1952535-0.1952118-0.5117777 0-0.7070312z"
                                        fill="white"
                                    />
                                </svg>
                                Upload Folder
                            </button>
                        </div>
                    </div>

                    <div class="row mb-2 d-flex justify-content-between">
                        <div class="col-sm-12 col-md-12 col-xl-8 mb-3">
                            <bread-crumbs />
                        </div>
                    </div>

                    <div>
                        <table class="table table-hover no-selection">
                            <file-browser-header />

                            <tbody>
                                <tr
                                    v-for="file in formattedFilesUploading"
                                    :key="file.ETag"
                                    scope="row"
                                >
                                    <td
                                        class="upload-text"
                                        aria-roledescription="file-uploading"
                                    >
                                        <span>
                                            <svg
                                                width="21"
                                                height="18"
                                                viewBox="0 0 16 16"
                                                fill="currentColor"
                                                xmlns="http://www.w3.org/2000/svg"
                                                class="bi bi-file-earmark ml-2 mr-1"
                                            >
                                                <path
                                                    d="M4 0h5.5v1H4a1 1 0 0 0-1 1v12a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1V4.5h1V14a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V2a2 2 0 0 1 2-2z"
                                                />
                                                <path
                                                    d="M9.5 3V0L14 4.5h-3A1.5 1.5 0 0 1 9.5 3z"
                                                />
                                            </svg>
                                            {{ filename(file) }}
                                        </span>
                                    </td>
                                    <td aria-roledescription="progress-bar">
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
                                    </td>
                                    <td>
                                        <button
                                            type="button"
                                            class="btn btn-danger btn-sm"
                                            @click="cancelUpload(file.Key)"
                                        >
                                            Cancel
                                        </button>
                                    </td>
                                    <td />
                                </tr>

                                <tr v-if="filesUploadingLength">
                                    <div class="files-uploading-count my-3">
                                        <div
                                            class="px-2"
                                            aria-roledescription="files-uploading-count"
                                        >
                                            {{ formattedFilesWaitingToBeUploaded }}
                                            waiting to be uploaded...
                                        </div>
                                    </div>
                                </tr>

                                <tr v-if="path.length > 0">
                                    <td class="px-3">
                                        <router-link to="../">
                                            <a
                                                id="navigate-back"
                                                href="javascript:null"
                                                class="px-2 font-weight-bold"
                                                @click="back"
                                            >..</a>
                                        </router-link>
                                    </td>
                                </tr>

                                <tr
                                    v-if="showCreateFolderInput"
                                    class="new-folder-row"
                                >
                                    <td span="3">
                                        <input
                                            v-model="createFolderInput"
                                            class="form-control input-folder"
                                            :class="{
                                                'folder-input':
                                                    createFolderInput.length > 0 &&
                                                    !createFolderEnabled
                                            }"
                                            type="text"
                                            placeholder="Name of the folder"
                                            @keypress.enter="createFolder"
                                        >
                                    </td>
                                    <td span="3">
                                        <button
                                            type="button"
                                            :disabled="!createFolderEnabled"
                                            class="btn btn-primary btn-sm px-4"
                                            @click="createFolder"
                                        >
                                            Save Folder
                                        </button>
                                        <span class="mx-1" />
                                        <button
                                            type="button"
                                            class="btn btn-light btn-sm px-4"
                                            @click="cancelFolderCreation"
                                        >
                                            Cancel
                                        </button>
                                    </td>
                                    <td span="3" />
                                    <td span="3">
                                        <div
                                            v-if="creatingFolderSpinner"
                                            class="spinner-border"
                                            role="status"
                                        />
                                    </td>
                                </tr>

                                <file-entry
                                    v-for="file in folders"
                                    :key="file.Key"
                                    :path="path"
                                    :file="file"
                                />

                                <file-entry
                                    v-for="file in singleFiles"
                                    :key="file.Key"
                                    :path="path"
                                    :file="file"
                                />
                            </tbody>
                        </table>
                    </div>

                    <div
                        v-if="fetchingFilesSpinner"
                        class="d-flex justify-content-center"
                    >
                        <div class="spinner-border" />
                    </div>

                    <div
                        v-if="displayUpload"
                        class="upload-help"
                        @click="buttonFileUpload"
                    >
                        <svg
                            width="300"
                            height="172"
                            viewBox="0 0 300 172"
                            fill="none"
                            xmlns="http://www.w3.org/2000/svg"
                        >
                            <path
                                d="M188.5 140C218.047 140 242 116.047 242 86.5C242 56.9528 218.047 33 188.5 33C158.953 33 135 56.9528 135 86.5C135 116.047 158.953 140 188.5 140Z"
                                fill="white"
                            />
                            <path
                                d="M123.5 167C147.524 167 167 147.524 167 123.5C167 99.4756 147.524 80 123.5 80C99.4756 80 80 99.4756 80 123.5C80 147.524 99.4756 167 123.5 167Z"
                                fill="white"
                            />
                            <path
                                d="M71.5 49C78.9558 49 85 42.9558 85 35.5C85 28.0442 78.9558 22 71.5 22C64.0442 22 58 28.0442 58 35.5C58 42.9558 64.0442 49 71.5 49Z"
                                fill="white"
                            />
                            <path
                                d="M262.5 143C268.851 143 274 137.851 274 131.5C274 125.149 268.851 120 262.5 120C256.149 120 251 125.149 251 131.5C251 137.851 256.149 143 262.5 143Z"
                                fill="white"
                            />
                            <path
                                d="M185.638 64.338L191 57M153 109L179.458 72.7948L153 109Z"
                                stroke="#276CFF"
                                stroke-width="2"
                                stroke-linecap="round"
                            />
                            <path
                                d="M121.08 153.429L115 161M153 108L127.16 144.343L153 108Z"
                                stroke="#276CFF"
                                stroke-width="2"
                                stroke-linecap="round"
                            />
                            <path
                                d="M134 71L115 31M152 109L139 81L152 109Z"
                                stroke="#FF458B"
                                stroke-width="2"
                                stroke-linecap="round"
                            />
                            <path
                                d="M180.73 129.5L210 151M153 108L173.027 123.357L153 108Z"
                                stroke="#FF458B"
                                stroke-width="2"
                                stroke-linecap="round"
                            />
                            <path
                                d="M86.7375 77.1845L72 70M152 109L109.06 88.0667L152 109Z"
                                stroke="#FFC600"
                                stroke-width="2"
                                stroke-linecap="round"
                            />
                            <path
                                d="M152.762 109.227L244.238 76.7727"
                                stroke="#00E567"
                                stroke-width="2"
                                stroke-linecap="round"
                            />
                            <path
                                d="M154.5 104.5L111 131"
                                stroke="#00E567"
                                stroke-width="2"
                                stroke-linecap="round"
                            />
                            <path
                                fill-rule="evenodd"
                                clip-rule="evenodd"
                                d="M224 57H238V71H224V57Z"
                                fill="#00E567"
                            />
                            <path
                                fill-rule="evenodd"
                                clip-rule="evenodd"
                                d="M127 2H137V12H127V2Z"
                                fill="#FF458B"
                            />
                            <path
                                fill-rule="evenodd"
                                clip-rule="evenodd"
                                d="M150 166H156V172H150V166Z"
                                fill="#FF458B"
                            />
                            <path
                                fill-rule="evenodd"
                                clip-rule="evenodd"
                                d="M44 0H50V6H44V0Z"
                                fill="#00E567"
                            />
                            <path
                                fill-rule="evenodd"
                                clip-rule="evenodd"
                                d="M294 111H300V117H294V111Z"
                                fill="#276CFF"
                            />
                            <path
                                fill-rule="evenodd"
                                clip-rule="evenodd"
                                d="M0 121H6V127H0V121Z"
                                fill="#276CFF"
                            />
                            <path
                                fill-rule="evenodd"
                                clip-rule="evenodd"
                                d="M268 86H274V92H268V86Z"
                                fill="#FFC600"
                            />
                            <path
                                fill-rule="evenodd"
                                clip-rule="evenodd"
                                d="M28 91H46V109H28V91Z"
                                fill="#FFC600"
                            />
                            <path
                                fill-rule="evenodd"
                                clip-rule="evenodd"
                                d="M181 21H203V43H181V21Z"
                                fill="#276CFF"
                            />
                            <path
                                fill-rule="evenodd"
                                clip-rule="evenodd"
                                d="M154.958 55L179 79.0416V136H122V55H154.958Z"
                                fill="#0218A7"
                            />
                            <path
                                d="M146.5 80H136.5C135.119 80 134 81.1193 134 82.5C134 83.8807 135.119 85 136.5 85H146.5C147.881 85 149 83.8807 149 82.5C149 81.1193 147.881 80 146.5 80Z"
                                fill="white"
                            />
                            <path
                                d="M164.5 92H136.5C135.119 92 134 93.1193 134 94.5C134 95.8807 135.119 97 136.5 97H164.5C165.881 97 167 95.8807 167 94.5C167 93.1193 165.881 92 164.5 92Z"
                                fill="white"
                            />
                            <path
                                d="M164.5 104H136.5C135.119 104 134 105.119 134 106.5C134 107.881 135.119 109 136.5 109H164.5C165.881 109 167 107.881 167 106.5C167 105.119 165.881 104 164.5 104Z"
                                fill="white"
                            />
                            <path
                                d="M164.5 116H136.5C135.119 116 134 117.119 134 118.5C134 119.881 135.119 121 136.5 121H164.5C165.881 121 167 119.881 167 118.5C167 117.119 165.881 116 164.5 116Z"
                                fill="white"
                            />
                            <path
                                fill-rule="evenodd"
                                clip-rule="evenodd"
                                d="M154.958 79.0416V55L179 79.0416H154.958Z"
                                fill="#276CFF"
                            />
                        </svg>
                        <p class="drop-files-text mt-4 mb-0">
                            Drop Files Here to Upload
                        </p>
                    </div>
                </div>

                <file-modal v-if="showFileModal" />

                <file-share-modal v-if="showFileShareModal" />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue, Watch } from 'vue-property-decorator';
import FileBrowserHeader from "./FileBrowserHeader.vue";
import FileEntry from "./FileEntry.vue";
import BreadCrumbs from "./BreadCrumbs.vue";
import FileModal from "./FileModal.vue";
import FileShareModal from "./FileShareModal.vue";

import { AnalyticsHttpApi } from '@/api/analytics';
import { BrowserFile } from "@/types/browser.ts";
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { RouteConfig } from "@/router";

// @vue/component
@Component({
    components: {
        FileEntry,
        BreadCrumbs,
        FileBrowserHeader,
        FileModal,
        FileShareModal
    },
})
export default class FileBrowser extends Vue {
    public createFolderInput = "";
    public creatingFolderSpinner = false;
    public deleteConfirmation = false;
    public fetchingFilesSpinner = false;

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
     * Check if the s3 client has been initialized in the store.
     */
    public get isInitialized(): boolean {
        return this.$store.getters["files/isInitialized"];
    }

    /**
     * Retrieve the current path from the store.
     */
    private get path(): string {
        return this.$store.state.files.path;
    }

    /**
     * Return files that are currently being uploaded from the store.
     */
    private get filesUploading(): BrowserFile[] {
        return this.$store.state.files.uploading;
    }

    /**
     * Return the length of the array of files currently being uploaded.
     */
    public get filesUploadingLength(): number {
        return this.filesUploading.length;
    }

    /**
     * Return up to five files currently being uploaded for display purposes.
     */
    public get formattedFilesUploading(): BrowserFile[] {
        if (this.filesUploadingLength > 5) {
            return this.filesUploading.slice(0, 5);
        }

        return this.filesUploading;
    }

    /**
     * Return the text of how many files in total are being uploaded to be displayed to give users more context.
     */
    public get formattedFilesWaitingToBeUploaded(): string {
        let file = "file";

        if (this.filesUploadingLength > 1) {
            file = "files";
        }

        return `${this.filesUploadingLength} ${file}`;
    }

    /**
     * Return a boolean signifying whether the current folder name abides by our convention.
     */
    public get createFolderEnabled(): boolean {
        const charsOtherThanSpaceExist =
            this.createFolderInput.trim().length > 0;

        const noForwardSlashes = this.createFolderInput.indexOf("/") === -1;

        const nameIsNotOnlyPeriods =
            [...this.createFolderInput.trim()].filter(
                (char) => char === "."
            ).length !== this.createFolderInput.trim().length;

        const notDuplicate =
            this.files.filter(
                (file) => file.Key === this.createFolderInput.trim()
            ).length === 0;

        return (
            charsOtherThanSpaceExist &&
            noForwardSlashes &&
            nameIsNotOnlyPeriods &&
            notDuplicate
        );
    }

    /**
     * Retrieve the current bucket from the store.
     */
    private get bucketName(): string {
        return this.$store.state.files.bucket;
    }

    /**
     * Retrieve all of the files sorted from the store.
     */
    private get files(): BrowserFile[] {
        return this.$store.getters["files/sortedFiles"];
    }

    /**
     * Return an array of BrowserFile type that are files and not folders.
     */
    public get singleFiles(): BrowserFile[] {
        return this.files.filter((f) => f.type === "file");
    }

    /**
     * Return an array of BrowserFile type that are folders and not files.
     */
    public get folders(): BrowserFile[] {
        return this.files.filter((f) => f.type === "folder");
    }

    /**
     * Retrieve the pathMatch from the current route.
     */
    private get routePath(): string {
        let pathMatch = this.$route.params.pathMatch;
        pathMatch = Array.isArray(pathMatch)
            ? pathMatch.join("/") + "/"
            : pathMatch;
        return pathMatch;
    }

    /**
     * Returns bucket name from store.
     */
    private get bucket(): string {
        return this.$store.state.objectsModule.fileComponentBucketName;
    }

    /**
     * Returns objects flow status from store.
     */
    private get isNewObjectsFlow(): string {
        return this.$store.state.appStateModule.isNewObjectsFlow;
    }

    /**
     * Return a boolean signifying whether the upload display is allowed to be shown.
     */
    public get displayUpload(): boolean {
        return !this.fetchingFilesSpinner;
    }

    /**
     * Return a boolean signifying whether the create folder input can be shown.
     */
    public get showCreateFolderInput(): boolean {
        return this.$store.state.files.createFolderInputShow;
    }

    /**
     * Return a boolean signifying whether the file modal can be shown.
     */
    public get showFileModal(): boolean {
        return this.$store.state.files.modalPath !== null;
    }

    /**
     * Return a boolean signifying whether the file share modal can be shown.
     */
    public get showFileShareModal(): null | string {
        return this.$store.state.files.fileShareModal;
    }

    /**
     * Watch for changes in the path and call goToRoutePath, navigating away from the current page.
     */
    @Watch("routePath")
    private async handleFoutePathChange() {
        await this.goToRoutePath();
    }

    /**
     * Set spinner state. If routePath is not present navigate away. If there's some error re-render the page with a call to list. All of this is done on the created lifecycle method.
     */
    public async created(): Promise<void> {
        if (!this.bucket) {
            if (this.isNewObjectsFlow) {
                await this.$router.push(RouteConfig.Buckets.with(RouteConfig.BucketsManagement).path);
            } else {
                await this.$router.push(RouteConfig.Buckets.with(RouteConfig.EncryptData).path);
            }

            return;
        }

        // display the spinner while files are being fetched
        this.fetchingFilesSpinner = true;

        if (!this.routePath) {
            try {
                await this.$router.push({
                    path: `${this.$store.state.files.browserRoot}${this.path}`
                });
            } catch (err) {
                await this.list("");
            }
        }

        // remove the spinner after files have been fetched
        this.fetchingFilesSpinner = false;
    }

    /**
     * Close modal, file share modal, dropdown, and remove all selected files from the store.
     */
    public closeModalDropdown(): void {
        if (this.$store.state.files.modalPath) {
            this.$store.commit("files/closeModal");
        }

        if (this.$store.state.files.fileShareModal) {
            this.$store.commit("files/closeFileShareModal");
        }

        if (this.$store.state.files.openedDropdown) {
            this.$store.dispatch("files/closeDropdown");
        }

        if (this.$store.state.files.selectedFile) {
            this.$store.dispatch("files/clearAllSelectedFiles");
        }
    }

    /**
     * Toggle the folder creation input in the store.
     */
    public toggleFolderCreationInput(): void {
        this.$store.dispatch(
            "files/updateCreateFolderInputShow",
            !this.$store.state.files.createFolderInputShow
        );
    }

    /**
     * Return the file name of the passed in file argument formatted.
     */
    public filename(file: BrowserFile): string {
        return file.Key.length > 25
            ? file.Key.slice(0, 25) + "..."
            : file.Key;
    }

    /**
     * Upload the current selected or dragged-and-dropped file.
     */
    public async upload(e: Event): Promise<void> {
        await this.$store.dispatch("files/upload", e);
        this.analytics.eventTriggered(AnalyticsEvent.OBJECT_UPLOADED);
        const target = e.target as HTMLInputElement;
        target.value = "";
    }

    /**
     * Cancel the upload of the current file that's passed in as an argument.
     */
    public cancelUpload(fileName: string): void {
        this.$store.dispatch("files/cancelUpload", fileName);
    }

    /**
     * Call the list method from the store, which will trigger a re-render and fetch all files under the current path passed in as an argument.
     */
    private async list(path: string): Promise<void> {
        await this.$store.dispatch("files/list", path, {
            root: true
        });
    }

    /**
     * Remove the folder creation input and close any opened dropdowns when a user chooses to navigate back to the previous folder.
     */
    public async back(): Promise<void> {
        this.$store.dispatch("files/updateCreateFolderInputShow", false);
        await this.$store.dispatch("files/closeDropdown");
    }

    /**
     * Navigate to the path under routePath.
     */
    private async goToRoutePath(): Promise<void> {
        if (typeof this.routePath === "string") {
            await this.$store.dispatch("files/closeDropdown");
            await this.list(this.routePath);
        }
    }

    /**
     * Open the operating system's file system for file upload.
     */
    public async buttonFileUpload(): Promise<void> {
        const fileInputElement = this.$refs.fileInput as HTMLInputElement;
        fileInputElement.click();
    }

    /**
     * Open the operating system's file system for folder upload.
     */
    public async buttonFolderUpload(): Promise<void> {
        const folderInputElement = this.$refs.folderInput as HTMLInputElement;
        folderInputElement.click();
    }

    /**
     * Create a folder from the name inside of the folder creation input.
     */
    public async createFolder(): Promise<void> {
        // exit function if folder name violates our naming convention
        if (!this.createFolderEnabled) return;

        // add spinner
        this.creatingFolderSpinner = true;

        // create folder
        await this.$store.dispatch(
            "files/createFolder",
            this.createFolderInput.trim()
        );

        // clear folder input
        this.createFolderInput = "";

        // remove the folder creation input
        this.$store.dispatch("files/updateCreateFolderInputShow", false);

        // remove the spinner
        this.creatingFolderSpinner = false;
    }

    /**
     * Cancel folder creation clearing out the input and hiding the folder creation input.
     */
    public cancelFolderCreation(): void {
        this.createFolderInput = "";
        this.$store.dispatch("files/updateCreateFolderInputShow", false);
    }
}
</script>

<style scoped>
@import './scoped-bootstrap.css';

.white-background {
    background-color: #fff;
}

.file-browser {
    min-height: 500px;
}

.no-selection {
    user-select: none;
    -moz-user-select: none;
    -khtml-user-select: none;
    -webkit-user-select: none;
    -o-user-select: none;
}

tbody {
    user-select: none;
}

.table-heading {
    color: #768394;
    border-top: 0;
    border-bottom: 1px solid #dee2e6;
    padding-left: 0;
    cursor: pointer;
}

.path {
    font-size: 18px;
    font-weight: 700;
}

.upload-help {
    font-size: 1.75rem;
    text-align: center;
    margin-top: 1.5rem;
    color: #93a1ae;
    border: 2px dashed #bec4cd;
    border-radius: 10px;
    padding: 80px 20px;
    background: #fafafb;
    cursor: pointer;
}

.metric {
    color: #444;
}

.div-responsive {
    min-height: 400px;
}

.folder-input:focus {
    color: #fe5d5d;
    box-shadow: 0 0 0 0.2rem rgba(254, 93, 93, 0.5) !important;
    border-color: #fe5d5d !important;
    outline: none !important;
}

.new-folder-row:hover {
    background: #fff;
}

.btn {
    line-height: 2.4;
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

.files-uploading-count {
    color: #0d6efd;
}
</style>
