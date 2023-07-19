// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.
<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <div class="modal__header">
                    <AccessEncryptionIcon />
                    <h1 class="modal__header__title">Skip passphrase</h1>
                </div>
                <p class="modal__info">
                    Do you want to remember this choice and always skip the passphrase when opening a project?
                </p>

                <div class="modal__buttons">
                    <VButton
                        label="No"
                        height="48px"
                        font-size="14px"
                        border-radius="10px"
                        :is-transparent="true"
                        :on-press="closeModal"
                    />
                    <VButton
                        label="Yes"
                        height="48px"
                        font-size="14px"
                        border-radius="10px"
                        :on-press="rememberSkip"
                    />
                </div>
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import AccessEncryptionIcon from '../../../static/images/accessGrants/newCreateFlow/accessEncryption.svg';

import { useAppStore } from '@/store/modules/appStore';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useUsersStore } from '@/store/modules/usersStore';
import { useNotify } from '@/utils/hooks';

import VButton from '@/components/common/VButton.vue';
import VModal from '@/components/common/VModal.vue';

const appStore = useAppStore();
const usersStore = useUsersStore();
const notify = useNotify();

/**
 * Remembers to skip passphrase entry next time.
 */
async function rememberSkip() {
    try {
        await usersStore.updateSettings({ passphrasePrompt: false });
        appStore.removeActiveModal();
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.SKIP_PASSPHRASE_MODAL);
    }
}

/**
 * Closes modal.
 */
function closeModal(): void {
    appStore.removeActiveModal();
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
