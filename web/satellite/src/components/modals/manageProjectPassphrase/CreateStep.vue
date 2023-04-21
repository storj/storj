// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="create-step">
        <h1 class="create-step__title">Create a new passphrase</h1>
        <p class="create-step__info">
            Creating a new passphrase allows you to upload data separately from the data uploaded with the current
            encryption passphrase.
        </p>
        <div class="create-step__buttons">
            <VButton
                label="Back"
                width="100%"
                height="48px"
                :is-white="true"
                :on-press="onCancel"
            />
            <VButton
                label="Next"
                width="100%"
                height="48px"
                :on-press="onNext"
            />
        </div>
    </div>
</template>

<script setup lang="ts">
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useAppStore } from '@/store/modules/appStore';

import VButton from '@/components/common/VButton.vue';

const props = withDefaults(defineProps<{
    onCancel?: () => void,
}>(), {
    onCancel: () => () => {},
});

const appStore = useAppStore();

/**
 * Starts create new passphrase flow.
 */
function onNext(): void {
    appStore.updateActiveModal(MODALS.manageProjectPassphrase);
    appStore.updateActiveModal(MODALS.createProjectPassphrase);
}
</script>

<style scoped lang="scss">
.create-step {
    display: flex;
    flex-direction: column;
    align-items: center;
    font-family: 'font_regular', sans-serif;
    max-width: 433px;

    &__title {
        font-family: 'font_bold', sans-serif;
        font-size: 32px;
        line-height: 39px;
        color: #1b2533;
        margin: 14px 0;
    }

    &__info {
        font-size: 14px;
        line-height: 19px;
        color: #354049;
        margin-bottom: 24px;
    }

    &__buttons {
        display: flex;
        align-items: center;
        justify-content: center;
        column-gap: 33px;
        width: 100%;

        @media screen and (max-width: 530px) {
            column-gap: unset;
            flex-direction: column-reverse;
            row-gap: 15px;
        }
    }
}
</style>
