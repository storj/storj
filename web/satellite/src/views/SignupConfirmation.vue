// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container v-if="!codeActivationEnabled" class="fill-height" fluid>
        <v-row justify="center" align="center">
            <v-col class="text-center py-5" cols="12">
                <icon-blue-checkmark />
                <h2 class="my-3">You are almost ready to use Storj</h2>
                <p>
                    A verification email has been sent to your email
                    <span class="font-weight-bold">{{ userEmail }}</span>
                </p>
                <p>
                    Check your inbox to activate your account and get started.
                </p>
                <v-btn
                    class="mt-7"
                    size="large"
                    :disabled="secondsToWait !== 0"
                    :loading="isLoading"
                    @click="resendMail"
                >
                    <template v-if="secondsToWait !== 0">
                        Resend in {{ timeToEnableResendEmailButton }}
                    </template>
                    <template v-else>
                        Resend Verification Email
                    </template>
                </v-btn>
            </v-col>

            <v-col cols="12">
                <p class="text-center text-body-2">
                    Or <a
                        class="link"
                        href="https://supportdcs.storj.io/hc/en-us/requests/new"
                        target="_blank"
                        rel="noopener noreferrer"
                    >contact our support team</a>
                </p>
            </v-col>
            <v-col cols="12">
                <p class="text-center text-body-2"><router-link class="link" :to="ROUTES.Login.path">Go to login page</router-link></p>
            </v-col>
        </v-row>
    </v-container>
    <v-container v-else class="fill-height">
        <v-row align="top" justify="center">
            <v-col cols="12" sm="10" md="7" lg="5">
                <v-card title="Check your inbox" class="pa-2 pa-sm-7">
                    <v-card-text>
                        <p>Enter the 6 digit confirmation code you received in your email to verify your account:</p>
                        <v-card class="my-4" rounded="lg" color="secondary" variant="outlined">
                            <v-otp-input v-model="code" :loading="isLoading" :error="isError" autofocus class="my-2" />
                        </v-card>

                        <v-btn :disabled="code.length < 6 || isLoading" color="primary" size="large" block @click="verifyCode">
                            Verify Account
                        </v-btn>
                    </v-card-text>
                </v-card>
                <p class="pt-9 text-center text-body-2">
                    Didn't receive a verification email?
                    <a class="link" @click="resendMail">
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
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import {
    VBtn,
    VCard,
    VCardText,
    VCol,
    VContainer,
    VRow,
    VOtpInput,
} from 'vuetify/components';

import { useNotify } from '@/utils/hooks';
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

import IconBlueCheckmark from '@/components/icons/IconBlueCheckmark.vue';

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

const code = ref('');
const signupId = ref<string>(props.signupReqId || '');
const isError = ref(false);
const secondsToWait = ref<number>(30);
const intervalId = ref<ReturnType<typeof setInterval>>();

const userEmail = computed((): string => {
    return props.email || decodeURIComponent(route.query.email?.toString() || '') || '';
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
            signupId.value = await auth.resendEmail(email);
            code.value = '';
        } catch (error) {
            notify.notifyError(error);
        }

        startResendEmailCountdown();
    });
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
                notify.notifyError(new Error('Invalid code'));
                return;
            }
            notify.notifyError(error);
            isError.value = true;
            return;
        }

        analyticsStore.eventTriggered(AnalyticsEvent.USER_SIGN_UP);
        appStore.toggleHasJustLoggedIn(true);
        usersStore.login();
        analyticsStore.pageVisit(ROUTES.Projects.path);
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
