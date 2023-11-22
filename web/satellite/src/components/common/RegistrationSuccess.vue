// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="register-success-area">
        <div class="register-success-area__logo-wrapper">
            <LogoIcon class="logo" @click="onLogoClick" />
        </div>
        <div class="register-success-area__container">
            <MailIcon />
            <template v-if="codeActivationEnabled">
                <h2 class="register-success-area__container__title" aria-roledescription="title">Check your inbox</h2>
                <p class="register-success-area__container__sub-title">
                    Enter the 6 digit confirmation code you received in your email to verify your account:
                </p>
                <div class="register-success-area__container__code-input">
                    <ConfirmMFAInput label="Activation code" :on-input="onConfirmInput" :is-error="isError" />
                </div>
                <div v-if="codeActivationEnabled" class="register-success-area__container__button-container">
                    <VButton
                        label="Verify"
                        width="450px"
                        height="50px"
                        :on-press="onVerifyClicked"
                        :is-disabled="code.length !== 6 || isLoading"
                    />
                </div>
            </template>
            <template v-else>
                <h2 class="register-success-area__container__title" aria-roledescription="title">You're almost there!</h2>
                <div v-if="showManualActivationMsg" class="register-success-area__container__sub-title fill">
                    If an account with the email address
                    <p class="register-success-area__container__sub-title__email">{{ userEmail }}</p>
                    exists, a verification email has been sent.
                </div>
                <p class="register-success-area__container__sub-title">
                    Check your inbox to activate your account and get started.
                </p>
                <p class="register-success-area__container__text">
                    Didn't receive a verification email?
                    <b class="register-success-area__container__verification-cooldown__bold-text">
                        {{ timeToEnableResendEmailButton }}
                    </b>
                </p>
            </template>

            <div class="register-success-area__container__button-container">
                <VButton
                    :label="resendMailLabel"
                    width="450px"
                    height="50px"
                    :is-white="codeActivationEnabled"
                    :on-press="onResendEmailButtonClick"
                    :is-disabled="secondsToWait !== 0 || isLoading"
                />
            </div>
            <p class="register-success-area__container__contact">
                or
                <a
                    class="register-success-area__container__contact__link"
                    href="https://supportdcs.storj.io/hc/en-us/requests/new"
                    target="_blank"
                    rel="noopener noreferrer"
                >
                    Contact our support team
                </a>
            </p>
        </div>
        <router-link :to="loginPath" class="register-success-area__login-link">Go to Login page</router-link>
    </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { AuthHttpApi } from '@/api/auth';
import { RouteConfig } from '@/types/router';
import { useNotify } from '@/utils/hooks';
import { useConfigStore } from '@/store/modules/configStore';
import { useLoading } from '@/composables/useLoading';
import { LocalData } from '@/utils/localData';
import { useUsersStore } from '@/store/modules/usersStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { ErrorUnauthorized } from '@/api/errors/ErrorUnauthorized';

import ConfirmMFAInput from '@/components/account/mfa/ConfirmMFAInput.vue';
import VButton from '@/components/common/VButton.vue';

import MailIcon from '@/../static/images/register/mail.svg';
import LogoIcon from '@/../static/images/logo.svg';

const props = withDefaults(defineProps<{
    email?: string;
    signupReqId?: string;
    showManualActivationMsg?: boolean;
}>(), {
    email: '',
    signupReqId: '',
    showManualActivationMsg: true,
});

const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();
const usersStore = useUsersStore();

const router = useRouter();
const route = useRoute();
const notify = useNotify();

const { isLoading, withLoading } = useLoading();

const auth: AuthHttpApi = new AuthHttpApi();
const loginPath: string = RouteConfig.Login.path;

const secondsToWait = ref<number>(30);
const intervalId = ref<ReturnType<typeof setInterval>>();
const isError = ref<boolean>(false);
const code = ref<string>('');
const signupId = ref<string>(props.signupReqId || '');

const userEmail = computed((): string => {
    return props.email || route.query.email?.toString() || '';
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
    return  configStore.state.config.signupActivationCodeEnabled && !!props.email;
});

/**
 * Returns the text for the resend email button.
 */
const resendMailLabel = computed((): string => {
    return  !codeActivationEnabled.value ? 'Resend Email' : `Resend Email${secondsToWait.value !== 0 ? ' in ' + timeToEnableResendEmailButton.value : ''}`;
});

/**
 * Reloads page.
 */
function onLogoClick(): void {
    location.replace(RouteConfig.Register.path);
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
async function onResendEmailButtonClick(): Promise<void> {
    const email = userEmail.value;
    if (secondsToWait.value !== 0 || !email) {
        return;
    }

    try {
        signupId.value = await auth.resendEmail(email);
    } catch (error) {
        notify.notifyError(error);
    }

    startResendEmailCountdown();
}

/**
 * Handles code verification.
 */
function onVerifyClicked(): void {
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

        usersStore.login();
        analyticsStore.pageVisit(RouteConfig.AllProjectsDashboard.path);
        await router.push(RouteConfig.AllProjectsDashboard.path);
    });
}

/**
 * Sets confirmation passcode value from input.
 */
function onConfirmInput(value: string): void {
    isError.value = false;
    code.value = value;
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

<style scoped lang="scss">
    .register-success-area {
        display: flex;
        flex-direction: column;
        align-items: center;
        font-family: 'font_regular', sans-serif;
        background-color: #f5f6fa;
        padding: 0 20px;
        box-sizing: border-box;
        position: fixed;
        inset: 0;
        overflow-y: scroll;

        &__logo-wrapper {
            text-align: center;
            margin-top: 60px;
        }

        &__container {
            display: flex;
            flex-direction: column;
            align-items: center;
            box-sizing: border-box;
            text-align: center;
            background-color: var(--c-white);
            border-radius: 20px;
            width: 75%;
            margin-top: 50px;
            padding: 40px;

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 40px;
                line-height: 1.2;
                color: #252525;
                margin: 25px 0;
            }

            &__sub-title {
                font-size: 16px;
                line-height: 21px;
                color: #252525;
                margin: 0;
                max-width: 350px;
                text-align: center;
                margin-bottom: 27px;

                &.fill {
                    max-width: unset;
                }

                &__email {
                    font-family: 'font_bold', sans-serif;
                }
            }

            &__text {
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                line-height: 21px;
                color: #252525;
            }

            &__verification-cooldown {
                font-family: 'font_medium', sans-serif;
                font-size: 12px;
                line-height: 16px;
                padding: 27px 0 0;
                margin: 0;

                &__bold-text {
                    color: #252525;
                }
            }

            &__button-container {
                width: 100%;
                display: flex;
                justify-content: center;
                align-items: center;
                margin-top: 15px;
            }

            &__code-input {
                width: 450px;
                display: flex;
                justify-content: center;
                align-items: center;
                margin-top: 10px;
                margin-bottom: 10px;
            }

            &__contact {
                margin-top: 20px;

                &__link {
                    color: var(--c-light-blue-5);

                    &:visited {
                        color: var(--c-light-blue-5);
                    }
                }
            }
        }

        &__login-link {
            font-family: 'font_bold', sans-serif;
            text-decoration: none;
            font-size: 14px;
            color: var(--c-light-blue-5);
            margin-top: 50px;
            padding-bottom: 50px;
        }
    }

    @media screen and (width <= 750px) {

        .register-success-area__container {
            width: 100%;
            padding: 60px;
        }
    }
</style>
