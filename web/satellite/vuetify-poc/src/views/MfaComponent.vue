// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card :title="model ? 'Enter your 2FA code' : 'Enter your recovery code'" class="pa-2 pa-sm-7">
        <v-card-text v-if="model">
            <p>Enter the 6 digit code from your two factor authenticator application to continue.</p>
            <v-card class="my-4" rounded="lg" color="secondary" variant="outlined">
                <v-otp-input
                    :model-value="otp"
                    :error="error"
                    :disabled="loading"
                    autofocus
                    class="my-2"
                    maxlength="6"
                    @update:modelValue="value => onValueChange(value)"
                />
            </v-card>

            <v-btn
                :disabled="otp.length < 6"
                :loading="loading"
                color="primary"
                block
                @click="verifyCode()"
            >
                <span v-if="otp.length === 0">6 digits left</span>

                <span v-else-if="otp.length < 6">
                    {{ 6 - otp.length }}
                    digits left
                </span>

                <span v-else>
                    Verify
                </span>
            </v-btn>
        </v-card-text>
        <v-card-text v-else>
            <p>Enter one of your recovery codes to continue.</p>
            <v-form v-model="formValid" @submit.prevent>
                <v-text-field
                    :model-value="recovery"
                    :error="error"
                    :disabled="loading"
                    :rules="[RequiredRule]"
                    label="Recovery Code"
                    class="mt-5"
                    required
                    maxlength="50"
                    @update:modelValue="value => onValueChange(value)"
                />
                <v-btn
                    :disabled="!formValid"
                    :loading="loading"
                    color="primary"
                    size="large"
                    block
                    @click="verifyCode()"
                >
                    Continue
                </v-btn>
            </v-form>
        </v-card-text>
    </v-card>
    <p class="pt-9 text-center text-body-2">
        Or use <span
            class="link"
            @click="model = !model"
        >
            {{ model ? 'a recovery code' : 'an OTP code' }}
        </span>
    </p>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import {
    VBtn,
    VCard,
    VCardText,
    VForm,
    VTextField,
    VOtpInput,
} from 'vuetify/components';

import { RequiredRule } from '@poc/types/common';

const props = defineProps<{
    modelValue: boolean;
    loading: boolean;
    error: boolean;
    recovery: string;
    otp: string;
}>();

const emit = defineEmits<{
    'update:modelValue': [value: boolean];
    'update:error': [value: boolean];
    'update:recovery': [value: string];
    'update:otp': [value: string];
    verify: [];
}>();

const formValid = ref(false);

/**
 * Whether to use the OTP code or recovery code.
 */
const model = computed<boolean>({
    get: () => props.modelValue,
    set: value => {
        emit('update:otp', '');
        emit('update:recovery', '');
        emit('update:error', false);
        emit('update:modelValue', value);
    },
});

function verifyCode() {
    emit('verify');
}

function onValueChange(value: string) {
    if (model.value) {
        if (props.recovery) {
            emit('update:recovery', '');
        }
        const val = value.slice(0, 6);
        if (isNaN(+val)) {
            return;
        }
        emit('update:otp', val);
    } else {
        emit('update:recovery', value);
        if (props.otp) {
            emit('update:otp', '');
        }
    }
    emit('update:error', false);
}
</script>