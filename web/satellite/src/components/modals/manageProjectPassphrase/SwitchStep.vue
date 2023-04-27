// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="switch-step">
        <p class="switch-step__info">
            Switch passphrases to view existing data that is uploaded with a different passphrase, or upload new data.
            Please note that you won’t see the previous data once you switch passphrases.
        </p>
        <VInput
            label="Encryption Passphrase"
            :is-password="true"
            width="100%"
            height="56px"
            placeholder="Enter Encryption Passphrase"
            :error="enterError"
            @setData="setPassphrase"
        />
        <div class="switch-step__buttons">
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
                :on-press="onSwitch"
                :is-disabled="!passphrase"
            />
        </div>
    </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';

import { useNotify } from '@/utils/hooks';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { EdgeCredentials } from '@/types/accessGrants';
import { useAppStore } from '@/store/modules/appStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';

import VButton from '@/components/common/VButton.vue';
import VInput from '@/components/common/VInput.vue';

const props = withDefaults(defineProps<{
    onCancel?: () => void,
}>(), {
    onCancel: () => () => {},
});

const bucketsStore = useBucketsStore();
const appStore = useAppStore();
const notify = useNotify();

const passphrase = ref<string>('');
const enterError = ref<string>('');

/**
 * Sets passphrase input value to local variable.
 * Resets error is present.
 * @param value
 */
function setPassphrase(value: string): void {
    if (enterError.value) {
        enterError.value = '';
    }

    passphrase.value = value;
}

/**
 * Sets new passphrase and generates new edge credentials.
 */
async function onSwitch(): Promise<void> {
    if (!passphrase.value) {
        enterError.value = 'Passphrase can\'t be empty';

        return;
    }

    bucketsStore.setEdgeCredentials(new EdgeCredentials());
    bucketsStore.setPassphrase(passphrase.value);
    bucketsStore.setPromptForPassphrase(false);

    notify.success('Passphrase was switched successfully');
    appStore.updateActiveModal(MODALS.manageProjectPassphrase);
}
</script>

<style scoped lang="scss">
.switch-step {
    display: flex;
    flex-direction: column;
    align-items: center;
    font-family: 'font_regular', sans-serif;
    max-width: 350px;

    &__info {
        font-size: 14px;
        line-height: 19px;
        color: #354049;
        padding-bottom: 16px;
        margin-bottom: 6px;
        border-bottom: 1px solid var(--c-grey-2);
        text-align: left;
    }

    &__buttons {
        display: flex;
        align-items: center;
        justify-content: center;
        column-gap: 16px;
        margin-top: 16px;
        padding-top: 24px;
        border-top: 1px solid var(--c-grey-2);
        width: 100%;

        @media screen and (max-width: 530px) {
            column-gap: unset;
            flex-direction: column-reverse;
            row-gap: 15px;
        }
    }
}
</style>
