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
                            <li v-for="sat in partneredSatellites" :key="sat.id">
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
                <VueRecaptcha
                    v-if="recaptchaEnabled"
                    ref="captcha"
                    :sitekey="recaptchaSiteKey"
                    :load-recaptcha-script="true"
                    size="invisible"
                    @verify="onCaptchaVerified"
                    @error="onCaptchaError"
                />
                <VueHcaptcha
                    v-else-if="hcaptchaEnabled"
                    ref="captcha"
                    :sitekey="hcaptchaSiteKey"
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

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import VueRecaptcha from 'vue-recaptcha';
import VueHcaptcha from '@hcaptcha/vue-hcaptcha';

import { AuthHttpApi } from '@/api/auth';
import { RouteConfig } from '@/router';
import { PartneredSatellite } from '@/types/common';
import { Validator } from '@/utils/validation';
import { MetaUtils } from '@/utils/meta';
import { AnalyticsHttpApi } from '@/api/analytics';

import VInput from '@/components/common/VInput.vue';
import VButton from '@/components/common/VButton.vue';

import LogoIcon from '@/../static/images/logo.svg';
import InfoIcon from '@/../static/images/notifications/info.svg';
import CloseIcon from '@/../static/images/notifications/closeSmall.svg';
import SelectedCheckIcon from '@/../static/images/common/selectedCheck.svg';
import BottomArrowIcon from '@/../static/images/common/lightBottomArrow.svg';

// @vue/component
@Component({
    components: {
        VInput,
        VButton,
        BottomArrowIcon,
        SelectedCheckIcon,
        LogoIcon,
        InfoIcon,
        CloseIcon,
        VueRecaptcha,
        VueHcaptcha,
    },
})
export default class ForgotPassword extends Vue {
    private email = '';
    private emailError = '';
    private captchaResponseToken = '';
    private isLoading = false;
    private isPasswordResetExpired = false;
    private isMessageShowing = true;

    private readonly recaptchaEnabled: boolean = MetaUtils.getMetaContent('login-recaptcha-enabled') === 'true';
    private readonly recaptchaSiteKey: string = MetaUtils.getMetaContent('login-recaptcha-site-key');
    private readonly hcaptchaEnabled: boolean = MetaUtils.getMetaContent('login-hcaptcha-enabled') === 'true';
    private readonly hcaptchaSiteKey: string = MetaUtils.getMetaContent('login-hcaptcha-site-key');

    private readonly auth: AuthHttpApi = new AuthHttpApi();

    public readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    // tardigrade logic
    public isDropdownShown = false;

    public readonly loginPath: string = RouteConfig.Login.path;

    public $refs!: {
        captcha: VueRecaptcha | VueHcaptcha;
    };

    public mounted(): void {
        this.isPasswordResetExpired = this.$route.query.expired === 'true';
    }

    /**
     * Close the expiry message banner.
     */
    public closeMessage() {
        this.isMessageShowing = false;
    }

    /**
     * Sets the email field to the given value.
     */
    public setEmail(value: string): void {
        this.email = value.trim();
        this.emailError = '';
    }

    /**
     * Name of the current satellite.
     */
    public get satelliteName(): string {
        return this.$store.state.appStateModule.satelliteName;
    }

    /**
     * Information about partnered satellites, including name and signup link.
     */
    public get partneredSatellites(): PartneredSatellite[] {
        return this.$store.state.appStateModule.partneredSatellites;
    }

    /**
     * Toggles satellite selection dropdown visibility (Tardigrade).
     */
    public toggleDropdown(): void {
        this.isDropdownShown = !this.isDropdownShown;
    }

    /**
     * Closes satellite selection dropdown (Tardigrade).
     */
    public closeDropdown(): void {
        this.isDropdownShown = false;
    }

    /**
     * Handles captcha verification response.
     */
    public onCaptchaVerified(response: string): void {
        this.captchaResponseToken = response;
        this.onSendConfigurations();
    }

    /**
     * Handles captcha error.
     */
    public onCaptchaError(): void {
        this.captchaResponseToken = '';
        this.$notify.error('The captcha encountered an error. Please try again.', null);
    }

    /**
     * Sends recovery password email.
     */
    public async onSendConfigurations(): Promise<void> {
        let activeElement = document.activeElement;
        if (activeElement && activeElement.id === 'resetDropdown') return;

        if (this.isLoading || !this.validateFields()) {
            return;
        }

        if (this.$refs.captcha && !this.captchaResponseToken) {
            this.$refs.captcha.execute();
            return;
        }

        if (this.isDropdownShown) {
            this.isDropdownShown = false;
            return;
        }

        this.isLoading = true;

        try {
            await this.auth.forgotPassword(this.email, this.captchaResponseToken);
            await this.$notify.success('Please look for instructions in your email');
        } catch (error) {
            await this.$notify.error(error.message, null);
        }

        this.$refs.captcha?.reset();
        this.captchaResponseToken = '';
        this.isLoading = false;
    }

    /**
     * Changes location to Login route.
     */
    public onBackToLoginClick(): void {
        this.analytics.pageVisit(RouteConfig.Login.path);
        this.$router.push(RouteConfig.Login.path);
    }

    /**
     * Redirects to storj.io homepage.
     */
    public onLogoClick(): void {
        const homepageURL = MetaUtils.getMetaContent('homepage-url');
        window.location.href = homepageURL;
    }

    /**
     * Returns whether the email address is properly structured.
     */
    private validateFields(): boolean {
        const isEmailValid = Validator.email(this.email);

        if (!isEmailValid) {
            this.emailError = 'Invalid Email';
        }

        return isEmailValid;
    }
}
</script>

<style scoped lang="scss">
    .forgot-area {
        display: flex;
        flex-direction: column;
        justify-content: flex-start;
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
                        justify-content: start;
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

    @media screen and (max-width: 414px) {

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
