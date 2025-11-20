// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <signup-confirmation v-if="codeActivationEnabled && confirmCode" :email="isInvited ? queryEmail : email" :signup-req-id="signupID" />
    <v-container v-else class="fill-height">
        <v-row justify="center">
            <v-col cols="12" sm="10" md="6" lg="5" xl="4" xxl="3">
                <v-card :title="title" subtitle="No credit card needed to create an account." class="pa-2 pa-sm-6 overflow-visible mt-1 mb-7 my-sm-8 my-md-0">
                    <v-card-item v-if="isInvited">
                        <v-alert
                            variant="tonal"
                            color="info"
                            density="comfortable"
                            border
                        >
                            <template #text>
                                {{ inviterEmail }} has invited you to a project on {{ configStore.brandName }}. Create an account on the {{ satellite.satellite }} region to join it.
                            </template>
                        </v-alert>
                    </v-card-item>

                    <v-card-text>
                        <v-form ref="form" v-model="formValid" class="pt-3" @submit.prevent="onSignupClick">
                            <v-select
                                v-model="satellite"
                                label="Satellite (Metadata Region)"
                                :items="satellites"
                                item-title="satellite"
                                :hint="satellite.hint"
                                hide-details="auto"
                                persistent-hint
                                return-object
                                chips
                                class="mb-5"
                            />

                            <v-text-field
                                v-if="isInvited"
                                id="Email Address"
                                :model-value="queryEmail"
                                class="mb-5"
                                label="Email address"
                                placeholder="Enter your email"
                                hide-details="auto"
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
                                class="mb-5"
                                label="Email address"
                                placeholder="Enter your email"
                                hide-details="auto"
                                maxlength="72"
                                name="email"
                                type="email"
                                :rules="emailRules"
                                flat
                                clearable
                                required
                                @update:model-value="checkSSO"
                            />

                            <div
                                class="pos-relative"
                                :class="{ hidden: !ssoUnavailable }"
                            >
                                <div class="password-field-container">
                                    <v-text-field
                                        id="Password"
                                        v-model="password"
                                        class="mb-5"
                                        label="Password"
                                        placeholder="Enter a password"
                                        color="secondary"
                                        hide-details="auto"
                                        :type="showPassword ? 'text' : 'password'"
                                        :rules="passwordRules"
                                        required
                                        @focus="showPasswordStrength = true"
                                    >
                                        <template #append-inner>
                                            <password-input-eye-icons
                                                :is-visible="showPassword"
                                                type="password"
                                                :aria-label="showPassword ? 'Hide password' : 'Show password'"
                                                @toggle-visibility="showPassword = !showPassword"
                                            />
                                        </template>
                                    </v-text-field>

                                    <transition name="fade">
                                        <password-strength
                                            v-if="showPasswordStrength && password"
                                            :email="email"
                                            :password="password"
                                            class="password-strength-indicator"
                                        />
                                    </transition>
                                </div>
                            </div>

                            <v-text-field
                                id="Retype Password"
                                ref="repPasswordField"
                                v-model="repPassword"
                                :class="{ hidden: !ssoUnavailable }"
                                label="Retype password"
                                placeholder="Enter a password"
                                color="secondary"
                                hide-details="auto"
                                :type="showPassword ? 'text' : 'password'"
                                :rules="repeatPasswordRules"
                                required
                            >
                                <template #append-inner>
                                    <password-input-eye-icons
                                        :is-visible="showPassword"
                                        type="password"
                                        @toggle-visibility="showPassword = !showPassword"
                                    />
                                </template>
                            </v-text-field>

                            <v-alert
                                v-if="isBetaSatellite"
                                class="my-2"
                                variant="tonal"
                                color="warning"
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
                                    deleted at any time and your storage/download limits
                                    can fluctuate. To use our production service please
                                    create an account on one of our production Satellites.
                                    <a href="https://storj.io/v2/signup/" target="_blank" rel="noopener noreferrer">https://storj.io/v2/signup/</a>
                                </template>
                            </v-alert>

                            <v-checkbox
                                id="Terms checkbox"
                                v-model="acceptedTerms"
                                class="my-3"
                                :rules="[RequiredRule]"
                                density="compact"
                                hide-details="auto"
                                required
                            >
                                <template #label>
                                    <p class="text-body-2 terms-text">
                                        I agree to the
                                        <a class="link font-weight-medium" :href="termsLink" target="_blank" rel="noopener">terms of service</a>
                                        and
                                        <a class="link font-weight-medium" :href="privacyLink" target="_blank" rel="noopener">privacy policy</a>.
                                    </p>
                                </template>
                            </v-checkbox>

                            <v-btn
                                type="submit"
                                :disabled="ssoEnabled && ssoUrl === SsoCheckState.NotChecked"
                                :loading="isLoading"
                                color="primary"
                                size="large"
                                block
                            >
                                Start your free trial
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
            <template v-if="partnerConfig && partnerConfig.title && partnerConfig.description">
                <v-col cols="12" sm="10" md="6" lg="5" xl="4" xxl="3">
                    <v-card class="pa-2 pa-sm-6 h-100 no-position">
                        <v-card-item>
                            <v-card-title class="text-wrap">
                                {{ partnerConfig.title }}
                            </v-card-title>
                            <v-card-subtitle class="text-wrap">
                                {{ partnerConfig.description }}
                            </v-card-subtitle>
                        </v-card-item>
                        <v-card-text>
                            <!-- eslint-disable-next-line vue/no-v-html -->
                            <div v-if="partnerConfig.customHtmlDescription" v-html="partnerConfig.customHtmlDescription" />
                            <a v-if="partnerConfig.partnerLogoBottomUrl" :href="partnerConfig.partnerUrl">
                                <img
                                    :src="partnerConfig.partnerLogoBottomUrl" :srcset="partnerConfig.partnerLogoBottomUrl"
                                    :alt="partnerConfig.name + ' logo'"
                                    height="44"
                                    class="mt-6 rounded white-background"
                                >
                            </a>
                        </v-card-text>
                    </v-card>
                </v-col>
            </template>
            <template v-else>
                <v-col cols="12" sm="10" md="6" lg="5" xl="4" xxl="3">
                    <v-card class="pa-2 pa-sm-6 h-100 no-position d-flex align-center">
                        <v-card-text>
                            <h1 class="font-weight-black signup-heading">
                                <template v-if="partnerConfig && partnerConfig.name">Start using {{ configStore.brandName }} on {{ partnerConfig.name }} today.</template>
                                <template v-else>Start using {{ configStore.brandName }} today.</template>
                            </h1>
                            <p class="text-subtitle-1 mt-4">
                                Whether migrating your data or just testing out {{ configStore.brandName }}, your journey starts here.
                            </p>

                            <p class="mt-6">
                                <v-icon color="primary"><Check :stroke-width="4" /></v-icon>
                                Upload and download 25GB free for 30 days.
                            </p>

                            <p class="mt-4">
                                <v-icon color="primary"><Check :stroke-width="4" /></v-icon>
                                Integrate with any S3 compatible application.
                            </p>

                            <p class="mt-4">
                                <v-icon color="primary"><Check :stroke-width="4" /></v-icon>
                                Total set up takes less than 5 min.
                            </p>

                            <p class="mt-4">
                                <v-icon color="primary"><Check :stroke-width="4" /></v-icon>
                                No credit card required.
                            </p>

                            <p class="mt-6">
                                Need help figuring out if {{ configStore.brandName }} is a fit for your business? <a :href="getInTouchUrl" target="_blank" rel="noopener noreferrer" class="link font-weight-bold">Schedule a meeting</a>.
                            </p>
                        </v-card-text>
                    </v-card>
                </v-col>
            </template>
            <v-row justify="center" class="v-col-12">
                <v-col>
                    <p class="pt-9 text-center text-body-2">Already have an account? <router-link class="link font-weight-bold" :to="ROUTES.Login.path">Login</router-link></p>
                </v-col>
            </v-row>
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
    VIcon,
} from 'vuetify/components';
import { computed, ComputedRef, onBeforeMount, ref, watch } from 'vue';
import VueHcaptcha from '@hcaptcha/vue3-hcaptcha';
import { useRoute, useRouter } from 'vue-router';
import { Check } from 'lucide-vue-next';

import { useConfigStore } from '@/store/modules/configStore';
import { EmailRule, GoodPasswordRule, RequiredRule } from '@/types/common';
import { MultiCaptchaConfig } from '@/types/config.gen';
import { PartnerConfig } from '@/types/partners';
import { AuthHttpApi } from '@/api/auth';
import { useNotify } from '@/composables/useNotify';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { ROUTES } from '@/router';
import { SsoCheckState } from '@/types/users';
import { APIError } from '@/utils/error';
import { useUsersStore } from '@/store/modules/usersStore';

import SignupConfirmation from '@/views/SignupConfirmation.vue';
import PasswordInputEyeIcons from '@/components/PasswordInputEyeIcons.vue';
import PasswordStrength from '@/components/PasswordStrength.vue';

const auth = new AuthHttpApi();

const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();
const usersStore = useUsersStore();

const router = useRouter();
const notify = useNotify();
const route = useRoute();

const isLoading = ref<boolean>(false);
const isCheckingSso = ref<boolean>(false);
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
const ssoUrl = ref<string>(SsoCheckState.NotChecked);
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
const ssoCheckTimeout = ref<NodeJS.Timeout>();

const satellitesHints = [
    { satellite: 'Storj', hint: 'Recommended satellite.' },
    { satellite: 'QA-Satellite', hint: 'This is the Storj beta satellite.' },
    { satellite: 'US1', hint: 'Recommended for North and South America' },
    { satellite: 'EU1', hint: 'Recommended for Europe and Africa' },
    { satellite: 'AP1', hint: 'Recommended for Asia and Oceania' },
];

const emailRules: ((_: string) => boolean | string)[] = [
    RequiredRule,
    (value) => EmailRule(value, true),
];

const partnerConfig = computed<PartnerConfig | null>(() =>
    (configStore.signupConfig.get(route.query.partner?.toString() ?? '') ?? null) as PartnerConfig | null,
);

const badPasswords = computed<Set<string>>(() => usersStore.state.badPasswords);
const liveCheckBadPassword = computed<boolean>(() => configStore.state.config.liveCheckBadPasswords);

const ssoEnabled = computed(() => configStore.state.config.ssoEnabled);

const title = computed<string>(() => `Create your ${configStore.brandName} account.`);
const termsLink = computed<string>(() => `${configStore.homepageUrl}/terms-of-service/`);
const privacyLink = computed<string>(() => `${configStore.homepageUrl}/privacy-policy/`);
const getInTouchUrl = computed<string>(() => configStore.state.branding.getInTouchUrl);

const passwordRules = computed(() => {
    const rules = [
        RequiredRule,
        (value: string) => value.length < passMinLength.value || value.length > passMaxLength.value
            ? `Password must be between ${passMinLength.value} and ${passMaxLength.value} characters`
            : true,
    ];
    if (liveCheckBadPassword.value) rules.push(GoodPasswordRule);

    if (!ssoEnabled.value) {
        return rules;
    }
    switch (ssoUrl.value) {
    case SsoCheckState.None:
    case SsoCheckState.Failed:
    case SsoCheckState.NotChecked:
        return rules;
    default:
        return [];
    }
});

const repeatPasswordRules = computed(() => {
    if (passwordRules.value.length === 0) {
        return [];
    }
    return [
        ...passwordRules.value,
        (value: string) => {
            if (password.value !== value) {
                return 'Passwords do not match';
            }
            return true;
        },
    ];
});

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
            window.location.href = satellite.address + ROUTES.Signup.path + window.location.search;
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

const ssoUnavailable = computed(() => {
    return !ssoEnabled.value || ssoUrl.value === SsoCheckState.None
      || ssoUrl.value === SsoCheckState.Failed
      || ssoUrl.value === SsoCheckState.NotChecked;
});

/**
 * Returns true if signup activation code is enabled.
 */
const codeActivationEnabled = computed((): boolean => {
    return configStore.state.config.signupActivationCodeEnabled;
});

/**
 * Indicates if satellite is in beta.
 */
const isBetaSatellite = computed((): boolean => {
    return configStore.state.config.isBetaSatellite && configStore.isDefaultBrand;
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

function checkSSO(mail: string) {
    if (!ssoEnabled.value) {
        return;
    }
    clearTimeout(ssoCheckTimeout.value);
    ssoUrl.value = SsoCheckState.NotChecked;
    if (!emailRules.every(rule => rule(mail) === true)) {
        return;
    }
    ssoCheckTimeout.value = setTimeout(async () => {
        isCheckingSso.value = true;
        let urlStr: string;
        try {
            urlStr = await auth.checkSSO(mail);
        } catch (error) {
            if (error instanceof APIError && error.status === 404) {
                ssoUrl.value = SsoCheckState.None;
                return;
            }
            ssoUrl.value = SsoCheckState.Failed;
            notify.notifyError(error);
            return;
        } finally {
            isCheckingSso.value = false;
        }
        try {
        // check if the URL is valid.
            new URL(urlStr);
            ssoUrl.value = urlStr;
        } catch {
            ssoUrl.value = SsoCheckState.Failed;
        }
    }, 1000);
}

/**
 * Holds on login button click logic.
 */
async function onSignupClick(): Promise<void> {
    form.value?.validate();
    if (!formValid.value || isLoading.value || (ssoEnabled.value && ssoUrl.value === SsoCheckState.NotChecked)) {
        return;
    }

    async function triggerSignup() {
        if (hcaptcha.value && !captchaResponseToken.value) {
            hcaptcha.value?.execute();
            return;
        }
        await signup();
    }

    isLoading.value = true;
    if (!ssoEnabled.value) {
        await triggerSignup();
        return;
    }

    let url: URL;
    switch (ssoUrl.value) {
    case SsoCheckState.None:
    case SsoCheckState.Failed:
        await triggerSignup();
        break;
    default:
        url = new URL(ssoUrl.value);
        url.searchParams.set('email', email.value);
        window.open(url.toString(), '_self');
    }
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
            inviterEmail: inviterEmail.value,
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

            if (await detectBraveBrowser()) {
                await router.push(braveSuccessPath);
            } else {
                window.location.href = altRoute;
            }
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
    if (liveCheckBadPassword.value && badPasswords.value.size === 0) {
        usersStore.getBadPasswords().catch(() => {});
    }

    if (route.query.partner) {
        partner.value = route.query.partner.toString();
    }

    if (route.query.promo) {
        signupPromoCode.value = route.query.promo.toString();
    }

    if (queryEmail.value) {
        checkSSO(queryEmail.value);
    }
});

watch(password, () => {
    if (repPassword.value) {
        repPasswordField.value?.validate();
    }
});
</script>

<style scoped>
.password-field-container {
    position: relative;
}

.password-strength-indicator {
    margin-top: 4px;
}

.fade-enter-active,
.fade-leave-active {
    transition: opacity 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
    opacity: 0;
}

.terms-text {
    letter-spacing: -0.3px !important;
}
</style>