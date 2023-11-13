// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="generated">
        <p v-if="isProjectPassphrase" class="generated__info">
            Please note that Storj does not know or store your encryption passphrase. If you lose it, you will not be
            able to recover your files.
        </p>
        <p v-else class="generated__info">
            This passphrase will be used to encrypt all the files you upload using this access grant. You will need it
            to access these files in the future.
        </p>
        <ButtonsContainer label="Save your encryption passphrase">
            <template #leftButton>
                <VButton
                    :label="isPassphraseCopied ? 'Copied' : 'Copy'"
                    width="100%"
                    height="40px"
                    font-size="14px"
                    :on-press="onCopy"
                    :icon="isPassphraseCopied ? 'check' : 'copy'"
                    :is-white="!isPassphraseCopied"
                    :is-white-green="isPassphraseCopied"
                />
            </template>
            <template #rightButton>
                <VButton
                    :label="isPassphraseDownloaded ? 'Downloaded' : 'Download'"
                    width="100%"
                    height="40px"
                    font-size="14px"
                    :on-press="onDownload"
                    :icon="isPassphraseDownloaded ? 'check' : 'download'"
                    :is-white="!isPassphraseDownloaded"
                    :is-white-green="isPassphraseDownloaded"
                />
            </template>
        </ButtonsContainer>
        <div class="generated__blurred">
            <ValueWithBlur
                button-label="Show Passphrase"
                :is-mnemonic="true"
                :value="passphrase"
            />
        </div>
        <div class="generated__toggle-container">
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
                    border-radius="10px"
                    :on-press="onBack"
                    :is-white="true"
                />
            </template>
            <template #rightButton>
                <VButton
                    :label="isProjectPassphrase ? 'Continue ->' : 'Create Access ->'"
                    width="100%"
                    height="48px"
                    font-size="14px"
                    border-radius="10px"
                    :on-press="onContinue"
                    :is-disabled="isButtonDisabled"
                />
            </template>
        </ButtonsContainer>
    </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';

import { useNotify } from '@/utils/hooks';
import { Download } from '@/utils/download';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import ButtonsContainer from '@/components/accessGrants/createFlow/components/ButtonsContainer.vue';
import ValueWithBlur from '@/components/accessGrants/createFlow/components/ValueWithBlur.vue';
import Toggle from '@/components/accessGrants/createFlow/components/Toggle.vue';
import VButton from '@/components/common/VButton.vue';

const props = withDefaults(defineProps<{
    name: string;
    passphrase: string;
    onBack: () => void;
    onContinue: () => void;
    isProjectPassphrase?: boolean;
}>(), {
    isProjectPassphrase: false,
});

const analyticsStore = useAnalyticsStore();

const notify = useNotify();

const isPassphraseSaved = ref<boolean>(false);
const isPassphraseCopied = ref<boolean>(false);
const isPassphraseDownloaded = ref<boolean>(false);

/**
 * Indicates if continue button is disabled.
 */
const isButtonDisabled = computed((): boolean => {
    return !(props.passphrase && isPassphraseSaved.value);
});

/**
 * Toggles 'passphrase is saved' checkbox.
 */
function togglePassphraseSaved(): void {
    isPassphraseSaved.value = !isPassphraseSaved.value;
}

/**
 * Saves passphrase to clipboard.
 */
function onCopy(): void {
    isPassphraseCopied.value = true;
    navigator.clipboard.writeText(props.passphrase);
    analyticsStore.eventTriggered(AnalyticsEvent.COPY_TO_CLIPBOARD_CLICKED);
    notify.success(`Passphrase was copied successfully`);
}

/**
 * Downloads passphrase into .txt file.
 */
function onDownload(): void {
    isPassphraseDownloaded.value = true;
    Download.file(props.passphrase, `passphrase-${props.name}-${new Date().toISOString()}.txt`);
    analyticsStore.eventTriggered(AnalyticsEvent.DOWNLOAD_TXT_CLICKED);
}
</script>

<style lang="scss" scoped>
.generated {
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

    &__toggle-container {
        padding: 16px 0;
        border-bottom: 1px solid var(--c-grey-2);
    }

    &__blurred {
        margin-top: 16px;
        padding: 16px 0;
        border-top: 1px solid var(--c-grey-2);
        border-bottom: 1px solid var(--c-grey-2);
    }
}
</style>
