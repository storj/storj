// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="recovery">
                <h1 class="recovery__title">Two-Factor Authentication</h1>
                <p v-if="isConfirmCode" class="recovery__subtitle">
                    Enter code from your favorite TOTP app to regenerate 2FA codes.
                </p>

                <div v-if="isConfirmCode" class="recovery__confirm">
                    <div class="recovery__confirm">
                        <h2 class="recovery__confirm__title">Confirm Authentication Code</h2>
                        <ConfirmMFAInput ref="mfaInput" :on-input="onConfirmInput" :is-error="isError" :is-recovery="isRecoveryCodeState" />
                        <span class="recovery__confirm__toggle" @click="toggleRecoveryCodeState">
                            Or use {{ isRecoveryCodeState ? '2FA code' : 'recovery code' }}
                        </span>
                    </div>

                    <div class="recovery__confirm__buttons">
                        <VButton
                            label="Cancel"
                            width="100%"
                            height="44px"
                            :is-white="true"
                            :on-press="closeModal"
                        />
                        <VButton
                            label="Regenerate"
                            width="100%"
                            height="44px"
                            :on-press="regenerate"
                            :is-disabled="!confirmPasscode || isLoading"
                        />
                    </div>
                </div>

                <template v-else>
                    <div class="recovery__codes">
                        <p class="recovery__codes__subtitle">
                            Please save these codes somewhere to be able to recover access to your account.
                        </p>
                        <p
                            v-for="(code, index) in userMFARecoveryCodes"
                            :key="index"
                        >
                            {{ code }}
                        </p>
                    </div>
                    <VButton
                        class="recovery__done-button"
                        label="Done"
                        width="100%"
                        height="44px"
                        :on-press="closeModal"
                    />
                </template>
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';

import { useUsersStore } from '@/store/modules/usersStore';
import { useAppStore } from '@/store/modules/appStore';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

import VButton from '@/components/common/VButton.vue';
import VModal from '@/components/common/VModal.vue';
import ConfirmMFAInput from '@/components/account/mfa/ConfirmMFAInput.vue';

interface ClearInput {
  clearInput(): void;
}

const usersStore = useUsersStore();
const appStore = useAppStore();
const notify = useNotify();
const { withLoading, isLoading } = useLoading();

const isConfirmCode = ref(true);
const confirmPasscode = ref<string>('');
const isError = ref<boolean>(false);
const isRecoveryCodeState = ref<boolean>(false);

const mfaInput = ref<ClearInput>();

/**
 * Returns MFA recovery codes from store.
 */
const userMFARecoveryCodes = computed((): string[] => {
    return usersStore.state.userMFARecoveryCodes;
});

/**
 * Closes modal.
 */
function closeModal(): void {
    appStore.removeActiveModal();
}

/**
 * Sets confirmation passcode value from input.
 */
function onConfirmInput(value: string): void {
    isError.value = false;
    confirmPasscode.value = value;
}

/**
 * Toggles whether the MFA recovery code input is shown.
 */
function toggleRecoveryCodeState(): void {
    isError.value = false;
    confirmPasscode.value = '';
    mfaInput.value?.clearInput();
    isRecoveryCodeState.value = !isRecoveryCodeState.value;
}

/**
 * Regenerates user MFA codes and sets view to Recovery Codes state.
 */
function regenerate(): void {
    if (!confirmPasscode.value || isLoading.value || isError.value) return;

    withLoading(async () => {
        try {
            const code = isRecoveryCodeState.value ? { recoveryCode: confirmPasscode.value } : { passcode: confirmPasscode.value };
            await usersStore.regenerateUserMFARecoveryCodes(code);
            isConfirmCode.value = false;
            confirmPasscode.value = '';

            notify.success('MFA codes were regenerated successfully');
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.MFA_CODES_MODAL);
            isError.value = true;
        }
    });
}
</script>

<style scoped lang="scss">
    .recovery {
        padding: 60px;
        background: #fff;
        border-radius: 6px;
        display: flex;
        flex-direction: column;
        align-items: center;
        font-family: 'font_regular', sans-serif;

        @media screen and (width <= 550px) {
            padding: 48px 24px;
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 28px;
            line-height: 34px;
            text-align: center;
            color: #000;
            margin: 0 0 30px;

            @media screen and (width <= 550px) {
                font-size: 24px;
                line-height: 28px;
                margin-bottom: 15px;
            }
        }

        &__subtitle {
            font-size: 16px;
            line-height: 21px;
            text-align: center;
            color: #000;
            margin: 0 0 45px;

            @media screen and (width <= 550px) {
                font-size: 14px;
                line-height: 18px;
                margin-bottom: 20px;
            }
        }

        &__confirm {
            padding: 25px;
            background: #f5f6fa;
            border-radius: 6px;
            width: calc(100% - 50px);
            display: flex;
            flex-direction: column;
            align-items: center;

            &__title {
                font-size: 16px;
                line-height: 19px;
                text-align: center;
                color: #000;
                margin-bottom: 20px;
            }

            &__toggle {
                font-size: 16px;
                color: #0068dc;
                cursor: pointer;
                margin-top: 20px;
                text-align: center;
            }

            &__buttons {
                display: flex;
                align-items: center;
                width: 100%;
                margin-top: 30px;
                column-gap: 20px;

                @media screen and (width <= 550px) {
                    flex-direction: column-reverse;
                    column-gap: unset;
                    row-gap: 10px;
                    margin-top: 20px;
                }
            }
        }

        &__codes {
            padding: 25px;
            background: #f5f6fa;
            border-radius: 6px;
            width: calc(100% - 50px);
            display: flex;
            flex-direction: column;
            align-items: center;

            &__subtitle {
                font-size: 16px;
                line-height: 21px;
                text-align: center;
                color: #000;
                margin: 0 0 30px;
                max-width: 485px;

                @media screen and (width <= 550px) {
                    font-size: 14px;
                    line-height: 18px;
                    margin-bottom: 15px;
                }
            }
        }

        &__done-button {
            margin-top: 30px;
        }
    }
</style>
