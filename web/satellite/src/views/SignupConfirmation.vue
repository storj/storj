// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container v-if="!codeActivationEnabled" class="fill-height" fluid>
        <v-row justify="center" align="center">
            <v-col cols="12" sm="9" md="7" lg="5" xl="4" xxl="3">
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

                    <p class="text-body-2">
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
                <p class="text-center text-body-2"><router-link class="link font-weight-bold" :to="ROUTES.Login.path">Go to Login</router-link></p>
            </v-col>
        </v-row>
    </v-container>
    <v-container v-else class="fill-height">
        <v-row justify="center">
            <v-col cols="12" sm="9" md="7" lg="5" xl="4" xxl="3">
                <v-card title="Check your inbox" class="pa-2 pa-sm-7">
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
                        <p>Enter the 6 digit confirmation code you received in your email to verify your account:</p>
                        <v-form @submit.prevent="verifyCode">
                            <v-card class="my-4" rounded="lg" color="secondary" variant="outlined">
                                <v-otp-input
                                    :model-value="code"
                                    :error="isError"
                                    :disabled="isLoading"
                                    autofocus
                                    class="my-2"
                                    @update:model-value="onValueChange"
                                />
                            </v-card>

                            <v-btn
                                type="submit"
                                :disabled="code.length < 6 || isLoading"
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
                <p class="pt-9 text-center text-body-2">
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
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
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
import { MultiCaptchaConfig } from '@/types/config.gen';

const props = withDefaults(defineProps<{
    email?: string;
    signupReqId?: string;
}>(), {
    email: '',
    signupReqId: '',
});

const auth: AuthHttpApi = new AuthHttpApi();

const appStore = useAppStore();
const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();
const usersStore = useUsersStore();

const router = useRouter();
const route = useRoute();
const notify = useNotify();

const { isLoading, withLoading } = useLoading();

const captchaResponseToken = ref('');
const captchaError = ref(false);
const hcaptcha = ref<VueHcaptcha | null>(null);

const code = ref('');
const signupId = ref<string>(props.signupReqId || '');
const isUnauthorizedMessageShown = ref<boolean>(false);
const isError = ref(false);
const secondsToWait = ref<number>(30);
const intervalId = ref<ReturnType<typeof setInterval>>();

const userEmail = computed((): string => {
    return props.email || decodeURIComponent(route.query.email?.toString() || '') || '';
});

/**
 * Holds on resend email button click logic.
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
 * Returns true if signup activation code is enabled.
 */
const codeActivationEnabled = computed((): boolean => {
    // code activation is not available if this page was arrived at via a link.
    return configStore.state.config.signupActivationCodeEnabled && !!props.email;
});

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
        const email = userEmail.value;
        if (secondsToWait.value !== 0 || !email) {
            return;
        }

        try {
            signupId.value = await auth.resendEmail(email, captchaResponseToken.value);
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
            const tokenInfo = await auth.verifySignupCode(props.email, code.value, signupId.value);
            LocalData.setSessionExpirationDate(tokenInfo.expiresAt);
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

/**
 * Lifecycle hook after initial render.
 * Starts resend email button availability countdown.
 */
onMounted(() => {
    startResendEmailCountdown();
});

/**
 * Lifecycle hook before component destroying.
 * Resets interval.
 */
onBeforeUnmount(() => {
    clearInterval(intervalId.value);
});
</script>
