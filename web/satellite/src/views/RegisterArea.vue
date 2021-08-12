// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="register-area" @keyup.enter="onCreateClick">
        <div class="register-area__logo-wrapper">
            <LogoIcon class="logo" @click="onLogoClick"/>
        </div>
        <div
            class="register-area__container"
            :class="{'professional-container': isProfessional}"
        >
            <div class="register-area__intro-area">
                <div class="register-area__intro-area__wrapper">
                    <h1 class="register-area__intro-area__title">Welcome to the decentralized cloud.</h1>
                    <p class="register-area__intro-area__sub-title">Join thousands of developers building on the safer, decentralized cloud, and start uploading data in just a few minutes.</p>
                </div>
                <RegisterGlobe
                    class="register-area__intro-area__globe-image"
                    :class="{'professional-globe': isProfessional}"
                />
                <RegisterGlobeSmall class="register-area__intro-area__globe-image-sm" />
            </div>
            <div class="register-area__input-area">
                <div
                    class="register-area__input-area__container"
                    :class="{ 'professional-container': isProfessional }"
                    v-if="!isRegistrationSuccessful"
                >
                    <div class="register-area__input-area__container__title-area">
                        <div class="register-area__input-area__container__title-container">
                            <h1 class="register-area__input-area__container__title-area__title">Get 150 GB Free</h1>
                        </div>
                        <div class="register-area__input-area__expand" @click.stop="toggleDropdown">
                            <span class="register-area__input-area__expand__value">{{ satelliteName }}</span>
                            <BottomArrowIcon />
                            <div class="register-area__input-area__expand__dropdown" v-if="isDropdownShown" v-click-outside="closeDropdown">
                                <div class="register-area__input-area__expand__dropdown__item" @click.stop="closeDropdown">
                                    <SelectedCheckIcon />
                                    <span class="register-area__input-area__expand__dropdown__item__name">{{ satelliteName }}</span>
                                </div>
                                <a v-for="sat in partneredSatellites" :key="sat.id" class="register-area__input-area__expand__dropdown__item" :href="sat.address + '/signup'">
                                    {{ sat.name }}
                                </a>
                            </div>
                        </div>
                    </div>
                    <div class="register-area__input-area__toggle__conatainer">
                        <ul class="register-area__input-area__toggle__wrapper">
                            <li
                                class="register-area__input-area__toggle__personal"
                                :class="{ 'active': !isProfessional }"
                                @click.prevent="toggleAccountType(false)"
                            >
                                Personal
                            </li>
                            <li
                                class="register-area__input-area__toggle__professional"
                                :class="{ 'active': isProfessional }"
                                @click.prevent="toggleAccountType(true)"
                            >
                                Business
                            </li>
                        </ul>
                    </div>
                    <div class="register-area__input-wrapper first-input">
                        <HeaderlessInput
                            class="full-input"
                            label="Full Name"
                            placeholder="Enter Full Name"
                            :error="fullNameError"
                            @setData="setFullName"
                            width="calc(100% - 2px)"
                            height="46px"
                        />
                    </div>
                    <div class="register-area__input-wrapper">
                        <HeaderlessInput
                            class="full-input"
                            label="Email Address"
                            placeholder="example@email.com"
                            :error="emailError"
                            @setData="setEmail"
                            width="calc(100% - 2px)"
                            height="46px"
                        />
                    </div>
                    <div v-if="isProfessional">
                        <div class="register-area__input-wrapper">
                            <HeaderlessInput
                                class="full-input"
                                label="Company Name"
                                placeholder="Acme Corp."
                                :error="companyNameError"
                                @setData="setCompanyName"
                                width="calc(100% - 2px)"
                                height="46px"
                            />
                        </div>
                        <div class="register-area__input-wrapper">
                            <HeaderlessInput
                                class="full-input"
                                label="Position"
                                placeholder="Position Title"
                                :error="positionError"
                                @setData="setPosition"
                                width="calc(100% - 2px)"
                                height="46px"
                            />
                        </div>
                        <div class="register-area__input-wrapper">
                            <SelectInput
                                class="full-input"
                                label="Employees"
                                @setData="setEmployeeCount"
                                width="calc(100% - 2px)"
                                height="46px"
                                :optionsList="employeeCountOptions"
                            />
                        </div>
                    </div>
                    <div class="register-input">
                        <div class="register-area__input-wrapper">
                            <HeaderlessInput
                                class="full-input"
                                label="Password"
                                placeholder="Enter Password"
                                :error="passwordError"
                                @setData="setPassword"
                                width="calc(100% - 2px)"
                                height="46px"
                                is-password="true"
                                @showPasswordStrength="showPasswordStrength"
                                @hidePasswordStrength="hidePasswordStrength"
                            />
                            <PasswordStrength
                                :password-string="password"
                                :is-shown="isPasswordStrengthShown"
                            />
                        </div>
                    </div>
                    <div class="register-area__input-wrapper">
                        <HeaderlessInput
                            class="full-input"
                            label="Retype Password"
                            placeholder="Retype Password"
                            :error="repeatedPasswordError"
                            @setData="setRepeatedPassword"
                            width="calc(100% - 2px)"
                            height="46px"
                            is-password="true"
                        />
                    </div>
                    <AddCouponCodeInput v-if="couponCodeSignupUIEnabled" />
                    <div v-if="isBetaSatellite" class="register-area__input-area__container__warning">
                        <div class="register-area__input-area__container__warning__header">
                            <label class="container">
                                <input type="checkbox" v-model="areBetaTermsAccepted">
                                <span class="checkmark" :class="{'error': areBetaTermsAcceptedError}"></span>
                            </label>
                            <h2 class="register-area__input-area__container__warning__header__label">
                                This is a BETA satellite
                            </h2>
                        </div>
                        <p class="register-area__input-area__container__warning__message">
                            This means any data you upload to this satellite can be
                            deleted at any time and your storage/bandwidth limits
                            can fluctuate. To use our production service please
                            create an account on one of our production Satellites.
                            <a href="https://storj.io/signup/" target="_blank" rel="noopener noreferrer">https://storj.io/signup/</a>
                        </p>
                    </div>
                    <div v-if="isProfessional" class="register-area__input-area__container__checkbox-area">
                        <label class="container">
                            <input id="sales" type="checkbox" v-model="haveSalesContact">
                            <span class="checkmark"></span>
                        </label>
                        <label class="register-area__input-area__container__checkbox-area__msg-box" for="sales">
                            <p class="register-area__input-area__container__checkbox-area__msg-box__msg">
                                Please have the Sales Team contact me
                            </p>
                        </label>
                    </div>
                    <div class="register-area__input-area__container__checkbox-area">
                        <label class="container">
                            <input id="terms" type="checkbox" v-model="isTermsAccepted">
                            <span class="checkmark" :class="{'error': isTermsAcceptedError}"></span>
                        </label>
                        <label class="register-area__input-area__container__checkbox-area__msg-box" for="terms">
                            <p class="register-area__input-area__container__checkbox-area__msg-box__msg">
                                I agree to the
                                <a class="register-area__input-area__container__checkbox-area__msg-box__msg__link" href="https://storj.io/terms-of-service/" target="_blank" rel="noopener">Terms of Service</a>
                                and
                                <a class="register-area__input-area__container__checkbox-area__msg-box__msg__link" href="https://storj.io/privacy-policy/" target="_blank" rel="noopener">Privacy Policy</a>
                            </p>
                        </label>
                    </div>
                    <div class="register-area__input-area__container__recaptcha-wrapper" v-if="recaptchaEnabled">
                        <div class="register-area__input-area__container__recaptcha-wrapper__label-container" v-if="recaptchaError">
                            <ErrorIcon/>
                            <p class="register-area__input-area__container__recaptcha-wrapper__label-container__error">reCAPTCHA is required</p>
                        </div>
                        <vue-recaptcha
                            :sitekey="recaptchaSiteKey"
                            loadRecaptchaScript="true"
                            @verify="onRecaptchaVerified"
                            @expired="onRecaptchaError"
                            @error="onRecaptchaError"
                            ref="recaptcha">
                        </vue-recaptcha>
                    </div>
                    <p class="register-area__input-area__container__button" @click.prevent="onCreateClick">Sign Up</p>
                </div>

                <RegistrationSuccess v-if="isRegistrationSuccessful"/>
            </div>
        </div>
        <div class="register-area__input-area__login-container">
            Already have an account? <router-link :to="loginPath" class="register-area__input-area__login-container__link">Login.</router-link>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import VueRecaptcha from 'vue-recaptcha';

import AddCouponCodeInput from '@/components/common/AddCouponCodeInput.vue';
import HeaderlessInput from '@/components/common/HeaderlessInput.vue';
import PasswordStrength from '@/components/common/PasswordStrength.vue';
import RegistrationSuccess from '@/components/common/RegistrationSuccess.vue';
import SelectInput from '@/components/common/SelectInput.vue';

import AuthIcon from '@/../static/images/AuthImage.svg';
import BottomArrowIcon from '@/../static/images/common/lightBottomArrow.svg';
import SelectedCheckIcon from '@/../static/images/common/selectedCheck.svg';
import LogoIcon from '@/../static/images/dcs-logo.svg';
import InfoIcon from '@/../static/images/info.svg';
import ErrorIcon from '@/../static/images/register/ErrorInfo.svg';
import RegisterGlobe from '@/../static/images/register/RegisterGlobe.svg';
import RegisterGlobeSmall from '@/../static/images/register/RegisterGlobeSmall.svg';

import { AuthHttpApi } from '@/api/auth';
import { RouteConfig } from '@/router';
import { PartneredSatellite } from '@/types/common';
import { User } from '@/types/users';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { LocalData } from '@/utils/localData';
import { MetaUtils } from '@/utils/meta';
import { Validator } from '@/utils/validation';

@Component({
    components: {
        HeaderlessInput,
        RegistrationSuccess,
        AuthIcon,
        BottomArrowIcon,
        ErrorIcon,
        SelectedCheckIcon,
        LogoIcon,
        InfoIcon,
        PasswordStrength,
        AddCouponCodeInput,
        SelectInput,
        RegisterGlobe,
        RegisterGlobeSmall,
        VueRecaptcha,
    },
})
export default class RegisterArea extends Vue {
    private readonly user = new User();

    // tardigrade logic
    private secret = '';

    private userId = '';
    private isTermsAccepted = false;
    private password = '';
    private repeatedPassword = '';

    // Only for beta sats (like US2).
    private areBetaTermsAccepted = false;
    private areBetaTermsAcceptedError = false;

    private fullNameError = '';
    private emailError = '';
    private passwordError = '';
    private repeatedPasswordError = '';
    private companyNameError = '';
    private employeeCountError = '';
    private positionError = '';
    private isTermsAcceptedError = false;
    private isLoading = false;
    private isProfessional = false;
    private haveSalesContact = false;

    private recaptchaError = false;
    private recaptchaResponseToken = '';

    private readonly auth: AuthHttpApi = new AuthHttpApi();

    private readonly recaptchaEnabled: boolean = MetaUtils.getMetaContent('recaptcha-enabled') === 'true';
    private readonly recaptchaSiteKey: string = MetaUtils.getMetaContent('recaptcha-site-key');

    public isPasswordStrengthShown = false;

    // tardigrade logic
    public isDropdownShown = false;

    // Employee Count dropdown options
    public employeeCountOptions = ['1-50', '51-1000', '1001+'];
    public optionsShown = false;

    public readonly loginPath: string = RouteConfig.Login.path;

    /**
     * Lifecycle hook on component destroy.
     * Sets view to default state.
     */
    public beforeDestroy(): void {
        if (this.isRegistrationSuccessful) {
            this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_SUCCESSFUL_REGISTRATION);
        }
    }

    /**
     * Lifecycle hook after initial render.
     * Sets up variables from route params.
     */
    public mounted(): void {
        if (this.$route.query.token) {
            this.secret = this.$route.query.token.toString();
        }

        if (this.$route.query.partner) {
            this.user.partner = this.$route.query.partner.toString();
        }
    }

    /**
     * Indicates if registration successful area shown.
     */
    public get isRegistrationSuccessful(): boolean {
        return this.$store.state.appStateModule.appState.isSuccessfulRegistrationShown;
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
     * Validates input fields and proceeds user creation.
     */
    public async onCreateClick(): Promise<void> {
        if (this.isLoading) {
            return;
        }

        this.isLoading = true;

        if (!this.validateFields()) {
            this.isLoading = false;

            return;
        }
        await this.createUser();

        this.isLoading = false;
    }

    /**
     * Reloads page.
     */
    public onLogoClick(): void {
        location.reload();
    }

    /**
     * Changes location to login route.
     */
    public onLoginClick(): void {
        this.$router.push(RouteConfig.Login.path);
    }

    /**
     * Sets user's email field from value string.
     */
    public setEmail(value: string): void {
        this.user.email = value.trim();
        this.emailError = '';
    }

    /**
     * Sets user's full name field from value string.
     */
    public setFullName(value: string): void {
        this.user.fullName = value.trim();
        this.fullNameError = '';
    }

    /**
     * Sets user's password field from value string.
     */
    public setPassword(value: string): void {
        this.user.password = value.trim();
        this.password = value;
        this.passwordError = '';
    }

    /**
     * Sets user's repeat password field from value string.
     */
    public setRepeatedPassword(value: string): void {
        this.repeatedPassword = value;
        this.repeatedPasswordError = '';
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
     * Indicates if satellite is in beta.
     */
    public get isBetaSatellite(): boolean {
        return this.$store.state.appStateModule.isBetaSatellite;
    }

    /**
     * Indicates if coupon code ui is enabled
     */
    public get couponCodeSignupUIEnabled(): boolean {
        return this.$store.state.appStateModule.couponCodeSigunpUIEnabled;
    }

    /**
     * Sets user's company name field from value string.
     */
    public setCompanyName(value: string): void {
        this.user.companyName = value.trim();
        this.companyNameError = '';
    }

    /**
     * Sets user's company size field from value string.
     */
    public setEmployeeCount(value: string): void {
        this.user.employeeCount = value;
        this.employeeCountError = '';
    }

    /**
     * Sets user's position field from value string.
     */
    public setPosition(value: string): void {
        this.user.position = value.trim();
        this.positionError = '';
    }

    /**
     * toggle user account type
     */
    public toggleAccountType(value: boolean): void {
        this.isProfessional = value;
    }

    /**
     * Handles reCAPTCHA verification response.
     */
    public onRecaptchaVerified(response: string): void {
        this.recaptchaResponseToken = response;
        this.recaptchaError = false;
    }

    /**
     * Handles reCAPTCHA error and expiry.
     */
    public onRecaptchaError(): void {
        this.recaptchaResponseToken = '';
    }

    /**
     * Validates input values to satisfy expected rules.
     */
    private validateFields(): boolean {
        let isNoErrors = true;

        if (!this.user.fullName.trim()) {
            this.fullNameError = 'Invalid Name';
            isNoErrors = false;
        }

        if (!Validator.email(this.user.email.trim())) {
            this.emailError = 'Invalid Email';
            isNoErrors = false;
        }

        if (!Validator.password(this.password)) {
            this.passwordError = 'Invalid Password';
            isNoErrors = false;
        }

        if (this.isProfessional) {

            if (!this.user.companyName.trim()) {
                this.companyNameError = 'No Company Name filled in';
                isNoErrors = false;
            }

            if (!this.user.position.trim()) {
                this.positionError = 'No Position filled in';
                isNoErrors = false;
            }

            if (!this.user.employeeCount.trim()) {
                this.employeeCountError = 'No Company Size filled in';
                isNoErrors = false;
            }

        }

        if (this.repeatedPassword !== this.password) {
            this.repeatedPasswordError = 'Password doesn\'t match';
            isNoErrors = false;
        }

        if (!this.isTermsAccepted) {
            this.isTermsAcceptedError = true;
            isNoErrors = false;
        }

        // only for beta US2 sats.
        if (this.isBetaSatellite && !this.areBetaTermsAccepted) {
            this.areBetaTermsAcceptedError = true;
            isNoErrors = false;
        }

        if (this.recaptchaEnabled && !this.recaptchaResponseToken) {
            this.recaptchaError = true;
            isNoErrors = false;
        }

        return isNoErrors;
    }

    /**
     * Creates user and toggles successful registration area visibility.
     */
    private async createUser(): Promise<void> {
        this.user.isProfessional = this.isProfessional;
        this.user.haveSalesContact = this.haveSalesContact;

        try {
            this.userId = await this.auth.register(this.user, this.secret, this.recaptchaResponseToken);
            LocalData.setUserId(this.userId);

            await this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_SUCCESSFUL_REGISTRATION);
        } catch (error) {
            if (this.$refs.recaptcha) {
                (this.$refs.recaptcha as VueRecaptcha).reset();
            }
            await this.$notify.error(error.message);
            this.isLoading = false;
        }
    }
}
</script>

<style scoped lang="scss">
    .register-area {
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
        }

        &__input-wrapper {
            margin-top: 20px;
        }

        &__input-wrapper.first-input {
            margin-top: 10px;
        }

        &__container {
            display: flex;
            background-color: #fff;
            border-radius: 20px;
            width: 75%;
            margin-top: 50px;
            padding: 70px 90px 30px 90px;
            max-width: 1200px;
        }

        &__intro-area {
            overflow: hidden;
            margin-bottom: -30px;

            &__wrapper {
                width: 80%;
            }

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 48px;
                font-style: normal;
                font-weight: 800;
                line-height: 59px;
                letter-spacing: 0;
                text-align: left;
                margin-bottom: 40px;
            }

            &__sub-title {
                font-size: 18px;
                font-style: normal;
                font-weight: 400;
                line-height: 30px;
                letter-spacing: -0.1007407009601593px;
                text-align: left;
            }

            &__globe-image {
                position: relative;
                top: 140px;
                left: 40px;
            }

            &__globe-image.professional-globe {
                top: 110px;
                left: 40px;
            }

            &__globe-image-sm {
                display: none;
            }
        }

        &__input-area {
            padding: 0 0 40px 20px;
            margin: 0 auto;
            width: 70%;

            &__expand {
                display: flex;
                align-items: center;
                cursor: pointer;
                position: relative;

                &__value {
                    font-family: 'font_normal', sans-serif;
                    font-weight: 700;
                    font-size: 16px;
                    line-height: 21px;
                    color: #afb7c1;
                    margin-right: 10px;
                }

                &__dropdown {
                    position: absolute;
                    top: 35px;
                    right: 0;
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

            &__toggle {

                &__wrapper {
                    display: flex;
                    justify-content: space-between;
                    margin: 20px 0 15px 0;
                    list-style: none;
                    padding: 0;
                }

                &__personal {
                    border-top-left-radius: 20px;
                    border-bottom-left-radius: 20px;
                    border-right: none;
                }

                &__professional {
                    border-top-right-radius: 20px;
                    border-bottom-right-radius: 20px;
                    border-left: none;
                    position: relative;
                    right: 1px;
                }

                &__personal,
                &__professional {
                    color: #376fff;
                    display: block;
                    width: 100%;
                    text-align: center;
                    padding: 8px;
                    border: 1px solid #376fff;
                    cursor: pointer;
                }

                &__personal.active,
                &__professional.active {
                    color: #fff;
                    background: #376fff;
                    font-weight: bold;
                }
            }

            &__container {

                &__title-area {
                    display: flex;
                    justify-content: space-between;
                    align-items: center;

                    &__title {
                        font-size: 24px;
                        line-height: 49px;
                        letter-spacing: -0.100741px;
                        color: #252525;
                        font-family: 'font_normal', sans-serif;
                        font-weight: 800;
                        white-space: nowrap;
                    }

                    &__satellite {
                        font-size: 16px;
                        line-height: 21px;
                        color: #848484;
                    }
                }

                &__warning {
                    margin-top: 30px;
                    padding: 15px;
                    width: calc(100% - 32px);
                    background: #fff9f7;
                    border: 1px solid #f84b00;
                    border-radius: 8px;

                    &__header {
                        display: flex;
                        align-items: center;

                        &__label {
                            font-style: normal;
                            font-family: 'font_bold', sans-serif;
                            font-size: 16px;
                            line-height: 19px;
                            color: #1b2533;
                            margin: 0;
                        }
                    }

                    &__message {
                        font-size: 16px;
                        line-height: 22px;
                        color: #1b2533;
                        margin: 8px 0 0 0;
                    }
                }

                &__checkbox-area {
                    display: flex;
                    align-items: center;
                    width: 100%;
                    margin-top: 30px;

                    &__msg-box {
                        font-size: 14px;
                        line-height: 20px;
                        color: #354049;

                        &__msg {
                            position: relative;
                            top: 2px;

                            &__link {
                                margin: 0 4px;
                                font-family: 'font_bold', sans-serif;
                                color: #000;
                                text-decoration: underline !important;

                                &:visited {
                                    color: inherit;
                                }
                            }
                        }
                    }
                }

                &__button {
                    font-family: 'font_normal', sans-serif;
                    font-weight: 700;
                    margin-top: 30px;
                    display: flex;
                    justify-content: center;
                    align-items: center;
                    background-color: #376fff;
                    border-radius: 50px;
                    color: #fff;
                    cursor: pointer;
                    width: 100%;
                    min-height: 48px;

                    &:hover {
                        background-color: #0059d0;
                    }
                }

                &__recaptcha-wrapper {
                    margin-top: 30px;

                    &__label-container {
                        display: flex;
                        justify-content: flex-start;
                        align-items: flex-end;
                        padding-bottom: 8px;
                        flex-direction: row;

                        &__error {
                            font-size: 16px;
                            margin-left: 10px;
                            color: #ff5560;
                        }
                    }
                }
            }

            &__footer {
                display: flex;
                justify-content: center;
                align-items: flex-start;
                margin-top: 40px;
                width: 100%;

                &__copyright {
                    font-size: 12px;
                    line-height: 18px;
                    color: #384b65;
                    padding-bottom: 20px;
                }

                &__link {
                    font-size: 12px;
                    line-height: 18px;
                    margin-left: 30px;
                    color: #376fff;
                    text-decoration: none;
                }
            }

            &__login-container {
                width: 100%;
                display: flex;
                align-items: center;
                justify-content: center;
                margin-top: 50px;
                padding-bottom: 50px;
                text-align: center;
                font-size: 14px;

                &__link {
                    font-family: 'font_bold', sans-serif;
                    text-decoration: none;
                    font-size: 14px;
                    color: #376fff;
                    margin-left: 5px;
                }
            }
        }
    }

    .logo {
        cursor: pointer;
    }

    .register-input {
        position: relative;
        width: 100%;
    }

    .input-wrap.full-input {
        width: calc(100% - 2px);
    }

    .container {
        display: block;
        position: relative;
        padding-left: 20px;
        height: 21px;
        width: 21px;
        cursor: pointer;
        font-size: 22px;
        -webkit-user-select: none;
        -moz-user-select: none;
        -ms-user-select: none;
        user-select: none;
        outline: none;
    }

    .container input {
        position: absolute;
        opacity: 0;
        cursor: pointer;
        height: 0;
        width: 0;
    }

    .checkmark {
        position: absolute;
        top: 0;
        left: 0;
        height: 21px;
        width: 21px;
        border: 2px solid #afb7c1;
        border-radius: 4px;
    }

    .container:hover input ~ .checkmark {
        background-color: white;
    }

    .container input:checked ~ .checkmark {
        border: 2px solid #afb7c1;
        background-color: transparent;
    }

    .checkmark:after {
        content: '';
        position: absolute;
        display: none;
    }

    .checkmark.error {
        border-color: red;
    }

    .container .checkmark:after {
        left: 7px;
        top: 3px;
        width: 5px;
        height: 10px;
        border: solid #354049;
        border-width: 0 3px 3px 0;
        -webkit-transform: rotate(45deg);
        -ms-transform: rotate(45deg);
        transform: rotate(45deg);
    }

    .container input:checked ~ .checkmark:after {
        display: block;
    }

    @media screen and (max-width: 1429px) {

        .register-area {

            &__intro-area {

                &__width {
                    width: 100%;
                }

                &__globe-image {
                    top: 110px;
                }
            }
        }
    }

    @media screen and (max-width: 1200px) {

        .register-area {

            &__intro-area {

                &__width {
                    width: 100%;
                }

                &__globe-image {
                    top: 110px;
                    left: 0;
                }

                &__globe-image.professional-globe {
                    left: 20px;
                }
            }
        }
    }

    @media screen and (max-width: 1060px) {

        .register-area {

            &__container {
                width: 70%;
            }

            &__intro-area {

                &__globe-image {
                    position: relative;
                    left: 0;
                    top: 110px;
                }

                &__globe-image.professional-globe {
                    left: 0;
                }
            }
        }
    }

    @media screen and (max-width: 1024px) {

        .register-area {
            position: relative;
            height: 100vh;

            &__container {
                display: inline;
                text-align: center;
                overflow: visible;
            }

            &__intro-area {
                margin: 0 auto 130px auto;
                overflow: visible;

                &__wrapper {
                    text-align: center;
                    margin: 0 auto;
                }

                &__globe-image {
                    top: 50px;
                    left: 0;
                }

                &__globe-image.professional-globe {
                    left: 0;
                    top: 50px;
                }

                &__title,
                &__sub-title {
                    text-align: center;
                }
            }

            &__input-area {
                display: block;
                width: 80%;
            }
        }
    }

    @media screen and (max-width: 700px) {

        .register-area {

            &__container {
                width: 70%;
                padding: 80px 40px 40px;
            }

            &__intro-area {
                margin: 0 auto;

                &__title {
                    font-size: 36px;
                    line-height: 40px;
                }

                &__sub-title {
                    font-size: 16px;
                    line-height: 23px;
                }

                &__globe-image {
                    display: none;
                }

                &__globe-image-sm {
                    display: block;
                    position: relative;
                    top: 40px;
                    margin: 0 auto;
                }
            }

            &__input-area {
                width: 100%;
                padding: 55px 0 0 0;

                &__container {
                    padding: 40px;
                    width: calc(100% - 80px);

                    &__checkbox-area {

                        &__msg-box {

                            &__msg {
                                position: relative;
                                top: 7px;
                                text-align: left;
                                left: 10px;
                            }
                        }
                    }
                }

                &__toggle {

                    &__professional {
                        right: 1px;
                        position: relative;
                    }
                }

                &__expand {

                    &__dropdown {
                        left: -200px;
                    }
                }
            }
        }
    }

    @media screen and (max-width: 414px) {

        .register-area {

            &__container {
                width: 90%;
                padding: 60px 10px 20px;
            }

            &__intro-area {
                margin: 0 auto 30px auto;

                &__title {
                    font-size: 34px;
                }
            }

            &__logo-wrapper {
                margin-top: 30px;
            }

            &__input-area {
                padding: 30px 0 0 0;

                &__container {
                    padding: 40px 20px;
                    width: calc(100% - 40px);

                    &__title-area {

                        &__title {
                            font-size: 20px;
                            line-height: 34px;
                        }
                    }
                }

                &__login-container {
                    margin-top: 40px;
                    padding-bottom: 40px;
                }
            }
        }
    }

    @media screen and (max-width: 320px) {

        .register-area {

            &__container {

                &__checkbox-area {

                    &__msg-box {

                        &__msg {
                            top: 6px;
                        }
                    }
                }
            }

            &__intro-area {

                &__title {
                    font-size: 29px;
                }
            }

            &__login-container {
                margin-top: 40px;
            }
        }
    }
</style>
