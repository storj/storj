// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="register-area" @keyup.enter="onCreateClick">
        <div
            class="register-area__container"
            :class="{'professional-container': isProfessional}"
        >
            <div class="register-area__intro-area">
                <div class="register-area__logo-wrapper">
                    <div v-if="!!viewConfig.partnerLogoTopUrl" class="register-area__logo-wrapper__container">
                        <a :href="viewConfig.partnerUrl">
                            <img
                                :src="viewConfig.partnerLogoTopUrl"
                                :srcset="viewConfig.partnerLogoTopUrl"
                                alt="partner logo"
                                class="register-area__logo-wrapper__logo logo"
                            >
                        </a>
                    </div>
                    <div v-if="viewConfig.partnerLogoTopUrl" class="logo-divider" />
                    <div class="register-area__logo-wrapper__container">
                        <LogoWithPartnerIcon v-if="viewConfig.partnerLogoTopUrl" class="logo-with-partner" @click="onLogoClick" />
                        <LogoIcon v-else class="logo-no-partner" @click="onLogoClick" />
                    </div>
                </div>
                <h1 class="register-area__intro-area__title">{{ viewConfig.title }}</h1>
                <p v-if="viewConfig.description" class="register-area__intro-area__sub-title">{{ viewConfig.description }}</p>
                <div class="register-area__intro-area__large-content">
                    <!-- eslint-disable-next-line vue/no-v-html -->
                    <div v-if="viewConfig.customHtmlDescription" class="register-area__intro-area__large-content__custom-html-container" v-html="viewConfig.customHtmlDescription" />
                    <div v-if="!!viewConfig.partnerLogoBottomUrl" class="register-area__logo-wrapper bottom">
                        <div class="register-area__logo-wrapper__container">
                            <img :src="viewConfig.partnerLogoBottomUrl" :srcset="viewConfig.partnerLogoBottomUrl" alt="partner logo" class="register-area__logo-wrapper__logo wide">
                        </div>
                    </div>
                    <RegisterGlobe
                        v-if="!viewConfig.partnerLogoBottomUrl && !viewConfig.customHtmlDescription"
                        class="register-area__intro-area__large-content__globe-image"
                        :class="{'professional-globe': isProfessional}"
                    />
                </div>
            </div>
            <div class="register-area__input-area">
                <div
                    class="register-area__input-area__container"
                    :class="{ 'professional-container': isProfessional }"
                >
                    <div class="register-area__input-area__container__title-area" @click.stop="toggleDropdown">
                        <div class="register-area__input-area__container__title-container">
                            <h1 class="register-area__input-area__container__title-area__title">Get 25 GB Free</h1>
                        </div>
                        <div class="register-area__input-area__expand">
                            <div class="register-area__input-area__info-button">
                                <InfoIcon />
                                <p class="register-area__input-area__info-button__message">
                                    {{ viewConfig.tooltip }}
                                </p>
                            </div>
                            <button
                                id="registerDropdown"
                                type="button"
                                aria-haspopup="listbox"
                                aria-roledescription="satellites-dropdown"
                                :aria-expanded="isDropdownShown"
                                class="register-area__input-area__expand__value"
                            >
                                {{ satelliteName }}
                            </button>
                            <BottomArrowIcon />
                            <ul v-if="isDropdownShown" v-click-outside="closeDropdown" tabindex="-1" role="listbox" class="register-area__input-area__expand__dropdown">
                                <li key="0" tabindex="0" role="option" class="register-area__input-area__expand__dropdown__item" @click.stop="closeDropdown">
                                    <SelectedCheckIcon />
                                    <span class="register-area__input-area__expand__dropdown__item__name">{{ satelliteName }}</span>
                                </li>
                                <li
                                    v-for="(sat, index) in partneredSatellites"
                                    :key="index + 1"
                                    role="option"
                                    tabindex="0"
                                    :data-value="sat.name"
                                    class="register-area__input-area__expand__dropdown__item"
                                    @click="clickSatellite(sat.address)"
                                    @keypress.enter="clickSatellite(sat.address)"
                                >
                                    {{ sat.name }}
                                </li>
                            </ul>
                        </div>
                    </div>
                    <div class="register-area__input-area__toggle__container">
                        <ul class="register-area__input-area__toggle__wrapper">
                            <li
                                class="register-area__input-area__toggle__personal account-tab"
                                :class="{ 'active': !isProfessional }"
                                tabindex="0"
                                @click.prevent="toggleAccountType(false)"
                                @keydown.space.prevent="toggleAccountType(false)"
                            >
                                Personal
                            </li>
                            <li
                                class="register-area__input-area__toggle__professional account-tab"
                                :class="{ 'active': isProfessional }"
                                aria-roledescription="professional-label"
                                tabindex="0"
                                @click.prevent="toggleAccountType(true)"
                                @keydown.space.prevent="toggleAccountType(true)"
                            >
                                Business
                            </li>
                        </ul>
                    </div>
                    <div class="register-area__input-wrapper first-input">
                        <VInput
                            label="Full Name"
                            placeholder="Enter Full Name"
                            :error="fullNameError"
                            role-description="name"
                            @setData="setFullName"
                        />
                    </div>
                    <div class="register-area__input-wrapper">
                        <VInput
                            label="Email Address"
                            placeholder="user@example.com"
                            :error="emailError"
                            role-description="email"
                            @setData="setEmail"
                        />
                    </div>
                    <div v-if="isProfessional">
                        <div class="register-area__input-wrapper">
                            <VInput
                                label="Company Name"
                                placeholder="Acme Corp."
                                :error="companyNameError"
                                role-description="company-name"
                                @setData="setCompanyName"
                            />
                        </div>
                        <div class="register-area__input-wrapper">
                            <VInput
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
                        <div class="register-area__input-wrapper">
                            <SelectInput
                                label="Storage needs"
                                :options-list="storageNeedsOptions"
                                @setData="setStorageNeeds"
                            />
                        </div>
                    </div>
                    <div class="register-input">
                        <div class="register-area__input-wrapper">
                            <VInput
                                label="Password"
                                placeholder="Enter Password"
                                :error="passwordError"
                                is-password
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
                        <VInput
                            label="Retype Password"
                            placeholder="Retype Password"
                            :error="repeatedPasswordError"
                            is-password
                            role-description="retype-password"
                            @setData="setRepeatedPassword"
                        />
                    </div>
                    <AddCouponCodeInput v-if="couponCodeSignupUIEnabled" />
                    <div v-if="isBetaSatellite" class="register-area__input-area__container__warning">
                        <div class="register-area__input-area__container__warning__header">
                            <label class="checkmark-container">
                                <input tabindex="-1" type="checkbox" @change="onBetaTermsAcceptedToggled">
                                <span class="checkmark" :class="{'error': areBetaTermsAcceptedError}" @keydown.space.prevent="onBetaTermsAcceptedToggled" />
                            </label>
                            <h2 class="register-area__input-area__container__warning__header__label">
                                This is a BETA satellite
                            </h2>
                        </div>
                        <p class="register-area__input-area__container__warning__message">
                            This means any data you upload to this satellite can be
                            deleted at any time and your storage/egress limits
                            can fluctuate. To use our production service please
                            create an account on one of our production Satellites.
                            <a href="https://storj.io/signup/" target="_blank" rel="noopener noreferrer">https://storj.io/signup/</a>
                        </p>
                    </div>
                    <div v-if="isProfessional" class="register-area__input-area__container__checkbox-area">
                        <label class="checkmark-container">
                            <input id="sales" v-model="haveSalesContact" tabindex="-1" type="checkbox">
                            <span tabindex="0" class="checkmark" @keydown.space.prevent="toggleCheckbox" />
                        </label>
                        <label class="register-area__input-area__container__checkbox-area__msg-box" for="sales">
                            <p class="register-area__input-area__container__checkbox-area__msg-box__msg">
                                Please have the Sales Team contact me
                            </p>
                        </label>
                    </div>
                    <div class="register-area__input-area__container__checkbox-area">
                        <label for="terms" class="checkmark-container">
                            <input id="terms" tabindex="-1" type="checkbox" @change="onTermsAcceptedToggled">
                            <span tabindex="0" class="checkmark" :class="{'error': isTermsAcceptedError}" @keydown.space.prevent="onTermsAcceptedToggled" />
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
                        class="register-area__input-area__container__button"
                        width="100%"
                        height="48px"
                        :label="viewConfig.signupButtonLabel"
                        border-radius="50px"
                        :is-disabled="isLoading"
                        :on-press="onCreateClick"
                    >
                        Sign In
                    </v-button>
                    <div class="register-area__input-area__login-container">
                        Already have an account? <router-link :to="loginPath" class="register-area__input-area__login-container__link">Login.</router-link>
                    </div>
                </div>
            </div>
            <div class="register-area__container__mobile-content">
                <!-- eslint-disable-next-line vue/no-v-html -->
                <div v-if="viewConfig.customHtmlDescription" class="register-area__container__mobile-content__custom-html-container" v-html="viewConfig.customHtmlDescription" />
                <div v-if="!!viewConfig.partnerLogoBottomUrl" class="register-area__logo-wrapper">
                    <div class="register-area__logo-wrapper__container">
                        <img :src="viewConfig.partnerLogoBottomUrl" :srcset="viewConfig.partnerLogoBottomUrl" alt="partner logo" class="register-area__logo-wrapper__logo wide">
                        <div class="logo-divider" />
                        <LogoIcon class="logo" @click="onLogoClick" />
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, onBeforeMount, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import VueHcaptcha from '@hcaptcha/vue3-hcaptcha';

import { AuthHttpApi } from '@/api/auth';
import { RouteConfig } from '@/router';
import { MultiCaptchaConfig, PartneredSatellite } from '@/types/config';
import { User } from '@/types/users';
import { useNotify } from '@/utils/hooks';
import { useConfigStore } from '@/store/modules/configStore';

import SelectInput from '@/components/common/SelectInput.vue';
import PasswordStrength from '@/components/common/PasswordStrength.vue';
import VButton from '@/components/common/VButton.vue';
import VInput from '@/components/common/VInput.vue';
import AddCouponCodeInput from '@/components/common/AddCouponCodeInput.vue';

import LogoWithPartnerIcon from '@/../static/images/partnerStorjLogo.svg';
import LogoIcon from '@/../static/images/logo.svg';
import SelectedCheckIcon from '@/../static/images/common/selectedCheck.svg';
import BottomArrowIcon from '@/../static/images/common/lightBottomArrow.svg';
import RegisterGlobe from '@/../static/images/register/RegisterGlobe.svg';
import InfoIcon from '@/../static/images/register/info.svg';

type ViewConfig = {
    title: string;
    partnerUrl: string;
    partnerLogoTopUrl: string;
    partnerLogoBottomUrl: string;
    description: string;
    customHtmlDescription: string;
    signupButtonLabel: string;
    tooltip: string;
}

// Storage needs dropdown options.
const storageNeedsOptions = ['Less than 150TB', '150-499TB', '500-999TB', 'PB+'] as const;
type StorageNeed = typeof storageNeedsOptions[number] | undefined;

const user = ref(new User());
const storageNeeds = ref<StorageNeed>();
const viewConfig = ref<ViewConfig | null>(null);

// DCS logic
const secret = ref('');

const isTermsAccepted = ref(false);
const password = ref('');
const repeatedPassword = ref('');

// Only for beta sats (like US2).
const areBetaTermsAccepted = ref(false);
const areBetaTermsAcceptedError = ref(false);

const fullNameError = ref('');
const emailError = ref('');
const passwordError = ref('');
const repeatedPasswordError = ref('');
const companyNameError = ref('');
const employeeCountError = ref('');
const storageNeedsError = ref('');
const positionError = ref('');
const isTermsAcceptedError = ref(false);
const isLoading = ref(false);
const isProfessional = ref(false);
const haveSalesContact = ref(false);

const captchaError = ref(false);
const captchaResponseToken = ref('');

const isPasswordStrengthShown = ref(false);

// DCS logic
const isDropdownShown = ref(false);

// Employee Count dropdown options
const employeeCountOptions = ['1-50', '51-1000', '1001+'];

const loginPath = RouteConfig.Login.path;

const captcha = ref<VueHcaptcha | null>(null);

const auth = new AuthHttpApi();

const configStore = useConfigStore();
const notify = useNotify();
const router = useRouter();
const route = useRoute();

/**
 * Lifecycle hook before initial render.
 * Sets up variables from route params and loads config.
 */
onBeforeMount(() => {
    if (route.query.token) {
        secret.value = route.query.token.toString();
    }

    if (route.query.partner) {
        user.value.partner = route.query.partner.toString();
    }

    if (route.query.promo) {
        user.value.signupPromoCode = route.query.promo.toString();
    }

    try {
        const config = require('@/views/registration/registrationViewConfig.json');
        viewConfig.value = user.value.partner && config[user.value.partner] ? config[user.value.partner] : config['default'];
    } catch (e) {
        notify.error('No configuration file for registration page.', null);
    }
});

/**
 * Redirects to chosen satellite.
 */
function clickSatellite(address): void {
    window.location.href = address;
}

/**
 * Toggles satellite selection dropdown visibility (Tardigrade).
 */
function toggleDropdown(): void {
    isDropdownShown.value = !isDropdownShown.value;
}

/**
 * Closes satellite selection dropdown (Tardigrade).
 */
function closeDropdown(): void {
    isDropdownShown.value = false;
}

/**
 * Makes password strength container visible.
 */
function showPasswordStrength(): void {
    isPasswordStrengthShown.value = true;
}

/**
 * Hides password strength container.
 */
function hidePasswordStrength(): void {
    isPasswordStrengthShown.value = false;
}

/**
 * Validates input fields and proceeds user creation.
 */
async function onCreateClick(): Promise<void> {
    if (isLoading.value && !isDropdownShown.value) {
        return;
    }

    const activeElement = document.activeElement;

    if (activeElement && activeElement.id === 'registerDropdown') return;

    if (isDropdownShown.value) {
        isDropdownShown.value = false;
        return;
    }

    await createUser();
}

/**
 * Redirects to storj.io homepage.
 */
function onLogoClick(): void {
    window.location.href = configStore.state.config.homepageURL;
}

/**
 * Sets user's email field from value string.
 */
function setEmail(value: string): void {
    user.value.email = value.trim();
    emailError.value = '';
}

/**
 * Sets user's full name field from value string.
 */
function setFullName(value: string): void {
    user.value.fullName = value.trim();
    fullNameError.value = '';
}

/**
 * Sets user's password field from value string.
 */
function setPassword(value: string): void {
    user.value.password = value;
    password.value = value;
    passwordError.value = '';
}

/**
 * Sets user's repeat password field from value string.
 */
function setRepeatedPassword(value: string): void {
    repeatedPassword.value = value;
    repeatedPasswordError.value = '';
}

/**
 * This component's captcha configuration.
 */
const captchaConfig = computed((): MultiCaptchaConfig => {
    return configStore.state.config.captcha.registration;
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
    const config = configStore.state.config;
    const satellites = config.partneredSatellites.filter(sat => sat.name !== config.satelliteName);
    return satellites.map((s: PartneredSatellite) => {
        s.address = `${s.address}/signup`;

        if (user.value.partner) {
            s.address = `${s.address}?partner=${user.value.partner}`;
        }

        return s;
    });
});

/**
 * Indicates if satellite is in beta.
 */
const isBetaSatellite = computed((): boolean => {
    return configStore.state.config.isBetaSatellite;
});

/**
 * Indicates if coupon code ui is enabled
 */
const couponCodeSignupUIEnabled = computed((): boolean => {
    return configStore.state.config.couponCodeSignupUIEnabled;
});

/**
 * Sets user's company name field from value string.
 */
function setCompanyName(value: string): void {
    user.value.companyName = value.trim();
    companyNameError.value = '';
}

/**
 * Sets user's company size field from value string.
 */
function setEmployeeCount(value: string): void {
    user.value.employeeCount = value;
    employeeCountError.value = '';
}

/**
 * Sets user's storage needs field.
 */
function setStorageNeeds(value: StorageNeed): void {
    storageNeeds.value = value;
    storageNeedsError.value = '';
}

/**
 * Sets user's position field from value string.
 */
function setPosition(value: string): void {
    user.value.position = value.trim();
    positionError.value = '';
}

/**
 * toggle user account type
 */
function toggleAccountType(value: boolean): void {
    isProfessional.value = value;
}

/**
 * Handles captcha verification response.
 */
function onCaptchaVerified(response: string): void {
    captchaResponseToken.value = response;
    captchaError.value = false;
    createUser();
}

/**
 * Handles captcha error.
 */
function onCaptchaError(): void {
    captchaResponseToken.value = '';
    notify.error('The captcha encountered an error. Please try again.', null);
}

/**
 * Executes when the Terms of Service checkbox has been toggled.
 */
function onTermsAcceptedToggled(event: KeyboardEvent): void {
    if (event.key === ' ' || event.code === 'Space' || event.keyCode === 32) {
        const checkbox = ((event.target as HTMLElement).parentElement as HTMLLabelElement).control as HTMLInputElement;

        checkbox.checked = !checkbox.checked;
        checkbox.setAttribute('checked', String(checkbox.checked));

        isTermsAccepted.value = checkbox.checked;
        isTermsAcceptedError.value = false;

    } else {
        isTermsAccepted.value = (event.target as HTMLInputElement).checked;
        isTermsAcceptedError.value = false;
    }

}

/**
 * Executes when the beta satellite terms checkbox has been toggled.
 */
function onBetaTermsAcceptedToggled(event: KeyboardEvent): void {
    if (event.key === ' ' || event.code === 'Space' || event.keyCode === 32) {
        const checkbox = ((event.target as HTMLElement).parentElement as HTMLLabelElement).control as HTMLInputElement;

        checkbox.checked = !checkbox.checked;
        checkbox.setAttribute('checked', String(checkbox.checked));

        areBetaTermsAccepted.value = checkbox.checked;
        isTermsAcceptedError.value = false;

    } else {
        areBetaTermsAccepted.value = (event.target as HTMLInputElement).checked;
        areBetaTermsAcceptedError.value = false;
    }
}

/**
 * Executes when the space bar is pressed on a focused checkbox.
 */
function toggleCheckbox(event: Event): void {
    const checkbox = ((event.target as HTMLElement).parentElement as HTMLLabelElement).control as HTMLInputElement;

    checkbox.checked = !checkbox.checked;
    checkbox.setAttribute('checked', String(checkbox.checked));
}

/**
 * Validates input values to satisfy expected rules.
 */
function validateFields(): boolean {
    let isNoErrors = true;

    if (!user.value.fullName) {
        fullNameError.value = 'Name can\'t be empty';
        isNoErrors = false;
    }

    if (!isEmailValid()) {
        emailError.value = 'Invalid Email';
        isNoErrors = false;
    }

    const config = configStore.state.config;

    if (password.value.length < config.passwordMinimumLength || password.value.length > config.passwordMaximumLength) {
        passwordError.value = 'Invalid Password';
        isNoErrors = false;
    }

    if (isProfessional.value) {

        if (!user.value.companyName) {
            companyNameError.value = 'No Company Name filled in';
            isNoErrors = false;
        }

        if (!user.value.position) {
            positionError.value = 'No Position filled in';
            isNoErrors = false;
        }

        if (!user.value.employeeCount) {
            employeeCountError.value = 'No Company Size filled in';
            isNoErrors = false;
        }

        if (!storageNeeds.value) {
            storageNeedsError.value = 'Storage Needs not filled in';
            isNoErrors = false;
        }

    }

    if (repeatedPassword.value !== password.value) {
        repeatedPasswordError.value = 'Password doesn\'t match';
        isNoErrors = false;
    }

    if (!isTermsAccepted.value) {
        isTermsAcceptedError.value = true;
        isNoErrors = false;
    }

    // only for beta US2 sats.
    if (isBetaSatellite.value && !areBetaTermsAccepted.value) {
        areBetaTermsAcceptedError.value = true;
        isNoErrors = false;
    }

    if (user.value.partner.length > 100) {
        notify.error('Partner must be less than or equal to 100 characters', null);
        return false;
    }

    if (user.value.signupPromoCode.length > 100) {
        notify.error('Promo code must be less than or equal to 100 characters', null);
        return false;
    }

    return isNoErrors;
}

/**
 * Detect if user uses Brave browser
 */
async function detectBraveBrowser(): Promise<boolean> {
    return (navigator['brave'] && await navigator['brave'].isBrave() || false);
}

/**
 * Validates email string.
 * We'll have this email validation for new users instead of using regular Validator.email method because of backwards compatibility.
 * We don't want to block old users who managed to create and verify their accounts with some weird email addresses.
 */
function isEmailValid(): boolean {
    // This regular expression fulfills our needs to validate international emails.
    // It was built according to RFC 5322 and then extended to include international characters using these resources
    // https://emailregex.com/
    // https://awik.io/international-email-address-validation-javascript/
    // eslint-disable-next-line no-misleading-character-class
    const regex = /^(([^<>()[\]\\.,;:\s@"]+(\.[^<>()[\]\\.,;:\s@"]+)*)|(".+"))@((\[[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}])|(([a-zA-Z\-0-9\u0080-\u00FF\u0100-\u017F\u0180-\u024F\u0250-\u02AF\u0300-\u036F\u0370-\u03FF\u0400-\u04FF\u0500-\u052F\u0530-\u058F\u0590-\u05FF\u0600-\u06FF\u0700-\u074F\u0750-\u077F\u0780-\u07BF\u07C0-\u07FF\u0900-\u097F\u0980-\u09FF\u0A00-\u0A7F\u0A80-\u0AFF\u0B00-\u0B7F\u0B80-\u0BFF\u0C00-\u0C7F\u0C80-\u0CFF\u0D00-\u0D7F\u0D80-\u0DFF\u0E00-\u0E7F\u0E80-\u0EFF\u0F00-\u0FFF\u1000-\u109F\u10A0-\u10FF\u1100-\u11FF\u1200-\u137F\u1380-\u139F\u13A0-\u13FF\u1400-\u167F\u1680-\u169F\u16A0-\u16FF\u1700-\u171F\u1720-\u173F\u1740-\u175F\u1760-\u177F\u1780-\u17FF\u1800-\u18AF\u1900-\u194F\u1950-\u197F\u1980-\u19DF\u19E0-\u19FF\u1A00-\u1A1F\u1B00-\u1B7F\u1D00-\u1D7F\u1D80-\u1DBF\u1DC0-\u1DFF\u1E00-\u1EFF\u1F00-\u1FFF\u20D0-\u20FF\u2100-\u214F\u2C00-\u2C5F\u2C60-\u2C7F\u2C80-\u2CFF\u2D00-\u2D2F\u2D30-\u2D7F\u2D80-\u2DDF\u2F00-\u2FDF\u2FF0-\u2FFF\u3040-\u309F\u30A0-\u30FF\u3100-\u312F\u3130-\u318F\u3190-\u319F\u31C0-\u31EF\u31F0-\u31FF\u3200-\u32FF\u3300-\u33FF\u3400-\u4DBF\u4DC0-\u4DFF\u4E00-\u9FFF\uA000-\uA48F\uA490-\uA4CF\uA700-\uA71F\uA800-\uA82F\uA840-\uA87F\uAC00-\uD7AF\uF900-\uFAFF]+\.)+[a-zA-Z\u0080-\u00FF\u0100-\u017F\u0180-\u024F\u0250-\u02AF\u0300-\u036F\u0370-\u03FF\u0400-\u04FF\u0500-\u052F\u0530-\u058F\u0590-\u05FF\u0600-\u06FF\u0700-\u074F\u0750-\u077F\u0780-\u07BF\u07C0-\u07FF\u0900-\u097F\u0980-\u09FF\u0A00-\u0A7F\u0A80-\u0AFF\u0B00-\u0B7F\u0B80-\u0BFF\u0C00-\u0C7F\u0C80-\u0CFF\u0D00-\u0D7F\u0D80-\u0DFF\u0E00-\u0E7F\u0E80-\u0EFF\u0F00-\u0FFF\u1000-\u109F\u10A0-\u10FF\u1100-\u11FF\u1200-\u137F\u1380-\u139F\u13A0-\u13FF\u1400-\u167F\u1680-\u169F\u16A0-\u16FF\u1700-\u171F\u1720-\u173F\u1740-\u175F\u1760-\u177F\u1780-\u17FF\u1800-\u18AF\u1900-\u194F\u1950-\u197F\u1980-\u19DF\u19E0-\u19FF\u1A00-\u1A1F\u1B00-\u1B7F\u1D00-\u1D7F\u1D80-\u1DBF\u1DC0-\u1DFF\u1E00-\u1EFF\u1F00-\u1FFF\u20D0-\u20FF\u2100-\u214F\u2C00-\u2C5F\u2C60-\u2C7F\u2C80-\u2CFF\u2D00-\u2D2F\u2D30-\u2D7F\u2D80-\u2DDF\u2F00-\u2FDF\u2FF0-\u2FFF\u3040-\u309F\u30A0-\u30FF\u3100-\u312F\u3130-\u318F\u3190-\u319F\u31C0-\u31EF\u31F0-\u31FF\u3200-\u32FF\u3300-\u33FF\u3400-\u4DBF\u4DC0-\u4DFF\u4E00-\u9FFF\uA000-\uA48F\uA490-\uA4CF\uA700-\uA71F\uA800-\uA82F\uA840-\uA87F\uAC00-\uD7AF\uF900-\uFAFF]{2,}))$/;
    return regex.test(user.value.email);
}

/**
 * Creates user and toggles successful registration area visibility.
 */
async function createUser(): Promise<void> {

    const activeElement = document.activeElement;

    if (activeElement && activeElement.classList.contains('account-tab')) {
        return;
    }

    if (!validateFields()) {
        return;
    }

    if (captcha.value && !captchaResponseToken.value) {
        captcha.value?.execute();
        return;
    }

    isLoading.value = true;
    user.value.isProfessional = isProfessional.value;
    user.value.haveSalesContact = haveSalesContact.value;

    try {
        await auth.register({ ...user.value, storageNeeds: storageNeeds.value }, secret.value, captchaResponseToken.value);

        // Brave browser conversions are tracked via the RegisterSuccess path in the satellite app
        // signups outside of the brave browser may use a configured URL to track conversions
        // if the URL is not configured, the RegisterSuccess path will be used for non-Brave browsers
        const internalRegisterSuccessPath = RouteConfig.RegisterSuccess.path;
        const configuredRegisterSuccessPath = configStore.state.config.optionalSignupSuccessURL || internalRegisterSuccessPath;

        const nonBraveSuccessPath = `${configuredRegisterSuccessPath}?email=${encodeURIComponent(user.value.email)}`;
        const braveSuccessPath = `${internalRegisterSuccessPath}?email=${encodeURIComponent(user.value.email)}`;

        await detectBraveBrowser() ? await router.push(braveSuccessPath) : window.location.href = nonBraveSuccessPath;
    } catch (error) {
        notify.error(error.message, null);
    }

    captcha.value?.reset();
    captchaResponseToken.value = '';
    isLoading.value = false;
}
</script>

<style scoped lang="scss">
    %subtitle-text {
        max-width: 550px;
        margin-top: 27px;
        font-size: 16px;
        font-family: 'font_regular', sans-serif;
        line-height: 24px;
        text-align: left;
    }

    .logo-divider {
        border-left: 1px solid var(--c-light-blue-2);
        height: 40px;
        margin: 0 10px;
    }

    .register-area {
        display: flex;
        flex-direction: column;
        align-items: center;
        font-family: 'font_regular', sans-serif;
        background-color: #f5f6fa;
        box-sizing: border-box;
        position: fixed;
        inset: 0;
        overflow-y: scroll;
        padding-top: 80px;
        height: 100vh;

        &__logo-wrapper {
            display: flex;
            align-items: center;
            justify-content: flex-start;
            height: 52px;
            margin-bottom: 32px;

            &__container {
                display: flex;
                align-items: center;
                justify-content: flex-end;
                height: 100%;
            }

            &__logo {
                height: 66px;
                max-width: 250px;
                max-height: 66px;

                &.wide {
                    object-fit: cover;
                    width: auto;
                    height: 56px;
                    max-width: unset;

                    @media screen and (width <= 1024px) {
                        object-fit: contain;
                        max-width: 45%;
                    }
                }
            }

            &.bottom {
                margin: 27px 0 0;
            }
        }

        &__input-wrapper.first-input {
            margin-top: 10px;
        }

        &__container {
            display: flex;
            width: 75%;
            justify-content: center;
            max-width: 1500px;

            @media screen and (width <= 1600px) {
                width: 90%;
            }

            &__mobile-content {
                @media screen and (width >= 1025px) {
                    display: none;
                }

                .register-area__logo-wrapper {
                    display: block;
                    margin-top: 24px;

                    &__container {
                        justify-content: center;
                    }
                }

                &__custom-html-container {
                    margin-top: 27px;

                    :deep(p) {
                        @extend %subtitle-text;

                        text-align: center;

                        strong {
                            font-family: 'font_bold', sans-serif;
                        }

                        a {
                            text-decoration: underline !important;
                            color: inherit !important;
                        }
                    }

                    :deep(ol) {
                        @extend %subtitle-text;

                        list-style-position: inside;
                    }
                }
            }
        }

        &__intro-area {
            box-sizing: border-box;
            overflow: hidden;
            padding: 40px 0 60px;
            max-width: 40%;
            margin-right: 80px;

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
            }

            &__sub-title {
                @extend %subtitle-text;
            }

            &__large-content {

                &__custom-html-container {
                    padding-bottom: 27px;

                    :deep(p) {
                        @extend %subtitle-text;

                        strong {
                            font-family: 'font_bold', sans-serif;
                        }

                        a {
                            text-decoration: underline !important;
                            color: inherit !important;
                        }
                    }

                    :deep(ol) {
                        @extend %subtitle-text;

                        list-style-position: inside;
                    }
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
            }
        }

        &__input-area {
            box-sizing: border-box;
            padding: 60px 80px;
            background-color: #fff;
            border-radius: 20px;
            width: 50%;

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
                    border: none;
                    cursor: pointer;
                    background: transparent;
                }

                &__dropdown {
                    position: absolute;
                    top: 35px;
                    right: 0;
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

            &__info-button {
                position: relative;
                cursor: pointer;
                margin-right: 3px;
                height: 18px;

                &:hover p {
                    visibility: visible;
                }

                &__image {
                    cursor: pointer;
                }

                &__message {
                    position: absolute;
                    top: 150%;
                    right: 12px;
                    transform: translateX(50%);
                    visibility: hidden;
                    background-color: var(--c-grey-6);
                    text-align: center;
                    border-radius: 4px;
                    font-family: 'font-medium', sans-serif;
                    color: white;
                    font-size: 12px;
                    line-height: 18px;
                    width: 221px;
                    box-sizing: border-box;
                    padding: 10px 8px;
                    z-index: 1001;

                    &:after {
                        content: '';
                        position: absolute;
                        bottom: 100%;
                        left: 50%;
                        border-width: 5px;
                        border-style: solid;
                        border-color: var(--c-grey-6) transparent transparent;
                        transform: rotate(180deg);
                    }
                }
            }

            &__toggle {

                &__wrapper {
                    display: flex;
                    justify-content: space-between;
                    margin: 20px 0 15px;
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
                        letter-spacing: -0.1007px;
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
                        margin: 8px 0 0;
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

                                &:focus {
                                    color: var(--c-blue-3);
                                }
                            }
                        }
                    }
                }

                &__button {
                    margin-top: 30px;
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
                margin: 30px 0;
                text-align: center;
                font-size: 14px;

                &__link {
                    font-family: 'font_bold', sans-serif;
                    text-decoration: none;
                    font-size: 14px;
                    color: #376fff;
                    margin-left: 5px;
                }

                &__link:focus {
                    text-decoration: underline !important;
                }
            }
        }
    }

    .logo {
        cursor: pointer;
    }

    .logo-with-partner {
        cursor: pointer;
        max-width: 100%;
    }

    .logo-no-partner {
        cursor: pointer;
        width: 100%;
    }

    .register-input {
        position: relative;
        width: 100%;
    }

    .input-wrap {
        margin-top: 10px;
    }

    .checkmark-container {
        display: block;
        position: relative;
        padding-left: 20px;
        height: 21px;
        width: 21px;
        cursor: pointer;
        font-size: 22px;
        user-select: none;
        outline: none;
    }

    .checkmark-container input {
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

    .checkmark-container:hover input ~ .checkmark {
        background-color: white;
    }

    .checkmark-container input:checked ~ .checkmark {
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

    .checkmark-container .checkmark:after {
        left: 7px;
        top: 3px;
        width: 5px;
        height: 10px;
        border: solid #354049;
        border-width: 0 3px 3px 0;
        transform: rotate(45deg);
    }

    .checkmark-container input:checked ~ .checkmark:after {
        display: block;
    }

    :deep(.grecaptcha-badge) {
        visibility: hidden;
    }

    @media screen and (width <= 1429px) {

        .register-area {

            &__intro-area {

                &__width {
                    width: 100%;
                }
            }
        }
    }

    @media screen and (width <= 1200px) {

        .register-area {

            &__intro-area {

                &__width {
                    width: 100%;
                }
            }
        }
    }

    @media screen and (width <= 1060px) {

        .register-area {

            &__container {
                width: 70%;
            }
        }
    }

    @media screen and (width <= 1024px) {

        .register-area {
            display: block;
            position: relative;
            width: 100%;
            padding: 10px;

            &__container {
                display: flex;
                flex-direction: column;
                align-items: center;
                justify-content: center;
                overflow: visible;
                width: 85%;
                margin: 0 auto;
            }

            .register-area__logo-wrapper.bottom {
                display: none;
            }

            &__intro-area {
                display: flex;
                flex-direction: column;
                align-items: center;
                justify-content: center;
                overflow: visible;
                max-width: 100%;
                margin: 0;

                &__wrapper {
                    text-align: center;
                    margin: 0 auto;
                }

                &__title,
                &__sub-title {
                    text-align: center;
                }

                &__large-content {

                    &__globe-image,
                    &__custom-html-container {
                        display: none;
                    }
                }
            }

            &__input-area {
                display: block;
                width: 100%;
            }
        }
    }

    @media screen and (width <= 700px) {

        .register-area {

            &__container {
                width: 90%;
                padding: 80px 30px 30px;
            }

            &__intro-area {
                margin: 0 auto;
                padding-top: 0;

                &__title {
                    font-size: 36px;
                    line-height: 40px;
                }

                &__sub-title {
                    font-size: 16px;
                    line-height: 23px;
                }
            }

            &__input-area {
                width: 100%;
                padding: 0;

                &__container {
                    padding: 30px 30px 0;
                    width: calc(100% - 60px);

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

                &__info-button {
                    display: none;
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

    @media screen and (width <= 1024px) {

        .register-area {

            &__container {
                padding: 40px 10px 20px;
            }

            &__logo-wrapper {
                height: auto;
                justify-content: center;

                img {
                    height: 62px;
                    max-width: 100%;
                }

                &__container {

                    a {
                        width: 100%;
                        height: 100%;
                    }
                }
            }

            &__intro-area {
                flex-direction: column;
                padding: 0 0 20px;
                height: auto;

                &__title {
                    font-size: 28px;
                }
            }
        }
    }

    @media screen and (width <= 414px) {

        .register-area {

            &__container {
                width: 90%;
            }

            &__intro-area__title {
                font-size: 34px;
            }

            &__input-area {
                padding: 0;

                &__container {
                    padding: 30px 15px;
                    width: calc(100% - 30px);

                    &__title-area {

                        &__title {
                            font-size: 20px;
                            line-height: 34px;
                        }
                    }
                }

                &__login-container {
                    margin-top: 40px;
                }
            }
        }
    }
</style>
