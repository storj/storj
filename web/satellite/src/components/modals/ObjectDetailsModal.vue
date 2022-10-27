// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <div class="modal__preview">
                    <img
                        v-if="previewAndMapFailed"
                        class="failed-preview"
                        src="/static/static/images/common/errorNotice.svg"
                        alt="failed preview"
                    >
                    <template v-else>
                        <img
                            v-if="previewIsImage && !isLoading"
                            class="preview img-fluid"
                            :src="objectPreviewUrl"
                            aria-roledescription="image-preview"
                            alt="preview"
                        >

                        <video
                            v-if="previewIsVideo && !isLoading"
                            class="preview"
                            controls
                            :src="objectPreviewUrl"
                            aria-roledescription="video-preview"
                        />

                        <audio
                            v-if="previewIsAudio && !isLoading"
                            class="preview"
                            controls
                            :src="objectPreviewUrl"
                            aria-roledescription="audio-preview"
                        />
                        <PlaceholderImage v-if="placeHolderDisplayable" />
                    </template>
                </div>
                <div class="modal__info">
                    <p class="modal__info__title">
                        {{ filePath }}
                    </p>
                    <p class="modal__info__size">
                        <span class="modal__info__size__label text-lighter">Size:</span>
                        {{ size }}
                    </p>
                    <p class="modal__info__size">
                        <span class="modal__info__size__label text-lighter">Created:</span>
                        {{ uploadDate }}
                    </p>
                    <VButton
                        class="modal__info__download-btn"
                        label="Download"
                        width="100%"
                        height="34px"
                        :on-press="download"
                    />
                    <div
                        v-if="objectLink"
                        class="modal__info__input-group"
                    >
                        <input
                            id="url"
                            class="form-control"
                            type="url"
                            :value="objectLink"
                            aria-describedby="generateShareLink"
                            readonly
                        >
                        <VButton
                            class="modal__info__input-group__copy"
                            :label="copyText"
                            :is-transparent="true"
                            font-size="14px"
                            width="unset"
                            height="34px"
                            :on-press="copy"
                        />
                    </div>
                    <VButton
                        v-else
                        label="Share"
                        width="100%"
                        height="34px"
                        :is-transparent="true"
                        :on-press="getSharedLink"
                    />
                    <VLoader v-if="isLoading" class="modal__info__loader" />
                    <div
                        v-if="objectMapUrl && !previewAndMapFailed"
                        class="modal__info__map"
                    >
                        <div class="storage-nodes">
                            Nodes storing this file
                        </div>
                        <img
                            class="object-map"
                            :src="objectMapUrl"
                            alt="object map"
                        >
                    </div>
                    <p v-if="!placeHolderDisplayable && !previewAndMapFailed && !isLoading" class="modal__info__note text-lighter">
                        Note: If you would like to share this object with others, please use the 'Share'
                        button rather than copying the path from the object preview. This object preview
                        link will expire within 24 hours.
                    </p>
                </div>
            </div>
        </template>
    </VModal>
</template>

<script lang="ts">
import { Component, Vue, Watch } from 'vue-property-decorator';
import prettyBytes from 'pretty-bytes';

import { BrowserFile } from '@/types/browser';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';

import VModal from '@/components/common/VModal.vue';
import VButton from '@/components/common/VButton.vue';
import VLoader from '@/components/common/VLoader.vue';

import PlaceholderImage from '@/../static/images/browser/placeholder.svg';

// @vue/component
@Component({
    components: {
        VModal,
        VButton,
        VLoader,
        PlaceholderImage,
    },
})
export default class ObjectDetailsModal extends Vue {
    public isLoading = false;
    public objectMapUrl = '';
    public objectPreviewUrl = '';
    public previewAndMapFailed = false;

    public objectLink = '';
    public copyText = 'Copy Link';

    /**
     * Call `fetchPreviewAndMapUrl` on the created lifecycle method.
     */
    public created(): void {
        this.fetchPreviewAndMapUrl();
    }

    /**
     * Retrieve the file object that the modal is set to from the store.
     */
    private get file(): BrowserFile {
        return this.$store.state.files.files.find(
            (file) => file.Key === this.filePath.split('/').slice(-1)[0],
        );
    }

    /**
     * Retrieve the filepath of the modal from the store.
     */
    public get filePath(): string {
        return this.$store.state.files.objectPathForModal;
    }

    /**
     * Format the file size to be displayed.
     */
    public get size(): string {
        return prettyBytes(
            this.$store.state.files.files.find(
                (file) => file.Key === this.file.Key,
            ).Size,
        );
    }

    /**
     * Format the upload date of the current file.
     */
    public get uploadDate(): string {
        return this.file.LastModified.toLocaleString().split(',')[0];
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
            this.extension.toLowerCase(),
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
            this.extension.toLowerCase(),
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
            this.previewIsAudio,
        ].every((value) => !value);
    }

    /**
     * Watch for changes on the filepath and call `fetchObjectMapUrl` the moment it updates.
     */
    @Watch('filePath')
    private handleFilePathChange() {
        this.fetchPreviewAndMapUrl();
    }

    /**
     * Get the object map url for the file being displayed.
     */
    private async fetchPreviewAndMapUrl(): Promise<void> {
        this.isLoading = true;
        const url: string = await this.$store.state.files.fetchPreviewAndMapUrl(
            this.filePath,
        );

        if (!url) {
            this.previewAndMapFailed = true;
            this.isLoading = false;

            return;
        }

        const mapURL = `${url}?map=1`;
        const previewURL = `${url}?view=1`;

        await new Promise((resolve) => {
            const preload = new Image();
            preload.onload = resolve;
            preload.src = mapURL;
        });

        this.objectMapUrl = mapURL;
        this.objectPreviewUrl = previewURL;
        this.isLoading = false;
    }

    /**
     * Download the current opened file.
     */
    public download(): void {
        try {
            this.$store.dispatch('files/download', this.file);
            this.$notify.warning('Do not share download link with other people. If you want to share this data better use "Share" option.');
        } catch (error) {
            this.$notify.error('Can not download your file');
        }
    }

    /**
     * Close the current opened file details modal.
     */
    public closeModal(): void {
        this.$store.commit(APP_STATE_MUTATIONS.TOGGLE_OBJECT_DETAILS_MODAL_SHOWN);
    }

    /**
     * Copy the current opened file.
     */
    public async copy(): Promise<void> {
        await this.$copyText(this.objectLink);
        await this.$notify.success('Link copied successfully.');
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
            this.filePath,
        );
    }
}
</script>

<style scoped lang="scss">
    .modal {
        box-sizing: border-box;
        display: flex;
        align-items: center;
        font-family: 'font_regular', sans-serif;
        width: 1140px;
        max-width: 1140px;

        @media screen and (max-width: 1200px) {
            width: 800px;
            max-width: 800px;
        }

        @media screen and (max-width: 900px) {
            width: 600px;
            max-width: 600px;
        }

        @media screen and (max-width: 660px) {
            flex-direction: column-reverse;
            width: calc(100vw - 50px);
            min-width: calc(100vw - 50px);
            max-width: calc(100vw - 50px);
        }

        @media screen and (max-width: 400px) {
            width: calc(100vw - 20px);
            min-width: calc(100vw - 20px);
            max-width: calc(100vw - 20px);
        }

        &__preview {
            width: 67%;
            max-width: 67%;
            min-height: 75vh;
            display: flex;
            align-items: center;
            justify-content: center;

            @media screen and (max-width: 900px) {
                width: 60%;
                max-width: 60%;
            }

            @media screen and (max-width: 660px) {
                width: 100%;
                min-width: 100%;
                max-width: 100%;
                min-height: 50vh;
                margin-top: 10px;
                border-radius: 0 0 10px 10px;
            }

            @media screen and (max-width: 500px) {
                min-height: 30vh;
            }
        }

        &__info {
            padding: 64px 32px;
            box-sizing: border-box;
            width: 33%;
            max-width: 33%;
            display: flex;
            flex-direction: column;
            align-items: flex-start;
            align-self: flex-start;

            @media screen and (max-width: 900px) {
                width: 40%;
                max-width: 40%;
            }

            @media screen and (max-width: 660px) {
                width: 100%;
                min-width: 100%;
                max-width: 100%;
                padding-bottom: 0;
            }

            @media screen and (max-width: 400px) {
                padding: 64px 20px 0;
            }

            &__title {
                display: inline-block;
                font-weight: bold;
                max-width: 100%;
                position: relative;
                font-size: 18px;
                text-overflow: ellipsis;
                white-space: nowrap;
                overflow: hidden;
                margin-bottom: 14px;
            }

            &__size {
                margin-bottom: 14px;
                font-size: 12px;

                &__label {
                    margin-right: 7px;
                }
            }

            &__download-btn {
                margin-bottom: 14px;
            }

            &__input-group {
                display: flex;
                align-items: center;
                width: 100%;

                @media screen and (max-width: 1200px) {
                    flex-direction: column;
                    align-items: flex-start;
                }

                @media screen and (max-width: 660px) {
                    flex-direction: row;
                    align-items: center;
                }

                @media screen and (max-width: 400px) {
                    flex-direction: column;
                    align-items: flex-start;
                }

                input {
                    box-sizing: border-box;
                    width: 100%;
                    padding: 0.375rem 0.75rem;
                    font-size: 1rem;
                    font-weight: 400;
                    line-height: 1.5;
                    color: #495057;
                    border: 1px solid #ced4da;
                    border-radius: 0.25rem;
                    background-color: #e9ecef;
                }

                &__copy {
                    padding: 0 10px;
                }
            }

            &__loader {
                margin: 20px 0;
            }

            &__map {
                margin-top: 36px;
            }

            &__note {
                margin-top: 16px;
                text-align: left;
            }
        }
    }

    .preview {
        width: 100%;
    }

    .object-map {
        width: 100%;
    }

    .storage-nodes {
        padding: 5px;
        background: rgb(0 0 0 / 80%);
        font-weight: normal;
        color: white;
        font-size: 0.8rem;
    }

    .text-lighter {
        color: #768394;
    }

    .failed-preview {
        width: 50%;
    }
</style>
