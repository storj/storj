// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="auto"
        max-width="450px"
        transition="fade-transition"
        persistent
        scrollable
    >
        <v-card rounded="xlg">
            <v-sheet>
                <v-card-item class="pa-6">
                    <template #prepend>
                        <v-sheet
                            class="border-sm d-flex justify-center align-center"
                            width="40"
                            height="40"
                            rounded="lg"
                        >
                            <component :is="MailPlus" :size="18" />
                        </v-sheet>
                    </template>
                    <v-card-title class="font-weight-bold">
                        Change Email
                    </v-card-title>
                    <template #append>
                        <v-btn
                            :icon="X"
                            variant="text"
                            size="small"
                            color="default"
                            @click="model = false"
                        />
                    </template>
                </v-card-item>
            </v-sheet>

            <v-divider />

            <v-card-text class="pa-0">
                <v-window v-model="step" :touch="false">
                    <v-window-item :value="ChangeEmailStep.InitStep">
                        <v-form class="pa-6" @submit.prevent>
                            <v-row>
                                <v-col>
                                    <p class="mb-4">You are about to change your email address associated with your {{ configStore.brandName }} account.</p>
                                    <p>Account:</p>
                                    <v-chip
                                        variant="tonal"
                                        class="font-weight-bold mt-2"
                                    >
                                        {{ user.email }}
                                    </v-chip>
                                </v-col>
                            </v-row>
                        </v-form>
                    </v-window-item>

                    <v-window-item :value="ChangeEmailStep.VerifyPasswordStep">
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
                                        autofocus
                                        required
                                    />
                                </v-col>
                            </v-row>
                        </v-form>
                    </v-window-item>

                    <v-window-item :value="ChangeEmailStep.Verify2faStep">
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
                                        @update:model-value="value => onOTPValueChange(value)"
                                    />
                                </v-col>
                            </v-row>
                        </v-form>
                    </v-window-item>

                    <v-window-item :value="ChangeEmailStep.VerifyOldEmailStep">
                        <v-form class="pa-6" @submit.prevent>
                            <v-row>
                                <v-col>
                                    <p>Enter the code you received on your old email.</p>
                                    <v-otp-input
                                        ref="otpInputVerifyOld"
                                        :model-value="codeVerifyOld"
                                        class="mt-6"
                                        type="number"
                                        maxlength="6"
                                        :error="isOTPInputError"
                                        @update:model-value="value => onOTPValueChange(value)"
                                    />
                                </v-col>
                            </v-row>
                        </v-form>
                    </v-window-item>

                    <v-window-item :value="ChangeEmailStep.SetNewEmailStep">
                        <v-form ref="newEmailForm" class="pa-6" @submit.prevent>
                            <v-row>
                                <v-col>
                                    <p>Enter your new email address.</p>
                                    <v-text-field
                                        v-model="newEmail"
                                        type="email"
                                        label="Email"
                                        class="mt-6"
                                        :rules="[RequiredRule, EmailRule]"
                                        autofocus
                                        required
                                    />
                                </v-col>
                            </v-row>
                        </v-form>
                    </v-window-item>

                    <v-window-item :value="ChangeEmailStep.VerifyNewEmailStep">
                        <v-form class="pa-6" @submit.prevent>
                            <v-row>
                                <v-col>
                                    <p>Enter the code you received on your new email.</p>
                                    <v-otp-input
                                        ref="otpInputVerifyNew"
                                        class="mt-6"
                                        :model-value="codeVerifyNew"
                                        type="number"
                                        maxlength="6"
                                        :error="isOTPInputError"
                                        @update:model-value="value => onOTPValueChange(value)"
                                    />
                                </v-col>
                            </v-row>
                        </v-form>
                    </v-window-item>

                    <v-window-item :value="ChangeEmailStep.SuccessStep">
                        <v-form class="pa-6" @submit.prevent>
                            <v-row>
                                <v-col>
                                    <p>Your email has been successfully updated.</p>
                                </v-col>
                            </v-row>
                        </v-form>
                    </v-window-item>
                </v-window>
            </v-card-text>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col v-if="step === ChangeEmailStep.InitStep">
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
                            v-if="step < ChangeEmailStep.SuccessStep"
                            color="primary"
                            variant="flat"
                            :loading="isLoading"
                            block
                            @click="proceed"
                        >
                            Next
                        </v-btn>

                        <v-btn
                            v-if="step === ChangeEmailStep.SuccessStep"
                            color="primary"
                            variant="flat"
                            block
                            @click="model = false"
                        >
                            Finish
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
    VCardText,
    VCardTitle,
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
import { MailPlus, X } from 'lucide-vue-next';

import { ChangeEmailStep } from '@/types/accountActions';
import { User } from '@/types/users';
import { EmailRule, RequiredRule } from '@/types/common';
import { useUsersStore } from '@/store/modules/usersStore';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useConfigStore } from '@/store/modules/configStore';

const userStore = useUsersStore();
const configStore = useConfigStore();

const model = defineModel<boolean>({ required: true });
const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const step = ref<ChangeEmailStep>(ChangeEmailStep.InitStep);
const password = ref<string>('');
const code2fa = ref<string>('');
const newEmail = ref<string>('');
const codeVerifyOld = ref<string>('');
const codeVerifyNew = ref<string>('');

const passwordForm = ref<VForm>();
const newEmailForm = ref<VForm>();
const otpInput2fa = ref<VOtpInput>();
const otpInputVerifyOld = ref<VOtpInput>();
const otpInputVerifyNew = ref<VOtpInput>();
const isOTPInputError = ref<boolean>(false);

const user = computed<User>(() => userStore.state.user);

async function proceed(): Promise<void> {
    await withLoading(async () => {
        try {
            switch (step.value) {
            case ChangeEmailStep.InitStep:
                step.value = ChangeEmailStep.VerifyPasswordStep;
                break;
            case ChangeEmailStep.VerifyPasswordStep:
                passwordForm.value?.validate();
                if (!passwordForm.value?.isValid) return;

                await userStore.changeEmail(ChangeEmailStep.VerifyPasswordStep, password.value);

                if (user.value.isMFAEnabled) {
                    step.value = ChangeEmailStep.Verify2faStep;
                } else {
                    step.value = ChangeEmailStep.VerifyOldEmailStep;
                }
                break;
            case ChangeEmailStep.Verify2faStep:
                if (code2fa.value.length !== 6) {
                    isOTPInputError.value = true;
                    return;
                }

                await userStore.changeEmail(ChangeEmailStep.Verify2faStep, code2fa.value.trim());

                step.value = ChangeEmailStep.VerifyOldEmailStep;
                break;
            case ChangeEmailStep.VerifyOldEmailStep:
                if (codeVerifyOld.value.length !== 6) {
                    isOTPInputError.value = true;
                    return;
                }

                await userStore.changeEmail(ChangeEmailStep.VerifyOldEmailStep, codeVerifyOld.value.trim());

                step.value = ChangeEmailStep.SetNewEmailStep;
                break;
            case ChangeEmailStep.SetNewEmailStep:
                newEmailForm.value?.validate();
                if (!newEmailForm.value?.isValid) return;

                await userStore.changeEmail(ChangeEmailStep.SetNewEmailStep, newEmail.value.trim());

                step.value = ChangeEmailStep.VerifyNewEmailStep;
                break;
            case ChangeEmailStep.VerifyNewEmailStep:
                if (codeVerifyNew.value.length !== 6) {
                    isOTPInputError.value = true;
                    return;
                }

                await userStore.changeEmail(ChangeEmailStep.VerifyNewEmailStep, codeVerifyNew.value.trim());
                await userStore.getUser();

                step.value = ChangeEmailStep.SuccessStep;

                notify.success('Your email has been successfully updated!');
                break;
            case ChangeEmailStep.SuccessStep:
                model.value = false;
            }
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.CHANGE_EMAIL_DIALOG);
        }
    });
}

function initialiseOTPInput() {
    setTimeout(() => {
        switch (step.value) {
        case ChangeEmailStep.Verify2faStep:
            otpInput2fa.value?.focus();
            break;
        case ChangeEmailStep.VerifyOldEmailStep:
            otpInputVerifyOld.value?.focus();
            break;
        case ChangeEmailStep.VerifyNewEmailStep:
            otpInputVerifyNew.value?.focus();
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
    case ChangeEmailStep.Verify2faStep:
        code2fa.value = val;
        break;
    case ChangeEmailStep.VerifyOldEmailStep:
        codeVerifyOld.value = val;
        break;
    case ChangeEmailStep.VerifyNewEmailStep:
        codeVerifyNew.value = val;
    }

    isOTPInputError.value = false;
}

watch(step, val => {
    if (
        val === ChangeEmailStep.Verify2faStep ||
        val === ChangeEmailStep.VerifyOldEmailStep ||
        val === ChangeEmailStep.VerifyNewEmailStep
    ) {
        initialiseOTPInput();
    }
});

watch(model, val => {
    if (!val) {
        step.value = ChangeEmailStep.InitStep;
        password.value = '';
        code2fa.value = '';
        codeVerifyOld.value = '';
        codeVerifyNew.value = '';
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
