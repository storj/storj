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
                        <img class="flex-shrink-0" src="@poc/assets/icon-mfa.svg" alt="MFA">
                        <v-card-title class="font-weight-bold ml-4">Setup Two-Factor</v-card-title>
                    </v-row>
                    <v-btn
                        icon="$close"
                        variant="text"
                        size="small"
                        color="default"
                        :disabled="isLoading"
                        @click="closeDialog"
                    />
                </v-row>
            </v-card-item>
            <v-divider class="mx-8" />
            <v-window v-model="step" :class="{ 'overflow-y-auto': step === 0 }">
                <!-- QR code step -->
                <v-window-item :value="0">
                    <v-card-item class="px-8 py-4">
                        <p>Scan this QR code in your two-factor application.</p>
                    </v-card-item>
                    <v-card-item align="center" justify="center" class="rounded-lg border mx-8 py-4" style="background: #edeef1;">
                        <v-col cols="auto">
                            <canvas ref="canvas" />
                        </v-col>
                    </v-card-item>
                    <v-divider class="mx-8 my-4" />
                    <v-card-item class="px-8 py-4 pt-0">
                        <p>Unable to scan? Enter the following code instead.</p>
                    </v-card-item>
                    <v-card-item class="rounded-lg border mx-8 pa-0" style="background: #FAFAFB;">
                        <v-col class="py-2 px-3" cols="auto">
                            <p class="font-weight-bold"> {{ userMFASecret }}</p>
                        </v-col>
                    </v-card-item>
                </v-window-item>

                <!-- Enter code step -->
                <v-window-item :value="1">
                    <v-card-item class="px-8 py-4">
                        <p>Enter the authentication code generated in your two-factor application to confirm your setup.</p>
                    </v-card-item>
                    <v-divider class="mx-8" />
                    <v-card-item class="px-8 pt-4 pb-0">
                        <v-form v-model="formValid" class="pt-1" :onsubmit="enable">
                            <v-text-field
                                v-model="confirmPasscode"
                                variant="outlined"
                                density="compact"
                                hint="e.g.: 000000"
                                :rules="rules"
                                :error-messages="isError ? 'Invalid code. Please re-enter.' : ''"
                                label="2FA Code"
                                required
                                autofocus
                            />
                        </v-form>
                    </v-card-item>
                </v-window-item>

                <!-- Save codes step -->
                <v-window-item :value="2">
                    <v-card-item class="px-8 py-4">
                        <p>Please save these codes somewhere to be able to recover access to your account.</p>
                    </v-card-item>
                    <v-divider class="mx-8" />
                    <v-card-item class="px-8 py-4">
                        <p
                            v-for="(code, index) in userMFARecoveryCodes"
                            :key="index"
                        >
                            {{ code }}
                        </p>
                    </v-card-item>
                </v-window-item>
            </v-window>
            <v-divider class="mx-8 my-4" />
            <v-card-actions dense class="px-7 pb-5 pt-0">
                <v-col v-if="step !== 2" class="pl-0">
                    <v-btn
                        variant="outlined"
                        color="default"
                        block
                        :disabled="isLoading"
                        :loading="isLoading"
                        @click="closeDialog"
                    >
                        Cancel
                    </v-btn>
                </v-col>
                <v-col class="px-0">
                    <v-btn
                        v-if="step === 0"
                        color="primary"
                        variant="flat"
                        block
                        :loading="isLoading"
                        @click="step++"
                    >
                        Continue
                    </v-btn>
                    <v-btn
                        v-else-if="step === 1"
                        color="primary"
                        variant="flat"
                        block
                        :loading="isLoading"
                        :disabled="!formValid"
                        @click="enable"
                    >
                        Enable
                    </v-btn>

                    <v-btn
                        v-else
                        color="primary"
                        variant="flat"
                        block
                        @click="closeDialog"
                    >
                        Done
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
    VWindow,
    VWindowItem,
} from 'vuetify/components';
import QRCode from 'qrcode';

import { useLoading } from '@/composables/useLoading';
import { useConfigStore } from '@/store/modules/configStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useNotify } from '@/utils/hooks';

const rules = [
    (value: string) => (!!value || 'Can\'t be empty'),
    (value: string) => (!value.includes(' ') || 'Can\'t contain spaces'),
    (value: string) => (!!parseInt(value) || 'Can only be numbers'),
    (value: string) => (value.length === 6 || 'Can only be 6 numbers long'),
];

const analyticsStore = useAnalyticsStore();
const { config } = useConfigStore().state;
const usersStore = useUsersStore();
const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const canvas = ref<HTMLCanvasElement>();
const innerContent = ref<Component | null>(null);

const props = defineProps<{
    modelValue: boolean,
}>();

const emit = defineEmits<{
    (event: 'update:modelValue', value: boolean): void,
}>();

const step = ref<number>(0);
const confirmPasscode = ref<string>('');
const isError = ref<boolean>(false);
const formValid = ref<boolean>(false);

const model = computed<boolean>({
    get: () => props.modelValue,
    set: value => emit('update:modelValue', value),
});

/**
 * Returns pre-generated MFA secret from store.
 */
const userMFASecret = computed((): string => {
    return usersStore.state.userMFASecret;
});

/**
 * Returns user MFA recovery codes from store.
 */
const userMFARecoveryCodes = computed((): string[] => {
    return usersStore.state.userMFARecoveryCodes;
});

/**
 * Returns satellite name from store.
 */
const satellite = computed((): string => {
    return config.satelliteName;
});

/**
 * Returns the 2FA QR link.
 */
const qrLink = computed((): string => {
    return `otpauth://totp/${encodeURIComponent(usersStore.state.user.email)}?secret=${userMFASecret.value}&issuer=${encodeURIComponent(`STORJ ${satellite.value}`)}&algorithm=SHA1&digits=6&period=30`;
});

/**
 * Enables user MFA and sets view to Recovery Codes state.
 */
function enable(): void {
    if (!formValid.value) return;

    withLoading(async () => {
        try {
            await usersStore.enableUserMFA(confirmPasscode.value);
            await usersStore.getUser();
            await showCodes();

            analyticsStore.eventTriggered(AnalyticsEvent.MFA_ENABLED);
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.ENABLE_MFA_MODAL);
            isError.value = true;
        }
    });
}

/**
 * Toggles view to MFA Recovery Codes state.
 */
async function showCodes() {
    try {
        await usersStore.generateUserMFARecoveryCodes();
        step.value = 2;
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.ENABLE_MFA_MODAL);
    }
}

function closeDialog() {
    model.value = false;
    isError.value = false;
}

watch(canvas, async val => {
    if (!val) return;
    try {
        await QRCode.toCanvas(canvas.value, qrLink.value);
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.ENABLE_MFA_MODAL);
    }
});

watch(confirmPasscode, () => {
    isError.value = false;
});

watch(innerContent, newContent => {
    if (newContent) return;
    // dialog has been closed
    step.value = 0;
    confirmPasscode.value = '';
});
</script>