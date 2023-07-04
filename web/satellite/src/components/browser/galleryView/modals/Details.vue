// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="onClose">
        <template #content>
            <div class="modal">
                <ModalHeader
                    :icon="DetailsIcon"
                    title="View Details"
                />
                <div class="modal__item">
                    <p class="modal__item__label">Name</p>
                    <p class="modal__item__label right" :title="object.Key">{{ object.Key }}</p>
                </div>
                <div class="modal__item">
                    <p class="modal__item__label">Size</p>
                    <p class="modal__item__label right">{{ prettyBytes(object.Size) }}</p>
                </div>
                <div class="modal__item">
                    <p class="modal__item__label">Last Edited</p>
                    <p class="modal__item__label right" :title="object.LastModified.toLocaleString()">
                        {{ object.LastModified.toLocaleString() }}
                    </p>
                </div>
                <div class="modal__item last">
                    <p class="modal__item__label">Bucket</p>
                    <p class="modal__item__label right" :title="bucket">{{ bucket }}</p>
                </div>
                <VButton
                    label="Close"
                    height="52px"
                    width="100%"
                    border-radius="10px"
                    font-size="14px"
                    :on-press="onClose"
                />
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import prettyBytes from 'pretty-bytes';
import { computed } from 'vue';

import { BrowserObject, useObjectBrowserStore } from '@/store/modules/objectBrowserStore';

import VModal from '@/components/common/VModal.vue';
import VButton from '@/components/common/VButton.vue';
import ModalHeader from '@/components/modals/ModalHeader.vue';

import DetailsIcon from '@/../static/images/browser/galleryView/modals/details.svg';

const obStore = useObjectBrowserStore();

const props = defineProps<{
    object: BrowserObject
    onClose: () => void
}>();

/**
 * Returns active bucket name from store.
 */
const bucket = computed((): string => {
    return obStore.state.bucket;
});
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
        width: 320px;
    }

    &__item {
        display: flex;
        align-items: center;
        justify-content: space-between;
        max-width: 100%;
        padding-bottom: 16px;

        &__label {
            font-weight: 500;
            font-size: 14px;
            line-height: 20px;
            color: var(--c-black);
        }
    }
}

.right {
    margin-left: 16px;
    overflow-x: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.last {
    border-bottom: 1px solid var(--c-grey-2);
    margin-bottom: 16px;
}
</style>
