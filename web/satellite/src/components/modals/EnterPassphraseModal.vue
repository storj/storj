// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <div class="modal__header">
                    <AccessEncryptionIcon />
                    <h1 class="modal__header__title">Enter passphrase</h1>
                </div>
                <p class="modal__info">
                    Enter your encryption passphrase to view and manage your data in the browser. This passphrase will
                    be used to unlock all buckets in this project.
                </p>
                <VInput
                    label="Encryption Passphrase"
                    placeholder="Enter a passphrase here"
                    :error="enterError"
                    role-description="passphrase"
                    is-password
                    @setData="setPassphrase"
                />
                <div class="modal__buttons">
                    <VButton
                        label="Skip"
                        height="48px"
                        font-size="14px"
                        border-radius="10px"
                        :is-transparent="true"
                        :on-press="skipPassphrase"
                    />
                    <VButton
                        label="Continue ->"
                        height="48px"
                        font-size="14px"
                        border-radius="10px"
                        :on-press="onContinue"
                        :is-disabled="!passphrase"
                    />
                </div>
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { ref } from 'vue';

import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAppStore } from '@/store/modules/appStore';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import VModal from '@/components/common/VModal.vue';
import VInput from '@/components/common/VInput.vue';
import VButton from '@/components/common/VButton.vue';

import AccessEncryptionIcon from '@/../static/images/accessGrants/newCreateFlow/accessEncryption.svg';

const analyticsStore = useAnalyticsStore();
const bucketsStore = useBucketsStore();
const appStore = useAppStore();

const enterError = ref<string>('');
const passphrase = ref<string>('');

/**
 * Sets passphrase.
 */
function onContinue(): void {
    if (!passphrase.value) {
        enterError.value = 'Passphrase can\'t be empty';
        analyticsStore.errorEventTriggered(AnalyticsErrorEventSource.OPEN_BUCKET_MODAL);

        return;
    }

    analyticsStore.eventTriggered(AnalyticsEvent.PASSPHRASE_CREATED, {
        method: 'enter',
    });

    bucketsStore.setPassphrase(passphrase.value);
    bucketsStore.setPromptForPassphrase(false);

    closeModal();
}

/**
 * Opens the SkipPassphrase modal for confirmation.
 */
function skipPassphrase(): void {
    appStore.updateActiveModal(MODALS.skipPassphrase);
}

/**
 * Closes enter passphrase modal.
 */
function closeModal(): void {
    appStore.removeActiveModal();
}

/**
 * Sets passphrase from child component.
 */
function setPassphrase(value: string): void {
    if (enterError.value) enterError.value = '';

    passphrase.value = value;
}
</script>

<style scoped lang="scss">
    .modal {
        font-family: 'font_regular', sans-serif;
        display: flex;
        flex-direction: column;
        padding: 32px;
        max-width: 350px;

        &__header {
            display: flex;
            align-items: center;
            padding-bottom: 16px;
            margin-bottom: 16px;
            border-bottom: 1px solid var(--c-grey-2);

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 24px;
                line-height: 31px;
                color: var(--c-grey-8);
                margin-left: 16px;
                text-align: left;
            }
        }

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
            column-gap: 20px;
            margin-top: 31px;
            width: 100%;

            @media screen and (width <= 500px) {
                flex-direction: column-reverse;
                column-gap: unset;
                row-gap: 20px;
            }
        }
    }
</style>
