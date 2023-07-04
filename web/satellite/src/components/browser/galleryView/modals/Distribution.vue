// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="onClose">
        <template #content>
            <div class="modal">
                <ModalHeader
                    :icon="GlobeIcon"
                    title="Geographic Distribution"
                />
                <img :src="mapUrl" class="modal__map" alt="map">
                <h1 class="modal__title">You’re getting this file from all over the world on the Storj network.</h1>
                <p class="modal__info">
                    Storj Decentralized Cloud Storage splits files into smaller pieces, then distributes those pieces
                    over a global network of Nodes and recompiles them securely on download. This map shows the
                    real-time locations of this file’s pieces.
                </p>
                <div class="modal__buttons">
                    <VButton
                        link="https://docs.storj.io/dcs/concepts/overview#t1eR9"
                        label="Learn More"
                        height="52px"
                        width="100%"
                        border-radius="10px"
                        font-size="14px"
                        :on-press="() => {}"
                        is-white
                    />
                    <VButton
                        label="Close"
                        height="52px"
                        width="100%"
                        border-radius="10px"
                        font-size="14px"
                        :on-press="onClose"
                    />
                </div>
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import VModal from '@/components/common/VModal.vue';
import VButton from '@/components/common/VButton.vue';
import ModalHeader from '@/components/modals/ModalHeader.vue';

import GlobeIcon from '@/../static/images/browser/galleryView/modals/globe.svg';

const props = defineProps<{
    mapUrl: string
    onClose: () => void
}>();
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

    &__map {
        width: 100%;
    }

    &__title {
        font-family: 'font_bold', sans-serif;
        font-size: 18px;
        line-height: 24px;
        letter-spacing: -0.02em;
        color: var(--c-blue-6);
        margin-bottom: 16px;
        text-align: left;
    }

    &__info {
        font-size: 14px;
        line-height: 21px;
        color: var(--c-blue-6);
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
