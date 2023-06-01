// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="forgot-area" @keyup.enter="onSendConfigurations">
        <div class="forgot-area__logo-wrapper">
            <LogoIcon class="forgot-area__logo-wrapper__logo" @click="onLogoClick" />
        </div>
        <div class="forgot-area__content-area">
            <div v-if="isMessageShowing && isPasswordResetExpired" class="forgot-area__content-area__message-banner">
                <div class="forgot-area__content-area__message-banner__content">
                    <div class="forgot-area__content-area__message-banner__content__left">
                        <InfoIcon class="forgot-area__content-area__message-banner__content__left__icon" />
                        <span class="forgot-area__content-area__message-banner__content__left__message">
                            The password reset link you clicked on has expired. Request a new link.
                        </span>
                    </div>
                    <CloseIcon class="forgot-area__content-area__message-banner__content__right" @click="closeMessage" />
                </div>
            </div>
            <div class="forgot-area__content-area__container">
                <div class="forgot-area__content-area__container__title-area">
                    <h1 class="forgot-area__content-area__container__title-area__title">Reset Password</h1>
                    <div class="forgot-area__expand" @click.stop="toggleDropdown">
                        <button
                            id="resetDropdown"
                            type="button"
                            aria-haspopup="listbox"
                            aria-roledescription="satellites-dropdown"
                            :aria-expanded="isDropdownShown"
                            class="forgot-area__expand__value"
                        >
                            {{ satelliteName }}
                        </button>
                        <BottomArrowIcon />
                        <ul v-if="isDropdownShown" v-click-outside="closeDropdown" role="listbox" tabindex="-1" class="forgot-area__expand__dropdown">
                            <li class="forgot-area__expand__dropdown__item" @click.stop="closeDropdown">
                                <SelectedCheckIcon />
                                <span class="forgot-area__expand__dropdown__item__name">{{ satelliteName }}</span>
                            </li>
                            <li v-for="sat in partneredSatellites" :key="sat.name">
                                <a class="forgot-area__expand__dropdown__item" :href="sat.address + '/forgot-password'">
                                    {{ sat.name }}
                                </a>
                            </li>
                        </ul>
                    </div>
                </div>
                <p class="forgot-area__content-area__container__message">If you’ve forgotten your account password, you can reset it here. Make sure you’re signing in to the right satellite.</p>
                <div class="forgot-area__content-area__container__input-wrapper">
                    <VInput
                        label="Email Address"
                        placeholder="user@example.com"
                        :error="emailError"
                        @setData="setEmail"
                    />
                </div>
                <VueHcaptcha
                    v-if="captchaConfig.hcaptcha.enabled"
                    ref="captcha"
                    :sitekey="captchaConfig.hcaptcha.siteKey"
                    :re-captcha-compat="false"
                    size="invisible"
                    @verify="onCaptchaVerified"
                    @error="onCaptchaError"
                />
                <v-button
                    class="forgot-area__content-area__container__button"
                    width="100%"
                    height="48px"
                    label="Reset Password"
                    border-radius="8px"
                    :is-disabled="isLoading"
                    :on-press="onSendConfigurations"
                >
                    Reset Password
                </v-button>
                <div class="forgot-area__content-area__container__login-container">
                    <router-link :to="loginPath" class="forgot-area__content-area__container__login-container__link">
                        Back to Login
                    </router-link>
                </div>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useRoute } from 'vue-router';
import VueHcaptcha from '@hcaptcha/vue3-hcaptcha';

import { AuthHttpApi } from '@/api/auth';
import { RouteConfig } from '@/router';
import { Validator } from '@/utils/validation';
import { useNotify } from '@/utils/hooks';
import { MultiCaptchaConfig, PartneredSatellite } from '@/types/config';
import { useConfigStore } from '@/store/modules/configStore';

import VInput from '@/components/common/VInput.vue';
import VButton from '@/components/common/VButton.vue';

import LogoIcon from '@/../static/images/logo.svg';
import InfoIcon from '@/../static/images/notifications/info.svg';
import CloseIcon from '@/../static/images/notifications/closeSmall.svg';
import SelectedCheckIcon from '@/../static/images/common/selectedCheck.svg';
import BottomArrowIcon from '@/../static/images/common/lightBottomArrow.svg';

const configStore = useConfigStore();
const notify = useNotify();
const route = useRoute();

const auth: AuthHttpApi = new AuthHttpApi();
const loginPath: string = RouteConfig.Login.path;

const email = ref<string>('');
const emailError = ref<string>('');
const captchaResponseToken = ref<string>('');
const isLoading = ref<boolean>(false);
const isPasswordResetExpired = ref<boolean>(false);
const isMessageShowing = ref<boolean>(true);
const isDropdownShown = ref<boolean>(false);
const captcha = ref<VueHcaptcha>();

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
    return configStore.state.config.partneredSatellites;
});

/**
 * This component's captcha configuration.
 */
const captchaConfig = computed((): MultiCaptchaConfig => {
    return configStore.state.config.captcha.login;
});

/**
 * Close the expiry message banner.
 */
function closeMessage() {
    isMessageShowing.value = false;
}

/**
 * Sets the email field to the given value.
 */
function setEmail(value: string): void {
    email.value = value.trim();
    emailError.value = '';
}

/**
 * Toggles satellite selection dropdown visibility (Partnered).
 */
function toggleDropdown(): void {
    isDropdownShown.value = !isDropdownShown.value;
}

/**
 * Closes satellite selection dropdown (Partnered).
 */
function closeDropdown(): void {
    isDropdownShown.value = false;
}

/**
 * Handles captcha verification response.
 */
function onCaptchaVerified(response: string): void {
    captchaResponseToken.value = response;
    onSendConfigurations();
}

/**
 * Handles captcha error.
 */
function onCaptchaError(): void {
    captchaResponseToken.value = '';
    notify.error('The captcha encountered an error. Please try again.', null);
}

/**
 * Sends recovery password email.
 */
async function onSendConfigurations(): Promise<void> {
    const activeElement = document.activeElement;
    if (activeElement && activeElement.id === 'resetDropdown') return;

    if (isLoading.value || !validateFields()) {
        return;
    }

    if (captcha.value && !captchaResponseToken.value) {
        captcha.value.execute();
        return;
    }

    if (isDropdownShown.value) {
        closeDropdown();
        return;
    }

    isLoading.value = true;

    try {
        await auth.forgotPassword(email.value, captchaResponseToken.value);
        await notify.success('Please look for instructions in your email');
    } catch (error) {
        await notify.error(error.message, null);
    }

    captcha.value?.reset();
    captchaResponseToken.value = '';
    isLoading.value = false;
}

/**
 * Redirects to storj.io homepage.
 */
function onLogoClick(): void {
    window.location.href = configStore.state.config.homepageURL;
}

/**
 * Returns whether the email address is properly structured.
 */
function validateFields(): boolean {
    const isEmailValid = Validator.email(email.value);

    if (!isEmailValid) {
        emailError.value = 'Invalid Email';
    }

    return isEmailValid;
}

onMounted((): void => {
    isPasswordResetExpired.value = route.query.expired === 'true';
});
</script>

<style scoped lang="scss">
    .forgot-area {
        display: flex;
        flex-direction: column;
        justify-content: flex-start;
        font-family: 'font_regular', sans-serif;
        background-color: #f5f6fa;
        position: fixed;
        inset: 0;
        min-height: 100%;
        overflow-y: scroll;

        &__logo-wrapper {
            text-align: center;
            margin: 70px 0;

            &__logo {
                cursor: pointer;
                width: 207px;
                height: 37px;
            }
        }

        &__expand {
            display: flex;
            align-items: center;
            cursor: pointer;
            position: relative;

            &__value {
                font-size: 16px;
                line-height: 21px;
                color: #acbace;
                margin-right: 10px;
                font-family: 'font_regular', sans-serif;
                font-weight: 700;
                background: none;
                border: none;
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
                list-style-type: none;

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

                    &__name {
                        font-family: 'font_bold', sans-serif;
                        margin-left: 15px;
                        font-size: 14px;
                        line-height: 20px;
                        color: #7e8b9c;
                    }

                    &:hover {
                        background-color: #f2f2f6;
                    }
                }

                &__item:focus-visible {
                    outline: -webkit-focus-ring-color auto 1px;
                }
            }
        }

        &__content-area {
            width: 100%;
            padding: 0 20px;
            margin-bottom: 50px;
            display: flex;
            flex-direction: column;
            align-items: center;
            box-sizing: border-box;

            &__container {
                width: 610px;
                padding: 60px 80px;
                display: flex;
                flex-direction: column;
                background-color: #fff;
                border-radius: 20px;
                box-sizing: border-box;

                &__title-area {
                    display: flex;
                    justify-content: space-between;
                    align-items: center;

                    &__title {
                        font-size: 24px;
                        margin: 10px 0;
                        letter-spacing: -0.1007px;
                        color: #252525;
                        font-family: 'font_bold', sans-serif;
                        font-weight: 800;
                    }
                }

                &__input-wrapper {
                    margin-top: 20px;
                }

                &__button {
                    margin-top: 40px;
                }

                &__login-container {
                    display: flex;
                    justify-content: center;
                    margin-top: 1.5rem;

                    &__link {
                        font-family: 'font_medium', sans-serif;
                        text-decoration: none;
                        font-size: 14px;
                        line-height: 18px;
                        color: #0149ff;
                    }

                    &__link:focus {
                        text-decoration: underline !important;
                    }
                }
            }

            &__message-banner {
                padding: 1.5rem;
                border-radius: 0.6rem;
                width: 570px;
                margin-bottom: 2.5rem;
                background-color: #ffe0e7;
                border: 1px solid #ffc0cf;
                box-shadow: 0 7px 20px rgb(0 0 0 / 15%);
                color: #000;

                &__content {
                    display: flex;
                    justify-content: space-between;
                    align-items: center;

                    &__left {
                        display: flex;
                        align-items: center;
                        justify-content: flex-start;
                        gap: 1.5rem;

                        &__message {
                            font-size: 0.95rem;
                            line-height: 1.4px;
                            margin: 0;
                        }

                        &__icon {
                            fill: #ff458b;
                        }
                    }
                }
            }
        }
    }

    @media screen and (width <= 750px) {

        .forgot-area {

            &__content-area {

                &__container {
                    width: 100%;
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

        .forgot-area {

            &__logo-wrapper {
                margin: 40px;
            }

            &__content-area {
                padding: 0;

                &__container {
                    padding: 60px;
                    border-radius: 0;
                }
            }
        }
    }
</style>
