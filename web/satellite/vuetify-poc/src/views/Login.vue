// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container class="fill-height">
        <v-row align="top" justify="center">
            <v-col cols="12" sm="9" md="7" lg="5" xl="4" xxl="3">
                <v-card v-if="!isMFARequired" title="Login to your Storj account" rounded="xlg" class="pa-2 pa-sm-7">
                    <v-card-text>
                        <v-alert
                            v-if="captchaError"
                            variant="tonal"
                            color="error"
                            text="HCaptcha is required"
                            rounded="lg"
                            density="comfortable"
                            class="mt-2 mb-3"
                            border
                        />
                        <v-alert
                            v-if="isActivatedBannerShown"
                            variant="tonal"
                            :color="isActivatedError ? 'error' : 'success'"
                            :title="isActivatedError ? 'Oops!' :'Success!'"
                            :text="isActivatedError ? 'This account has already been verified.' : 'Account verified.'"
                            rounded="lg"
                            density="comfortable"
                            class="mt-2 mb-3"
                            border
                        />
                        <v-alert
                            v-if="inviteInvalid"
                            variant="tonal"
                            color="error"
                            title="Oops!"
                            text="The invite link you used has expired or is invalid."
                            rounded="lg"
                            density="comfortable"
                            class="mt-2 mb-3"
                            border
                        />
                        <v-alert
                            v-if="isBadLoginMessageShown"
                            variant="tonal"
                            color="error"
                            title="Invalid Credentials"
                            text="Login failed. Please check if this is the correct satellite for your account. If you are
                            sure your credentials are correct, please check your email inbox for a notification with
                            further instructions."
                            rounded="lg"
                            density="comfortable"
                            class="mt-2 mb-3"
                            border
                        />
                        <v-form ref="form" v-model="formValid" class="pt-4" @submit.prevent>
                            <v-select
                                v-model="satellite"
                                label="Satellite"
                                :items="satellites"
                                item-title="satellite"
                                :hint="satellite.hint"
                                persistent-hint
                                return-object
                                chips
                                class="mb-5"
                            />

                            <v-text-field
                                id="Email Address"
                                v-model="email"
                                class="mb-2"
                                label="Email address"
                                placeholder="Enter your email"
                                name="email"
                                type="email"
                                :rules="emailRules"
                                flat
                                clearable
                                required
                            />

                            <v-text-field
                                id="Password"
                                v-model="password"
                                class="mb-2"
                                label="Password"
                                placeholder="Enter your password"
                                color="secondary"
                                :type="showPassword ? 'text' : 'password'"
                                :rules="passwordRules"
                                required
                            >
                                <template #append-inner>
                                    <password-input-eye-icons
                                        :is-visible="showPassword"
                                        type="password"
                                        @toggleVisibility="showPassword = !showPassword"
                                    />
                                </template>
                            </v-text-field>

                            <v-btn
                                color="primary"
                                size="large"
                                block
                                :loading="isLoading"
                                @click="onLoginClick"
                            >
                                Continue
                            </v-btn>
                        </v-form>
                    </v-card-text>
                </v-card>
                <login2-f-a
                    v-else
                    v-model="useOTP"
                    v-model:error="isMFAError"
                    v-model:otp="passcode"
                    v-model:recovery="recoveryCode"
                    :loading="isLoading"
                    @verify="onLoginClick"
                />
                <VueHcaptcha
                    v-if="captchaConfig.hcaptcha.enabled"
                    ref="hcaptcha"
                    :sitekey="captchaConfig.hcaptcha.siteKey"
                    :re-captcha-compat="false"
                    size="invisible"
                    @verify="onCaptchaVerified"
                    @expired="onCaptchaError"
                    @error="onCaptchaError"
                />
                <p v-if="!isMFARequired" class="mt-7 text-center text-body-2">Forgot your password? <router-link class="link" to="/password-reset">Reset password</router-link></p>
                <p class="mt-5 text-center text-body-2">Don't have an account? <router-link class="link" to="/signup">Sign Up</router-link></p>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { VAlert, VBtn, VCard, VCardText, VCol, VContainer, VForm, VRow, VSelect, VTextField } from 'vuetify/components';
import { computed, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import VueHcaptcha from '@hcaptcha/vue3-hcaptcha';

import { EmailRule, RequiredRule, ValidationRule } from '@poc/types/common';
import { AuthHttpApi } from '@/api/auth';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useAppStore } from '@/store/modules/appStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useNotify } from '@/utils/hooks';
import { MultiCaptchaConfig } from '@/types/config.gen';
import { LocalData } from '@/utils/localData';
import { TokenInfo } from '@/types/users';
import { ErrorMFARequired } from '@/api/errors/ErrorMFARequired';
import { ErrorUnauthorized } from '@/api/errors/ErrorUnauthorized';
import { ErrorTooManyRequests } from '@/api/errors/ErrorTooManyRequests';
import { ErrorBadRequest } from '@/api/errors/ErrorBadRequest';

import Login2FA from '@poc/views/Login2FA.vue';
import PasswordInputEyeIcons from '@poc/components/PasswordInputEyeIcons.vue';

const auth = new AuthHttpApi();

const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();
const appStore = useAppStore();
const usersStore = useUsersStore();
const notify = useNotify();
const router = useRouter();
const route = useRoute();

const valid = ref(false);
const checked = ref(false);
const showPassword = ref(false);
const isLoading = ref<boolean>(false);
const isBadLoginMessageShown = ref<boolean>(false);
const formValid = ref<boolean>(false);
const inviteInvalid = ref(true);
const isActivatedBannerShown = ref(false);
const isActivatedError = ref(false);
const captchaError = ref(false);
const useOTP = ref(true);
const isMFARequired = ref(false);
const isMFAError = ref(false);

const captchaResponseToken = ref('');
const email = ref('');
const password = ref('');
const passcode = ref('');
const recoveryCode = ref('');
const pathEmail = ref<string | null>(null);
const returnURL = ref('/projects');

const hcaptcha = ref<VueHcaptcha | null>(null);
const form = ref<VForm | null>(null);

const satellitesHints = [
    { satellite: 'US1', hint: 'Recommended for North and South America' },
    { satellite: 'EU1', hint: 'Recommended for Europe and Africa' },
    { satellite: 'AP1', hint: 'Recommended for Asia and Australia' },
];

const passwordRules: ValidationRule<string>[] = [
    RequiredRule,
];

const emailRules: ValidationRule<string>[] = [
    RequiredRule,
    EmailRule,
];

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
            window.location.href = satellite.address + configStore.optionalV2Path + '/login';
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
 * This component's captcha configuration.
 */
const captchaConfig = computed((): MultiCaptchaConfig => {
    return configStore.state.config.captcha.login;
});

/**
 * Handles captcha verification response.
 */
function onCaptchaVerified(response: string): void {
    captchaResponseToken.value = response;
    captchaError.value = false;
    login();
}

/**
 * Handles captcha error and expiry.
 */
function onCaptchaError(): void {
    captchaResponseToken.value = '';
    captchaError.value = true;
}

/**
 * Holds on login button click logic.
 */
async function onLoginClick(): Promise<void> {
    form.value?.validate();
    if (!formValid.value || isLoading.value) {
        return;
    }

    isLoading.value = true;
    if (hcaptcha.value && !captchaResponseToken.value) {
        hcaptcha.value?.execute();
        return;
    }

    await login();
}

/**
 * Performs login action.
 * Then changes location to project dashboard page.
 */
async function login(): Promise<void> {
    try {
        const tokenInfo: TokenInfo = await auth.token(email.value, password.value, captchaResponseToken.value, passcode.value, recoveryCode.value);
        LocalData.setSessionExpirationDate(tokenInfo.expiresAt);
    } catch (error) {
        if (hcaptcha.value) {
            hcaptcha.value?.reset();
            captchaResponseToken.value = '';
        }

        if (error instanceof ErrorMFARequired) {
            isMFARequired.value = true;
            isLoading.value = false;
            return;
        }

        if (isMFARequired.value && !(error instanceof ErrorTooManyRequests)) {
            if (error instanceof ErrorBadRequest || error instanceof ErrorUnauthorized) {
                notify.error(error.message);
            }

            isMFAError.value = true;
            isLoading.value = false;
            return;
        }

        if (error instanceof ErrorUnauthorized) {
            isBadLoginMessageShown.value = true;
            isLoading.value = false;
            return;
        }

        notify.notifyError(error);
        isLoading.value = false;
        return;
    }

    usersStore.login();
    isLoading.value = false;

    analyticsStore.pageVisit(returnURL.value);
    await router.push(returnURL.value);
}

/**
 * Lifecycle hook after initial render.
 * Makes activated banner visible on successful account activation.
 */
onMounted(() => {
    inviteInvalid.value = (route.query.invite_invalid as string ?? null) === 'true';
    pathEmail.value = route.query.email as string ?? null;
    if (pathEmail.value) {
        email.value = pathEmail.value.trim();
    }

    isActivatedBannerShown.value = !!route.query.activated;
    isActivatedError.value = route.query.activated === 'false';

    if (route.query.return_url) returnURL.value = route.query.return_url as string;
});
</script>