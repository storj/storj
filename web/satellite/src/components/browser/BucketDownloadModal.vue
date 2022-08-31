// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div @click="stopClickPropagation">
        <div
            id="download-modal"
            class="modal fade show modal-open"
            tabindex="-1"
            aria-labelledby="shareModalLabel"
            aria-hidden="true"
        >
            <div class="modal-dialog modal-dialog-centered">
                <div class="modal-content text-center border-0 p-2 p-sm-4">
                    <div class="modal-header border-0">
                        <h5 class="modal-title pt-2">Download Bucket</h5>
                        <button
                            type="button"
                            class="close"
                            data-dismiss="modal"
                            aria-label="Close"
                            @click="close"
                        >
                            <span
                                aria-hidden="true"
                                aria-roledescription="close-share-modal"
                            >&times;</span>
                        </button>
                    </div>
                    <div class="modal-body pt-0 text-center">
                        <p class="modal-text mb-5">The following bucket will be compressed and downloaded</p>
                        <span class="bucket-name">
                            <bucket-icon class="bucket-icon mr-2" />
                            {{ bucketName }}
                        </span>
                        <div class="d-flex justify-content-around btn-wrapper mt-5">
                            <button 
                                class="btn btn-block close-button" 
                                @click="close"
                            >
                                Close
                            </button>
                            <button 
                                class="btn btn-primary btn-block download-button"
                                :class="{ downloading: downloading }"
                                :is-disabled="downloading"
                                @click="onDownloadBucketClick"
                            >   
                                {{ downloading ? "Downloading" : "Download Bucket" }}
                            </button>
                        </div>
                    </div>
                </div>
            </div>
        </div>
        <div id="backdrop" class="modal-backdrop fade show modal-open" />
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { BrowserFile } from '@/types/browser';

import BucketIcon from '@/../static/images/objects/bucketDownload.svg';

// @vue/component
@Component({
    components: {
        BucketIcon,
    },
})
export default class BucketDownloadModal extends Vue {
    public downloading = false;

    @Prop({ default: '' })
    public readonly bucketName: string;

    @Prop()
    private readonly files: BrowserFile[];

    @Prop()
    private readonly path: string;

    /**
     * Retrieve the path to the current file that has the downloadModal opened from the store.
     */
    private get filePath(): void {
        return this.$store.state.files.downloadModal;
    }

    /**
     * Close the BucketDownloadModal.
     */
    public close(): void {
        this.$store.commit('files/closeDownloadModal');
    }

    /**
     * Toggles share bucket modal.
     */
    public onDownloadBucketClick(): void {
        this.downloading = true;

        const params = {
            path: this.path,
            files: this.files,
        };

        try {
            this.$store.dispatch('files/downloadAll', params);
            this.$notify.warning('Do not share download link with other people. If you want to share this data better use "Share" option.');
        } catch (error) {
            this.$notify.error('Can not download your file');
        }

        this.downloading = false;

    }

    /**
     * Stop the propagation of a click event only if the target is an element without share-modal as the id.
     */
    public stopClickPropagation(e: Event): void {
        const target = e.target as HTMLElement;
        if (target.id !== 'download-modal') {
            e.stopPropagation();
        }
    }
}
</script>

<style scoped>
.modal-open {
    display: block !important;
}

.modal-title {
    word-break: break-word;
    font-weight: bold;
}

.closex {
    cursor: pointer;
}

#download-modal {
    z-index: 1070;
}

#backdrop {
    z-index: 1060 !important;
}

.modal-header {
    display: block;
}

.close {
    position: absolute;
    top: 15px;
    right: 25px;
}

.modal-title {
    font-size: 22px;
    font-family: 'font_bold', sans-serif;
}

.bucket-name {
    background: #F4F5F7;
    border-radius: 30px;
    padding: 10px 36px;
    font-family: 'font_bold', sans-serif;
}

.modal-header {
    display: block !important;
}

.modal-content {
    min-width: 531px;
}

.modal-text {
    font-family: 'font_regular', sans-serif;
    font-size: 16px;
}

.btn {
    border-radius: 8px;
    height: 48px;
    font-family: 'font_bold', sans-serif;
    max-width: 223px;
}

.download-button {
    background-color: #0149ff;
    color: #fff;
    margin-top: 0 !important;
}

.download-button.downloading,
.download-button.downloading:focus,
.download-button.downloading:hover {
    background-color: #00AC26;
}

.close-button {
    border: 1px solid #D8DEE3 !important;
}

</style>
