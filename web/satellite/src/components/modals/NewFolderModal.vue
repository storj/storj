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
                        :is-transparent="true"
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

<script setup lang="ts">
import { computed, ref } from 'vue';

import { BrowserFile } from '@/types/browser';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { useNotify, useStore } from '@/utils/hooks';

import VModal from '@/components/common/VModal.vue';
import VButton from '@/components/common/VButton.vue';
import VLoader from '@/components/common/VLoader.vue';
import VInput from '@/components/common/VInput.vue';

const store = useStore();
const notify = useNotify();

const createFolderName = ref<string>('');
const createFolderError = ref<string>('');
const isLoading = ref<boolean>(false);

/**
 * Retrieve all the files sorted from the store.
 */
const files = computed((): BrowserFile[] => {
    return store.getters['files/sortedFiles'];
});

/**
 * Return a boolean signifying whether the current folder name abides by our convention.
 */
const createFolderEnabled = computed((): boolean => {
    const charsOtherThanSpaceExist = createFolderName.value.trim().length > 0;
    const noForwardSlashes = createFolderName.value.indexOf('/') === -1;

    const nameIsNotOnlyPeriods =
        [...createFolderName.value.trim()].filter(
            (char) => char === '.',
        ).length !== createFolderName.value.trim().length;

    const notDuplicate = files.value.filter(
        (file) => file.Key === createFolderName.value.trim(),
    ).length === 0;

    return (
        charsOtherThanSpaceExist &&
        noForwardSlashes &&
        nameIsNotOnlyPeriods &&
        notDuplicate
    );
});

/**
 * Close the NewFolderModal.
 */
function close(): void {
    store.commit(APP_STATE_MUTATIONS.UPDATE_ACTIVE_MODAL, MODALS.newFolder);
}

/**
 * Sets folder name from input value.
 */
function setName(value: string): void {
    if (createFolderError.value) {
        createFolderError.value = '';
    }

    createFolderName.value = value;
}

/**
 * Create a folder from the name inside of the folder creation input.
 */
async function createFolder(): Promise<void> {
    if (isLoading.value) return;

    if (!createFolderEnabled.value) {
        createFolderError.value = 'Invalid name';

        return;
    }

    isLoading.value = true;

    try {
        await store.dispatch('files/createFolder', createFolderName.value.trim());
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.CREATE_FOLDER_MODAL);
    }

    createFolderName.value = '';
    isLoading.value = false;
    close();
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
