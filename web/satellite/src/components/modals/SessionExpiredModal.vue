// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :is-closable="false">
        <template #content>
            <div class="modal">
                <div class="modal__header">
                    <Icon class="modal__header__icon" />
                    <h1 class="modal__header__title">Session Expired</h1>
                </div>
                <p class="modal__info">
                    To protect your account and data, you’ve been automatically logged out. You can change your session
                    timeout preferences in your account settings.
                </p>
                <VButton
                    label="Go to login"
                    width="100%"
                    height="40px"
                    font-size="14px"
                    border-radius="10px"
                    :on-press="onRedirect"
                />
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import VButton from '@/components/common/VButton.vue';
import VModal from '@/components/common/VModal.vue';

import Icon from '@/../static/images/session/inactivityTimer.svg';

const props = defineProps<{
    onRedirect: () => void;
}>();
</script>

<style scoped lang="scss">
    .modal {
        max-width: 410px;
        padding: 32px;
        box-sizing: border-box;
        font-family: 'font_regular', sans-serif;

        &__header {
            display: flex;
            align-items: center;
            margin-bottom: 32px;

            &__icon {
                margin-right: 16px;
                min-width: 40px;
                width: 40px;
                height: 40px;
            }

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 24px;
                line-height: 31px;
                letter-spacing: -0.02em;
                color: var(--c-black);
            }
        }

        &__info {
            font-size: 16px;
            line-height: 24px;
            color: var(--c-black);
            margin-bottom: 40px;
            text-align: left;
        }
    }
</style>
