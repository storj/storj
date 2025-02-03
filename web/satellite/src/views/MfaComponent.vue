// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card :title="model ? 'Enter your 2FA code' : 'Enter your recovery code'" class="pa-2 pa-sm-6">
        <v-card-text v-if="model">
            <p>Enter the 6 digit code from your two factor authenticator application to continue.</p>
            <v-form v-model="formValid" @submit.prevent="verifyCode">
                <v-card class="my-4" rounded="lg" color="secondary" variant="outlined">
                    <v-otp-input
                        :model-value="otp"
                        :error="error"
                        :disabled="loading"
                        autofocus
                        class="my-2"
                        maxlength="6"
                        @update:model-value="value => onValueChange(value)"
                    />
                </v-card>

                <v-btn
                    type="submit"
                    :disabled="otp.length < 6"
                    :loading="loading"
                    color="primary"
                    block
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
            </v-form>
        </v-card-text>
        <v-card-text v-else>
            <p>Enter one of your recovery codes to continue.</p>
            <v-form v-model="formValid" @submit.prevent="verifyCode">
                <v-text-field
                    :model-value="recovery"
                    :error="error"
                    :disabled="loading"
                    :rules="[RequiredRule]"
                    label="Recovery Code"
                    class="mt-5"
                    required
                    maxlength="50"
                    @update:model-value="value => onValueChange(value)"
                />
                <v-btn
                    type="submit"
                    :disabled="!formValid"
                    :loading="loading"
                    color="primary"
                    size="large"
                    block
                >
                    Continue
                </v-btn>
            </v-form>
        </v-card-text>
    </v-card>
    <p class="mt-8 text-center text-body-2">
        Or use a <a
            class="link font-weight-bold"
            @click="model = !model"
        >
            {{ model ? 'Recovery Code' : '2FA Code' }}
        </a>
    </p>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue';
import {
    VBtn,
    VCard,
    VCardText,
    VForm,
    VTextField,
    VOtpInput,
} from 'vuetify/components';

import { RequiredRule } from '@/types/common';

defineProps<{
    loading: boolean;
}>();

const model = defineModel<boolean>({ required: true });
const error = defineModel<boolean>('error', { required: true });
const recovery = defineModel<string>('recovery', { required: true });
const otp = defineModel<string>('otp', { required: true });

const emit = defineEmits<{
    verify: [];
}>();

const formValid = ref(false);

function verifyCode() {
    emit('verify');
}

function onValueChange(value: string) {
    if (model.value) {
        if (recovery.value) {
            recovery.value = '';
        }
        const val = value.slice(0, 6);
        if (isNaN(+val)) {
            return;
        }
        otp.value = val;
    } else {
        recovery.value = value;
        if (otp.value) {
            otp.value = '';
        }
    }
    error.value = false;
}

watch(model, () => {
    otp.value = '';
    recovery.value = '';
    error.value = false;
});
</script>
