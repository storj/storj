// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="auto"
        max-width="450px"
        transition="fade-transition"
        persistent
    >
        <v-card rounded="xlg">
            <v-sheet>
                <v-card-item class="pa-6">
                    <template #prepend>
                        <v-sheet
                            class="border-sm d-flex justify-center align-center text-error"
                            width="40"
                            height="40"
                            rounded="lg"
                        >
                            <component :is="Trash2" :size="18" />
                        </v-sheet>
                    </template>
                    <v-card-title class="font-weight-bold text-error">
                        Delete Account
                    </v-card-title>
                    <template #append>
                        <v-btn
                            icon="$close"
                            variant="text"
                            size="small"
                            color="default"
                            @click="model = false"
                        />
                    </template>
                </v-card-item>
            </v-sheet>

            <v-divider />

            <v-window v-model="step">
                <v-window-item :value="DeleteAccountStep.InitStep">
                    <v-form class="pa-6" @submit.prevent>
                        <v-row>
                            <v-col>
                                <p class="font-weight-bold mb-4">
                                    You are about to permanently delete your Storj account, all of your projects and
                                    all associated data. This action cannot be undone.
                                </p>
                                <p>Account:</p>
                                <v-chip variant="tonal">
                                    {{ user.email }}
                                </v-chip>
                            </v-col>
                        </v-row>
                    </v-form>
                </v-window-item>

                <v-window-item :value="DeleteAccountStep.VerifyPasswordStep">
                    <v-form ref="passwordForm" class="pa-6" @submit.prevent>
                        <v-row>
                            <v-col>
                                <p>Enter your account password to continue.</p>
                                <v-text-field
                                    v-model="password"
                                    type="password"
                                    label="Password"
                                    class="mt-6"
                                    :rules="[RequiredRule]"
                                    required
                                />
                            </v-col>
                        </v-row>
                    </v-form>
                </v-window-item>

                <v-window-item v-if="user.isMFAEnabled" :value="DeleteAccountStep.Verify2faStep">
                    <v-form class="pa-6" @submit.prevent>
                        <v-row>
                            <v-col>
                                <p>Enter the code from your 2FA application.</p>
                                <v-otp-input
                                    ref="otpInput2fa"
                                    :model-value="code2fa"
                                    class="mt-6"
                                    type="number"
                                    maxlength="6"
                                    :error="isOTPInputError"
                                    @update:modelValue="value => onOTPValueChange(value)"
                                />
                            </v-col>
                        </v-row>
                    </v-form>
                </v-window-item>

                <v-window-item :value="DeleteAccountStep.VerifyEmailStep">
                    <v-form class="pa-6" @submit.prevent>
                        <v-row>
                            <v-col>
                                <p>Enter the 6-digit code you received on email.</p>
                                <v-otp-input
                                    ref="otpInputVerify"
                                    :model-value="verifyEmailCode"
                                    class="mt-6"
                                    type="number"
                                    maxlength="6"
                                    :error="isOTPInputError"
                                    @update:modelValue="value => onOTPValueChange(value)"
                                />
                            </v-col>
                        </v-row>
                    </v-form>
                </v-window-item>

                <v-window-item :value="DeleteAccountStep.ConfirmDeleteStep">
                    <v-form class="pa-6" @submit.prevent>
                        <v-row>
                            <v-col>
                                <p>Please confirm that you want to permanently delete your account and all associated data.</p>
                                <v-chip
                                    variant="tonal"
                                    class="my-4 font-weight-bold"
                                >
                                    {{ user.email }}
                                </v-chip>
                                <v-checkbox-btn v-model="isDeleteConfirmed" label="I confirm to delete this account." density="compact" />
                            </v-col>
                        </v-row>
                    </v-form>
                </v-window-item>

                <v-window-item :value="DeleteAccountStep.FinalConfirmDeleteStep">
                    <v-form class="pa-6" @submit.prevent>
                        <v-row>
                            <v-col>
                                <p class="font-weight-bold">This action will delete all of your data and projects within 30 days. You will be logged out immediatelly and receive a confirmation email.</p>
                            </v-col>
                        </v-row>
                    </v-form>
                </v-window-item>
            </v-window>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col v-if="step === DeleteAccountStep.InitStep">
                        <v-btn
                            variant="outlined"
                            color="default"
                            block
                            @click="model = false"
                        >
                            Cancel
                        </v-btn>
                    </v-col>

                    <v-col>
                        <v-btn
                            v-if="step < DeleteAccountStep.FinalConfirmDeleteStep"
                            color="primary"
                            variant="flat"
                            :loading="isLoading"
                            :disabled="step === DeleteAccountStep.ConfirmDeleteStep && !isDeleteConfirmed"
                            block
                            @click="proceed"
                        >
                            Next
                        </v-btn>

                        <v-btn
                            v-if="step === DeleteAccountStep.FinalConfirmDeleteStep"
                            color="error"
                            variant="flat"
                            block
                            @click="proceed"
                        >
                            Delete Account
                        </v-btn>
                    </v-col>
                </v-row>
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
    VCheckboxBtn,
    VChip,
    VCol,
    VDialog,
    VDivider,
    VForm,
    VOtpInput,
    VRow,
    VSheet,
    VTextField,
    VWindow,
    VWindowItem,
} from 'vuetify/components';
import { Trash2 } from 'lucide-vue-next';

import { DeleteAccountStep } from '@/types/accountActions';
import { User } from '@/types/users';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/utils/hooks';
import { useUsersStore } from '@/store/modules/usersStore';
import { RequiredRule } from '@/types/common';

const userStore = useUsersStore();

const model = defineModel<boolean>({ required: true });
const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const step = ref<DeleteAccountStep>(DeleteAccountStep.InitStep);
const password = ref<string>('');
const code2fa = ref<string>('');
const verifyEmailCode = ref<string>('');

const passwordForm = ref<VForm>();
const otpInput2fa = ref<VOtpInput>();
const otpInputVerify = ref<VOtpInput>();
const isDeleteConfirmed = ref<boolean>(false);
const isOTPInputError = ref<boolean>(false);

const user = computed<User>(() => userStore.state.user);

async function proceed(): Promise<void> {
    await withLoading(async () => {
        try {
            switch (step.value) {
            case DeleteAccountStep.InitStep:
                step.value = DeleteAccountStep.VerifyPasswordStep;
                break;
            case DeleteAccountStep.VerifyPasswordStep:
                passwordForm.value?.validate();
                if (!passwordForm.value?.isValid) return;

                await userStore.deleteAccount(DeleteAccountStep.VerifyPasswordStep, password.value);

                if (user.value.isMFAEnabled) {
                    step.value = DeleteAccountStep.Verify2faStep;
                } else {
                    step.value = DeleteAccountStep.VerifyEmailStep;
                }
                break;
            case DeleteAccountStep.Verify2faStep:
                if (code2fa.value.length !== 6) {
                    isOTPInputError.value = true;
                    return;
                }

                await userStore.deleteAccount(DeleteAccountStep.Verify2faStep, code2fa.value.trim());

                step.value = DeleteAccountStep.VerifyEmailStep;
                break;
            case DeleteAccountStep.VerifyEmailStep:
                if (verifyEmailCode.value.length !== 6) {
                    isOTPInputError.value = true;
                    return;
                }

                await userStore.deleteAccount(DeleteAccountStep.VerifyEmailStep, verifyEmailCode.value.trim());

                step.value = DeleteAccountStep.ConfirmDeleteStep;
                break;
            case DeleteAccountStep.ConfirmDeleteStep:
                if (!isDeleteConfirmed.value) return;

                step.value = DeleteAccountStep.FinalConfirmDeleteStep;
                break;
            case DeleteAccountStep.FinalConfirmDeleteStep:
                await userStore.deleteAccount(DeleteAccountStep.ConfirmDeleteStep, '');

                notify.success('Your account has been marked for deletion!');
                model.value = false;

                setTimeout(() => {
                    window.location.href = `${window.location.origin}/login`;
                }, 2000);
            }
        } catch (error) {
            notify.error(error.message);
        }
    });
}

function initialiseOTPInput() {
    setTimeout(() => {
        switch (step.value) {
        case DeleteAccountStep.Verify2faStep:
            otpInput2fa.value?.focus();
            break;
        case DeleteAccountStep.VerifyEmailStep:
            otpInputVerify.value?.focus();
        }
    }, 0);
}

function onKeyUp(event: KeyboardEvent): void {
    if (event.key === 'Enter') proceed();
}

function onOTPValueChange(value: string): void {
    const val = value.slice(0, 6);
    if (isNaN(+val)) {
        return;
    }

    switch (step.value) {
    case DeleteAccountStep.Verify2faStep:
        code2fa.value = val;
        break;
    case DeleteAccountStep.VerifyEmailStep:
        verifyEmailCode.value = val;
    }

    isOTPInputError.value = false;
}

watch(step, val => {
    if (
        val === DeleteAccountStep.Verify2faStep ||
        val === DeleteAccountStep.VerifyEmailStep
    ) {
        initialiseOTPInput();
    }
});

watch(model, val => {
    if (!val) {
        step.value = DeleteAccountStep.InitStep;
        password.value = '';
        code2fa.value = '';
        verifyEmailCode.value = '';
        isDeleteConfirmed.value = false;
        isOTPInputError.value = false;

        document.removeEventListener('keyup', onKeyUp);
    } else {
        document.addEventListener('keyup', onKeyUp);
    }
});

onBeforeUnmount(() => {
    document.removeEventListener('keyup', onKeyUp);
});
</script>
