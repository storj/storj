// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="clear-step">
        <p class="clear-step__info">
            By choosing to clear your passphrase for this session, your data will become locked while you can use the
            rest of the dashboard.
        </p>
        <div class="clear-step__buttons">
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
                :on-press="onClear"
            />
        </div>
    </div>
</template>

<script setup lang="ts">
import { useNotify } from '@/utils/hooks';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useAppStore } from '@/store/modules/appStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';

import VButton from '@/components/common/VButton.vue';

const props = withDefaults(defineProps<{
    onCancel?: () => void,
}>(), {
    onCancel: () => () => {},
});

const bucketsStore = useBucketsStore();
const appStore = useAppStore();
const notify = useNotify();

/**
 * Clears passphrase and edge credentials.
 */
function onClear(): void {
    bucketsStore.clearS3Data();
    appStore.updateActiveModal(MODALS.manageProjectPassphrase);
    notify.success('Passphrase was cleared successfully');
}
</script>

<style scoped lang="scss">
.clear-step {
    display: flex;
    flex-direction: column;
    font-family: 'font_regular', sans-serif;
    max-width: 350px;

    &__info {
        font-size: 14px;
        line-height: 19px;
        color: #354049;
        padding-bottom: 16px;
        margin-bottom: 16px;
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
