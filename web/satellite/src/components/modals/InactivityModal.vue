// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="onClose">
        <template #content>
            <div class="modal">
                <Icon class="modal__icon" />
                <h1 class="modal__title">Your session is about to expire due to inactivity in <span class="modal__title__timer">{{ seconds }} second{{ seconds != 1 ? 's' : '' }}</span></h1>
                <p class="modal__info">Do you want to stay logged in?</p>
                <div class="modal__buttons">
                    <VButton
                        label="Stay Logged In"
                        height="40px"
                        font-size="13px"
                        class="modal__buttons__button"
                        :on-press="withLoading(onContinue)"
                        :disabled="isLoading"
                    />
                    <VButton
                        label="Log out"
                        height="40px"
                        font-size="13px"
                        :is-transparent="true"
                        class="modal__buttons__button logout"
                        :on-press="withLoading(onLogout)"
                        :disabled="isLoading"
                    />
                </div>
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';

import VButton from '@/components/common/VButton.vue';
import VModal from '@/components/common/VModal.vue';

import Icon from '@/../static/images/session/inactivityTimer.svg';

const props = withDefaults(defineProps<{
    onContinue: () => Promise<void>;
    onLogout: () => Promise<void>;
    onClose: () => void;
    initialSeconds: number;
}>(), {
    onContinue: () => Promise.resolve,
    onLogout: () =>  Promise.resolve,
    onClose: () => {},
    initialSeconds: 60,
});

const seconds = ref<number>(0);
const isLoading = ref<boolean>(false);

/**
 * Returns a function that disables modal interaction during execution.
 */
function withLoading(fn: () => Promise<void>): () => Promise<void> {
    return async () => {
        if (isLoading.value) return;

        isLoading.value = true;
        await fn();
        isLoading.value = false;
    };
}

/**
 * Lifecycle hook after initial render.
 * Starts timer that decreases number of seconds until session expiration.
 */
onMounted(async (): Promise<void> => {
    seconds.value = props.initialSeconds;
    const id: ReturnType<typeof setInterval> = setInterval(() => {
        if (--seconds.value <= 0) clearInterval(id);
    }, 1000);
});
</script>

<style scoped lang="scss">
    .modal {
        max-width: 500px;
        padding: 32px;
        box-sizing: border-box;
        font-family: 'font_regular', sans-serif;
        text-align: left;

        &__icon {
            margin-bottom: 24px;
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 28px;
            line-height: 36px;
            letter-spacing: -0.02em;
            color: #000;
            margin-bottom: 8px;

            &__timer {
                color: var(--c-pink-4);
            }
        }

        &__info {
            font-family: 'font_regular', sans-serif;
            font-size: 16px;
            line-height: 24px;
            color: #000;
            margin-bottom: 16px;
        }

        &__buttons {
            display: flex;
            flex-direction: row;

            &__button {
                padding: 16px;
                box-sizing: border-box;
                letter-spacing: -0.02em;

                &.logout {
                    margin-left: 8px;
                }
            }
        }
    }
</style>
