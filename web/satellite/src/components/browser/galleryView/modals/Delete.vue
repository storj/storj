// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="onClose">
        <template #content>
            <div class="modal">
                <ModalHeader
                    :icon="DeleteIcon"
                    title="Delete File"
                />
                <p class="modal__info">The following file will be deleted.</p>
                <p class="modal__name">{{ object?.Key || '' }}</p>
                <div class="modal__buttons">
                    <VButton
                        label="Cancel"
                        height="52px"
                        width="100%"
                        border-radius="10px"
                        font-size="14px"
                        :on-press="onClose"
                        is-white
                    />
                    <VButton
                        label="Delete"
                        height="52px"
                        width="100%"
                        border-radius="10px"
                        font-size="14px"
                        :on-press="onDelete"
                        :is-disabled="loading"
                        is-solid-delete
                    />
                </div>
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { ref } from 'vue';

import { BrowserObject, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';

import VModal from '@/components/common/VModal.vue';
import VButton from '@/components/common/VButton.vue';
import ModalHeader from '@/components/modals/ModalHeader.vue';

import DeleteIcon from '@/../static/images/browser/galleryView/modals/delete.svg';

const obStore = useObjectBrowserStore();

const props = defineProps<{
    object: BrowserObject | undefined
    onDelete: () => Promise<void>
    onClose: () => void
}>();

const loading = ref<boolean>(false);
</script>

<style scoped lang="scss">
.modal {
    padding: 32px;
    font-family: 'font_regular', sans-serif;
    background-color: var(--c-white);
    box-shadow: 0 20px 30px rgb(10 27 44 / 20%);
    border-radius: 20px;
    width: 410px;
    box-sizing: border-box;

    @media screen and (width <= 520px) {
        width: unset;
    }

    &__info {
        font-size: 14px;
        line-height: 20px;
        color: var(--c-black);
        margin-bottom: 30px;
        text-align: left;
    }

    &__name {
        font-family: 'font_bold', sans-serif;
        font-size: 14px;
        line-height: 20px;
        color: var(--c-black);
        text-align: left;
    }

    &__buttons {
        display: flex;
        align-items: center;
        column-gap: 16px;
        padding-top: 16px;
        margin-top: 16px;
        border-top: 1px solid var(--c-grey-2);
    }
}
</style>
