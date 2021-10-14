<script lang="ts"> /* eslint-disable */ </script>

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
                            class="btn btn-sm btn-primary btn-block"
                            @click="buttonFileUpload"
                        >
                            <svg
                                width="16"
                                height="16"
                                viewBox="0 0 16 16"
                                fill="none"
                                class="mr-2"
                                xmlns="http://www.w3.org/2000/svg"
                            >
                                <path
                                    d="M7.49407 0.453655L7.49129 0.450658L2.40012 5.94819L3.53149 7.16987L7.20001 3.20854L7.20001 11.6808H8.80001V3.39988L12.2913 7.16983L13.4227 5.94815L7.91419 0L7.49407 0.453655Z"
                                    fill="white"
                                />
                                <path
                                    d="M16 14.2723H0V16H16V14.2723Z"
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
                            class="btn btn-sm btn-primary btn-block"
                            @click="buttonFolderUpload"
                        >
                            <svg
                                width="16"
                                height="16"
                                viewBox="0 0 16 16"
                                fill="none"
                                class="mr-2"
                                xmlns="http://www.w3.org/2000/svg"
                            >
                                <path
                                    d="M7.49407 0.453655L7.49129 0.450658L2.40012 5.94819L3.53149 7.16987L7.20001 3.20854L7.20001 11.6808H8.80001V3.39988L12.2913 7.16983L13.4227 5.94815L7.91419 0L7.49407 0.453655Z"
                                    fill="white"
                                />
                                <path
                                    d="M16 14.2723H0V16H16V14.2723Z"
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
                                        :disabled="!createFolderEnabled"
                                        class="btn btn-primary btn-sm px-4"
                                        @click="createFolder"
                                    >
                                        Save Folder
                                    </button>
                                    <span class="mx-1" />
                                    <button
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

<script>
import FileBrowserHeader from "./FileBrowserHeader.vue";
import FileEntry from "./FileEntry.vue";
import BreadCrumbs from "./BreadCrumbs.vue";
import FileModal from "./FileModal.vue";
import FileShareModal from "./FileShareModal";

// Computed property creators

const fromFilesStore = (prop) =>
    function () {
        return this.$store.state.files[prop];
    };

export default {
    data: () => ({
        createFolderInput: "",
        creatingFolderSpinner: false,
        deleteConfirmation: false,
        fetchingFilesSpinner: false
    }),
    computed: {
        isInitialized() {
            return this.$store.getters["files/isInitialized"];
        },

        path: fromFilesStore("path"),

        filesUploading() {
            return fromFilesStore("uploading").bind(this)();
        },

        filesUploadingLength() {
            return this.filesUploading.length;
        },

        formattedFilesUploading() {
            if (this.filesUploadingLength > 5) {
                return this.filesUploading.slice(0, 5);
            }

            return this.filesUploading;
        },

        formattedFilesWaitingToBeUploaded() {
            let file = "file";

            if (this.filesUploadingLength > 1) {
                file = "files";
            }

            return `${this.filesUploadingLength} ${file}`;
        },

        createFolderEnabled() {
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
        },

        bucketName() {
            return this.$store.state.files.bucket;
        },

        files() {
            return this.$store.getters["files/sortedFiles"];
        },

        singleFiles() {
            return this.files.filter((f) => f.type === "file");
        },

        folders() {
            return this.files.filter((f) => f.type === "folder");
        },

        routePath() {
            let pathMatch = this.$route.params.pathMatch;
            pathMatch = Array.isArray(pathMatch)
                ? pathMatch.join("/") + "/"
                : pathMatch;
            return pathMatch;
        },

        displayUpload() {
            return this.fetchingFilesSpinner === false;
        },

        showCreateFolderInput() {
            return this.$store.state.files.createFolderInputShow === true;
        },

        showFileModal() {
            return this.$store.state.files.modalPath !== null;
        },

        showFileShareModal() {
            return this.$store.state.files.fileShareModal;
        }
    },
    watch: {
        async routePath() {
            await this.goToRoutePath();
        }
    },
    async created() {
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
    },
    methods: {
        closeModalDropdown() {
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
        },

        toggleFolderCreationInput() {
            this.$store.dispatch(
                "files/updateCreateFolderInputShow",
                !this.$store.state.files.createFolderInputShow
            );
        },

        filename(file) {
            return file.Key.length > 25
                ? file.Key.slice(0, 25) + "..."
                : file.Key;
        },

        async upload(e) {
            await this.$store.dispatch("files/upload", e);
            e.target.value = "";
        },

        cancelUpload(fileName) {
            this.$store.dispatch("files/cancelUpload", fileName);
        },

        async list(path) {
            await this.$store.dispatch("files/list", path, {
                root: true
            });
        },

        async go(path) {
            await this.$store.dispatch("files/closeDropdown");
            await this.list(this.path + path);
        },

        async back() {
            this.$store.dispatch("files/updateCreateFolderInputShow", false);
            await this.$store.dispatch("files/closeDropdown");
        },

        async goToRoutePath() {
            if (typeof this.routePath === "string") {
                await this.$store.dispatch("files/closeDropdown");
                await this.list(this.routePath);
            }
        },

        async buttonFileUpload() {
            const fileInputElement = this.$refs.fileInput;
            fileInputElement.click();
        },

        async buttonFolderUpload() {
            const folderInputElement = this.$refs.folderInput;
            folderInputElement.click();
        },

        async createFolder() {
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
        },

        cancelFolderCreation() {
            this.createFolderInput = "";
            this.$store.dispatch("files/updateCreateFolderInputShow", false);
        }
    },

    components: {
        FileEntry,
        BreadCrumbs,
        FileBrowserHeader,
        FileModal,
        FileShareModal
    }
};
</script>

<style scoped>
/* stylelint-disable */
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

f tbody {
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
