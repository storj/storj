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
                        Delete Project
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
                    <v-window-item :value="DeleteProjectStep.InitStep">
                        <v-form class="pa-6" @submit.prevent>
                            <v-row>
                                <v-col>
                                    <p class="font-weight-bold mb-4">
                                        You are about to delete your project.
                                    </p>
                                    <p>Project:</p>
                                    <v-chip variant="tonal">
                                        {{ project.name }}
                                    </v-chip>
                                </v-col>
                            </v-row>
                        </v-form>
                    </v-window-item>

                    <v-window-item :value="DeleteProjectStep.DeleteBucketsStep">
                        <v-form class="pa-6">
                            <v-row>
                                <v-col>
                                    <p class="font-weight-bold mb-4">
                                        Before we proceed with your project deletion request,
                                        please delete all of your data and buckets.
                                    </p>
                                    <p class="font-weight-bold mb-4">Total buckets: <v-chip color="error">{{ buckets }}</v-chip></p>
                                    <v-alert variant="tonal" type="info">
                                        Once you delete all of your buckets, then you can proceed with project deletion.
                                    </v-alert>
                                </v-col>
                            </v-row>
                        </v-form>
                    </v-window-item>

                    <v-window-item :value="DeleteProjectStep.LockEnabledBucketsStep">
                        <div class="pa-6">
                            <v-alert variant="tonal" type="error">
                                You have {{ buckets }} bucket{{ buckets > 1 ? 's' : '' }} with Object Lock enabled.
                                Objects in th{{ buckets > 1 ? 'ese buckets' : 'is bucket' }} may be protected from deletion due to retention policies.
                                If the bucket{{ buckets > 1 ? 's are' : ' is' }} empty, delete {{ buckets > 1 ? 'them' : 'it' }} to proceed.
                            </v-alert>
                        </div>
                    </v-window-item>

                    <v-window-item :value="DeleteProjectStep.DeleteAccessKeysStep">
                        <v-form class="pa-6">
                            <v-row>
                                <v-col>
                                    <p class="font-weight-bold mb-4">
                                        Before we proceed with your project deletion request,
                                        please delete all of your access keys:
                                    </p>
                                    <p class="font-weight-bold mb-4">Total access keys: <v-chip color="error">{{ apiKeys }}</v-chip></p>
                                    <v-alert variant="tonal" type="info">
                                        Once you delete all of your access keys, then you can proceed with project deletion.
                                    </v-alert>
                                </v-col>
                            </v-row>
                        </v-form>
                    </v-window-item>

                    <v-window-item :value="DeleteProjectStep.WaitForInvoicingStep">
                        <v-form class="pa-6">
                            <v-row>
                                <v-col>
                                    <p class="font-weight-bold mb-4">
                                        There's some recent usage in your project that hasn't been billed yet. To delete your project, please
                                        follow these steps:
                                    </p>
                                    <p class="mb-4">1. Please wait until the end of the current billing cycle (typically the end of the month).</p>
                                    <p class="mb-4">2. We'll generate your invoice early in the following month (usually around the 4th day).</p>
                                </v-col>
                            </v-row>
                        </v-form>
                    </v-window-item>

                    <v-window-item :value="DeleteProjectStep.VerifyPasswordStep">
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

                    <v-window-item v-if="user.isMFAEnabled" :value="DeleteProjectStep.Verify2faStep">
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

                    <v-window-item :value="DeleteProjectStep.VerifyEmailStep">
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

                    <v-window-item :value="DeleteProjectStep.ConfirmDeleteStep">
                        <v-form class="pa-6" @submit.prevent>
                            <v-row>
                                <v-col>
                                    <p>Please confirm that you want to delete your project.</p>
                                    <v-chip
                                        variant="tonal"
                                        class="my-4 font-weight-bold"
                                    >
                                        {{ project.name }}
                                    </v-chip>
                                    <v-checkbox-btn v-model="isDeleteConfirmed" label="I want to delete this project." density="compact" />
                                </v-col>
                            </v-row>
                        </v-form>
                    </v-window-item>
                </v-window>
            </v-card-text>

            <v-divider />

            <v-card-actions class="pa-6">
                <v-row>
                    <v-col v-if="step === DeleteProjectStep.InitStep || step === DeleteProjectStep.LockEnabledBucketsStep">
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
                            v-if="step === DeleteProjectStep.DeleteBucketsStep || step === DeleteProjectStep.LockEnabledBucketsStep"
                            variant="flat"
                            block
                            @click="goToBuckets"
                        >
                            Go to buckets
                        </v-btn>
                        <v-btn
                            v-else-if="step === DeleteProjectStep.DeleteAccessKeysStep"
                            variant="flat"
                            block
                            @click="goToAccesses"
                        >
                            Go to accesses
                        </v-btn>
                        <v-btn
                            v-else-if="step === DeleteProjectStep.WaitForInvoicingStep"
                            variant="flat"
                            block
                            @click="model = false"
                        >
                            Close
                        </v-btn>
                        <v-btn
                            v-else-if="step < DeleteProjectStep.ConfirmDeleteStep"
                            color="primary"
                            variant="flat"
                            block
                            @click="proceed"
                        >
                            Next
                        </v-btn>
                        <v-btn
                            v-if="step === DeleteProjectStep.ConfirmDeleteStep"
                            color="error"
                            variant="flat"
                            :loading="isLoading"
                            :disabled="step === DeleteProjectStep.ConfirmDeleteStep && !isDeleteConfirmed"
                            block
                            @click="proceed"
                        >
                            Delete Project
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

import { DeleteProjectStep } from '@/types/accountActions';
import { User } from '@/types/users';
import { Project, ProjectDeletionData } from '@/types/projects';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { ROUTES } from '@/router';
import { useUsersStore } from '@/store/modules/usersStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { RequiredRule } from '@/types/common';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

const router = useRouter();

const deleteResp = ref<ProjectDeletionData | null>(null);

const userStore = useUsersStore();
const projectStore = useProjectsStore();

const model = defineModel<boolean>({ required: true });
const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const step = ref<DeleteProjectStep>(DeleteProjectStep.InitStep);
const password = ref<string>('');
const code2fa = ref<string>('');
const verifyEmailCode = ref<string>('');

const passwordForm = ref<VForm>();
const otpInput2fa = ref<VOtpInput>();
const otpInputVerify = ref<VOtpInput>();
const isDeleteConfirmed = ref<boolean>(false);
const isOTPInputError = ref<boolean>(false);

const buckets = ref<number>(0);
const apiKeys = ref<number>(0);

const user = computed<User>(() => userStore.state.user);

/**
 * Returns selected project from the store.
 */
const project = computed<Project>(() => {
    return projectStore.state.selectedProject;
});

function chooseRestrictionStep(deleteResp: ProjectDeletionData) {
    switch (true) {
    case deleteResp.buckets > 0:
        step.value = DeleteProjectStep.DeleteBucketsStep;
        buckets.value = deleteResp.buckets;
        break;
    case deleteResp.lockEnabledBuckets > 0:
        step.value = DeleteProjectStep.LockEnabledBucketsStep;
        buckets.value = deleteResp.lockEnabledBuckets;
        break;
    case deleteResp.apiKeys > 0:
        step.value = DeleteProjectStep.DeleteAccessKeysStep;
        apiKeys.value = deleteResp.apiKeys;
        break;
    case deleteResp.currentUsage || deleteResp.invoicingIncomplete:
        step.value = DeleteProjectStep.WaitForInvoicingStep;
        break;
    default:
        // this should never happen
        throw new Error('project deletion was restricted for an unknown reason');
    }
}

async function proceed(): Promise<void> {
    await withLoading(async () => {
        try {
            switch (step.value) {
            case DeleteProjectStep.InitStep:
                deleteResp.value = await projectStore.deleteProject(project.value.id, DeleteProjectStep.InitStep, '');
                if (!deleteResp.value) {
                    step.value = DeleteProjectStep.VerifyPasswordStep;
                } else {
                    chooseRestrictionStep(deleteResp.value);
                }
                break;
            case DeleteProjectStep.VerifyPasswordStep:
                passwordForm.value?.validate();
                if (!passwordForm.value?.isValid) return;

                deleteResp.value = await projectStore.deleteProject(project.value.id, DeleteProjectStep.VerifyPasswordStep, password.value);
                if (!deleteResp.value) {
                    if (user.value.isMFAEnabled) {
                        step.value = DeleteProjectStep.Verify2faStep;
                    } else {
                        step.value = DeleteProjectStep.VerifyEmailStep;
                    }
                } else {
                    chooseRestrictionStep(deleteResp.value);
                }
                break;
            case DeleteProjectStep.Verify2faStep:
                if (code2fa.value.length !== 6) {
                    isOTPInputError.value = true;
                    return;
                }

                deleteResp.value = await projectStore.deleteProject(project.value.id, DeleteProjectStep.Verify2faStep, code2fa.value.trim());
                if (!deleteResp.value) {
                    step.value = DeleteProjectStep.VerifyEmailStep;
                } else {
                    chooseRestrictionStep(deleteResp.value);
                }

                break;
            case DeleteProjectStep.VerifyEmailStep:
                if (verifyEmailCode.value.length !== 6) {
                    isOTPInputError.value = true;
                    return;
                }

                deleteResp.value = await projectStore.deleteProject(project.value.id, DeleteProjectStep.VerifyEmailStep, verifyEmailCode.value.trim());
                if (!deleteResp.value) {
                    step.value = DeleteProjectStep.ConfirmDeleteStep;
                } else {
                    chooseRestrictionStep(deleteResp.value);
                }

                break;
            case DeleteProjectStep.ConfirmDeleteStep:
                if (!isDeleteConfirmed.value) return;

                deleteResp.value = await projectStore.deleteProject(project.value.id, DeleteProjectStep.ConfirmDeleteStep, '');
                if (!deleteResp.value) {
                    notify.success('Project deleted');
                    router.push(ROUTES.Projects.path);
                } else {
                    chooseRestrictionStep(deleteResp.value);
                }
            }
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.PROJECT_DELETE_DIALOG);
        }
    });
}

function initialiseOTPInput() {
    setTimeout(() => {
        switch (step.value) {
        case DeleteProjectStep.Verify2faStep:
            otpInput2fa.value?.focus();
            break;
        case DeleteProjectStep.VerifyEmailStep:
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
    case DeleteProjectStep.Verify2faStep:
        code2fa.value = val;
        break;
    case DeleteProjectStep.VerifyEmailStep:
        verifyEmailCode.value = val;
    }

    isOTPInputError.value = false;
}

async function goToAccesses() {
    await router.push(ROUTES.Access.path);
}

async function goToBuckets() {
    await router.push(ROUTES.Buckets.path);
}

watch(step, val => {
    if (
        val === DeleteProjectStep.Verify2faStep ||
        val === DeleteProjectStep.VerifyEmailStep
    ) {
        initialiseOTPInput();
    }
});

watch(model, val => {
    if (!val) {
        step.value = DeleteProjectStep.InitStep;
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
