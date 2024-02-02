// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <signup-confirmation v-if="codeActivationEnabled && confirmCode" :email="email" :signup-req-id="signupID" />
    <v-container v-else class="fill-height">
        <v-row align="top" justify="center">
            <v-col cols="12" sm="10" md="6" lg="5" xl="4" xxl="3">
                <v-card title="Create your Storj account" subtitle="Get 25GB free storage and download" class="pa-2 pa-sm-7 overflow-visible">
                    <v-card-item v-if="isInvited">
                        <v-alert
                            variant="tonal"
                            color="info"
                            rounded="lg"
                            density="comfortable"
                            border
                        >
                            <template #text>
                                {{ inviterEmail }} has invited you to a project on Storj. Create an account on the {{ satellite.satellite }} region to join it.
                            </template>
                        </v-alert>
                    </v-card-item>

                    <v-card-text>
                        <v-form ref="form" v-model="formValid" class="pt-4">
                            <v-select
                                v-model="satellite"
                                label="Satellite"
                                :items="satellites"
                                item-title="satellite"
                                :hint="satellite.hint"
                                persistent-hint
                                return-object
                                chips
                                class="mb-5"
                            />

                            <v-text-field
                                v-if="isInvited"
                                id="Email Address"
                                :model-value="queryEmail"
                                class="mb-2"
                                label="Email address"
                                placeholder="Enter your email"
                                name="email"
                                type="email"
                                :rules="emailRules"
                                flat
                                disabled
                                required
                            />

                            <v-text-field
                                v-else
                                id="Email Address"
                                v-model="email"
                                class="mb-2"
                                label="Email address"
                                placeholder="Enter your email"
                                maxlength="72"
                                name="email"
                                type="email"
                                :rules="emailRules"
                                flat
                                clearable
                                required
                            />

                            <div class="pos-relative">
                                <v-text-field
                                    id="Password"
                                    v-model="password"
                                    class="mb-2"
                                    label="Password"
                                    placeholder="Enter a password"
                                    color="secondary"
                                    :type="showPassword ? 'text' : 'password'"
                                    :rules="passwordRules"
                                    @update:focused="showPasswordStrength = !showPasswordStrength"
                                >
                                    <template #append-inner>
                                        <password-input-eye-icons
                                            :is-visible="showPassword"
                                            type="password"
                                            @toggleVisibility="showPassword = !showPassword"
                                        />
                                    </template>
                                </v-text-field>
                                <password-strength
                                    v-if="showPasswordStrength"
                                    :password="password"
                                />
                            </div>

                            <v-text-field
                                id="Retype Password"
                                ref="repPasswordField"
                                v-model="repPassword"
                                label="Retype password"
                                placeholder="Enter a password"
                                color="secondary"
                                :type="showPassword ? 'text' : 'password'"
                                :rules="repeatPasswordRules"
                            >
                                <template #append-inner>
                                    <password-input-eye-icons
                                        :is-visible="showPassword"
                                        type="password"
                                        @toggleVisibility="showPassword = !showPassword"
                                    />
                                </template>
                            </v-text-field>

                            <v-alert
                                v-if="isBetaSatellite"
                                class="my-2"
                                variant="tonal"
                                color="warning"
                                rounded="lg"
                                density="comfortable"
                                border
                            >
                                <template #title>
                                    <v-checkbox
                                        id="Beta terms checkbox"
                                        v-model="acceptedBetaTerms"
                                        :rules="[RequiredRule]"
                                        density="compact"
                                        hide-details="auto"
                                        required
                                    >
                                        <template #label>
                                            This is a BETA satellite
                                        </template>
                                    </v-checkbox>
                                </template>
                                <template #text>
                                    This means any data you upload to this satellite can be
                                    deleted at any time and your storage/egress limits
                                    can fluctuate. To use our production service please
                                    create an account on one of our production Satellites.
                                    <a href="https://storj.io/v2/signup/" target="_blank" rel="noopener noreferrer">https://storj.io/v2/signup/</a>
                                </template>
                            </v-alert>

                            <v-checkbox
                                id="Terms checkbox"
                                v-model="acceptedTerms"
                                class="mb-5"
                                :rules="[RequiredRule]"
                                density="compact"
                                hide-details="auto"
                                required
                            >
                                <template #label>
                                    <p class="text-body-2">
                                        I agree to the
                                        <a class="link" href="https://storj.io/terms-of-service/" target="_blank" rel="noopener">Terms of Service</a>
                                        and
                                        <a class="link" href="https://storj.io/privacy-policy/" target="_blank" rel="noopener">Privacy Policy</a>.
                                    </p>
                                </template>
                            </v-checkbox>

                            <v-btn
                                :loading="isLoading"
                                color="primary"
                                size="large"
                                block
                                @click="onSignupClick"
                            >
                                Create your account
                            </v-btn>
                        </v-form>
                    </v-card-text>
                </v-card>

                <VueHcaptcha
                    v-if="captchaConfig?.hcaptcha.enabled"
                    ref="hcaptcha"
                    :sitekey="captchaConfig.hcaptcha.siteKey"
                    :re-captcha-compat="false"
                    size="invisible"
                    @verify="onCaptchaVerified"
                    @expired="onCaptchaError"
                    @error="onCaptchaError"
                />
            </v-col>
            <template v-if="partner">
                <v-col v-if="viewConfig" cols="12" sm="10" md="6" lg="5" xl="4" xxl="3">
                    <v-card class="pa-2 pa-sm-7 h-100 no-position">
                        <v-card-item>
                            <v-card-title class="text-wrap">
                                {{ viewConfig.title }}
                            </v-card-title>
                            <v-card-subtitle class="text-wrap">
                                {{ viewConfig.description }}
                            </v-card-subtitle>
                        </v-card-item>
                        <v-card-text>
                            <!-- eslint-disable-next-line vue/no-v-html -->
                            <div v-if="viewConfig.customHtmlDescription" v-html="viewConfig.customHtmlDescription" />
                            <a v-if="viewConfig.partnerLogoTopUrl" :href="viewConfig.partnerUrl">
                                <img :src="viewConfig.partnerLogoTopUrl" :srcset="viewConfig.partnerLogoTopUrl" alt="partner logo" height="44" class="mt-6 mr-5">
                            </a>
                            <a v-if="viewConfig.partnerLogoBottomUrl" :href="viewConfig.partnerUrl">
                                <img :src="viewConfig.partnerLogoBottomUrl" :srcset="viewConfig.partnerLogoBottomUrl" alt="partner logo" height="44" class="mt-6">
                            </a>
                        </v-card-text>
                    </v-card>
                </v-col>
            </template>
            <template v-else>
                <v-col cols="12" sm="10" md="6" lg="5" xl="4" xxl="3">
                    <v-card class="pa-2 pa-sm-7 h-100 no-position">
                        <v-card-text>
                            <p class="text-subtitle-2">
                                Get unparalleled security
                            </p>
                            <p class="text-body-2">
                                Every file is encrypted, split into pieces, then stored across thousands of nodes globally, helping to prevent breaches, ransomware attacks and downtime.
                            </p>

                            <p class="text-subtitle-2 mt-4">
                                Cut your cloud costs
                            </p>
                            <p class="text-body-2">
                                80% lower cost than competing storage solutions.
                            </p>

                            <p class="text-subtitle-2 mt-4">
                                Best Performance
                            </p>
                            <p class="text-body-2">
                                Get consistent, lightning fast, CDN-like performace globally. Legacy cloud storage simply can't match Storj.
                            </p>

                            <p class="text-subtitle-2 mt-4">
                                S3 Compatibility
                            </p>
                            <p class="text-body-2">
                                Storj works with the tools you are already using, with drop-in S3 compatibility for simple integration.
                            </p>

                            <p class="text-subtitle-2 mt-4">
                                Sustainability
                            </p>
                            <p class="text-body-2">
                                Dramatically reduce your carbon footprint when you switch to the greenest data storage on earth.
                            </p>
                        </v-card-text>
                    </v-card>
                </v-col>
            </template>
        </v-row>
        <v-row justify="center" class="v-col-12">
            <v-col>
                <p class="pt-9 text-center text-body-2">Already have an account? <router-link class="link" :to="ROUTES.Login.path">Login</router-link></p>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import {
    VAlert,
    VBtn,
    VCard,
    VCardItem,
    VCardText,
    VCardTitle,
    VCardSubtitle,
    VCheckbox,
    VCol,
    VContainer,
    VForm,
    VRow,
    VSelect,
    VTextField,
} from 'vuetify/components';
import { computed, ComputedRef, onBeforeMount, ref, watch } from 'vue';
import VueHcaptcha from '@hcaptcha/vue3-hcaptcha';
import { useRoute, useRouter } from 'vue-router';

import { useConfigStore } from '@/store/modules/configStore';
import { useAppStore } from '@/store/modules/appStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { EmailRule, RequiredRule, ValidationRule } from '@poc/types/common';
import { MultiCaptchaConfig } from '@/types/config.gen';
import { AuthHttpApi } from '@/api/auth';
import { useNotify } from '@/utils/hooks';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { ROUTES } from '@poc/router';

import SignupConfirmation from '@poc/views/SignupConfirmation.vue';
import PasswordInputEyeIcons from '@poc/components/PasswordInputEyeIcons.vue';
import PasswordStrength from '@poc/components/PasswordStrength.vue';

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

const auth = new AuthHttpApi();

const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();
const appStore = useAppStore();
const usersStore = useUsersStore();
const router = useRouter();
const notify = useNotify();
const route = useRoute();

const isLoading = ref<boolean>(false);
const formValid = ref<boolean>(false);
const acceptedBetaTerms = ref(false);
const acceptedTerms = ref(false);
const showPassword = ref(false);
const captchaError = ref(false);
const confirmCode = ref(false);
const showPasswordStrength = ref(false);

const signupID = ref('');
const partner = ref('');
const signupPromoCode = ref('');
const captchaResponseToken = ref('');
const email = ref('');
const password = ref('');
const repPassword = ref('');

const secret = queryRef('token');

const queryEmail = queryRef('email');
const inviterEmail = queryRef('inviter_email');

const hcaptcha = ref<VueHcaptcha | null>(null);
const form = ref<VForm | null>(null);
const repPasswordField = ref<VTextField | null>(null);
const viewConfig = ref<ViewConfig | null>(null);

const satellitesHints = [
    { satellite: 'US1', hint: 'Recommended for North and South America' },
    { satellite: 'EU1', hint: 'Recommended for Europe and Africa' },
    { satellite: 'AP1', hint: 'Recommended for Asia and Australia' },
];

const passwordRules: ValidationRule<string>[] = [
    RequiredRule,
    (value) => value.length < passMinLength.value || value.length > passMaxLength.value
        ? `Password must be between ${passMinLength.value} and ${passMaxLength.value} characters`
        : true,
];

const emailRules: ValidationRule<string>[] = [
    RequiredRule,
    (value) => EmailRule(value, true),
];

const repeatPasswordRules = computed<ValidationRule<string>[]>(() => [
    ...passwordRules,
    (value: string) => {
        if (password.value !== value) {
            return 'Passwords do not match';
        }
        return true;
    },
]);

/**
 * Returns the maximum password length from the store.
 */
const passMaxLength = computed((): number => {
    return configStore.state.config.passwordMaximumLength;
});

/**
 * Returns the minimum password length from the store.
 */
const passMinLength = computed((): number => {
    return configStore.state.config.passwordMinimumLength;
});

/**
 * Name of the current satellite.
 */
const satellite = computed({
    get: () => {
        const satName = configStore.state.config.satelliteName ?? '';
        const item = satellitesHints.find(item => item.satellite === satName);
        return item ?? { satellite: satName, hint: '' };
    },
    set: value => {
        const sats = configStore.state.config.partneredSatellites ?? [];
        const satellite = sats.find(sat => sat.name === value.satellite);
        if (satellite) {
            window.location.href = satellite.address + ROUTES.Signup.path;
        }
    },
});

/**
 * Information about partnered satellites.
 */
const satellites = computed(() => {
    const satellites = configStore.state.config.partneredSatellites ?? [];
    return satellites.map(satellite => {
        const item = satellitesHints.find(item => item.satellite === satellite.name);
        return item ?? { satellite: satellite.name, hint: '' };
    });
});

/**
 * Returns true if signup activation code is enabled.
 */
const codeActivationEnabled = computed((): boolean => {
    return  configStore.state.config.signupActivationCodeEnabled;
});

/**
 * Indicates if satellite is in beta.
 */
const isBetaSatellite = computed((): boolean => {
    return configStore.state.config.isBetaSatellite;
});

/**
 * Returns whether the current URL's query parameters indicate that the user was
 * redirected from a project invitation link.
 */
const isInvited = computed((): boolean => {
    return !!inviterEmail.value && !!queryEmail.value;
});

/**
 * This component's captcha configuration.
 */
const captchaConfig = computed((): MultiCaptchaConfig | undefined => {
    return configStore.state.config.captcha?.registration;
});

/**
 * queryRef returns a computed reference to a query parameter.
 * Nonexistent keys or keys with no value produce an empty string.
 */
function queryRef(key: string): ComputedRef<string> {
    return computed((): string => {
        const param = route.query[key] || '';
        return (typeof param === 'string') ? param : (param[0] || '');
    });
}

/**
 * Handles captcha verification response.
 */
function onCaptchaVerified(response: string): void {
    captchaResponseToken.value = response;
    captchaError.value = false;
    signup();
}

/**
 * Handles captcha error and expiry.
 */
function onCaptchaError(): void {
    captchaResponseToken.value = '';
    captchaError.value = true;
}

/**
 * Holds on login button click logic.
 */
async function onSignupClick(): Promise<void> {
    form.value?.validate();
    if (!formValid.value || isLoading.value) {
        return;
    }

    isLoading.value = true;
    if (hcaptcha.value && !captchaResponseToken.value) {
        hcaptcha.value?.execute();
        return;
    }

    await signup();
}

/**
 * Creates user.
 */
async function signup(): Promise<void> {
    const finalEmail = isInvited.value ? queryEmail.value : email.value;
    try {
        signupID.value = await auth.register({
            email: finalEmail,
            password: password.value,
            partner: partner.value,
            signupPromoCode: signupPromoCode.value,
            isMinimal: true,
        }, secret.value, captchaResponseToken.value);

        if (!codeActivationEnabled.value) {
            analyticsStore.eventTriggered(AnalyticsEvent.USER_SIGN_UP);
            // Brave browser conversions are tracked via the RegisterSuccess path in the satellite app
            // signups outside of the brave browser may use a configured URL to track conversions
            // if the URL is not configured, the RegisterSuccess path will be used for non-Brave browsers
            const internalRegisterSuccessPath = ROUTES.SignupConfirmation.path;
            const configuredRegisterSuccessPath = configStore.state.config.optionalSignupSuccessURL || internalRegisterSuccessPath;

            const nonBraveSuccessPath = `${configuredRegisterSuccessPath}?email=${encodeURIComponent(email.value)}`;
            const braveSuccessPath = `${internalRegisterSuccessPath}?email=${encodeURIComponent(email.value)}`;

            const altRoute = `${window.location.origin}/${nonBraveSuccessPath}`;
            await detectBraveBrowser() ? await router.push(braveSuccessPath) : window.location.href = altRoute;
        } else {
            confirmCode.value = true;
        }
    } catch (error) {
        notify.notifyError(error);
    }

    hcaptcha.value?.reset();
    captchaResponseToken.value = '';
    isLoading.value = false;
}

/**
 * Detect if user uses Brave browser
 */
async function detectBraveBrowser(): Promise<boolean> {
    return (navigator['brave'] && await navigator['brave'].isBrave()) || false;
}

onBeforeMount(async () => {
    if (route.query.partner) {
        partner.value = route.query.partner.toString();
    }

    if (route.query.promo) {
        signupPromoCode.value = route.query.promo.toString();
    }

    // If partner.value is true, attempt to load the partner-specific configuration
    if (partner.value) {
        try {
            const config = (await import('@/views/registration/registrationViewConfig.json')).default;
            viewConfig.value = config[partner.value];
        } catch (e) {
        // Handle errors, such as a missing configuration file
            notify.error('No configuration file for registration page.');
        }
    }
});

watch(password, () => {
    if (repPassword.value) {
        repPasswordField.value?.validate();
    }
});
</script>
