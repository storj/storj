// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="activate-area">
        <div class="activate-area__logo-wrapper">
            <LogoIcon class="activate-area__logo-wrapper__logo" @click="onLogoClick" />
        </div>
        <div class="activate-area__content-area">
            <div v-if="isMessageShowing && isActivationExpired && !isResendSuccessShown" class="activate-area__content-area__message-banner">
                <div class="activate-area__content-area__message-banner__content">
                    <div class="activate-area__content-area__message-banner__content__left">
                        <InfoIcon class="activate-area__content-area__message-banner__content__left__icon" />
                        <span class="activate-area__content-area__message-banner__content__left__message">
                            The verification link you clicked on has expired. Request a new link.
                        </span>
                    </div>
                    <CloseIcon class="activate-area__content-area__message-banner__content__right" @click="closeMessage" />
                </div>
            </div>
            <RegistrationSuccess v-if="isResendSuccessShown" :email="email" />
            <div v-else class="activate-area__content-area__container">
                <h1 class="activate-area__content-area__container__title">Verify Account</h1>
                <p class="login-area__content-area__activation-banner__message">
                    If you haven’t verified your account yet, input your email to receive a new verification link. Make sure you’re signing in to the right satellite.
                </p>
                <div class="activate-area__content-area__container__input-wrapper">
                    <VInput
                        label="Email Address"
                        placeholder="user@example.com"
                        :error="emailError"
                        height="46px"
                        width="100%"
                        @setData="setEmail"
                    />
                </div>
                <v-button
                    class="activate-area__content-area__container__button"
                    width="100%"
                    height="48px"
                    label="Activate Account"
                    border-radius="8px"
                    :is-disabled="isLoading"
                    :on-press="onActivateClick"
                />
                <div class="activate-area__content-area__container__login-row">
                    <router-link :to="loginPath" class="activate-area__content-area__container__login-row__link">
                        Back to Login
                    </router-link>
                </div>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useRoute } from 'vue-router';

import { AuthHttpApi } from '@/api/auth';
import { RouteConfig } from '@/router';
import { Validator } from '@/utils/validation';
import { useNotify } from '@/utils/hooks';
import { useConfigStore } from '@/store/modules/configStore';

import RegistrationSuccess from '@/components/common/RegistrationSuccess.vue';
import VInput from '@/components/common/VInput.vue';
import VButton from '@/components/common/VButton.vue';

import LogoIcon from '@/../static/images/logo.svg';
import InfoIcon from '@/../static/images/notifications/info.svg';
import CloseIcon from '@/../static/images/notifications/closeSmall.svg';

const configStore = useConfigStore();
const notify = useNotify();
const route = useRoute();

const auth: AuthHttpApi = new AuthHttpApi();
const loginPath: string = RouteConfig.Login.path;

const email = ref<string>('');
const emailError = ref<string>('');
const isResendSuccessShown = ref<boolean>(false);
const isActivationExpired = ref<boolean>(false);
const isMessageShowing = ref<boolean>(true);
const isLoading = ref<boolean>(false);

/**
 * Close the expiry message banner.
 */
function closeMessage(): void {
    isMessageShowing.value = false;
}

/**
 * onActivateClick validates input fields and requests resending of activation email.
 */
async function onActivateClick(): Promise<void> {
    if (!Validator.email(email.value)) {
        emailError.value = 'Invalid email';
        return;
    }

    try {
        await auth.resendEmail(email.value);
        isResendSuccessShown.value = true;
    } catch (error) {
        notify.error(error.message, null);
    }
}

/**
 * setEmail sets the email property to the given value.
 */
function setEmail(value: string): void {
    email.value = value.trim();
    emailError.value = '';
}

/**
 * Redirects to storj.io homepage.
 */
function onLogoClick(): void {
    window.location.href = configStore.state.config.homepageURL;
}

onMounted((): void => {
    isActivationExpired.value = route.query.expired === 'true';
});
</script>

<style lang="scss" scoped>
    .activate-area {
        display: flex;
        flex-direction: column;
        justify-content: flex-start;
        align-items: center;
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

                &__input-wrapper {
                    margin-top: 20px;
                }

                &__title {
                    font-size: 24px;
                    margin: 10px 0;
                    color: #252525;
                    font-family: 'font_bold', sans-serif;
                }

                &__button {
                    margin-top: 40px;
                }

                &__login-row {
                    display: flex;
                    justify-content: center;
                    margin-top: 1.5rem;

                    &__link {
                        font-family: 'font_medium', sans-serif;
                        text-decoration: none;
                        font-size: 14px;
                        color: #0149ff;
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

        .activate-area {

            &__content-area {

                &__container {
                    width: 100%;
                    padding: 60px;
                }
            }
        }
    }

    @media screen and (width <= 414px) {

        .activate-area {

            &__logo-wrapper {
                margin: 40px;
            }

            &__content-area {
                padding: 0;

                &__container {
                    padding: 0 20px 20px;
                    background: transparent;
                }
            }
        }
    }
</style>
