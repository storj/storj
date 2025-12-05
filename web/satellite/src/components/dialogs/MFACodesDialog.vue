// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        persistent
        width="auto"
        max-width="460px"
        transition="fade-transition"
    >
        <v-card>
            <v-card-item class="pa-6">
                <template #prepend>
                    <img class="d-block" src="@/assets/icon-mfa.svg" alt="MFA">
                </template>
                <v-card-title class="font-weight-bold">Two-Factor Recovery Codes</v-card-title>
                <template #append>
                    <v-btn
                        :icon="X"
                        variant="text"
                        size="small"
                        color="default"
                        :disabled="isLoading"
                        @click="model = false"
                    />
                </template>
            </v-card-item>
            <v-divider />

            <template v-if="isConfirmCode">
                <v-card-item class="px-6 py-4">
                    <p>Enter the authentication code generated in your two-factor application to regenerate recovery codes.</p>
                </v-card-item>
                <v-divider />
                <v-card-item class="px-6 pt-4 pb-0">
                    <v-otp-input
                        v-if="!useRecoveryCode"
                        ref="otpInput"
                        class="pt-2"
                        :model-value="confirmPasscode"
                        :error="isError"
                        :disabled="isLoading"
                        type="number"
                        autofocus
                        maxlength="6"
                        @update:model-value="value => onValueChange(value)"
                    />
                    <v-form v-else v-model="formValid" class="pt-2" @submit.prevent="regenerate">
                        <v-text-field
                            v-model="confirmPasscode"
                            variant="outlined"
                            :rules="recoveryCodeRules"
                            :error-messages="isError ? 'Invalid code. Please re-enter.' : ''"
                            label="Recovery code"
                            :hide-details="false"
                            :maxlength="50"
                            required
                            autofocus
                        />
                    </v-form>
                </v-card-item>
                <v-card-item class="px-6 py-0 text-center">
                    <a class="text-decoration-underline text-cursor-pointer" @click="toggleRecoveryCodeState">
                        {{ useRecoveryCode ? "or use 2FA code" : "or use a recovery code" }}
                    </a>
                </v-card-item>

                <v-divider class="my-4" />
            </template>
            <template v-else>
                <v-card-item class="px-6 py-4">
                    <p>Please save these codes somewhere to be able to recover access to your account.</p>
                </v-card-item>
                <v-divider />
                <v-card-item class="px-6 py-4">
                    <p
                        v-for="(code, index) in userMFARecoveryCodes"
                        :key="index"
                    >
                        {{ code }}
                    </p>
                </v-card-item>
                <v-divider class="mb-4" />
            </template>

            <v-card-actions dense class="px-6 pb-5 pt-0">
                <v-col v-if="!isConfirmCode" class="px-0">
                    <v-btn
                        color="primary"
                        variant="flat"
                        block
                        @click="model = false"
                    >
                        Done
                    </v-btn>
                </v-col>
                <v-col v-if="isConfirmCode" class="pl-0">
                    <v-btn
                        :disabled="isLoading"
                        color="default"
                        variant="outlined"
                        block
                        @click="model = false"
                    >
                        Cancel
                    </v-btn>
                </v-col>
                <v-col v-if="isConfirmCode" class="pr-0">
                    <v-btn
                        :loading="isLoading"
                        :disabled="!formValid"
                        color="primary"
                        variant="flat"
                        block
                        @click="regenerate"
                    >
                        Regenerate
                    </v-btn>
                </v-col>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import {
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardTitle,
    VCol,
    VDialog,
    VDivider,
    VForm,
    VOtpInput,
    VTextField,
} from 'vuetify/components';
import { X } from 'lucide-vue-next';

import { useUsersStore } from '@/store/modules/usersStore';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

const usersStore = useUsersStore();
const notify = useNotify();
const { withLoading, isLoading } = useLoading();

const model = defineModel<boolean>({ required: true });

const otpInput = ref<VOtpInput>();

const confirmPasscode = ref<string>('');
const isError = ref<boolean>(false);
const formValid = ref<boolean>(false);
const isConfirmCode = ref(true);
const useRecoveryCode = ref<boolean>(false);

const recoveryCodeRules = [ (value: string) => (!!value || 'Can\'t be empty') ];

/**
 * Returns user MFA recovery codes from store.
 */
const userMFARecoveryCodes = computed((): string[] => {
    return usersStore.state.userMFARecoveryCodes;
});

/**
 * Toggles whether the MFA recovery code input is shown.
 */
function toggleRecoveryCodeState(): void {
    isError.value = false;
    confirmPasscode.value = '';
    useRecoveryCode.value = !useRecoveryCode.value;
    if (useRecoveryCode.value) {
        cleanUpOTPInput();
    } else {
        initialiseOTPInput();
    }
}

function onValueChange(value: string) {
    if (!useRecoveryCode.value) {
        const val = value.slice(0, 6);
        if (isNaN(+val)) {
            confirmPasscode.value = '';
            return;
        }
        confirmPasscode.value = val;
        formValid.value = val.length === 6;
    }
    isError.value = false;
}

/**
 * Regenerates user MFA codes and sets view to Recovery Codes state.
 */
function regenerate(): void {
    if (!confirmPasscode.value || isError.value || !formValid.value) return;

    withLoading(async () => {
        try {
            const code = useRecoveryCode.value ? { recoveryCode: confirmPasscode.value } : { passcode: confirmPasscode.value };
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

function initialiseOTPInput() {
    setTimeout(() => {
        otpInput.value?.focus();
    }, 0);

    document.addEventListener('keyup', onKeyUp);
}

function cleanUpOTPInput() {
    document.removeEventListener('keyup', onKeyUp);
}

function onKeyUp(event: KeyboardEvent) {
    if (event.key === 'Enter' && otpInput.value?.isFocused) {
        regenerate();
    }
}

watch(confirmPasscode, () => {
    isError.value = false;
});

watch(model, shown => {
    if (shown) {
        initialiseOTPInput();
        return;
    }
    isConfirmCode.value = true;
    useRecoveryCode.value = false;
    confirmPasscode.value = '';
    isError.value = false;
    cleanUpOTPInput();
});

onBeforeUnmount(() => {
    cleanUpOTPInput();
});
</script>
