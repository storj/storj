// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="create-step">
        <p class="create-step__info">
            Creating a new passphrase allows you to upload data separately from the data uploaded with the current
            encryption passphrase.
        </p>
        <div class="create-step__buttons">
            <VButton
                label="Back"
                width="100%"
                height="52px"
                font-size="14px"
                border-radius="10px"
                :is-white="true"
                :on-press="onCancel"
            />
            <VButton
                label="Continue ->"
                width="100%"
                height="52px"
                font-size="14px"
                border-radius="10px"
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
    font-family: 'font_regular', sans-serif;
    max-width: 350px;

    &__info {
        font-size: 14px;
        line-height: 19px;
        color: #354049;
        padding-bottom: 16px;
        margin-bottom: 24px;
        border-bottom: 1px solid var(--c-grey-2);
        text-align: left;
    }

    &__buttons {
        display: flex;
        align-items: center;
        justify-content: center;
        column-gap: 16px;
        width: 100%;

        @media screen and (max-width: 530px) {
            column-gap: unset;
            flex-direction: column-reverse;
            row-gap: 15px;
        }
    }
}
</style>
