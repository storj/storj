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
                    <v-window-item :value="DeleteAccountStep.InitStep">
                        <v-form class="pa-6" @submit.prevent>
                            <v-row>
                                <v-col>
                                    <p class="font-weight-bold mb-4">
                                        You are about to permanently delete your {{ configStore.brandName }} account, all of your projects and
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

                    <v-window-item :value="DeleteAccountStep.LockEnabledBucketsStep">
                        <div class="pa-6">
                            <v-alert variant="tonal" type="warning">
                                You have {{ buckets }} bucket{{ buckets > 1 ? 's' : '' }} with Object Lock enabled.
                                By proceeding, you will lose any objects in {{ buckets > 1 ? 'these' : 'this' }}
                                bucket{{ buckets > 1 ? 's' : '' }} even if they are locked.
                            </v-alert>
                        </div>
                    </v-window-item>

                    <v-window-item :value="DeleteAccountStep.DeleteBucketsStep">
                        <v-form class="pa-6">
                            <v-row>
                                <v-col>
                                    <p class="font-weight-bold mb-4">
                                        Before we proceed with your account deletion request,
                                        please delete all of your data and buckets.
                                    </p>
                                    <p class="font-weight-bold mb-4">Projects: <v-chip color="error">{{ ownedProjects }}</v-chip></p>
                                    <p class="font-weight-bold mb-4">Total buckets: <v-chip color="error">{{ buckets }}</v-chip></p>
                                    <v-alert variant="tonal" type="info">
                                        Once you delete all of your buckets, then you can proceed with account deletion.
                                    </v-alert>
                                </v-col>
                            </v-row>
                        </v-form>
                    </v-window-item>

                    <v-window-item :value="DeleteAccountStep.DeleteAccessKeysStep">
                        <v-form class="pa-6">
                            <v-row>
                                <v-col>
                                    <p class="font-weight-bold mb-4">
                                        Before we proceed with your account deletion request,
                                        please delete all of your access keys:
                                    </p>
                                    <p class="font-weight-bold mb-4">Projects: <v-chip color="error">{{ ownedProjects }}</v-chip></p>
                                    <p class="font-weight-bold mb-4">Total access keys: <v-chip color="error">{{ apiKeys }}</v-chip></p>
                                    <v-alert variant="tonal" type="info">
                                        Once you delete all of your access keys, then you can proceed with account deletion.
                                    </v-alert>
                                </v-col>
                            </v-row>
                        </v-form>
                    </v-window-item>

                    <v-window-item :value="DeleteAccountStep.PayInvoicesStep">
                        <v-form class="pa-6">
                            <v-row>
                                <v-col>
                                    <p class="font-weight-bold mb-4">
                                        Before we proceed with your account deletion request,
                                        please review the following information:
                                    </p>
                                    <p class="font-weight-bold mb-4">Invoice status: <v-chip color="error">{{ unpaidInvoices }} unpaid {{ unpaidInvoices > 1 ? 'invoices' : 'invoice' }}</v-chip></p>
                                    <p class="font-weight-bold mb-4">Unpaid invoices amount: <v-chip color="error">{{ centsToDollars(amountOwed) }}</v-chip></p>
                                    <v-alert variant="tonal" type="error">
                                        Please pay all of your outstanding invoices by adding a new payment method to delete your account.
                                    </v-alert>
                                    <!-- <v-btn class="mt-4" color="error" block @click="goToBilling()">Pay Invoices</v-btn> -->
                                </v-col>
                            </v-row>
                        </v-form>
                    </v-window-item>

                    <v-window-item :value="DeleteAccountStep.WaitForInvoicingStep">
                        <v-form class="pa-6">
                            <v-row>
                                <v-col>
                                    <p class="font-weight-bold mb-4">
                                        There's some recent usage on your account that hasn't been paid yet. To delete your account, please
                                        follow these steps:
                                    </p>
                                    <p class="mb-4">1. Please wait until the end of the current billing cycle (typically the end of the month).</p>
                                    <p class="mb-4">2. We'll generate your final invoice early in the following month (usually around the 4th day).</p>
                                    <p class="mb-4">3. Once the invoice is paid, you can resume the account deletion process.</p>
                                    <v-alert variant="tonal" type="info">
                                        If you have an outstanding balance or need immediate assistance, please contact our support team to
                                        request account deletion and discuss any potential refunds.
                                    </v-alert>
                                    <!-- <v-btn class="mt-4" block @click="model = false">Close</v-btn> -->
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
                                        @update:model-value="value => onOTPValueChange(value)"
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
                                        @update:model-value="value => onOTPValueChange(value)"
                                    />
                                </v-col>
                            </v-row>
                        </v-form>
                    </v-window-item>

                    <v-window-item :value="DeleteAccountStep.ConfirmDeleteStep">
                        <v-form class="pa-6" @submit.prevent>
                            <v-row>
                                <v-col>
                                    <p>Please confirm that you want to permanently delete your account.</p>
                                    <v-chip
                                        variant="tonal"
                                        class="my-4 font-weight-bold"
                                    >
                                        {{ user.email }}
                                    </v-chip>
                                    <v-checkbox-btn v-model="isDeleteConfirmed" label="I want to delete this account." density="compact" />
                                </v-col>
                            </v-row>
                        </v-form>
                    </v-window-item>

                    <v-window-item :value="DeleteAccountStep.FinalConfirmDeleteStep">
                        <v-form class="pa-6" @submit.prevent>
                            <v-row>
                                <v-col>
                                    <p class="font-weight-bold">This action will delete your account. You will be logged out immediately and receive a confirmation email.</p>
                                </v-col>
                            </v-row>
                        </v-form>
                    </v-window-item>
                </v-window>
            </v-card-text>

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
                            v-if="step === DeleteAccountStep.DeleteBucketsStep || step === DeleteAccountStep.DeleteAccessKeysStep"
                            variant="flat"
                            block
                            @click="goToProjects"
                        >
                            Go to projects
                        </v-btn>
                        <v-btn
                            v-else-if="step === DeleteAccountStep.LockEnabledBucketsStep"
                            color="warning"
                            variant="flat"
                            :loading="isLoading"
                            block
                            @click="proceed(true)"
                        >
                            Next
                        </v-btn>
                        <v-btn
                            v-else-if="step === DeleteAccountStep.PayInvoicesStep"
                            color="error"
                            variant="flat"
                            block
                            @click="goToBilling"
                        >
                            Pay invoices
                        </v-btn>
                        <v-btn
                            v-else-if="step === DeleteAccountStep.WaitForInvoicingStep"
                            variant="flat"
                            block
                            @click="model = false"
                        >
                            Close
                        </v-btn>
                        <v-btn
                            v-else-if="step < DeleteAccountStep.FinalConfirmDeleteStep"
                            color="primary"
                            variant="flat"
                            :loading="isLoading"
                            :disabled="step === DeleteAccountStep.ConfirmDeleteStep && !isDeleteConfirmed"
                            block
                            @click="proceed()"
                        >
                            Next
                        </v-btn>

                        <v-btn
                            v-if="step === DeleteAccountStep.FinalConfirmDeleteStep"
                            :loading="isLoading"
                            color="error"
                            variant="flat"
                            block
                            @click="proceed()"
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
import { useRouter } from 'vue-router';
import {
    VAlert,
    VBtn,
    VCard,
    VCardActions,
    VCardItem,
    VCardText,
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
import { Trash2, X } from 'lucide-vue-next';

import { centsToDollars } from '@/utils/strings';
import { DeleteAccountStep, SKIP_OBJECT_LOCK_ENABLED_BUCKETS } from '@/types/accountActions';
import { AccountDeletionData, User } from '@/types/users';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { ROUTES } from '@/router';
import { useUsersStore } from '@/store/modules/usersStore';
import { RequiredRule } from '@/types/common';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useConfigStore } from '@/store/modules/configStore';

const router = useRouter();

const deleteResp = ref<AccountDeletionData | null>(null);

const userStore = useUsersStore();
const configStore = useConfigStore();

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

const ownedProjects = ref<number>(0);
const buckets = ref<number>(0);
const apiKeys = ref<number>(0);
const unpaidInvoices = ref<number>(0);
const amountOwed = ref<number>(0);

const user = computed<User>(() => userStore.state.user);

function chooseRestrictionStep(deleteResp: AccountDeletionData) {
    switch (true) {
    case deleteResp.buckets > 0:
        step.value = DeleteAccountStep.DeleteBucketsStep;
        ownedProjects.value = deleteResp.ownedProjects;
        buckets.value = deleteResp.buckets;
        break;
    case deleteResp.lockEnabledBuckets > 0:
        step.value = DeleteAccountStep.LockEnabledBucketsStep;
        buckets.value = deleteResp.lockEnabledBuckets;
        break;
    case deleteResp.apiKeys > 0:
        step.value = DeleteAccountStep.DeleteAccessKeysStep;
        ownedProjects.value = deleteResp.ownedProjects;
        apiKeys.value = deleteResp.apiKeys;
        break;
    case deleteResp.unpaidInvoices > 0:
        step.value = DeleteAccountStep.PayInvoicesStep;
        unpaidInvoices.value = deleteResp.unpaidInvoices;
        amountOwed.value = deleteResp.amountOwed;
        break;
    case deleteResp.currentUsage || deleteResp.invoicingIncomplete:
        step.value = DeleteAccountStep.WaitForInvoicingStep;
        break;
    default:
        // this should never happen
        throw new Error('account deletion was restricted for an unknown reason');
    }
}

async function proceed(skipLockEnabledBuckets = false): Promise<void> {
    await withLoading(async () => {
        try {
            switch (step.value) {
            case DeleteAccountStep.InitStep:
            case DeleteAccountStep.LockEnabledBucketsStep:
                deleteResp.value = await userStore.deleteAccount(
                    DeleteAccountStep.InitStep,
                    skipLockEnabledBuckets ? SKIP_OBJECT_LOCK_ENABLED_BUCKETS : '',
                );
                if (!deleteResp.value) {
                    step.value = DeleteAccountStep.VerifyPasswordStep;
                } else {
                    chooseRestrictionStep(deleteResp.value);
                }
                break;
            case DeleteAccountStep.VerifyPasswordStep:
                passwordForm.value?.validate();
                if (!passwordForm.value?.isValid) return;

                deleteResp.value = await userStore.deleteAccount(DeleteAccountStep.VerifyPasswordStep, password.value);
                if (!deleteResp.value) {
                    if (user.value.isMFAEnabled) {
                        step.value = DeleteAccountStep.Verify2faStep;
                    } else {
                        step.value = DeleteAccountStep.VerifyEmailStep;
                    }
                } else {
                    chooseRestrictionStep(deleteResp.value);
                }
                break;
            case DeleteAccountStep.Verify2faStep:
                if (code2fa.value.length !== 6) {
                    isOTPInputError.value = true;
                    return;
                }

                deleteResp.value = await userStore.deleteAccount(DeleteAccountStep.Verify2faStep, code2fa.value.trim());
                if (!deleteResp.value) {
                    step.value = DeleteAccountStep.VerifyEmailStep;
                } else {
                    chooseRestrictionStep(deleteResp.value);
                }

                break;
            case DeleteAccountStep.VerifyEmailStep:
                if (verifyEmailCode.value.length !== 6) {
                    isOTPInputError.value = true;
                    return;
                }

                deleteResp.value = await userStore.deleteAccount(DeleteAccountStep.VerifyEmailStep, verifyEmailCode.value.trim());
                if (!deleteResp.value) {
                    step.value = DeleteAccountStep.ConfirmDeleteStep;
                } else {
                    chooseRestrictionStep(deleteResp.value);
                }

                break;
            case DeleteAccountStep.ConfirmDeleteStep:
                if (!isDeleteConfirmed.value) return;

                step.value = DeleteAccountStep.FinalConfirmDeleteStep;
                break;
            case DeleteAccountStep.FinalConfirmDeleteStep:
                deleteResp.value = await userStore.deleteAccount(DeleteAccountStep.ConfirmDeleteStep, '');
                if (!deleteResp.value) {
                    notify.success('Good bye!');
                    model.value = false;

                    setTimeout(() => {
                        window.location.href = `${window.location.origin}/login`;
                    }, 2000);
                } else {
                    chooseRestrictionStep(deleteResp.value);
                }
            }
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.ACCOUNT_DELETE_DIALOG);
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

async function goToProjects() {
    await router.push(ROUTES.Projects.path);
}

async function goToBilling() {
    await router.push(ROUTES.Billing.path + '?tab=payment-methods');
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
