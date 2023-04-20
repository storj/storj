// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="enter-passphrase">
        <p class="enter-passphrase__info">{{ info }}</p>
        <div class="enter-passphrase__input-container">
            <VInput
                label="Encryption Passphrase"
                placeholder="Enter Encryption Passphrase"
                :is-password="true"
                :init-value="passphrase"
                @setData="setPassphrase"
            />
        </div>
        <div v-if="isNewPassphrase" class="enter-passphrase__toggle-container">
            <Toggle
                :checked="isPassphraseSaved"
                :on-check="togglePassphraseSaved"
                label="Yes, I saved my encryption passphrase."
            />
        </div>
        <ButtonsContainer>
            <template #leftButton>
                <VButton
                    label="Back"
                    width="100%"
                    height="48px"
                    font-size="14px"
                    :on-press="onBack"
                    :is-white="true"
                />
            </template>
            <template #rightButton>
                <VButton
                    label="Create Access ->"
                    width="100%"
                    height="48px"
                    font-size="14px"
                    :on-press="onContinue"
                    :is-disabled="isButtonDisabled"
                />
            </template>
        </ButtonsContainer>
    </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';

import ButtonsContainer from '@/components/accessGrants/createFlow/components/ButtonsContainer.vue';
import Toggle from '@/components/accessGrants/createFlow/components/Toggle.vue';
import VButton from '@/components/common/VButton.vue';
import VInput from '@/components/common/VInput.vue';

const props = defineProps<{
    isNewPassphrase: boolean;
    info: string;
    passphrase: string;
    setPassphrase: (value: string) => void;
    onBack: () => void;
    onContinue: () => void;
}>();

const isPassphraseSaved = ref<boolean>(false);

/**
 * Indicates if continue button is disabled.
 */
const isButtonDisabled = computed((): boolean => {
    if (props.isNewPassphrase) {
        return !(props.passphrase && isPassphraseSaved.value);
    }

    return !props.passphrase;
});

/**
 * Toggles 'passphrase is saved' checkbox.
 */
function togglePassphraseSaved(): void {
    isPassphraseSaved.value = !isPassphraseSaved.value;
}
</script>

<style lang="scss" scoped>
.enter-passphrase {
    font-family: 'font_regular', sans-serif;

    &__info {
        font-size: 14px;
        line-height: 20px;
        color: var(--c-blue-6);
        padding: 16px 0;
        margin-bottom: 16px;
        border-bottom: 1px solid var(--c-grey-2);
        text-align: left;
    }

    &__input-container {
        padding-bottom: 16px;
        border-bottom: 1px solid var(--c-grey-2);
    }

    &__toggle-container {
        padding: 16px 0;
        border-bottom: 1px solid var(--c-grey-2);
    }
}
</style>
