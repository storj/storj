// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="login-area" @keyup.enter="onLoginClick">
        <div class="login-area__logo-wrapper">
            <LogoIcon class="logo" @click="onLogoClick" />
        </div>
        <div class="login-area__content-area">
            <div v-if="isActivatedBannerShown" class="login-area__content-area__activation-banner" :class="{'error': isActivatedError}">
                <p class="login-area__content-area__activation-banner__message">
                    <template v-if="!isActivatedError"><b>Success!</b> Account verified.</template>
                    <template v-else><b>Oops!</b> This account has already been verified.</template>
                </p>
            </div>
            <div v-if="inviteInvalid" class="login-area__content-area__activation-banner error">
                <p class="login-area__content-area__activation-banner__message">
                    <b>Oops!</b> The invite link you used has expired or is invalid.
                </p>
            </div>
            <div class="login-area__content-area__container">
                <div class="login-area__content-area__container__title-area">
                    <h1 class="login-area__content-area__container__title-area__title" aria-roledescription="sign-in-title">Sign In</h1>

                    <div class="login-area__expand" @click.stop="toggleDropdown">
                        <button
                            id="loginDropdown"
                            type="button"
                            aria-haspopup="listbox"
                            aria-roledescription="satellites-dropdown"
                            :aria-expanded="isDropdownShown"
                            class="login-area__expand__value"
                        >
                            {{ satelliteName }}
                        </button>
                        <BottomArrowIcon />
                        <ul v-if="isDropdownShown" v-click-outside="closeDropdown" tabindex="-1" role="listbox" class="login-area__expand__dropdown">
                            <li key="0" tabindex="0" role="option" class="login-area__expand__dropdown__item" @click.stop="closeDropdown">
                                <SelectedCheckIcon />
                                <span class="login-area__expand__dropdown__item__name">{{ satelliteName }}</span>
                            </li>
                            <li
                                v-for="(sat, index) in partneredSatellites"
                                :key="index + 1"
                                role="option"
                                tabindex="0"
                                :data-value="sat.name"
                                class="login-area__expand__dropdown__item"
                                @click="clickSatellite(sat.address)"
                                @keypress.enter="clickSatellite(sat.address)"
                            >
                                {{ sat.name }}
                            </li>
                        </ul>
                    </div>
                </div>
                <template v-if="!isMFARequired">
                    <div v-if="isBadLoginMessageShown" class="info-box error">
                        <div class="info-box__header">
                            <WarningIcon />
                            <h2 class="info-box__header__label">Invalid Credentials</h2>
                        </div>
                        <p class="info-box__message">
                            Login failed. Please check if this is the correct satellite for your account. If you are
                            sure your credentials are correct, please check your email inbox for a notification with
                            further instructions.
                        </p>
                    </div>
                    <div class="login-area__input-wrapper">
                        <VInput
                            label="Email Address"
                            placeholder="user@example.com"
                            :init-value="email"
                            :disabled="!!pathEmail"
                            :error="emailError"
                            role-description="email"
                            @setData="setEmail"
                        />
                    </div>
                    <div class="login-area__input-wrapper">
                        <VInput
                            label="Password"
                            placeholder="Password"
                            :error="passwordError"
                            is-password
                            :autocomplete="autocompleteValue"
                            role-description="password"
                            @setData="setPassword"
                        />
                    </div>
                </template>
                <template v-else>
                    <div class="info-box">
                        <div class="info-box__header">
                            <GreyWarningIcon />
                            <h2 class="info-box__header__label">
                                Two-Factor Authentication Required
                            </h2>
                        </div>
                        <p class="info-box__message">
                            You'll need the six-digit code from your authenticator app to continue.
                        </p>
                    </div>
                    <div class="login-area__input-wrapper">
                        <ConfirmMFAInput ref="mfaInput" :on-input="onConfirmInput" :is-error="isMFAError" :is-recovery="isRecoveryCodeState" />
                    </div>
                    <span v-if="!isRecoveryCodeState" class="login-area__content-area__container__recovery" @click="setRecoveryCodeState">
                        Or use recovery code
                    </span>
                </template>
                <div v-if="captchaConfig.hcaptcha.enabled" class="login-area__content-area__container__captcha-wrapper">
                    <div v-if="captchaError" class="login-area__content-area__container__captcha-wrapper__label-container">
                        <ErrorIcon />
                        <p class="login-area__content-area__container__captcha-wrapper__label-container__error">HCaptcha is required</p>
                    </div>
                    <VueHcaptcha
                        ref="hcaptcha"
                        :sitekey="captchaConfig.hcaptcha.siteKey"
                        :re-captcha-compat="false"
                        size="invisible"
                        @verify="onCaptchaVerified"
                        @expired="onCaptchaError"
                        @error="onCaptchaError"
                    />
                </div>
                <v-button
                    class="login-area__content-area__container__button"
                    width="100%"
                    height="48px"
                    label="Sign In"
                    border-radius="6px"
                    :is-disabled="isLoading"
                    :on-press="onLoginClick"
                />
                <span v-if="isMFARequired" class="login-area__content-area__container__cancel" :class="{ disabled: isLoading }" @click.prevent="onMFACancelClick">
                    Cancel
                </span>
            </div>
            <p class="login-area__content-area__footer-item">
                Forgot your sign in details?
                <router-link :to="forgotPasswordPath" class="link">
                    Reset Password
                </router-link>
            </p>
            <router-link :to="registerPath" class="login-area__content-area__footer-item link">
                Need to create an account?
            </router-link>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import VueHcaptcha from '@hcaptcha/vue3-hcaptcha';
import { useRoute, useRouter } from 'vue-router';

import { AuthHttpApi } from '@/api/auth';
import { ErrorMFARequired } from '@/api/errors/ErrorMFARequired';
import { RouteConfig } from '@/types/router';
import { FetchState } from '@/utils/constants/fetchStateEnum';
import { Validator } from '@/utils/validation';
import { ErrorUnauthorized } from '@/api/errors/ErrorUnauthorized';
import { ErrorBadRequest } from '@/api/errors/ErrorBadRequest';
import { ErrorTooManyRequests } from '@/api/errors/ErrorTooManyRequests';
import { TokenInfo } from '@/types/users';
import { LocalData } from '@/utils/localData';
import { useNotify } from '@/utils/hooks';
import { useUsersStore } from '@/store/modules/usersStore';
import { useAppStore } from '@/store/modules/appStore';
import { useConfigStore } from '@/store/modules/configStore';
import { MultiCaptchaConfig, PartneredSatellite } from '@/types/config';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import VButton from '@/components/common/VButton.vue';
import VInput from '@/components/common/VInput.vue';
import ConfirmMFAInput from '@/components/account/mfa/ConfirmMFAInput.vue';

import ErrorIcon from '@/../static/images/register/ErrorInfo.svg';
import LogoIcon from '@/../static/images/logo.svg';
import SelectedCheckIcon from '@/../static/images/common/selectedCheck.svg';
import BottomArrowIcon from '@/../static/images/common/lightBottomArrow.svg';
import GreyWarningIcon from '@/../static/images/common/greyWarning.svg';
import WarningIcon from '@/../static/images/accessGrants/warning.svg';

interface ClearInput {
    clearInput(): void;
}

const email = ref('');
const password = ref('');
const passcode = ref('');
const recoveryCode = ref('');
const isLoading = ref(false);
const emailError = ref('');
const passwordError = ref('');
const captchaError = ref(false);
const captchaResponseToken = ref('');
const isActivatedBannerShown = ref(false);
const isActivatedError = ref(false);
const isMFARequired = ref(false);
const isMFAError = ref(false);
const isRecoveryCodeState = ref(false);
const isBadLoginMessageShown = ref(false);
const isDropdownShown = ref(false);

const pathEmail = ref<string | null>(null);
const inviteInvalid = ref(false);

const returnURL = ref(RouteConfig.AllProjectsDashboard.path);

const hcaptcha = ref<VueHcaptcha | null>(null);
const mfaInput = ref<typeof ConfirmMFAInput & ClearInput | null>(null);

const forgotPasswordPath: string = RouteConfig.ForgotPassword.path;
const registerPath: string = RouteConfig.Register.path;

const auth = new AuthHttpApi();

const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();
const appStore = useAppStore();
const usersStore = useUsersStore();
const notify = useNotify();
const router = useRouter();
const route = useRoute();

/**
 * Returns formatted autocomplete value.
 */
const autocompleteValue = computed((): string => {
    return `section-${satelliteName.value.substring(0, 2).toLowerCase()} current-password`;
});

/**
 * Name of the current satellite.
 */
const satelliteName = computed((): string => {
    return configStore.state.config.satelliteName;
});

/**
 * Information about partnered satellites, including name and signup link.
 */
const partneredSatellites = computed((): PartneredSatellite[] => {
    const satellites = configStore.state.config.partneredSatellites;
    return satellites.filter(s => s.name !== satelliteName.value);
});

/**
 * This component's captcha configuration.
 */
const captchaConfig = computed((): MultiCaptchaConfig => {
    return configStore.state.config.captcha.login;
});

/**
 * Lifecycle hook after initial render.
 * Makes activated banner visible on successful account activation.
 */
onMounted(() => {
    inviteInvalid.value = (route.query.invite_invalid as string ?? null) === 'true';
    pathEmail.value = route.query.email as string ?? null;
    if (pathEmail.value) {
        setEmail(pathEmail.value);
    }

    isActivatedBannerShown.value = !!route.query.activated;
    isActivatedError.value = route.query.activated === 'false';

    if (route.query.return_url) returnURL.value = route.query.return_url as string;
});

/**
 * Clears confirm MFA input.
 */
function clearConfirmMFAInput(): void {
    mfaInput.value?.clearInput();
}

/**
 * Redirects to storj.io homepage.
 */
function onLogoClick(): void {
    const homepageURL = configStore.state.config.homepageURL;
    if (homepageURL) window.location.href = homepageURL;
}

/**
 * Sets page to recovery code state.
 */
function setRecoveryCodeState(): void {
    isMFAError.value = false;
    passcode.value = '';
    clearConfirmMFAInput();
    isRecoveryCodeState.value = true;
}

/**
 * Cancels MFA passcode input state.
 */
function onMFACancelClick(): void {
    isMFARequired.value = false;
    isRecoveryCodeState.value = false;
    isMFAError.value = false;
    passcode.value = '';
    recoveryCode.value = '';
}

/**
 * Sets confirmation passcode value from input.
 */
function onConfirmInput(value: string): void {
    isMFAError.value = false;

    isRecoveryCodeState.value ? recoveryCode.value = value.trim() : passcode.value = value.trim();
}

/**
 * Sets email string on change.
 */
function setEmail(value: string): void {
    email.value = value.trim();
    emailError.value = '';
}

/**
 * Sets password string on change.
 */
function setPassword(value: string): void {
    password.value = value;
    passwordError.value = '';
}

/**
 * Redirects to chosen satellite.
 */
function clickSatellite(address: string): void {
    window.location.href = address + '/login';
}

/**
 * Toggles satellite selection dropdown visibility (Tardigrade).
 */
function toggleDropdown(): void {
    if (pathEmail.value) {
        // this page was opened from an email link, so don't allow satellite selection.
        return;
    }
    isDropdownShown.value = !isDropdownShown.value;
}

/**
 * Closes satellite selection dropdown (Tardigrade).
 */
function closeDropdown(): void {
    isDropdownShown.value = false;
}

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
    if (isLoading.value && !isDropdownShown.value) {
        return;
    }

    const activeElement = document.activeElement;

    if (activeElement && activeElement.id === 'loginDropdown') return;

    if (isDropdownShown.value) {
        isDropdownShown.value = false;
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
    if (!validateFields()) {
        isLoading.value = false;

        return;
    }

    try {
        const tokenInfo: TokenInfo = await auth.token(email.value, password.value, captchaResponseToken.value, passcode.value, recoveryCode.value);
        LocalData.setSessionExpirationDate(tokenInfo.expiresAt);
    } catch (error) {
        if (hcaptcha.value) {
            hcaptcha.value?.reset();
            captchaResponseToken.value = '';
        }

        if (error instanceof ErrorMFARequired) {
            if (isMFARequired.value) isMFAError.value = true;

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
    appStore.changeState(FetchState.LOADING);
    isLoading.value = false;

    analyticsStore.pageVisit(returnURL.value);
    await router.push(returnURL.value);
}

/**
 * Validates email and password input strings.
 */
function validateFields(): boolean {
    let isNoErrors = true;

    if (!Validator.email(email.value)) {
        emailError.value = 'Invalid Email';
        isNoErrors = false;
    }

    if (password.value.length < configStore.state.config.passwordMinimumLength) {
        passwordError.value = 'Invalid Password';
        isNoErrors = false;
    }

    return isNoErrors;
}
</script>

<style scoped lang="scss">
    .login-area {
        display: flex;
        flex-direction: column;
        font-family: 'font_regular', sans-serif;
        background-color: #F6F7FA;
        position: fixed;
        inset: 0;
        min-height: 100%;
        overflow-y: scroll;

        &__logo-wrapper {
            text-align: center;
            margin: 60px 0;
        }

        &__divider {
            margin: 0 20px;
            height: 22px;
            width: 2px;
            background-color: #acbace;
        }

        &__input-wrapper {
            width: 100%;
        }

        &__expand {
            display: flex;
            align-items: center;
            cursor: pointer;
            position: relative;

            &__value {
                font-size: 16px;
                line-height: 21px;
                color: #777;
                margin-right: 10px;
                font-family: 'font_regular', sans-serif;
                font-weight: 700;
                border: none;
                cursor: pointer;
                background: transparent;

                &:hover {
                    color: var(--c-blue-3);
                }
            }

            &__dropdown {
                position: absolute;
                top: 35px;
                left: 0;
                background-color: #fff;
                z-index: 1000;
                border: 1px solid #c5cbdb;
                box-shadow: 0 8px 34px rgb(161 173 185 / 41%);
                border-radius: 6px;
                min-width: 250px;

                &__item {
                    display: flex;
                    align-items: center;
                    justify-content: flex-start;
                    padding: 12px 25px;
                    font-size: 14px;
                    line-height: 20px;
                    color: #7e8b9c;
                    cursor: pointer;
                    text-decoration: none;
                    border-radius: 6px;

                    &__name {
                        font-family: 'font_bold', sans-serif;
                        margin-left: 15px;
                        font-size: 14px;
                        line-height: 20px;
                        color: #333;
                    }

                    &:hover {
                        background-color: #f2f2f6;
                    }
                }
            }
        }

        &__content-area {
            background-color: #f5f6fa;
            padding: 0 20px;
            margin-bottom: 50px;
            display: flex;
            flex-direction: column;
            align-items: center;
            border-radius: 20px;
            box-sizing: border-box;

            &__activation-banner {
                padding: 20px;
                background-color: rgb(39 174 96 / 10%);
                border: 1px solid #27ae60;
                color: #27ae60;
                border-radius: 6px;
                width: 570px;
                margin-bottom: 30px;

                &.error {
                    background-color: #fff3f2;
                    border: 1px solid #e30011;
                    color: #e30011;
                }

                &__message {
                    font-size: 16px;
                    line-height: 21px;
                    margin: 0;
                }
            }

            &__container {
                display: flex;
                flex-direction: column;
                padding: 26px 40px 40px;
                background-color: #fff;
                width: 460px;
                border-radius: 20px;
                box-sizing: border-box;
                margin-bottom: 20px;
                border: 1px solid #eee;

                &__title-area {
                    display: flex;
                    justify-content: space-between;
                    align-items: center;

                    &__title {
                        font-size: 21px;
                        line-height: 49px;
                        letter-spacing: -0.1px;
                        color: #091C45;
                        font-family: 'font_bold', sans-serif;
                        font-weight: 800;
                    }

                    &__satellite {
                        font-size: 16px;
                        line-height: 21px;
                        color: #848484;
                    }
                }

                &__captcha-wrapper__label-container {
                    margin-top: 30px;
                    display: flex;
                    justify-content: flex-start;
                    align-items: flex-end;
                    padding-bottom: 8px;

                    &__error {
                        font-size: 16px;
                        margin-left: 10px;
                        color: #ff5560;
                    }
                }

                &__button {
                    margin-top: 30px;
                }

                &__cancel {
                    align-self: center;
                    font-size: 16px;
                    line-height: 21px;
                    color: #0068dc;
                    text-align: center;
                    margin-top: 30px;
                    cursor: pointer;
                }

                &__recovery {
                    font-size: 16px;
                    line-height: 19px;
                    color: #0068dc;
                    cursor: pointer;
                    margin-top: 20px;
                    text-align: center;
                    width: 100%;
                }
            }

            &__footer-item {
                margin-top: 30px;
                font-size: 14px;
            }

            &__footer-item:focus {
                text-decoration: underline !important;
            }
        }
    }

    .logo {
        cursor: pointer;
    }

    .disabled {
        pointer-events: none;
        color: #acb0bc;
    }

    .link {
        font-family: 'font_medium', sans-serif;
        color: var(--c-blue-3);
    }

    .link:hover {
        color: var(--c-blue-5);
    }

    .link:focus {
        text-decoration: underline !important;
    }

    .info-box {
        background-color: #f7f8fb;
        border-radius: 6px;
        padding: 20px;
        margin-top: 25px;
        width: 100%;
        box-sizing: border-box;

        &.error {
            background-color: #fff9f7;
            border: 1px solid #f84b00;
        }

        &__header {
            display: flex;
            align-items: center;

            &__label {
                font-family: 'font_bold', sans-serif;
                font-size: 16px;
                color: #1b2533;
                margin-left: 15px;
            }
        }

        &__message {
            font-size: 16px;
            color: #1b2533;
            margin-top: 10px;
        }
    }

    :deep(.grecaptcha-badge) {
        visibility: hidden;
    }

    @media screen and (width <= 750px) {

        .login-area {

            &__content-area {

                &__container {
                    width: 100%;
                    min-width: 360px;
                }
            }

            &__expand {

                &__dropdown {
                    left: -200px;
                }
            }
        }
    }

    @media screen and (width <= 414px) {

        .login-area {

            &__logo-wrapper {
                margin: 40px;
            }

            &__content-area {
                padding: 0;
            }
        }
    }
</style>
