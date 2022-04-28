// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="login-area" @keyup.enter="onLogin">
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
            <div class="login-area__content-area__container">
                <div class="login-area__content-area__container__title-area">
                    <h1 class="login-area__content-area__container__title-area__title" aria-roledescription="sign-in-title">Sign In</h1>

                    <div class="login-area__expand" @click.stop="toggleDropdown">
                        <span class="login-area__expand__value">{{ satelliteName }}</span>
                        <BottomArrowIcon />
                        <div v-if="isDropdownShown" v-click-outside="closeDropdown" class="login-area__expand__dropdown">
                            <div class="login-area__expand__dropdown__item" @click.stop="closeDropdown">
                                <SelectedCheckIcon />
                                <span class="login-area__expand__dropdown__item__name">{{ satelliteName }}</span>
                            </div>
                            <a v-for="sat in partneredSatellites" :key="sat.id" class="login-area__expand__dropdown__item" :href="sat.address + '/login'">
                                {{ sat.name }}
                            </a>
                        </div>
                    </div>
                </div>
                <template v-if="!isMFARequired">
                    <div v-if="isBadLoginMessageShown" class="info-box error">
                        <div class="info-box__header">
                            <WarningIcon />
                            <h2 class="info-box__header__label">Invalid Credentials</h2>
                        </div>
                        <p class="info-box__message">
                            Your login credentials are incorrect. If you didn’t receive an activation email, click <router-link :to="activatePath" class="link">here</router-link>.
                        </p>
                    </div>
                    <div class="login-area__input-wrapper">
                        <HeaderlessInput
                            label="Email Address"
                            placeholder="user@example.com"
                            :error="emailError"
                            role-description="email"
                            @setData="setEmail"
                        />
                    </div>
                    <div class="login-area__input-wrapper">
                        <HeaderlessInput
                            label="Password"
                            placeholder="Password"
                            :error="passwordError"
                            is-password="true"
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
                <p class="login-area__content-area__container__button" :class="{ 'disabled-button': isLoading }" @click.prevent="onLogin">Sign In</p>
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

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import ConfirmMFAInput from '@/components/account/mfa/ConfirmMFAInput.vue';
import HeaderlessInput from '@/components/common/HeaderlessInput.vue';

import WarningIcon from '@/../static/images/accessGrants/warning.svg';
import GreyWarningIcon from '@/../static/images/common/greyWarning.svg';
import BottomArrowIcon from '@/../static/images/common/lightBottomArrow.svg';
import SelectedCheckIcon from '@/../static/images/common/selectedCheck.svg';
import LogoIcon from '@/../static/images/logo.svg';

import { AuthHttpApi } from '@/api/auth';
import { ErrorMFARequired } from '@/api/errors/ErrorMFARequired';
import { RouteConfig } from '@/router';
import { PartneredSatellite } from '@/types/common';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { AppState } from '@/utils/constants/appStateEnum';
import { Validator } from '@/utils/validation';
import { ErrorUnauthorized } from '@/api/errors/ErrorUnauthorized';

interface ClearInput {
    clearInput(): void;
}

// @vue/component
@Component({
    components: {
        HeaderlessInput,
        BottomArrowIcon,
        SelectedCheckIcon,
        LogoIcon,
        WarningIcon,
        GreyWarningIcon,
        ConfirmMFAInput,
    },
})
export default class Login extends Vue {
    private email = '';
    private password = '';
    private passcode = '';
    private recoveryCode = '';
    private isLoading = false;
    private emailError = '';
    private passwordError = '';

    private readonly auth: AuthHttpApi = new AuthHttpApi();

    public readonly forgotPasswordPath: string = RouteConfig.ForgotPassword.path;
    public returnURL: string = RouteConfig.ProjectDashboard.path;
    public isActivatedBannerShown = false;
    public isActivatedError = false;
    public isMFARequired = false;
    public isMFAError = false;
    public isRecoveryCodeState = false;
    public isBadLoginMessageShown = false;

    // Tardigrade logic
    public isDropdownShown = false;

    public readonly registerPath: string = RouteConfig.Register.path;
    public readonly activatePath: string = RouteConfig.Activate.path;

    public $refs!: {
        mfaInput: ConfirmMFAInput & ClearInput;
    };

    /**
     * Clears confirm MFA input.
     */
    public clearConfirmMFAInput(): void {
        this.$refs.mfaInput.clearInput();
    }

    /**
     * Lifecycle hook after initial render.
     * Makes activated banner visible on successful account activation.
     */
    public mounted(): void {
        this.isActivatedBannerShown = !!this.$route.query.activated;
        this.isActivatedError = this.$route.query.activated === 'false';

        this.returnURL = this.$route.query.return_url as string || this.returnURL;
    }

    /**
     * Reloads page.
     */
    public onLogoClick(): void {
        location.reload();
    }

    /**
     * Sets page to recovery code state.
     */
    public setRecoveryCodeState(): void {
        this.isMFAError = false;
        this.passcode = '';
        this.clearConfirmMFAInput();
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

        this.isRecoveryCodeState ? this.recoveryCode = value.trim() : this.passcode = value.trim();
    }

    /**
     * Sets email string on change.
     */
    public setEmail(value: string): void {
        this.email = value.trim();
        this.emailError = '';
    }

    /**
     * Sets password string on change.
     */
    public setPassword(value: string): void {
        this.password = value;
        this.passwordError = '';
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
     * Performs login action.
     * Then changes location to project dashboard page.
     */
    public async onLogin(): Promise<void> {
        if (this.isLoading) {
            return;
        }

        this.isLoading = true;

        if (!this.validateFields()) {
            this.isLoading = false;

            return;
        }

        try {
            await this.auth.token(this.email, this.password, this.passcode, this.recoveryCode);
        } catch (error) {
            if (error instanceof ErrorMFARequired) {
                if (this.isMFARequired) this.isMFAError = true;

                this.isMFARequired = true;
                this.isLoading = false;

                return;
            }

            if (this.isMFARequired) {
                this.isMFAError = true;
                this.isLoading = false;
                return;
            }

            if (error instanceof ErrorUnauthorized) {
                this.isBadLoginMessageShown = true;
                this.isLoading = false;
                return;
            }

            await this.$notify.error(error.message);
            this.isLoading = false;
            return;
        }

        await this.$store.dispatch(APP_STATE_ACTIONS.CHANGE_STATE, AppState.LOADING);
        this.isLoading = false;

        await this.$router.push(this.returnURL);
    }

    /**
     * Validates email and password input strings.
     */
    private validateFields(): boolean {
        let isNoErrors = true;

        if (!Validator.email(this.email)) {
            this.emailError = 'Invalid Email';
            isNoErrors = false;
        }

        if (!Validator.password(this.password)) {
            this.passwordError = 'Invalid Password';
            isNoErrors = false;
        }

        return isNoErrors;
    }
}
</script>

<style scoped lang="scss">
    .login-area {
        display: flex;
        flex-direction: column;
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
        }

        &__divider {
            margin: 0 20px;
            height: 22px;
            width: 2px;
            background-color: #acbace;
        }

        &__input-wrapper {
            margin-top: 20px;
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
                color: #acbace;
                margin-right: 10px;
                font-family: 'font_regular', sans-serif;
                font-weight: 700;
            }

            &__dropdown {
                position: absolute;
                top: 35px;
                left: 0;
                background-color: #fff;
                z-index: 1000;
                border: 1px solid #c5cbdb;
                box-shadow: 0 8px 34px rgba(161, 173, 185, 0.41);
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
                background-color: rgba(39, 174, 96, 0.1);
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
                padding: 60px 80px;
                background-color: #fff;
                width: 610px;
                border-radius: 20px;
                box-sizing: border-box;
                margin-bottom: 20px;

                &__title-area {
                    display: flex;
                    justify-content: space-between;
                    align-items: center;

                    &__title {
                        font-size: 24px;
                        line-height: 49px;
                        letter-spacing: -0.100741px;
                        color: #252525;
                        font-family: 'font_bold', sans-serif;
                        font-weight: 800;
                    }

                    &__satellite {
                        font-size: 16px;
                        line-height: 21px;
                        color: #848484;
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

            &__footer-item {
                margin-top: 30px;
                font-size: 14px;
            }
        }
    }

    .logo {
        cursor: pointer;
    }

    .disabled,
    .disabled-button {
        pointer-events: none;
        color: #acb0bc;
    }

    .disabled-button {
        background-color: #dadde5;
        border-color: #dadde5;
    }

    .link {
        color: #376fff;
        font-family: 'font_medium', sans-serif;
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

        .login-area {

            &__content-area {

                &__container {
                    width: 100%;
                    padding: 60px;
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

        .login-area {

            &__logo-wrapper {
                margin: 40px;
            }

            &__content-area {
                padding: 0;

                &__container {
                    padding: 0 20px 20px 20px;
                    background: transparent;
                }
            }
        }
    }
</style>
