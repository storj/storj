// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="reset-area" @keyup.enter="onResetClick">
        <div class="reset-area__logo-wrapper">
            <LogoIcon class="reset-area__logo-wrapper_logo" @click="onLogoClick" />
        </div>
        <div class="reset-area__content-area">
            <div class="reset-area__content-area__container" :class="{'success': isSuccessfulPasswordResetShown}">
                <h1 v-if="!isSuccessfulPasswordResetShown" class="reset-area__content-area__container__title">Reset Password</h1>
                <template v-if="isSuccessfulPasswordResetShown">
                    <KeyIcon />
                    <h2 class="reset-area__content-area__container__title success">Success!</h2>
                    <p class="reset-area__content-area__container__sub-title">
                        You have successfully changed your password.
                    </p>
                </template>
                <template v-else-if="isMFARequired">
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
                    <div class="reset-area__content-area__container__input-wrapper">
                        <ConfirmMFAInput ref="mfaInput" :on-input="onConfirmInput" :is-error="isMFAError" :is-recovery="isRecoveryCodeState" />
                    </div>
                    <span v-if="!isRecoveryCodeState" class="reset-area__content-area__container__recovery" @click="setRecoveryCodeState">
                        Or use recovery code
                    </span>
                </template>
                <template v-else>
                    <p class="reset-area__content-area__container__message">Please enter your new password.</p>
                    <div class="reset-area__content-area__container__input-wrapper password">
                        <VInput
                            label="Password"
                            placeholder="Enter Password"
                            :error="passwordError"
                            is-password
                            @setData="setPassword"
                            @showPasswordStrength="showPasswordStrength"
                            @hidePasswordStrength="hidePasswordStrength"
                        />
                        <PasswordStrength
                            :password-string="password"
                            :is-shown="isPasswordStrengthShown"
                        />
                    </div>
                    <div class="reset-area__content-area__container__input-wrapper">
                        <VInput
                            label="Retype Password"
                            placeholder="Retype Password"
                            :error="repeatedPasswordError"
                            is-password
                            @setData="setRepeatedPassword"
                        />
                    </div>
                </template>
                <p v-if="!isSuccessfulPasswordResetShown" class="reset-area__content-area__container__button" @click.prevent="onResetClick">Reset Password</p>
                <span v-if="isMFARequired && !isSuccessfulPasswordResetShown" class="reset-area__content-area__container__cancel" :class="{ disabled: isLoading }" @click.prevent="onMFACancelClick">
                    Cancel
                </span>
            </div>
            <router-link :to="loginPath" class="reset-area__content-area__login-link">
                Back to Login
            </router-link>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { AuthHttpApi } from '@/api/auth';
import { ErrorMFARequired } from '@/api/errors/ErrorMFARequired';
import { RouteConfig } from '@/router';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { Validator } from '@/utils/validation';
import { MetaUtils } from '@/utils/meta';
import { AnalyticsHttpApi } from '@/api/analytics';
import { ErrorTokenExpired } from '@/api/errors/ErrorTokenExpired';

import PasswordStrength from '@/components/common/PasswordStrength.vue';
import VInput from '@/components/common/VInput.vue';
import ConfirmMFAInput from '@/components/account/mfa/ConfirmMFAInput.vue';

import KeyIcon from '@/../static/images/resetPassword/success.svg';
import LogoIcon from '@/../static/images/logo.svg';
import GreyWarningIcon from '@/../static/images/common/greyWarning.svg';

// @vue/component
@Component({
    components: {
        LogoIcon,
        VInput,
        PasswordStrength,
        KeyIcon,
        ConfirmMFAInput,
        GreyWarningIcon,
    },
})

export default class ResetPassword extends Vue {
    private token = '';
    private password = '';
    private repeatedPassword = '';
    private passcode = '';
    private recoveryCode = '';

    private passwordError = '';
    private repeatedPasswordError = '';
    private isLoading = false;
    private isMFARequired = false;
    private isMFAError = false;
    private isRecoveryCodeState = false;

    private readonly auth: AuthHttpApi = new AuthHttpApi();

    public isPasswordStrengthShown = false;

    public readonly loginPath: string = RouteConfig.Login.path;

    public readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    public $refs!: {
        mfaInput: ConfirmMFAInput;
    };

    /**
     * Lifecycle hook on component destroy.
     * Sets view to default state.
     */
    public beforeDestroy(): void {
        if (this.isSuccessfulPasswordResetShown) {
            this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_SUCCESSFUL_PASSWORD_RESET);
        }
    }

    /**
     * Lifecycle hook after initial render.
     * Initializes recovery token from route param
     * and redirects to login if token doesn't exist.
     */
    public mounted(): void {
        if (this.$route.query.token) {
            this.token = this.$route.query.token.toString();
        } else {
            this.analytics.pageVisit(RouteConfig.Login.path);
            this.$router.push(RouteConfig.Login.path);
        }
    }

    /**
     * Returns whether the successful password reset area is shown.
     */
    public get isSuccessfulPasswordResetShown() : boolean {
        return this.$store.state.appStateModule.viewsState.isSuccessfulPasswordResetShown;
    }

    /**
     * Validates input fields and requests password reset.
     */
    public async onResetClick(): Promise<void> {
        if (this.isLoading) {
            return;
        }

        this.isLoading = true;

        if (!this.validateFields()) {
            this.isLoading = false;

            return;
        }

        try {
            await this.auth.resetPassword(this.token, this.password, this.passcode.trim(), this.recoveryCode.trim());
            this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_SUCCESSFUL_PASSWORD_RESET);
        } catch (error) {
            this.isLoading = false;

            if (error instanceof ErrorMFARequired) {
                if (this.isMFARequired) this.isMFAError = true;
                this.isMFARequired = true;
                return;
            }

            if (error instanceof ErrorTokenExpired) {
                await this.$router.push(`${RouteConfig.ForgotPassword.path}?expired=true`);
                return;
            }

            if (this.isMFARequired) {
                this.isMFAError = true;
                return;
            }

            await this.$notify.error(error.message, null);
        }

        this.isLoading = false;
    }

    /**
     * Validates input values to satisfy expected rules.
     */
    private validateFields(): boolean {
        let isNoErrors = true;

        if (!Validator.password(this.password)) {
            this.passwordError = 'Invalid password';
            isNoErrors = false;
        }

        if (this.repeatedPassword !== this.password) {
            this.repeatedPasswordError = 'Password doesn\'t match';
            isNoErrors = false;
        }

        return isNoErrors;
    }

    /**
     * Makes password strength container visible.
     */
    public showPasswordStrength(): void {
        this.isPasswordStrengthShown = true;
    }

    /**
     * Hides password strength container.
     */
    public hidePasswordStrength(): void {
        this.isPasswordStrengthShown = false;
    }

    /**
     * Redirects to storj.io homepage.
     */
    public onLogoClick(): void {
        const homepageURL = MetaUtils.getMetaContent('homepage-url');
        window.location.href = homepageURL;
    }

    /**
     * Sets user's password field from value string.
     */
    public setPassword(value: string): void {
        this.password = value.trim();
        this.passwordError = '';
    }

    /**
     * Sets user's repeat password field from value string.
     */
    public setRepeatedPassword(value: string): void {
        this.repeatedPassword = value.trim();
        this.repeatedPasswordError = '';
    }

    /**
     * Sets page to recovery code state.
     */
    public setRecoveryCodeState(): void {
        this.isMFAError = false;
        this.passcode = '';
        this.$refs.mfaInput.clearInput();
        this.isRecoveryCodeState = true;
    }

    /**
     * Cancels MFA passcode input state.
     */
    public onMFACancelClick(): void {
        this.isMFARequired = false;
        this.isRecoveryCodeState = false;
        this.isMFAError = false;
        this.passcode = '';
        this.recoveryCode = '';
    }

    /**
     * Sets confirmation passcode value from input.
     */
    public onConfirmInput(value: string): void {
        this.isMFAError = false;

        this.isRecoveryCodeState ? this.recoveryCode = value : this.passcode = value;
    }
}
</script>

<style scoped lang="scss">
    .reset-area {
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

                &.success {
                    align-items: center;
                    text-align: center;
                }

                &__input-wrapper {
                    margin-top: 20px;

                    &.password {
                        position: relative;
                    }
                }

                &__title {
                    font-size: 24px;
                    margin: 10px 0;
                    letter-spacing: -0.1007px;
                    color: #252525;
                    font-family: 'font_bold', sans-serif;
                    font-weight: 800;

                    &.success {
                        font-size: 40px;
                        margin: 25px 0;
                    }
                }

                &__button {
                    font-family: 'font_regular', sans-serif;
                    font-weight: 700;
                    margin-top: 40px;
                    display: flex;
                    justify-content: center;
                    align-items: center;
                    background-color: #376fff;
                    border-radius: 50px;
                    color: #fff;
                    cursor: pointer;
                    width: 100%;
                    height: 48px;

                    &:hover {
                        background-color: #0059d0;
                    }
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

            &__login-link {
                font-family: 'font_medium', sans-serif;
                text-decoration: none;
                font-size: 14px;
                line-height: 18px;
                color: #376fff;
                margin-top: 50px;
            }
        }
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

    @media screen and (max-width: 750px) {

        .reset-area {

            &__content-area {

                &__container {
                    width: 100%;
                }
            }
        }
    }

    @media screen and (max-width: 414px) {

        .reset-area {

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
