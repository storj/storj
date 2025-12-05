// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        :persistent="isLoading"
        width="auto"
        max-width="460px"
        transition="fade-transition"
    >
        <v-card ref="innerContent">
            <v-card-item class="pa-6">
                <template #prepend>
                    <v-sheet
                        class="border-sm d-flex justify-center align-center"
                        width="40"
                        height="40"
                        rounded="lg"
                    >
                        <component :is="RectangleEllipsis" :size="18" />
                    </v-sheet>
                </template>
                <v-card-title class="font-weight-bold">Disable Two-Factor</v-card-title>
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
            <v-card-item class="px-6 py-4">
                <p>Enter the authentication code generated in your two-factor application to disable 2FA.</p>
            </v-card-item>
            <v-divider />
            <v-card-item class="px-6 pt-4 pb-0">
                <v-otp-input
                    v-if="!useRecoveryCode"
                    ref="otpInput"
                    class="pt-2"
                    :model-value="confirmCode"
                    :error="isError"
                    :disabled="isLoading"
                    type="number"
                    autofocus
                    maxlength="6"
                    @update:model-value="value => onValueChange(value)"
                />
                <v-form v-else v-model="formValid" class="pt-2" @submit.prevent="disable">
                    <v-text-field
                        v-model="confirmCode"
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
            <v-card-actions dense class="px-6 pb-5 pt-0">
                <v-col class="pl-0">
                    <v-btn
                        variant="outlined"
                        color="default"
                        block
                        :disabled="isLoading"
                        @click="model = false"
                    >
                        Cancel
                    </v-btn>
                </v-col>
                <v-col class="px-0">
                    <v-btn
                        color="primary"
                        variant="flat"
                        block
                        :loading="isLoading"
                        :disabled="!formValid"
                        @click="disable"
                    >
                        Disable
                    </v-btn>
                </v-col>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue';
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
    VSheet,
} from 'vuetify/components';
import { RectangleEllipsis, X } from 'lucide-vue-next';

import { useLoading } from '@/composables/useLoading';
import { useUsersStore } from '@/store/modules/usersStore';
import { DisableMFARequest } from '@/types/users';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/composables/useNotify';

const usersStore = useUsersStore();
const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const innerContent = ref<VCard | null>(null);

const model = defineModel<boolean>({ required: true });

const otpInput = ref<VOtpInput>();

const confirmCode = ref<string>('');
const isError = ref<boolean>(false);
const useRecoveryCode = ref<boolean>(false);
const formValid = ref<boolean>(false);

const recoveryCodeRules = [ (value: string) => (!!value || 'Can\'t be empty') ];

function onValueChange(value: string) {
    if (!useRecoveryCode.value) {
        const val = value.slice(0, 6);
        if (isNaN(+val)) {
            confirmCode.value = '';
            return;
        }
        confirmCode.value = val;
        formValid.value = val.length === 6;
    }
    isError.value = false;
}

/**
 * Toggles whether the MFA recovery code input is shown.
 */
function toggleRecoveryCodeState(): void {
    isError.value = false;
    confirmCode.value = '';
    useRecoveryCode.value = !useRecoveryCode.value;
    if (useRecoveryCode.value) {
        cleanUpOTPInput();
    } else {
        initialiseOTPInput();
    }
}

/**
 * Disables user MFA.
 */
function disable(): void {
    if (!formValid.value) return;

    const request = new DisableMFARequest();
    if (useRecoveryCode.value) {
        request.recoveryCode = confirmCode.value;
    } else {
        request.passcode = confirmCode.value;
    }

    withLoading(async () => {
        try {
            await usersStore.disableUserMFA(request);
            await usersStore.getUser();

            notify.success('MFA was disabled successfully');
            model.value = false;
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.DISABLE_MFA_MODAL);
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
        disable();
    }
}

watch(confirmCode, () => {
    isError.value = false;
});

watch(innerContent, newContent => {
    if (newContent) {
        initialiseOTPInput();
        return;
    }
    // dialog has been closed
    isError.value = false;
    confirmCode.value = '';
    useRecoveryCode.value = false;
    cleanUpOTPInput();
});

onBeforeUnmount(() => {
    cleanUpOTPInput();
});
</script>
