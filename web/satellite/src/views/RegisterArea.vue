// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="register-area" @keyup.enter="onCreateClick">
        <div class="register-area__logo-wrapper">
            <LogoIcon class="logo" @click="onLogoClick" />
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
                >
                    <div class="register-area__input-area__container__title-area">
                        <div class="register-area__input-area__container__title-container">
                            <h1 class="register-area__input-area__container__title-area__title">Get 150 GB Free</h1>
                        </div>
                        <div class="register-area__input-area__expand" aria-roledescription="satellites-dropdown" @click.stop="toggleDropdown">
                            <span class="register-area__input-area__expand__value">{{ satelliteName }}</span>
                            <BottomArrowIcon />
                            <div v-if="isDropdownShown" v-click-outside="closeDropdown" class="register-area__input-area__expand__dropdown">
                                <div class="register-area__input-area__expand__dropdown__item" @click.stop="closeDropdown">
                                    <SelectedCheckIcon />
                                    <span class="register-area__input-area__expand__dropdown__item__name">{{ satelliteName }}</span>
                                </div>
                                <a
                                    v-for="(sat, index) in partneredSatellites"
                                    :key="index"
                                    class="register-area__input-area__expand__dropdown__item"
                                    :href="`${sat.address}/signup`"
                                >
                                    {{ sat.name }}
                                </a>
                            </div>
                        </div>
                    </div>
                    <div class="register-area__input-area__toggle__container">
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
                                aria-roledescription="professional-label"
                                @click.prevent="toggleAccountType(true)"
                            >
                                Business
                            </li>
                        </ul>
                    </div>
                    <div class="register-area__input-wrapper first-input">
                        <HeaderlessInput
                            label="Full Name"
                            placeholder="Enter Full Name"
                            :error="fullNameError"
                            role-description="name"
                            @setData="setFullName"
                        />
                    </div>
                    <div class="register-area__input-wrapper">
                        <HeaderlessInput
                            label="Email Address"
                            placeholder="user@example.com"
                            :error="emailError"
                            role-description="email"
                            @setData="setEmail"
                        />
                    </div>
                    <div v-if="isProfessional">
                        <div class="register-area__input-wrapper">
                            <HeaderlessInput
                                label="Company Name"
                                placeholder="Acme Corp."
                                :error="companyNameError"
                                role-description="company-name"
                                @setData="setCompanyName"
                            />
                        </div>
                        <div class="register-area__input-wrapper">
                            <HeaderlessInput
                                label="Position"
                                placeholder="Position Title"
                                :error="positionError"
                                role-description="position"
                                @setData="setPosition"
                            />
                        </div>
                        <div class="register-area__input-wrapper">
                            <SelectInput
                                label="Employees"
                                :options-list="employeeCountOptions"
                                @setData="setEmployeeCount"
                            />
                        </div>
                    </div>
                    <div class="register-input">
                        <div class="register-area__input-wrapper">
                            <HeaderlessInput
                                label="Password"
                                placeholder="Enter Password"
                                :error="passwordError"
                                is-password="true"
                                role-description="password"
                                @setData="setPassword"
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
                            label="Retype Password"
                            placeholder="Retype Password"
                            :error="repeatedPasswordError"
                            is-password="true"
                            role-description="retype-password"
                            @setData="setRepeatedPassword"
                        />
                    </div>
                    <AddCouponCodeInput v-if="couponCodeSignupUIEnabled" />
                    <div v-if="isBetaSatellite" class="register-area__input-area__container__warning">
                        <div class="register-area__input-area__container__warning__header">
                            <label class="container">
                                <input v-model="areBetaTermsAccepted" type="checkbox">
                                <span class="checkmark" :class="{'error': areBetaTermsAcceptedError}" />
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
                            <input id="sales" v-model="haveSalesContact" type="checkbox">
                            <span class="checkmark" />
                        </label>
                        <label class="register-area__input-area__container__checkbox-area__msg-box" for="sales">
                            <p class="register-area__input-area__container__checkbox-area__msg-box__msg">
                                Please have the Sales Team contact me
                            </p>
                        </label>
                    </div>
                    <div class="register-area__input-area__container__checkbox-area">
                        <label class="container">
                            <input id="terms" v-model="isTermsAccepted" type="checkbox">
                            <span class="checkmark" :class="{'error': isTermsAcceptedError}" />
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
                    <div v-if="recaptchaEnabled" class="register-area__input-area__container__recaptcha-wrapper">
                        <div v-if="recaptchaError" class="register-area__input-area__container__recaptcha-wrapper__label-container">
                            <ErrorIcon />
                            <p class="register-area__input-area__container__recaptcha-wrapper__label-container__error">reCAPTCHA is required</p>
                        </div>
                        <VueRecaptcha
                            ref="recaptcha"
                            :sitekey="recaptchaSiteKey"
                            load-recaptcha-script="true"
                            size="invisible"
                            @verify="onRecaptchaVerified"
                            @expired="onRecaptchaError"
                            @error="onRecaptchaError"
                        />
                    </div>
                    <p class="register-area__input-area__container__button" @click.prevent="onCreateClick">Sign Up</p>
                </div>
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
import SelectInput from '@/components/common/SelectInput.vue';

import BottomArrowIcon from '@/../static/images/common/lightBottomArrow.svg';
import SelectedCheckIcon from '@/../static/images/common/selectedCheck.svg';
import LogoIcon from '@/../static/images/logo.svg';
import ErrorIcon from '@/../static/images/register/ErrorInfo.svg';
import RegisterGlobe from '@/../static/images/register/RegisterGlobe.svg';
import RegisterGlobeSmall from '@/../static/images/register/RegisterGlobeSmall.svg';

import { AuthHttpApi } from '@/api/auth';
import { RouteConfig } from '@/router';
import { PartneredSatellite } from '@/types/common';
import { User } from '@/types/users';
import { MetaUtils } from '@/utils/meta';
import { Validator } from '@/utils/validation';

// @vue/component
@Component({
    components: {
        HeaderlessInput,
        BottomArrowIcon,
        ErrorIcon,
        SelectedCheckIcon,
        LogoIcon,
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

    // DCS logic
    private secret = '';

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

    // DCS logic
    public isDropdownShown = false;

    // Employee Count dropdown options
    public employeeCountOptions = ['1-50', '51-1000', '1001+'];
    public optionsShown = false;

    public readonly loginPath: string = RouteConfig.Login.path;

    public $refs!: {
        recaptcha: VueRecaptcha;
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

        if (this.$route.query.promo) {
            this.user.signupPromoCode = this.$route.query.promo.toString();
        }
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
        if (this.$refs.recaptcha && !this.recaptchaResponseToken) {
            this.$refs.recaptcha.execute();
            return;
        }

        await this.createUser();
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
        this.user.password = value;
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
        this.createUser();
    }

    /**
     * Handles reCAPTCHA error and expiry.
     */
    public onRecaptchaError(): void {
        this.recaptchaResponseToken = '';
        this.recaptchaError = true;
    }

    /**
     * Validates input values to satisfy expected rules.
     */
    private validateFields(): boolean {
        let isNoErrors = true;

        if (!this.user.fullName) {
            this.fullNameError = 'Name can\'t be empty';
            isNoErrors = false;
        }

        if (!this.isEmailValid()) {
            this.emailError = 'Invalid Email';
            isNoErrors = false;
        }

        if (!Validator.password(this.password)) {
            this.passwordError = 'Invalid Password';
            isNoErrors = false;
        }

        if (this.isProfessional) {

            if (!this.user.companyName) {
                this.companyNameError = 'No Company Name filled in';
                isNoErrors = false;
            }

            if (!this.user.position) {
                this.positionError = 'No Position filled in';
                isNoErrors = false;
            }

            if (!this.user.employeeCount) {
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

        return isNoErrors;
    }

    /**
     * Detect if user uses Brave browser
     */
    public async detectBraveBrowser(): Promise<boolean> {
        return (navigator['brave'] && await navigator['brave'].isBrave() || false)
    }

    /**
     * Validates email string.
     * We'll have this email validation for new users instead of using regular Validator.email method because of backwards compatibility.
     * We don't want to block old users who managed to create and verify their accounts with some weird email addresses.
     */
    private isEmailValid(): boolean {
        // This regular expression fulfills our needs to validate international emails.
        // It was built according to RFC 5322 and then extended to include international characters.
        // eslint-disable-next-line no-misleading-character-class
        const regex = /^(([^<>()[\]\\.,;:\s@"]+(\.[^<>()[\]\\.,;:\s@"]+)*)|(".+"))@((\[[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}])|(([a-zA-Z\-0-9\u0080-\u00FF\u0100-\u017F\u0180-\u024F\u0250-\u02AF\u0300-\u036F\u0370-\u03FF\u0400-\u04FF\u0500-\u052F\u0530-\u058F\u0590-\u05FF\u0600-\u06FF\u0700-\u074F\u0750-\u077F\u0780-\u07BF\u07C0-\u07FF\u0900-\u097F\u0980-\u09FF\u0A00-\u0A7F\u0A80-\u0AFF\u0B00-\u0B7F\u0B80-\u0BFF\u0C00-\u0C7F\u0C80-\u0CFF\u0D00-\u0D7F\u0D80-\u0DFF\u0E00-\u0E7F\u0E80-\u0EFF\u0F00-\u0FFF\u1000-\u109F\u10A0-\u10FF\u1100-\u11FF\u1200-\u137F\u1380-\u139F\u13A0-\u13FF\u1400-\u167F\u1680-\u169F\u16A0-\u16FF\u1700-\u171F\u1720-\u173F\u1740-\u175F\u1760-\u177F\u1780-\u17FF\u1800-\u18AF\u1900-\u194F\u1950-\u197F\u1980-\u19DF\u19E0-\u19FF\u1A00-\u1A1F\u1B00-\u1B7F\u1D00-\u1D7F\u1D80-\u1DBF\u1DC0-\u1DFF\u1E00-\u1EFF\u1F00-\u1FFF\u20D0-\u20FF\u2100-\u214F\u2C00-\u2C5F\u2C60-\u2C7F\u2C80-\u2CFF\u2D00-\u2D2F\u2D30-\u2D7F\u2D80-\u2DDF\u2F00-\u2FDF\u2FF0-\u2FFF\u3040-\u309F\u30A0-\u30FF\u3100-\u312F\u3130-\u318F\u3190-\u319F\u31C0-\u31EF\u31F0-\u31FF\u3200-\u32FF\u3300-\u33FF\u3400-\u4DBF\u4DC0-\u4DFF\u4E00-\u9FFF\uA000-\uA48F\uA490-\uA4CF\uA700-\uA71F\uA800-\uA82F\uA840-\uA87F\uAC00-\uD7AF\uF900-\uFAFF]+\.)+[a-zA-Z\u0080-\u00FF\u0100-\u017F\u0180-\u024F\u0250-\u02AF\u0300-\u036F\u0370-\u03FF\u0400-\u04FF\u0500-\u052F\u0530-\u058F\u0590-\u05FF\u0600-\u06FF\u0700-\u074F\u0750-\u077F\u0780-\u07BF\u07C0-\u07FF\u0900-\u097F\u0980-\u09FF\u0A00-\u0A7F\u0A80-\u0AFF\u0B00-\u0B7F\u0B80-\u0BFF\u0C00-\u0C7F\u0C80-\u0CFF\u0D00-\u0D7F\u0D80-\u0DFF\u0E00-\u0E7F\u0E80-\u0EFF\u0F00-\u0FFF\u1000-\u109F\u10A0-\u10FF\u1100-\u11FF\u1200-\u137F\u1380-\u139F\u13A0-\u13FF\u1400-\u167F\u1680-\u169F\u16A0-\u16FF\u1700-\u171F\u1720-\u173F\u1740-\u175F\u1760-\u177F\u1780-\u17FF\u1800-\u18AF\u1900-\u194F\u1950-\u197F\u1980-\u19DF\u19E0-\u19FF\u1A00-\u1A1F\u1B00-\u1B7F\u1D00-\u1D7F\u1D80-\u1DBF\u1DC0-\u1DFF\u1E00-\u1EFF\u1F00-\u1FFF\u20D0-\u20FF\u2100-\u214F\u2C00-\u2C5F\u2C60-\u2C7F\u2C80-\u2CFF\u2D00-\u2D2F\u2D30-\u2D7F\u2D80-\u2DDF\u2F00-\u2FDF\u2FF0-\u2FFF\u3040-\u309F\u30A0-\u30FF\u3100-\u312F\u3130-\u318F\u3190-\u319F\u31C0-\u31EF\u31F0-\u31FF\u3200-\u32FF\u3300-\u33FF\u3400-\u4DBF\u4DC0-\u4DFF\u4E00-\u9FFF\uA000-\uA48F\uA490-\uA4CF\uA700-\uA71F\uA800-\uA82F\uA840-\uA87F\uAC00-\uD7AF\uF900-\uFAFF]{2,}))$/;
        return regex.test(this.user.email);
    }

    /**
     * Creates user and toggles successful registration area visibility.
     */
    private async createUser(): Promise<void> {
        if (this.isLoading) {
            return;
        }

        if (!this.validateFields()) {
            return;
        }

        this.isLoading = true;

        this.user.isProfessional = this.isProfessional;
        this.user.haveSalesContact = this.haveSalesContact;

        try {
            await this.auth.register(this.user, this.secret, this.recaptchaResponseToken);

            // Brave browser conversions are tracked via the RegisterSuccess path in the satellite app
            // signups outside of the brave browser may use a configured URL to track conversions
            // if the URL is not configured, the RegisterSuccess path will be used for non-Brave browsers
            const internalRegisterSuccessPath = RouteConfig.RegisterSuccess.path;
            const configuredRegisterSuccessPath = MetaUtils.getMetaContent('optional-signup-success-url') || internalRegisterSuccessPath;

            const nonBraveSuccessPath = `${configuredRegisterSuccessPath}?email=${encodeURIComponent(this.user.email)}`;
            const braveSuccessPath = `${internalRegisterSuccessPath}?email=${encodeURIComponent(this.user.email)}`;

            await this.detectBraveBrowser() ? await this.$router.push(braveSuccessPath) : window.location.href = nonBraveSuccessPath;
        } catch (error) {
            if (this.$refs.recaptcha) {
                this.$refs.recaptcha.reset();
                this.recaptchaResponseToken = '';
            }
            await this.$notify.error(error.message);
        }
        this.isLoading = false;
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
                    font-family: 'font_regular', sans-serif;
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
                        font-family: 'font_regular', sans-serif;
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
                    font-family: 'font_regular', sans-serif;
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

    ::v-deep .grecaptcha-badge {
        z-index: 99;
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
