// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="container demo" @click="stopClickPropagation">
        <div
            id="detail-modal"
            class="modal right fade in show modal-open"
            tabindex="-1"
            role="dialog"
            aria-labelledby="modalLabel"
        >
            <div
                class="modal-dialog modal-xl modal-dialog-centered"
                role="document"
            >
                <div class="modal-content">
                    <div class="modal-body p-0">
                        <div class="container-fluid p-0">
                            <div class="row">
                                <div class="col-6 col-lg-8">
                                    <div
                                        class="
											file-preview-wrapper
											d-flex
											align-items-center
											justify-content-center
										"
                                    >
                                        <img
                                            v-if="previewIsImage"
                                            ref="previewImage"
                                            class="preview img-fluid"
                                            src="/static/static/images/common/loader.svg"
                                            aria-roledescription="image-preview"
                                        >

                                        <video
                                            v-if="previewIsVideo"
                                            ref="previewVideo"
                                            class="preview"
                                            controls
                                            src=""
                                            aria-roledescription="video-preview"
                                        />

                                        <audio
                                            v-if="previewIsAudio"
                                            ref="previewAudio"
                                            class="preview"
                                            controls
                                            src=""
                                            aria-roledescription="audio-preview"
                                        />
                                        <PlaceholderImage v-if="placeHolderDisplayable" />
                                    </div>
                                </div>
                                <div class="col-6 col-lg-4 pr-5">
                                    <div class="text-right">
                                        <svg
                                            id="close-modal"
                                            xmlns="http://www.w3.org/2000/svg"
                                            width="2em"
                                            height="2em"
                                            fill="#6e6e6e"
                                            class="bi bi-x mt-4 closex"
                                            viewBox="0 0 16 16"
                                            @click="closeModal"
                                        >
                                            <path
                                                d="M4.646 4.646a.5.5 0 0 1 .708 0L8 7.293l2.646-2.647a.5.5 0 0 1 .708.708L8.707 8l2.647 2.646a.5.5 0 0 1-.708.708L8 8.707l-2.646 2.647a.5.5 0 0 1-.708-.708L7.293 8 4.646 5.354a.5.5 0 0 1 0-.708z"
                                            />
                                        </svg>
                                    </div>

                                    <div class="mb-3">
                                        <span class="file-path">{{
                                            filePath
                                        }}</span>
                                    </div>

                                    <p class="size mb-3">
                                        <span class="text-lighter mr-2">Size:</span>
                                        {{ size }}
                                    </p>
                                    <p class="size mb-3">
                                        <span class="text-lighter mr-2">Created:</span>
                                        {{ uploadDate }}
                                    </p>

                                    <button
                                        class="
											btn btn-primary btn-block
											mb-3
											mt-4
										"
                                        type="button"
                                        download
                                        @click="download"
                                    >
                                        Download
                                        <svg
                                            width="14"
                                            height="15"
                                            viewBox="0 0 14 15"
                                            alt="Download"
                                            class="ml-2"
                                            fill="none"
                                            xmlns="http://www.w3.org/2000/svg"
                                        >
                                            <path
                                                d="M6.0498 7.98517V0H8.0498V7.91442L10.4965 5.46774L11.9107 6.88196L7.01443 11.7782L2.11816 6.88196L3.53238 5.46774L6.0498 7.98517Z"
                                                fill="white"
                                            />
                                            <path
                                                d="M0 13L14 13V15L0 15V13Z"
                                                fill="white"
                                            />
                                        </svg>
                                    </button>

                                    <div
                                        v-if="objectLink"
                                        class="input-group mt-4"
                                    >
                                        <input
                                            id="url"
                                            class="form-control"
                                            type="url"
                                            :value="objectLink"
                                            aria-describedby="generateShareLink"
                                            readonly
                                        >
                                        <div class="input-group-append">
                                            <button
                                                id="generateShareLink"
                                                type="button"
                                                name="copy"
                                                class="
													btn
													btn-outline-secondary
													btn-copy-link
												"
                                                @click="copy"
                                            >
                                                {{ copyText }}
                                            </button>
                                        </div>
                                    </div>

                                    <button
                                        v-else
                                        class="btn btn-light btn-block"
                                        type="button"
                                        @click="getSharedLink"
                                    >
                                        <span class="share-btn">
                                            Share
                                            <svg
                                                width="16"
                                                height="16"
                                                viewBox="0 0 16 16"
                                                alt="Share"
                                                class="ml-2"
                                                fill="none"
                                                xmlns="http://www.w3.org/2000/svg"
                                            >
                                                <path
                                                    d="M8.86084 11.7782L8.86084 3.79305L11.3783 6.31048L12.7925 4.89626L7.89622 0L2.99995 4.89626L4.41417 6.31048L6.86084 3.8638L6.86084 11.7782L8.86084 11.7782Z"
                                                    fill="#384B65"
                                                />
                                                <path
                                                    d="M4.5 8.12502H0.125V15.875H15.875V8.12502H11.5V9.87502H14.125V14.125H1.875V9.87502H4.5V8.12502Z"
                                                    fill="#384B65"
                                                />
                                            </svg>
                                        </span>
                                    </button>

                                    <div class="mt-5">
                                        <div class="storage-nodes">
                                            Nodes storing this file
                                        </div>
                                        <img ref="objectMap" src="/static/static/images/common/loader.svg" class="object-map" alt="object map">
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
        <div id="backdrop2" class="modal-backdrop fade show modal-open" />
    </div>
</template>

<script lang="ts">
import { Component, Vue, Watch } from 'vue-property-decorator';
import { BrowserFile } from '@/types/browser.ts';
import prettyBytes from 'pretty-bytes';

import PlaceholderImage from '@/../static/images/browser/placeholder.svg'

// @vue/component
@Component({
    components: {
        PlaceholderImage,
    }
})
export default class FileModal extends Vue {
    public objectLink = '';
    public copyText = 'Copy Link';

    public $refs!: {
        objectMap: HTMLImageElement;
        previewImage: HTMLImageElement;
        previewVideo: HTMLVideoElement;
        previewAudio: HTMLAudioElement;
    }

    /**
     * Retrieve the file object that the modal is set to from the store.
     */
    private get file(): BrowserFile {
        return this.$store.state.files.files.find(
            (file) => file.Key === this.filePath.split("/").slice(-1)[0]
        );
    }

    /**
     * Retrieve the filepath of the modal from the store.
     */
    public get filePath(): string {
        return this.$store.state.files.modalPath;
    }

    /**
     * Format the file size to be displayed.
     */
    public get size(): string {
        return prettyBytes(
            this.$store.state.files.files.find(
                (file) => file.Key === this.file.Key
            ).Size
        );
    }

    /**
     * Format the upload date of the current file.
     */
    public get uploadDate(): string {
        return this.file.LastModified.toLocaleString().split(",")[0];
    }

    /**
     * Get the extension of the current file.
     */
    private get extension(): string | undefined {
        return this.filePath.split('.').pop();
    }

    /**
     * Check to see if the current file is an image file.
     */
    public get previewIsImage(): boolean {
        if (typeof this.extension !== 'string') {
            return false;
        }

        return ['bmp', 'svg', 'jpg', 'jpeg', 'png', 'ico', 'gif'].includes(
            this.extension.toLowerCase()
        );
    }

    /**
     * Check to see if the current file is a video file.
     */
    public get previewIsVideo(): boolean {
        if (typeof this.extension !== 'string') {
            return false;
        }

        return ['m4v', 'mp4', 'webm', 'mov', 'mkv'].includes(
            this.extension.toLowerCase()
        );
    }

    /**
     * Check to see if the current file is an audio file.
     */
    public get previewIsAudio(): boolean {
        if (typeof this.extension !== 'string') {
            return false;
        }

        return ['mp3', 'wav', 'ogg'].includes(this.extension.toLowerCase());
    }

    /**
     * Check to see if the current file is neither an image file, video file, or audio file.
     */
    public get placeHolderDisplayable(): boolean {
        return [
            this.previewIsImage,
            this.previewIsVideo,
            this.previewIsAudio
        ].every((value) => !value);
    }

    /**
     * Watch for changes on the filepath and call `fetchObjectMapUrl` the moment it updates.
     */
    @Watch("filePath")
    private handleFilePathChange() {
        this.fetchObjectMap();
        if (!this.placeHolderDisplayable) this.setPreview();
    }

    /**
     * Call `fetchObjectMapUrl` on the mounted lifecycle method.
     */
    public mounted(): void {
        this.fetchObjectMap();
        if (!this.placeHolderDisplayable) this.setPreview();
    }

    /**
     * Get the object map for the file being displayed.
     */
    private async fetchObjectMap(): Promise<void> {
        try {
            if (!this.$store.state.files.fetchObjectMap) {
                return;
            }

            const objectMap: Blob | null = await this.$store.state.files.fetchObjectMap(
                this.filePath
            );

            if (!objectMap) {
                return;
            }

            this.$refs.objectMap.src = URL.createObjectURL(objectMap);
        } catch (error) {
            await this.$notify.error(error.message);
        }
    }

    /**
     * Download the current opened file.
     */
    public download(): void {
        this.$store.dispatch('files/download', this.file);
    }

    /**
     * Set preview object.
     */
    public async setPreview(): Promise<void> {
        try {
            if (!this.$store.state.files.fetchObjectPreview) {
                return;
            }

            const object: Blob | null = await this.$store.state.files.fetchObjectPreview(
                this.filePath
            );

            if (!object) {
                return;
            }

            const objectURL = URL.createObjectURL(object);

            switch (true) {
            case this.previewIsImage:
                this.$refs.previewImage.src = objectURL;
                break;
            case this.previewIsVideo:
                this.$refs.previewVideo.src = objectURL;
                break;
            case this.previewIsAudio:
                this.$refs.previewAudio.src = objectURL;
            }
        } catch (error) {
            await this.$notify.error(error.message);
        }
    }

    /**
     * Close the current opened file.
     */
    public closeModal(): void {
        this.$store.commit('files/closeModal');
    }

    /**
     * Copy the current opened file.
     */
    public async copy(): Promise<void> {
        await navigator.clipboard.writeText(this.objectLink);
        this.copyText = 'Copied!';
        setTimeout(() => {
            this.copyText = 'Copy Link';
        }, 2000);
    }

    /**
     * Get the share link of the current opened file.
     */
    public async getSharedLink(): Promise<void> {
        this.objectLink = await this.$store.state.files.fetchSharedLink(
            this.filePath
        );
    }

    /**
     * Stop the propagation of a click event only if the target is an element without detail-modal as the id.
     */
    public stopClickPropagation(e: Event): void {
        const target = e.target as HTMLElement;
        if (target.id !== "detail-modal") {
            e.stopPropagation();
        }
    }
}
</script>

<style scoped>

.modal-header {
    border-bottom-color: #eee;
    background-color: #fafafa;
}

.file-preview-wrapper {

    /* Testing background for file preview */

    /* background: #000; */
    background: #f9fafc;
    height: 100%;
    min-height: 75vh;
    border-right: 1px solid #eee;
}

.btn-demo {
    margin: 15px;
    padding: 10px 15px;
    border-radius: 0;
    font-size: 16px;
    background-color: #fff;
}

.btn-demo:focus {
    outline: 0;
}

.closex {
    cursor: pointer;
}

.modal-open {
    display: block !important;
}

.file-path {
    display: inline-block;
    font-weight: bold;
    max-width: 100%;
    position: relative;
    font-size: 18px;
    text-overflow: ellipsis;
    white-space: nowrap;
    overflow: hidden;
}

.preview {
    width: 100%;
}

.object-map {
    width: 100%;
}

.storage-nodes {
    padding: 5px;
    background: rgba(0, 0, 0, 0.8);
    font-weight: normal;
    color: white;
    font-size: 0.8rem;
}

.size {
    font-size: 0.9rem;
    font-weight: normal;
}

.btn {
    line-height: 2.4;
}

.btn-primary {
    background: #376fff;
    border-color: #376fff;
}

.btn-light {
    background: #e6e9ef;
    border-color: #e6e9ef;
}

.share-btn {
    font-weight: bold;
}

.text-lighter {
    color: #768394;
}

.btn-copy-link {
    border-top-right-radius: 4px;
    border-bottom-right-radius: 4px;
    font-size: 14px;
    padding: 0 16px;
}
</style>
