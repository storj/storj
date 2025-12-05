// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container class="fill-height">
        <v-row justify="center">
            <v-col cols="12" sm="9" md="7" lg="5" xl="4" xxl="3">
                <v-card title="Verify Account" class="pa-2 pa-sm-7">
                    <v-card-item>
                        <v-alert
                            v-if="isActivationExpired"
                            variant="tonal"
                            color="error"
                            density="comfortable"
                            border
                            closable
                        >
                            <template #text>
                                The verification link you clicked on has expired. Request a new link.
                            </template>
                        </v-alert>
                    </v-card-item>
                    <v-card-text>
                        <p>If you haven’t verified your account yet, input your email to receive a new verification link. Make sure you’re signing on the right satellite.</p>
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
                                :loading="isLoading"
                                :disabled="!formValid"
                                @click="onActivateClick"
                            >
                                Get Activation Link
                            </v-btn>
                        </v-form>
                    </v-card-text>
                </v-card>
                <p class="pt-6 text-center text-body-2">Go back to <router-link class="link font-weight-bold" :to="ROUTES.Login.path">Login</router-link></p>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import {
    VAlert,
    VBtn,
    VCard, VCardItem,
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
import { EmailRule, RequiredRule, ValidationRule } from '@/types/common';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { AuthHttpApi } from '@/api/auth';
import { MultiCaptchaConfig } from '@/types/config.gen';
import { ROUTES } from '@/router';

const auth: AuthHttpApi = new AuthHttpApi();
const configStore = useConfigStore();

const route = useRoute();
const router = useRouter();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const email = ref<string>('');
const isActivationExpired = ref<boolean>(false);
const formValid = ref<boolean>(false);
const captchaResponseToken = ref<string>('');

const captcha = ref<VueHcaptcha>();

const satellitesHints = [
    { satellite: 'US1', hint: 'Recommended for North and South America' },
    { satellite: 'EU1', hint: 'Recommended for Europe and Africa' },
    { satellite: 'AP1', hint: 'Recommended for Asia and Oceania' },
];
const emailRules: ValidationRule<string>[] = [
    RequiredRule,
    EmailRule,
];

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
            window.location.href = satellite.address + ROUTES.Activate.path;
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
 * Handles captcha verification response.
 */
function onCaptchaVerified(response: string): void {
    captchaResponseToken.value = response;
    onActivateClick();
}

/**
 * Handles captcha error.
 */
function onCaptchaError(): void {
    captchaResponseToken.value = '';
    notify.error('The captcha encountered an error. Please try again.', null);
}

/**
 * onActivateClick validates input fields and requests resending of activation email.
 */
async function onActivateClick(): Promise<void> {
    if (!formValid.value) {
        return;
    }
    if (captcha.value && !captchaResponseToken.value) {
        captcha.value.execute();
        return;
    }

    await withLoading(async () => {
        try {
            await auth.resendEmail(email.value, captchaResponseToken.value);
            notify.success('Activation link sent');
            router.push({
                name: ROUTES.SignupConfirmation.name,
                query: { email: encodeURIComponent(email.value) },
            });
        } catch (error) {
            notify.notifyError(error);
        }
    });
    captcha.value?.reset();
    captchaResponseToken.value = '';
}

onMounted((): void => {
    isActivationExpired.value = route.query.expired === 'true';
});
</script>
