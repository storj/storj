// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="close">
        <template #content>
            <div class="modal">
                <h5 class="modal__title">New Folder</h5>
                <VInput
                    label="Folder name"
                    placeholder="Enter name"
                    :error="createFolderError"
                    @setData="setName"
                    @keypress.enter="createFolder"
                />
                <div class="modal__button-container">
                    <VButton
                        label="Cancel"
                        width="100%"
                        height="48px"
                        :on-press="close"
                        is-transparent="true"
                    />
                    <VButton
                        label="Create Folder"
                        width="100%"
                        height="48px"
                        :on-press="createFolder"
                        :is-disabled="!createFolderName"
                    />
                </div>
                <div v-if="isLoading" class="modal__blur">
                    <VLoader width="50px" height="50px" />
                </div>
            </div>
        </template>
    </VModal>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { BrowserFile } from '@/types/browser';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';

import VModal from '@/components/common/VModal.vue';
import VButton from '@/components/common/VButton.vue';
import VLoader from '@/components/common/VLoader.vue';
import VInput from '@/components/common/VInput.vue';

// @vue/component
@Component({
    components: {
        VModal,
        VButton,
        VLoader,
        VInput,
    },
})
export default class NewFolderModal extends Vue {
    public $refs!: {
        folderInput: HTMLInputElement;
        fileInput: HTMLInputElement;
    };

    public createFolderName = '';
    public createFolderError = '';
    public isLoading = false;

    /**
     * Close the NewFolderModal.
     */
    public close(): void {
        this.$store.commit(APP_STATE_MUTATIONS.TOGGLE_NEW_FOLDER_MODAL_SHOWN);
    }

    /**
     * Sets folder name from input value.
     */
    public setName(value: string): void {
        if (this.createFolderError) {
            this.createFolderError = '';
        }

        this.createFolderName = value;
    }

    /**
     * Retrieve all the files sorted from the store.
     */
    private get files(): BrowserFile[] {
        return this.$store.getters['files/sortedFiles'];
    }

    /**
     * Return a boolean signifying whether the current folder name abides by our convention.
     */
    public get createFolderEnabled(): boolean {
        const charsOtherThanSpaceExist =
            this.createFolderName.trim().length > 0;

        const noForwardSlashes = this.createFolderName.indexOf('/') === -1;

        const nameIsNotOnlyPeriods =
            [...this.createFolderName.trim()].filter(
                (char) => char === '.',
            ).length !== this.createFolderName.trim().length;

        const notDuplicate =
            this.files.filter(
                (file) => file.Key === this.createFolderName.trim(),
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
        if (this.isLoading) return;
        
        this.isLoading = true;

        if (!this.createFolderEnabled) {
            this.createFolderError = 'Invalid name';

            return;
        }

        await this.$store.dispatch(
            'files/createFolder',
            this.createFolderName.trim(),
        );

        this.createFolderName = '';
        this.isLoading = false;
        this.close();
    }
}
</script>

<style scoped lang="scss">
    .modal {
        width: 400px;
        padding: 32px;
        box-sizing: border-box;
        display: flex;
        align-items: center;
        flex-direction: column;
        font-family: 'font_regular', sans-serif;

        @media screen and (max-width: 450px) {
            width: 320px;
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 28px;
            line-height: 34px;
            color: #1b2533;
            margin-bottom: 40px;
            text-align: center;
        }

        &__button-container {
            width: 100%;
            display: flex;
            align-items: center;
            justify-content: space-between;
            margin-top: 30px;
            column-gap: 20px;

            @media screen and (max-width: 550px) {
                margin-top: 20px;
                column-gap: unset;
                row-gap: 8px;
                flex-direction: column-reverse;
            }
        }

        &__blur {
            position: absolute;
            top: 0;
            left: 0;
            height: 100%;
            width: 100%;
            background-color: rgb(229 229 229 / 20%);
            border-radius: 8px;
            z-index: 100;
            display: flex;
            align-items: center;
            justify-content: center;
        }
    }
</style>
