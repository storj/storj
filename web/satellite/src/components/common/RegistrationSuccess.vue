// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="register-success-container">
        <div class="register-success-container__logo-wrapper">
            <LogoIcon class="logo" @click="onLogoClick" />
        </div>
        <div class="register-success-area">
            <div class="register-success-area__form-container">
                <MailIcon />
                <h2 class="register-success-area__form-container__title" aria-roledescription="title">You're almost there!</h2>
                <div v-if="showManualActivationMsg" class="register-success-area__form-container__sub-title">
                    If an account with the email address
                    <p class="register-success-area__form-container__sub-title__email">{{ userEmail }}</p>
                    exists, a verification email has been sent.
                </div>
                <p class="register-success-area__form-container__sub-title">
                    Check your inbox to activate your account and get started.
                </p>
                <p class="register-success-area__form-container__text">
                    Didn't receive a verification email?
                    <b class="register-success-area__form-container__verification-cooldown__bold-text">
                        {{ timeToEnableResendEmailButton }}
                    </b>
                </p>
                <div class="register-success-area__form-container__button-container">
                    <VButton
                        label="Resend Email"
                        width="450px"
                        height="50px"
                        :on-press="onResendEmailButtonClick"
                        :is-disabled="secondsToWait !== 0"
                    />
                </div>
                <p class="register-success-area__form-container__contact">
                    or
                    <a
                        class="register-success-area__form-container__contact__link"
                        href="https://supportdcs.storj.io/hc/en-us/requests/new"
                        target="_blank"
                        rel="noopener noreferrer"
                    >
                        Contact our support team
                    </a>
                </p>
            </div>
        </div>
        <router-link :to="loginPath" class="register-success-area__login-link">Go to Login page</router-link>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import { AuthHttpApi } from '@/api/auth';
import { RouteConfig } from '@/router';

import VButton from '@/components/common/VButton.vue';

import LogoIcon from '@/../static/images/logo.svg';
import MailIcon from '@/../static/images/register/mail.svg';

// @vue/component
@Component({
    components: {
        VButton,
        LogoIcon,
        MailIcon,
    },
})
export default class RegistrationSuccess extends Vue {
    @Prop({ default: '' })
    private readonly email: string;
    @Prop({ default: true })
    private readonly showManualActivationMsg: boolean;

    private secondsToWait = 30;
    private intervalID: ReturnType<typeof setInterval>;

    private readonly auth: AuthHttpApi = new AuthHttpApi();

    public readonly loginPath: string = RouteConfig.Login.path;

    /**
     * Lifecycle hook after initial render.
     * Starts resend email button availability countdown.
     */
    public mounted(): void {
        this.startResendEmailCountdown();
    }

    /**
     * Lifecycle hook before component destroying.
     * Resets interval.
     */
    public beforeDestroy(): void {
        if (this.intervalID) {
            clearInterval(this.intervalID);
        }
    }

    /**
     * Gets email (either passed in as prop or via query param).
     */
    public get userEmail(): string {
        return this.email || this.$route.query.email.toString();
    }

    /**
     * Reloads page.
     */
    public onLogoClick(): void {
        location.replace(RouteConfig.Register.path);
    }

    /**
     * Checks if page is inside iframe.
     */
    public get isInsideIframe(): boolean {
        return window.self !== window.top;
    }

    /**
     * Returns the time left until the Resend Email button is enabled in mm:ss form.
     */
    public get timeToEnableResendEmailButton(): string {
        return `${Math.floor(this.secondsToWait / 60).toString().padStart(2, '0')}:${(this.secondsToWait % 60).toString().padStart(2, '0')}`;
    }

    /**
     * Resend email if interval timer is expired.
     */
    public async onResendEmailButtonClick(): Promise<void> {
        const email = this.userEmail;
        if (this.secondsToWait != 0 || !email) {
            return;
        }

        try {
            await this.auth.resendEmail(email);
        } catch (error) {
            await this.$notify.error(error.message);
        }

        this.startResendEmailCountdown();
    }

    /**
     * Resets timer blocking email resend button spamming.
     */
    private startResendEmailCountdown(): void {
        this.secondsToWait = 30;

        this.intervalID = setInterval(() => {
            if (--this.secondsToWait <= 0) {
                clearInterval(this.intervalID);
            }
        }, 1000);
    }
}
</script>

<style scoped lang="scss">
    .register-success-container {
        display: flex;
        flex-direction: column;
        align-items: center;
        font-family: 'font_regular', sans-serif;
        background-color: #f5f6fa;
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        overflow-y: scroll;

        &__logo-wrapper {
            text-align: center;
            margin-top: 60px;

            svg {
                width: 207px;
                height: 37px;
            }
        }
    }

    .register-success-area {
        display: flex;
        align-items: center;
        justify-content: center;
        font-family: 'font_regular', sans-serif;
        background-color: #fff;
        border-radius: 20px;
        width: 75%;
        height: 50vh;
        margin-top: 50px;
        padding: 70px 90px 30px;
        max-width: 1200px;

        &__form-container {
            text-align: center;
            display: flex;
            flex-direction: column;
            align-items: center;

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

            &__contact {
                margin-top: 20px;

                &__link {
                    color: #376fff;

                    &:visited {
                        color: #376fff;
                    }
                }
            }
        }

        &__login-link {
            font-family: 'font_bold', sans-serif;
            text-decoration: none;
            font-size: 14px;
            color: #376fff;
            margin-top: 50px;
            padding-bottom: 50px;
        }
    }

    @media screen and (max-width: 650px) {

        .register-success-area {
            height: auto;

            &__form-container {
                padding: 50px;
            }
        }

        :deep(.container) {
            width: 100% !important;
        }
    }

    @media screen and (max-width: 500px) {

        .register-success-area {
            height: auto;

            &__form-container {
                padding: 50px 20px;
            }
        }
    }
</style>
