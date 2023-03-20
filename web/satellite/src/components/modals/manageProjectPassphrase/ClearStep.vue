// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="clear-step">
        <h1 class="clear-step__title">Clear my passphrase</h1>
        <p class="clear-step__info">
            By choosing to clear your passphrase for this session, your data will become locked while you can use the
            rest of the dashboard.
        </p>
        <div class="clear-step__buttons">
            <VButton
                label="Back"
                width="100%"
                height="48px"
                :is-white="true"
                :on-press="onCancel"
            />
            <VButton
                label="Clear my passphrase"
                width="100%"
                height="48px"
                :on-press="onClear"
            />
        </div>
    </div>
</template>

<script setup lang="ts">
import { useNotify, useStore } from '@/utils/hooks';
import { OBJECTS_MUTATIONS } from '@/store/modules/objects';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';

import VButton from '@/components/common/VButton.vue';

const props = withDefaults(defineProps<{
    onCancel?: () => void,
}>(), {
    onCancel: () => () => {},
});

const store = useStore();
const notify = useNotify();

/**
 * Clears passphrase and edge credentials.
 */
function onClear(): void {
    store.commit(OBJECTS_MUTATIONS.CLEAR);
    store.commit(APP_STATE_MUTATIONS.UPDATE_ACTIVE_MODAL, MODALS.manageProjectPassphrase);
    notify.success('Passphrase was cleared successfully');
}
</script>

<style scoped lang="scss">
.clear-step {
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
