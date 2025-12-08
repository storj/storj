// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container class="fill-height">
        <v-row justify="center">
            <v-col cols="12" sm="9" md="7" lg="5" xl="4" xxl="3">
                <v-card v-if="!isMFARequired" title="Welcome back" :subtitle="subtitle" class="pa-2 pa-sm-6 pb-sm-7">
                    <v-card-text>
                        <v-alert
                            v-if="captchaError"
                            variant="tonal"
                            color="error"
                            text="hCaptcha is required. If you are using a VPN, try disabling it."
                            density="comfortable"
                            class="mt-2 mb-3"
                            border
                        />
                        <v-alert
                            v-if="isActivatedBannerShown"
                            variant="tonal"
                            :color="isActivatedError ? 'error' : 'success'"
                            :title="isActivatedError ? 'Oops!' :'Success!'"
                            :text="isActivatedError ? 'This account has already been verified.' : 'Account verified.'"
                            density="comfortable"
                            class="mt-1 mb-3"
                            border
                        />
                        <v-alert
                            v-if="inviteInvalid"
                            variant="tonal"
                            color="error"
                            title="Oops!"
                            text="The invite link you used has expired or is invalid."
                            density="comfortable"
                            class="mt-1 mb-3"
                            border
                        />
                        <v-alert
                            v-if="ssoFailed"
                            variant="tonal"
                            color="error"
                            title="Single Sign-on Failed"
                            text="Single sign-on failed. Please check with your administrator."
                            density="comfortable"
                            class="mt-1 mb-3"
                            border
                        />
                        <v-alert
                            v-if="isBadLoginMessageShown"
                            variant="tonal"
                            color="error"
                            title="Invalid Credentials"
                            text="Login failed. Please check if this is the correct satellite for your account. If you are
                            sure your credentials are correct, please check your email inbox for a notification with
                            further instructions."
                            density="comfortable"
                            class="mt-1 mb-3"
                            border
                        />
                        <v-form ref="form" v-model="formValid" class="pt-4" @submit.prevent="onLoginClick">
                            <v-select
                                v-model="satellite"
                                label="Satellite"
                                :items="satellites"
                                item-title="satellite"
                                :hint="satellite.hint"
                                persistent-hint
                                return-object
                                chips
                                class="mb-6"
                            />

                            <v-text-field
                                id="Email Address"
                                v-model="email"
                                class="mb-2"
                                label="Email address"
                                placeholder="Enter your email"
                                name="email"
                                type="email"
                                :rules="emailRules"
                                flat
                                clearable
                                required
                                :disabled="!!pathEmail"
                                @update:model-value="checkSSO"
                            />

                            <v-text-field
                                id="Password"
                                v-model="password"
                                :class="{ hidden: !ssoUnavailable }"
                                class="mb-2"
                                label="Password"
                                placeholder="Enter your password"
                                color="secondary"
                                :type="showPassword ? 'text' : 'password'"
                                :rules="passwordRules"
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

                            <v-row>
                                <v-col>
                                    <v-checkbox
                                        v-if="ssoUnavailable"
                                        v-model="rememberForOneWeek"
                                        label="Remember Me"
                                        density="compact"
                                        class="mt-n4 mb-3 text-body-2"
                                        hide-details
                                    >
                                        <v-tooltip
                                            activator="parent"
                                            location="top"
                                        >
                                            Stay logged in for 7 days.
                                        </v-tooltip>
                                    </v-checkbox>
                                </v-col>

                                <v-col v-if="ssoUnavailable">
                                    <p class="text-right mt-n2 mb-4">
                                        <router-link class="link" :to="ROUTES.ForgotPassword.path">
                                            Forgot Password
                                        </router-link>
                                    </p>
                                </v-col>
                            </v-row>

                            <v-btn
                                type="submit"
                                color="primary"
                                size="large"
                                block
                                :disabled="ssoEnabled && ssoUrl === SsoCheckState.NotChecked"
                                :loading="isLoading || isCheckingSso"
                            >
                                Continue
                            </v-btn>
                        </v-form>
                    </v-card-text>
                </v-card>
                <mfa-component
                    v-else
                    v-model="useOTP"
                    v-model:error="isMFAError"
                    v-model:otp="passcode"
                    v-model:recovery="recoveryCode"
                    :loading="isLoading"
                    @verify="onLoginClick"
                />
                <VueHcaptcha
                    v-if="captchaConfig.hcaptcha.enabled"
                    ref="hcaptcha"
                    :sitekey="captchaConfig.hcaptcha.siteKey"
                    :re-captcha-compat="false"
                    size="invisible"
                    @verify="onCaptchaVerified"
                    @expired="onCaptchaError"
                    @error="onCaptchaError"
                />
                <p class="mt-5 text-center text-body-2">Don't have an account? <router-link class="link font-weight-bold" :to="ROUTES.Signup.path">Sign Up</router-link></p>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { VAlert, VBtn, VCard, VCardText, VCol, VContainer, VForm, VRow, VSelect, VTextField, VCheckbox, VTooltip } from 'vuetify/components';
import { computed, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import VueHcaptcha from '@hcaptcha/vue3-hcaptcha';

import { EmailRule, RequiredRule } from '@/types/common';
import { AuthHttpApi } from '@/api/auth';
import { useConfigStore } from '@/store/modules/configStore';
import { useAppStore } from '@/store/modules/appStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { useNotify } from '@/composables/useNotify';
import { MultiCaptchaConfig } from '@/types/config.gen';
import { LocalData } from '@/utils/localData';
import { SsoCheckState, TokenInfo } from '@/types/users';
import { ErrorMFARequired } from '@/api/errors/ErrorMFARequired';
import { ErrorUnauthorized } from '@/api/errors/ErrorUnauthorized';
import { ErrorTooManyRequests } from '@/api/errors/ErrorTooManyRequests';
import { ErrorBadRequest } from '@/api/errors/ErrorBadRequest';
import { ROUTES } from '@/router';
import { APIError } from '@/utils/error';

import MfaComponent from '@/views/MfaComponent.vue';
import PasswordInputEyeIcons from '@/components/PasswordInputEyeIcons.vue';

const auth = new AuthHttpApi();

const configStore = useConfigStore();
const appStore = useAppStore();
const usersStore = useUsersStore();
const notify = useNotify();
const router = useRouter();
const route = useRoute();

const showPassword = ref(false);
const isLoading = ref<boolean>(false);
const isCheckingSso = ref<boolean>(false);
const isBadLoginMessageShown = ref<boolean>(false);
const formValid = ref<boolean>(false);
const ssoFailed = ref(false);
const inviteInvalid = ref(false);
const isActivatedBannerShown = ref(false);
const isActivatedError = ref(false);
const captchaError = ref(false);
const useOTP = ref(true);
const isMFARequired = ref(false);
const isMFAError = ref(false);
const rememberForOneWeek = ref<boolean>(false);

const ssoUrl = ref<string>(SsoCheckState.NotChecked);
const captchaResponseToken = ref('');
const email = ref('');
const password = ref('');
const passcode = ref('');
const recoveryCode = ref('');
const pathEmail = ref<string | null>(null);
const returnURL = ref(ROUTES.Projects.path);

const ssoCheckTimeout = ref<NodeJS.Timeout>();
const hcaptcha = ref<VueHcaptcha | null>(null);
const form = ref<VForm | null>(null);

const satellitesHints = [
    { satellite: 'Storj', hint: 'Recommended satellite.' },
    { satellite: 'QA-Satellite', hint: 'This is the Storj beta satellite.' },
    { satellite: 'US1', hint: 'Recommended for North and South America' },
    { satellite: 'EU1', hint: 'Recommended for Europe and Africa' },
    { satellite: 'AP1', hint: 'Recommended for Asia and Oceania' },
];

const emailRules: ((_: string) => boolean | string)[] = [
    RequiredRule,
    EmailRule,
];

const subtitle = computed<string>(() => `Log in to your ${configStore.brandName} account`);

const ssoEnabled = computed(() => configStore.state.config.ssoEnabled);

const csrfToken = computed<string>(() => configStore.state.config.csrfToken);

const passwordRules = computed(() => {
    if (!ssoEnabled.value) {
        return [ RequiredRule ];
    }
    switch (ssoUrl.value) {
    case SsoCheckState.None:
    case SsoCheckState.Failed:
    case SsoCheckState.NotChecked:
        return [ RequiredRule ];
    default:
        return [];
    }
});

/**
 * Name of the current satellite.
 */
const satellite = computed({
    get: () => {
        const satName = configStore.state.config.satelliteName;
        const item = satellitesHints.find(item => item.satellite === satName);
        return item ?? { satellite: satName, hint: '' };
    },
    set: value => {
        const sats = configStore.state.config.partneredSatellites ?? [];
        const satellite = sats.find(sat => sat.name === value.satellite);
        if (satellite) {
            window.location.href = satellite.address + ROUTES.Login.path;
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
 * This component's captcha configuration.
 */
const captchaConfig = computed((): MultiCaptchaConfig => {
    return configStore.state.config.captcha.login;
});

/**
 * Handles captcha verification response.
 */
function onCaptchaVerified(response: string): void {
    captchaResponseToken.value = response;
    captchaError.value = false;
    login();
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
async function onLoginClick(): Promise<void> {
    form.value?.validate();
    if (!formValid.value || isLoading.value || (ssoEnabled.value && ssoUrl.value === SsoCheckState.NotChecked)) {
        return;
    }

    async function triggerLogin() {
        if (!isMFARequired.value && hcaptcha.value && !captchaResponseToken.value) {
            hcaptcha.value?.execute();
            return;
        }
        await login();
    }

    isLoading.value = true;
    if (!ssoEnabled.value) {
        await triggerLogin();
        return;
    }

    let url: URL;
    switch (ssoUrl.value) {
    case SsoCheckState.None:
    case SsoCheckState.Failed:
        await triggerLogin();
        break;
    default:
        url = new URL(ssoUrl.value);
        url.searchParams.set('email', email.value);
        window.open(url.toString(), '_self');
    }
}

/**
 * Performs login action.
 * Then changes location to project dashboard page.
 */
async function login(): Promise<void> {
    try {
        const tokenInfo: TokenInfo = await auth.token(email.value, password.value, captchaResponseToken.value, passcode.value, recoveryCode.value, csrfToken.value, rememberForOneWeek.value);
        LocalData.setSessionExpirationDate(tokenInfo.expiresAt);
        if (rememberForOneWeek.value) {
            LocalData.setCustomSessionDuration(604800); // 7 days in seconds.
        } else if (LocalData.getCustomSessionDuration()) {
            LocalData.removeCustomSessionDuration();
        }
    } catch (error) {
        if (hcaptcha.value) {
            hcaptcha.value?.reset();
            captchaResponseToken.value = '';
        }

        if (error instanceof ErrorMFARequired) {
            isLoading.value = false;
            isMFARequired.value = true;
            return;
        }

        if (isMFARequired.value && !(error instanceof ErrorTooManyRequests)) {
            if (error instanceof ErrorBadRequest || error instanceof ErrorUnauthorized) {
                notify.notifyError(error);
            }

            isMFAError.value = true;
            isLoading.value = false;
            return;
        }

        if (error instanceof ErrorUnauthorized) {
            isBadLoginMessageShown.value = true;
            isLoading.value = false;
            return;
        }

        notify.notifyError(error);
        isLoading.value = false;
        return;
    }

    appStore.toggleHasJustLoggedIn(true);
    usersStore.login();
    isLoading.value = false;

    await router.push(returnURL.value);
}

/**
 * Lifecycle hook after initial render.
 * Makes activated banner visible on successful account activation.
 */
onMounted(() => {
    inviteInvalid.value = (route.query.invite_invalid as string ?? null) === 'true';
    pathEmail.value = route.query.email as string ?? null;
    if (pathEmail.value) {
        email.value = pathEmail.value.trim();
        checkSSO(email.value);
    }

    ssoFailed.value = !!route.query.sso_failed;
    isActivatedBannerShown.value = !!route.query.activated;
    isActivatedError.value = route.query.activated === 'false';

    if (route.query.return_url) returnURL.value = route.query.return_url as string;
});
</script>
