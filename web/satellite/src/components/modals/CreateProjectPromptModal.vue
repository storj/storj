// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="modal">
                <img
                    class="modal__icon"
                    src="@/../static/images/account/billing/paidTier/prompt.png"
                    alt="Prompt Image"
                >
                <h1 class="modal__title" aria-roledescription="modal-title">
                    Get more projects<br>when you upgrade
                </h1>
                <p class="modal__info">
                    Upgrade your Free Account to create<br>more projects and gain access to higher limits.
                </p>
                <div class="modal__btn">
                    <VButton
                        width="100%"
                        height="56px"
                        border-radius="8px"
                        label="Upgrade to Pro Account ->"
                        :on-press="onClick"
                    />
                </div>
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useAppStore } from '@/store/modules/appStore';

import VButton from '@/components/common/VButton.vue';
import VModal from '@/components/common/VModal.vue';

const appStore = useAppStore();

/**
 * Holds on button click logic.
 * Closes this modal and opens upgrade account modal.
 */
function onClick(): void {
    appStore.updateActiveModal(MODALS.createProjectPrompt);
    appStore.updateActiveModal(MODALS.upgradeAccount);
}

/**
 * Closes create project prompt modal.
 */
function closeModal(): void {
    appStore.updateActiveModal(MODALS.createProjectPrompt);
}
</script>

<style scoped lang="scss">
    .modal {
        width: 600px;
        padding: 54px 0 65px;
        display: flex;
        align-items: center;
        flex-direction: column;
        font-family: 'font_regular', sans-serif;

        @media screen and (max-width: 768px) {
            width: unset;
            padding: 54px 24px 65px;
        }

        @media screen and (max-width: 450px) {
            padding: 54px 12px 32px;
        }

        &__icon {
            max-height: 154px;
            max-width: 118px;

            @media screen and (max-width: 450px) {
                max-height: 127px;
                max-width: 97px;
            }
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 28px;
            line-height: 34px;
            color: #1b2533;
            margin-top: 40px;
            text-align: center;

            @media screen and (max-width: 450px) {
                margin-top: 24px;
                font-size: 24px;
                line-height: 31px;
            }
        }

        &__info {
            font-family: 'font_regular', sans-serif;
            font-size: 16px;
            line-height: 21px;
            text-align: center;
            color: #354049;
            margin: 15px 0 45px;

            @media screen and (max-width: 450px) {
                margin: 10px 0 24px;
                font-size: 14px;
                line-height: 20px;
            }
        }

        &__btn {
            display: flex;
            justify-content: center;
            width: 256px;

            @media screen and (max-width: 768px) {
                width: 224px;
            }
        }
    }
</style>
