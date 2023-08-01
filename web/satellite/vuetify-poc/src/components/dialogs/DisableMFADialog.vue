// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        width="auto"
        min-width="320px"
        max-width="460px"
        transition="fade-transition"
    >
        <v-card ref="innerContent" rounded="xlg">
            <v-card-item class="pl-7 pr-0 pb-5 pt-0">
                <v-row align="start" justify="space-between" class="ma-0">
                    <v-row align="center" class="ma-0 pt-5">
                        <img class="flex-shrink-0" src="@poc/assets/icon-mfa.svg" alt="Change password">
                        <v-card-title class="font-weight-bold ml-4">Disable Two-Factor</v-card-title>
                    </v-row>
                    <v-btn
                        icon="$close"
                        variant="text"
                        size="small"
                        color="default"
                        :disabled="isLoading"
                        @click="model = false"
                    />
                </v-row>
            </v-card-item>
            <v-divider class="mx-8" />
            <v-card-item class="px-8 py-4">
                <p>Enter the authentication code generated in your two-factor application to disable 2FA.</p>
            </v-card-item>
            <v-divider class="mx-8" />
            <v-card-item class="px-8 pt-4 pb-0">
                <v-form v-model="formValid" class="pt-1" :onsubmit="disable">
                    <v-text-field
                        v-model="confirmCode"
                        variant="outlined"
                        density="compact"
                        :hint="useRecoveryCode ? '' : 'e.g.: 000000'"
                        :rules="rules"
                        :error-messages="isError ? 'Invalid code. Please re-enter.' : ''"
                        :label="useRecoveryCode ? 'Recovery code' : '2FA Code'"
                        required
                        autofocus
                    />
                </v-form>
            </v-card-item>
            <v-card-item class="px-8 py-0">
                <a class="text-decoration-underline" style="cursor: pointer;" @click="toggleRecoveryCodeState">
                    {{ useRecoveryCode ? "or use 2FA code" : "or use a recovery code" }}
                </a>
            </v-card-item>
            <v-divider class="mx-8 my-4" />
            <v-card-actions dense class="px-7 pb-5 pt-0">
                <v-col class="pl-0">
                    <v-btn
                        variant="outlined"
                        color="default"
                        block
                        :disabled="isLoading"
                        :loading="isLoading"
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
import { Component, computed, ref, watch } from 'vue';
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
    VRow,
    VTextField,
} from 'vuetify/components';

import { useLoading } from '@/composables/useLoading';
import { useConfigStore } from '@/store/modules/configStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { DisableMFARequest } from '@/types/users';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';

const { config } = useConfigStore().state;
const usersStore = useUsersStore();
const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const innerContent = ref<Component | null>(null);

const props = defineProps<{
    modelValue: boolean,
}>();

const emit = defineEmits<{
    (event: 'update:modelValue', value: boolean): void,
}>();

const confirmCode = ref<string>('');
const isError = ref<boolean>(false);
const useRecoveryCode = ref<boolean>(false);
const formValid = ref<boolean>(false);

const model = computed<boolean>({
    get: () => props.modelValue,
    set: value => emit('update:modelValue', value),
});

/**
 * Returns validation rules based on whether recovery
 * code input is being used.
 */
const rules = computed(() => {
    if (useRecoveryCode.value) {
        return [
            (value: string) => (!!value || 'Can\'t be empty'),
        ];
    }
    return [
        (value: string) => (!!value || 'Can\'t be empty'),
        (value: string) => (!value.includes(' ') || 'Can\'t contain spaces'),
        (value: string) => (!!parseInt(value) || 'Can only be numbers'),
        (value: string) => (value.length === 6 || 'Can only be 6 numbers long'),
    ];
});

/**
 * Toggles whether the MFA recovery code input is shown.
 */
function toggleRecoveryCodeState(): void {
    isError.value = false;
    confirmCode.value = '';
    useRecoveryCode.value = !useRecoveryCode.value;
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

watch(confirmCode, () => {
    isError.value = false;
});

watch(innerContent, newContent => {
    if (newContent) return;
    // dialog has been closed
    isError.value = false;
    confirmCode.value = '';
    useRecoveryCode.value = false;
});
</script>