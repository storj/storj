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
        <v-card rounded="xlg">
            <v-card-item class="pa-5 pl-7">
                <template #prepend>
                    <img class="d-block" src="@/assets/icon-mfa.svg" alt="MFA">
                </template>
                <v-card-title class="font-weight-bold">Two-Factor Recovery Codes</v-card-title>
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
            <v-divider />

            <template v-if="isConfirmCode">
                <v-card-item class="px-8 py-4">
                    <p>Enter the authentication code generated in your two-factor application to regenerate recovery codes.</p>
                </v-card-item>
                <v-divider />
                <v-card-item class="px-8 pt-4 pb-0">
                    <v-form v-model="formValid" class="pt-2" @submit.prevent="regenerate">
                        <v-text-field
                            v-model="confirmPasscode"
                            variant="outlined"
                            density="compact"
                            :hint="useRecoveryCode ? '' : 'e.g.: 000000'"
                            :rules="rules"
                            :error-messages="isError ? 'Invalid code. Please re-enter.' : ''"
                            :label="useRecoveryCode ? 'Recovery code' : '2FA Code'"
                            :hide-details="false"
                            :maxlength="useRecoveryCode ? 50 : 6"
                            required
                            autofocus
                        />
                    </v-form>
                </v-card-item>
                <v-card-item class="px-8 py-0">
                    <a class="text-decoration-underline text-cursor-pointer" @click="toggleRecoveryCodeState">
                        {{ useRecoveryCode ? "or use 2FA code" : "or use a recovery code" }}
                    </a>
                </v-card-item>

                <v-divider class="my-4" />
            </template>
            <template v-else>
                <v-card-item class="px-8 py-4">
                    <p>Please save these codes somewhere to be able to recover access to your account.</p>
                </v-card-item>
                <v-divider />
                <v-card-item class="px-8 py-4">
                    <p
                        v-for="(code, index) in userMFARecoveryCodes"
                        :key="index"
                    >
                        {{ code }}
                    </p>
                </v-card-item>
                <v-divider class="mb-4" />
            </template>

            <v-card-actions dense class="px-7 pb-5 pt-0">
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
import { computed, ref, watch } from 'vue';
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
    VTextField,
} from 'vuetify/components';

import { AuthHttpApi } from '@/api/auth';
import { useUsersStore } from '@/store/modules/usersStore';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

const auth: AuthHttpApi = new AuthHttpApi();

const usersStore = useUsersStore();
const notify = useNotify();
const { withLoading, isLoading } = useLoading();

const props = defineProps<{
    modelValue: boolean,
}>();

const emit = defineEmits<{
    (event: 'update:modelValue', value: boolean): void,
}>();

const confirmPasscode = ref<string>('');
const isError = ref<boolean>(false);
const formValid = ref<boolean>(false);
const isConfirmCode = ref(true);
const useRecoveryCode = ref<boolean>(false);

const model = computed<boolean>({
    get: () => props.modelValue,
    set: value => emit('update:modelValue', value),
});

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
}

/**
 * Regenerates user MFA codes and sets view to Recovery Codes state.
 */
function regenerate(): void {
    if (!confirmPasscode.value || isLoading.value || isError.value) return;

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

watch(confirmPasscode, () => {
    isError.value = false;
});

watch(model, shown => {
    if (shown) {
        return;
    }
    isConfirmCode.value = true;
    useRecoveryCode.value = false;
    confirmPasscode.value = '';
    isError.value = false;
});
</script>
