// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="() => withLoading(onClose)">
        <template #content>
            <div class="timeout-modal">
                <div class="timeout-modal__header">
                    <Icon class="timeout-modal__header__icon" />
                    <h1 class="timeout-modal__header__title">
                        Session Timeout
                    </h1>
                </div>

                <div class="timeout-modal__divider" />

                <p class="timeout-modal__info">Select your session timeout duration.</p>

                <div class="timeout-modal__divider" />

                <div>
                    <p class="timeout-modal__label">Session timeout duration</p>
                    <timeout-selector :selected="sessionDuration" @select="durationChange" />
                </div>

                <div class="timeout-modal__divider" />

                <div class="timeout-modal__buttons">
                    <VButton
                        label="Cancel"
                        width="100%"
                        border-radius="10px"
                        font-size="13px"
                        is-white
                        class="timeout-modal__buttons__button cancel"
                        :on-press="() => withLoading(onClose)"
                        :is-disabled="isLoading"
                    />
                    <VButton
                        label="Save"
                        width="100%"
                        border-radius="10px"
                        font-size="13px"
                        class="timeout-modal__buttons__button"
                        :on-press="() => withLoading(save)"
                        :is-disabled="isLoading || !hasChanged"
                    />
                </div>
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';

import { useNotify } from '@/utils/hooks';
import { Duration } from '@/utils/time';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useUsersStore } from '@/store/modules/usersStore';
import { useAppStore } from '@/store/modules/appStore';
import { useLoading } from '@/composables/useLoading';

import VButton from '@/components/common/VButton.vue';
import VModal from '@/components/common/VModal.vue';
import TimeoutSelector from '@/components/modals/editSessionTimeout/TimeoutSelector.vue';

import Icon from '@/../static/images/session/inactivityTimer.svg';

const appStore = useAppStore();
const usersStore = useUsersStore();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const sessionDuration = ref<Duration | null>(null);

/**
 * Lifecycle hook after initial render.
 * Make the current selected duration the already configured one.
 */
onMounted(() => {
    sessionDuration.value = userDuration.value;
});

/**
 * Returns duration from store.
 */
const userDuration = computed((): Duration | null => {
    return usersStore.state.settings.sessionDuration;
});

/**
 * Whether the user has changed this setting.
 */
const hasChanged = computed((): boolean => {
    if (!sessionDuration.value) {
        return false;
    }
    return !userDuration.value?.isEqualTo(sessionDuration.value as Duration);
});

/**
* durationChange is called when the user selects a different duration.
* @param duration the user's selection.
* */
function durationChange(duration: Duration) {
    sessionDuration.value = duration;
}

/**
* save submits the changed duration.
* */
async function save() {
    isLoading.value = true;
    try {
        await usersStore.updateSettings({ sessionDuration: sessionDuration.value?.nanoseconds ?? 0 });
        notify.success(`Session timeout changed successfully. Your session timeout is ${sessionDuration.value?.shortString}.`);
        onClose();
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.EDIT_TIMEOUT_MODAL);
    } finally {
        isLoading.value = false;
    }
}

/**
 * onClose is called to close this modal.
 * */
function onClose(): void {
    appStore.removeActiveModal();
}
</script>

<style scoped lang="scss">
.timeout-modal {
    width: calc(100vw - 48px);
    max-width: 410px;
    padding: 32px;
    display: flex;
    flex-direction: column;
    gap: 16px;
    box-sizing: border-box;
    font-family: 'font_regular', sans-serif;
    text-align: left;

    @media screen and (width <= 400px) {
        width: 100vw;
    }

    &__header {
        display: flex;
        align-items: center;
        gap: 20px;

        &__icon {
            height: 40px;
            width: 40px;
            flex-shrink: 0;
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 24px;
            line-height: 31px;
        }
    }

    &__divider {
        height: 1px;
        background-color: var(--c-grey-2);
    }

    &__info {
        font-family: 'font_regular', sans-serif;
        font-size: 14px;
        line-height: 20px;
    }

    &__label {
        margin-bottom: 4px;
        font-family: 'font_medium', sans-serif;
        font-size: 14px;
        line-height: 20px;
        color: var(--c-blue-6);
    }

    &__buttons {
        display: flex;
        gap: 16px;

        @media screen and (width <= 500px) {
            flex-direction: column-reverse;
        }

        &__button {
            padding: 16px;
            box-sizing: border-box;

            &.cancel {
                box-shadow: 0 0 20px rgb(0 0 0 / 4%);

                :deep(.label) {
                    color: var(--c-black) !important;
                }
            }
        }
    }
}
</style>
