// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="withLoading(onClose)">
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

                <p class="timeout-modal__label">Session timeout duration</p>
                <timeout-selector :selected="sessionDuration" @select="durationChange" />

                <div class="timeout-modal__divider" />

                <div class="timeout-modal__buttons">
                    <VButton
                        label="Cancel"
                        width="100%"
                        height="40px"
                        border-radius="10px"
                        font-size="13px"
                        is-white
                        class="timeout-modal__buttons__button"
                        :on-press="withLoading(onClose)"
                        :is-disabled="isLoading"
                    />
                    <VButton
                        label="Save"
                        width="100%"
                        height="40px"
                        border-radius="10px"
                        font-size="13px"
                        class="timeout-modal__buttons__button save"
                        :on-press="withLoading(save)"
                        :is-disabled="isLoading || !hasChanged"
                    />
                </div>
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';

import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { useNotify, useStore } from '@/utils/hooks';
import { Duration } from '@/utils/time';
import { USER_ACTIONS } from '@/store/modules/users';
import { SetUserSettingsData } from '@/types/users';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

import VButton from '@/components/common/VButton.vue';
import VModal from '@/components/common/VModal.vue';
import TimeoutSelector from '@/components/modals/editSessionTimeout/TimeoutSelector.vue';

import Icon from '@/../static/images/session/inactivityTimer.svg';

const store = useStore();
const notify = useNotify();

const isLoading = ref(false);
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
    return store.state.usersModule.settings.sessionDuration;
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
        await store.dispatch(USER_ACTIONS.UPDATE_SETTINGS, {
            sessionDuration: sessionDuration.value?.nanoseconds ?? 0,
        } as SetUserSettingsData);
        notify.success(`Session timeout changed successfully. Your session timeout is ${sessionDuration.value?.shortString}.`);
        onClose();
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.EDIT_TIMEOUT_MODAL);
    } finally {
        isLoading.value = false;
    }
}

/**
 * onClose is called to close this modal.
 * */
async function onClose() {
    store.commit(APP_STATE_MUTATIONS.REMOVE_ACTIVE_MODAL);
}

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
</script>

<style scoped lang="scss">
.timeout-modal {
    padding: 32px;
    box-sizing: border-box;
    font-family: 'font_regular', sans-serif;
    text-align: left;

    &__header {
        display: flex;
        align-items: center;
        gap: 20px;
        margin-bottom: 20px;

        @media screen and (max-width: 500px) {
            flex-direction: column;
            align-items: flex-start;
            gap: 10px;
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 28px;
            line-height: 36px;
        }
    }

    &__divider {
        margin: 20px 0;
        border: 1px solid var(--c-grey-2);
    }

    &__info {
        font-family: 'font_regular', sans-serif;
        font-size: 16px;
        line-height: 24px;
    }

    &__label {
        font-family: 'font_regular', sans-serif;
        font-size: 14px;
        line-height: 24px;
        margin-bottom: 10px;
    }

    &__buttons {
        display: flex;
        gap: 16px;

        @media screen and (max-width: 500px) {
            flex-direction: column-reverse;
        }

        &__button {
            padding: 16px;
            box-sizing: border-box;
        }
    }
}
</style>
