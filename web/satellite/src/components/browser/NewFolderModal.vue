// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div :click="stopClickPropagation">
        <div
            id="new-folder-modal"
            class="modal fade show modal-open"
            tabindex="-1"
            aria-labelledby="newFolderModalLabel"
            aria-hidden="true"
        >
            <div class="modal-dialog modal-dialog-centered">
                <div class="modal-content">
                    <div class="modal-header">
                        <h5 class="modal-title">New Folder</h5>
                        <button
                            type="button"
                            class="close"
                            data-dismiss="modal"
                            aria-label="Close"
                            @click="close"
                        >
                            <span
                                aria-hidden="true"
                                aria-roledescription="close-new-folder-modal"
                            >&times;</span>
                        </button>
                    </div>
                    <div class="modal-body">
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
                    </div>
                    <div class="modal-footer">
                        <button 
                            type="button" 
                            class="btn btn-light" 
                            data-bs-dismiss="modal" 
                            @click="close"
                        >
                            Close
                        </button>
                        <button 
                            type="button" 
                            :disabled="!createFolderEnabled" 
                            class="btn btn-primary"
                            @click="createFolder"
                        >
                            Save changes
                        </button>
                    </div>  
                    <div
                        v-if="creatingFolderSpinner"
                        class="d-flex justify-content-center spinner-wrapper"
                    >
                        <div class="spinner-border" />
                    </div>
                </div>
            </div>
        </div>
        <div id="backdrop" class="modal-backdrop fade show modal-open" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { BrowserFile } from '@/types/browser';

// @vue/component
@Component({})
export default class NewFolderModal extends Vue {
    public $refs!: {
        folderInput: HTMLInputElement;
        fileInput: HTMLInputElement;
    };
    public createFolderInput = '';
    public creatingFolderSpinner = false;

    /**
     * Close the NewFolderModal.
     */
    public close(): void {
        this.$store.commit('files/closeNewFolderModal');
    }

    /**
     * Retrieve all of the files sorted from the store.
     */
    private get files(): BrowserFile[] {
        return this.$store.getters['files/sortedFiles'];
    }

    /**
     * Return a boolean signifying whether the current folder name abides by our convention.
     */
    public get createFolderEnabled(): boolean {
        const charsOtherThanSpaceExist =
            this.createFolderInput.trim().length > 0;

        const noForwardSlashes = this.createFolderInput.indexOf('/') === -1;

        const nameIsNotOnlyPeriods =
            [...this.createFolderInput.trim()].filter(
                (char) => char === '.',
            ).length !== this.createFolderInput.trim().length;

        const notDuplicate =
            this.files.filter(
                (file) => file.Key === this.createFolderInput.trim(),
            ).length === 0;

        return (
            charsOtherThanSpaceExist &&
            noForwardSlashes &&
            nameIsNotOnlyPeriods &&
            notDuplicate
        );
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
            'files/createFolder',
            this.createFolderInput.trim(),
        );

        // clear folder input
        this.createFolderInput = '';

        // remove the folder creation input
        this.$store.dispatch('files/updateCreateFolderInputShow', false);

        // remove the spinner
        this.creatingFolderSpinner = false;
    }

    /**
     * Cancel folder creation clearing out the input and hiding the folder creation input.
     */
    public cancelFolderCreation(): void {
        this.createFolderInput = '';
        this.$store.dispatch('files/updateCreateFolderInputShow', false);
    }

    /**
     * Stop the propagation of a click event only if the target is an element without new-folder-modal as the id.
     */
    public stopClickPropagation(e: Event): void {
        const target = e.target as HTMLElement;
        if (target.id !== 'new-folder-modal') {
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

#new-folder-modal {
    z-index: 1070;
}

#backdrop {
    z-index: 1060 !important;
}

.spinner-wrapper {
    margin-bottom: 16px;
}
</style>
