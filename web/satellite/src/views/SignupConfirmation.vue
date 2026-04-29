// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container v-if="!codeActivationEnabled" class="fill-height align-content-center" fluid>
        <v-row justify="center" align="center">
            <v-col cols="12" sm="9" md="7" lg="5" xl="5" xxl="5">
                <v-card class="pa-2 pa-sm-7">
                    <h2 class="mb-3">You are almost ready to use {{ configStore.brandName }}</h2>
                    <p>
                        A verification email has been sent to your email
                        <span class="font-weight-bold">{{ userEmail }}</span>
                    </p>
                    <p>
                        Check your inbox to activate your account and get started.
                    </p>
                    <v-btn
                        class="my-5"
                        size="large"
                        :disabled="secondsToWait !== 0"
                        :loading="isLoading"
                        @click="onResendClick"
                    >
                        <template v-if="secondsToWait !== 0">
                            Resend in {{ timeToEnableResendEmailButton }}
                        </template>
                        <template v-else>
                            Resend Verification Email
                        </template>
                    </v-btn>

                    <p class="text-body-medium">
                        Or <a
                            class="link"
                            :href="configStore.supportUrl"
                            target="_blank"
                            rel="noopener noreferrer"
                        >contact the {{ configStore.brandName }} support team</a>
                    </p>
                </v-card>
            </v-col>
            <v-col cols="12">
                <p class="text-center text-body-medium"><router-link class="link font-weight-bold" :to="ROUTES.Login.path">Go to Login</router-link></p>
            </v-col>
        </v-row>
    </v-container>
    <v-container v-else class="fill-height align-content-center">
        <v-row justify="center">
            <v-col cols="12" sm="9" md="7" lg="5" xl="5" xxl="5">
                <v-card :title="signupId ? 'Check your inbox' : 'Activate your account'" class="pa-2 pa-sm-7">
                    <v-card-text>
                        <v-alert
                            v-if="isUnauthorizedMessageShown"
                            variant="tonal"
                            color="error"
                            title="Invalid Code"
                            text="Account activation failed. If you are sure your code is correct, please check your email inbox for a notification with further instructions."
                            density="comfortable"
                            class="mt-1 mb-3"
                            border
                        />
                        <v-form v-model="formValid" @submit.prevent="onFormSubmit">
                            <v-card class="my-4 pa-6" rounded="lg" color="secondary" variant="outlined">
                                <template v-if="!queryEmail">
                                    <p class="mb-6">Enter the email address you used to sign up:</p>
                                    <v-text-field
                                        v-model="providedEmail"
                                        :class="{'mb-6': !!signupId}"
                                        label="Email address"
                                        placeholder="Enter your email"
                                        hide-details="auto"
                                        maxlength="72"
                                        name="email"
                                        type="email"
                                        :rules="emailRules"
                                        :disabled="!!signupId"
                                        :loading="isLoading"
                                        flat
                                        required
                                    />
                                </template>
                                <template v-if="signupId">
                                    <p class="mb-3">
                                        Enter the 6-digit confirmation code sent to
                                        <strong>{{ userEmail }}</strong>:
                                    </p>
                                    <v-otp-input
                                        :model-value="code"
                                        :error="isError"
                                        :disabled="isLoading"
                                        autofocus
                                        class="my-2"
                                        @update:model-value="onValueChange"
                                    />
                                </template>
                            </v-card>

                            <v-btn
                                v-if="!signupId"
                                type="submit"
                                :disabled="!formValid"
                                :loading="isLoading"
                                color="primary"
                                size="large"
                                block
                            >
                                Send Activation Code
                            </v-btn>
                            <v-btn
                                v-else
                                type="submit"
                                :disabled="code.length < 6"
                                :loading="isLoading"
                                color="primary"
                                size="large"
                                block
                            >
                                Verify Account
                            </v-btn>
                        </v-form>
                    </v-card-text>
                </v-card>
                <p v-if="signupId" class="pt-9 text-center text-body-medium">
                    Didn't receive a verification email?
                    <a class="link" @click="onResendClick">
                        <template v-if="secondsToWait !== 0">
                            Resend in {{ timeToEnableResendEmailButton }}
                        </template>
                        <template v-else>
                            Resend
                        </template>
                    </a>
                </p>
            </v-col>
        </v-row>
    </v-container>
    <VueHcaptcha
        v-if="captchaConfig?.hcaptcha.enabled"
        ref="hcaptcha"
        :sitekey="captchaConfig.hcaptcha.siteKey"
        :re-captcha-compat="false"
        size="invisible"
        @verify="onCaptchaVerified"
        @expired="onCaptchaError"
        @error="onCaptchaError"
    />
</template>

<script setup lang="ts">
import { computed, onBeforeMount, onBeforeUnmount, onMounted, ref } from 'vue';
import VueHcaptcha from '@hcaptcha/vue3-hcaptcha';
import { useRoute, useRouter } from 'vue-router';
import {
    VBtn,
    VCard,
    VCardText,
    VCol,
    VContainer,
    VForm,
    VRow,
    VOtpInput,
    VAlert,
    VTextField,
} from 'vuetify/components';

import { useNotify } from '@/composables/useNotify';
import { AuthHttpApi } from '@/api/auth';
import { useLoading } from '@/composables/useLoading';
import { useConfigStore } from '@/store/modules/configStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { LocalData } from '@/utils/localData';
import { ErrorUnauthorized } from '@/api/errors/ErrorUnauthorized';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useAppStore } from '@/store/modules/appStore';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { ROUTES } from '@/router';
import type { MultiCaptchaConfig } from '@/types/config.gen';
import { EmailRule, RequiredRule } from '@/types/common';

const auth: AuthHttpApi = new AuthHttpApi();

const appStore = useAppStore();
const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();
const usersStore = useUsersStore();

const router = useRouter();
const route = useRoute();
const notify = useNotify();

const { isLoading, withLoading } = useLoading();

const emailRules: ((_: string) => boolean | string)[] = [
    RequiredRule,
    (value) => EmailRule(value, true),
];

const queryEmail = ref<string>('');
const providedEmail = ref<string>('');
const signupId = ref<string>('');

const captchaResponseToken = ref<string>('');
const captchaError = ref<boolean>(false);
const hcaptcha = ref<VueHcaptcha | null>(null);

const formValid = ref<boolean>(false);
const code = ref('');
const isUnauthorizedMessageShown = ref<boolean>(false);
const isError = ref(false);
const secondsToWait = ref<number>(0);
const intervalId = ref<ReturnType<typeof setInterval>>();

const userEmail = computed<string>(() => queryEmail.value || providedEmail.value);

const codeActivationEnabled = computed((): boolean => {
    return configStore.state.config.signupActivationCodeEnabled;
});

/**
 * This component's captcha configuration.
 */
const captchaConfig = computed((): MultiCaptchaConfig | undefined => {
    return configStore.state.config.captcha?.registration;
});

/**
 * Returns the time left until the Resend Email button is enabled in mm:ss form.
 */
const timeToEnableResendEmailButton = computed((): string => {
    return `${Math.floor(secondsToWait.value / 60).toString().padStart(2, '0')}:${(secondsToWait.value % 60).toString().padStart(2, '0')}`;
});

/**
 * Dispatches form submit to the appropriate action based on current phase.
 */
function onFormSubmit(): void {
    if (!signupId.value) {
        onResendClick();
    } else {
        verifyCode();
    }
}

/**
 * Holds on resend/send email button click logic.
 */
async function onResendClick(): Promise<void> {
    if (hcaptcha.value && !captchaResponseToken.value) {
        hcaptcha.value?.execute();
        return;
    }

    resendMail();
}

/**
 * Handles captcha verification response.
 */
function onCaptchaVerified(response: string): void {
    captchaResponseToken.value = response;
    captchaError.value = false;
    resendMail();
}

/**
 * Handles captcha error and expiry.
 */
function onCaptchaError(): void {
    captchaResponseToken.value = '';
    captchaError.value = true;
}

/**
 * Resets timer blocking email resend button spamming.
 */
function startResendEmailCountdown(): void {
    secondsToWait.value = 30;

    intervalId.value = setInterval(() => {
        if (--secondsToWait.value <= 0) {
            clearInterval(intervalId.value);
        }
    }, 1000);
}

/**
 * Resend email if interval timer is expired.
 */
function resendMail(): void {
    withLoading(async () => {
        if (secondsToWait.value !== 0 || !userEmail.value) {
            return;
        }

        try {
            signupId.value = await auth.resendEmail(userEmail.value, captchaResponseToken.value);
            code.value = '';
        } catch (error) {
            notify.notifyError(error);
        }

        hcaptcha.value?.reset();
        captchaResponseToken.value = '';

        startResendEmailCountdown();
    });
}

function onValueChange(value: string) {
    const val = value.slice(0, 6);
    if (isNaN(+val)) {
        return;
    }
    code.value = val;
}

/**
 * Handles code verification.
 */
function verifyCode(): void {
    isError.value = false;
    if (code.value.length < 6 || code.value.length > 6) {
        isError.value = true;
        return;
    }
    withLoading(async () => {
        try {
            const tokenInfo = await auth.verifySignupCode(userEmail.value, code.value, signupId.value);
            LocalData.setSessionExpirationDate(tokenInfo.expiresAt);
            LocalData.removeSessionHasExpired();
        } catch (error) {
            if (error instanceof ErrorUnauthorized) {
                isUnauthorizedMessageShown.value = true;
                return;
            }
            notify.notifyError(error);
            isError.value = true;
            return;
        }

        analyticsStore.eventTriggered(AnalyticsEvent.USER_SIGN_UP);
        appStore.toggleHasJustLoggedIn(true);
        usersStore.login();
        await router.push(ROUTES.Projects.path);
    });
}

onBeforeMount(() => {
    const emailQueryParam = route.query.email;
    queryEmail.value = typeof emailQueryParam === 'string' ? emailQueryParam : '';

    const signupIDQueryParam = route.query.signupReqId;
    signupId.value = typeof signupIDQueryParam === 'string' ? signupIDQueryParam : '';
});

/**
 * Lifecycle hook after initial render.
 * Starts resend countdown when an email was already sent: legacy mode (link email)
 * or code mode arriving fresh from signup (signupId known).
 */
onMounted(() => {
    if (!codeActivationEnabled.value || signupId.value) {
        startResendEmailCountdown();
    }
});

/**
 * Lifecycle hook before component destroying.
 * Resets interval.
 */
onBeforeUnmount(() => {
    clearInterval(intervalId.value);
});
</script>
