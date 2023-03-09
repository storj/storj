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
                >
                    Reset Password
                </v-button>
                <div class="activate-area__content-area__container__login-row">
                    <router-link :to="loginPath" class="activate-area__content-area__container__login-row__link">
                        Back to Login
                    </router-link>
                </div>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { AuthHttpApi } from '@/api/auth';
import { RouteConfig } from '@/router';
import { Validator } from '@/utils/validation';
import { MetaUtils } from '@/utils/meta';

import RegistrationSuccess from '@/components/common/RegistrationSuccess.vue';
import VInput from '@/components/common/VInput.vue';
import VButton from '@/components/common/VButton.vue';

import LogoIcon from '@/../static/images/logo.svg';
import InfoIcon from '@/../static/images/notifications/info.svg';
import CloseIcon from '@/../static/images/notifications/closeSmall.svg';

// @vue/component
@Component({
    components: {
        LogoIcon,
        InfoIcon,
        CloseIcon,
        VButton,
        VInput,
        RegistrationSuccess,
    },
})
export default class ActivateAccount extends Vue {
    private email = '';
    private emailError = '';
    private isResendSuccessShown = false;
    private isActivationExpired = false;
    private isMessageShowing = true;
    private isLoading = false;

    public readonly loginPath: string = RouteConfig.Login.path;

    private readonly auth: AuthHttpApi = new AuthHttpApi();

    public mounted(): void {
        this.isActivationExpired = this.$route.query.expired === 'true';
    }

    /**
     * Close the expiry message banner.
     */
    public closeMessage() {
        this.isMessageShowing = false;
    }

    /**
     * onActivateClick validates input fields and requests resending of activation email.
     */
    public async onActivateClick(): Promise<void> {
        if (!Validator.email(this.email)) {
            this.emailError = 'Invalid email';
            return;
        }

        try {
            await this.auth.resendEmail(this.email);
            this.isResendSuccessShown = true;
        } catch (error) {
            this.$notify.error(error.message, null);
        }
    }

    /**
     * setEmail sets the email property to the given value.
     */
    public setEmail(value: string): void {
        this.email = value.trim();
        this.emailError = '';
    }

    /**
     * Redirects to storj.io homepage.
     */
    public onLogoClick(): void {
        const homepageURL = MetaUtils.getMetaContent('homepage-url');
        window.location.href = homepageURL;
    }
}
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
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
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

    @media screen and (max-width: 750px) {

        .activate-area {

            &__content-area {

                &__container {
                    width: 100%;
                    padding: 60px;
                }
            }
        }
    }

    @media screen and (max-width: 414px) {

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
