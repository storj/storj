// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container class="fill-height">
        <v-row align="top" justify="center">
            <v-col cols="12" sm="9" md="7" lg="5" xl="4" xxl="3">
                <v-card title="Did you forgot your password?" class="pa-2 pa-sm-7">
                    <v-card-text>
                        <p>Select your account satellite, enter your email address, and we will send you a password reset link.</p>
                        <v-form v-model="formValid" class="pt-4" @submit.prevent>
                            <v-select
                                v-model="satellite"
                                label="Satellite"
                                :items="satellites"
                                item-title="satellite"
                                :hint="satellite.hint"
                                persistent-hint
                                return-object
                                chips
                                class="mt-3 mb-2"
                            />
                            <v-text-field
                                v-model="email"
                                label="Email address"
                                name="email"
                                type="email"
                                :rules="emailRules"
                                autofocus
                                clearable
                                required
                                class="my-2"
                            />
                            <VueHcaptcha
                                v-if="captchaConfig.hcaptcha.enabled"
                                ref="captcha"
                                :sitekey="captchaConfig.hcaptcha.siteKey"
                                :re-captcha-compat="false"
                                size="invisible"
                                @verify="onCaptchaVerified"
                                @error="onCaptchaError"
                            />
                            <v-btn
                                color="primary"
                                size="large"
                                block
                                :disabled="isLoading || !formValid"
                                @click="onPasswordReset"
                            >
                                Request Password Reset
                            </v-btn>
                        </v-form>
                    </v-card-text>
                </v-card>
                <p class="pt-6 text-center text-body-2">Go back to <router-link class="link" to="/login">login</router-link></p>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { useRouter } from 'vue-router';
import {
    VBtn,
    VCard,
    VCardText,
    VCol,
    VContainer,
    VForm,
    VRow,
    VSelect,
    VTextField,
} from 'vuetify/components';
import VueHcaptcha from '@hcaptcha/vue3-hcaptcha';

import { useConfigStore } from '@/store/modules/configStore';
import { EmailRule, RequiredRule, ValidationRule } from '@poc/types/common';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/utils/hooks';
import { AuthHttpApi } from '@/api/auth';
import { MultiCaptchaConfig } from '@/types/config.gen';

const configStore = useConfigStore();

const router = useRouter();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const auth: AuthHttpApi = new AuthHttpApi();
const satellitesHints = [
    { satellite: 'US1', hint: 'Recommended for North and South America' },
    { satellite: 'EU1', hint: 'Recommended for Europe and Africa' },
    { satellite: 'AP1', hint: 'Recommended for Asia and Australia' },
];
const emailRules: ValidationRule<string>[] = [
    RequiredRule,
    EmailRule,
];

const formValid = ref<boolean>(false);
const email = ref('');
const captcha = ref<VueHcaptcha>();
const captchaResponseToken = ref<string>('');

/**
 * This component's captcha configuration.
 */
const captchaConfig = computed((): MultiCaptchaConfig => {
    return configStore.state.config.captcha.login;
});

/**
 * Name of the current satellite.
 */
const satellite = computed({
    get: () => {
        const satName = configStore.state.config.satelliteName;
        const item = satellitesHints.find(item => item.satellite === satName);
        return item ?? { satellite: satName, hint: '' };
    },
    set: value => {
        const sats = configStore.state.config.partneredSatellites ?? [];
        const satellite = sats.find(sat => sat.name === value.satellite);
        if (satellite) {
            window.location.href = satellite.address + configStore.optionalV2Path + '/password-reset';
        }
    },
});

/**
 * Information about partnered satellites.
 */
const satellites = computed(() => {
    const satellites = configStore.state.config.partneredSatellites ?? [];
    return satellites.map(satellite => {
        const item = satellitesHints.find(item => item.satellite === satellite.name);
        return item ?? { satellite: satellite.name, hint: '' };
    });
});

/**
 * Sends recovery password email.
 */
async function onPasswordReset(): Promise<void> {
    if (captcha.value && !captchaResponseToken.value) {
        captcha.value.execute();
        return;
    }

    await withLoading(async () => {
        try {
            await auth.forgotPassword(email.value, captchaResponseToken.value);
            notify.success('Please look for instructions in your email');
            router.push('/password-reset-confirmation');
        } catch (error) {
            notify.notifyError(error);
        }
    });

    captcha.value?.reset();
    captchaResponseToken.value = '';
}

/**
 * Handles captcha verification response.
 */
function onCaptchaVerified(response: string): void {
    captchaResponseToken.value = response;
    onPasswordReset();
}

/**
 * Handles captcha error.
 */
function onCaptchaError(): void {
    captchaResponseToken.value = '';
    notify.error('The captcha encountered an error. Please try again.', null);
}
</script>
