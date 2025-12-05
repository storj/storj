// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container class="fill-height">
        <v-row justify="center">
            <v-col cols="12" sm="9" md="7" lg="5" xl="4" xxl="3">
                <v-card v-if="!isMFARequired" title="Reset Password" subtitle="Please enter your new password." class="pa-2 pa-sm-7 overflow-visible">
                    <v-card-text>
                        <v-form ref="form" v-model="formValid" class="pt-4" @submit.prevent>
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
                                            @toggle-visibility="showPassword = !showPassword"
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
                                        @toggle-visibility="showPassword = !showPassword"
                                    />
                                </template>
                            </v-text-field>

                            <v-btn
                                color="primary"
                                size="large"
                                block
                                :loading="isLoading"
                                @click="onResetClick"
                            >
                                Reset Password
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
                    @verify="onResetClick"
                />
                <p class="mt-5 text-center text-body-2"><router-link class="link" :to="ROUTES.Login.path">Back to login</router-link></p>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { VBtn, VCard, VCardText, VCol, VContainer, VForm, VRow, VTextField } from 'vuetify/components';
import { computed, onBeforeMount, onMounted, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import { GoodPasswordRule, RequiredRule, ValidationRule } from '@/types/common';
import { ErrorMFARequired } from '@/api/errors/ErrorMFARequired';
import { ErrorTokenExpired } from '@/api/errors/ErrorTokenExpired';
import { ErrorTooManyAttempts } from '@/api/errors/ErrorTooManyAttempts';
import { AuthHttpApi } from '@/api/auth';
import { useNotify } from '@/composables/useNotify';
import { useLoading } from '@/composables/useLoading';
import { ROUTES } from '@/router';
import { useConfigStore } from '@/store/modules/configStore';
import { useUsersStore } from '@/store/modules/usersStore';

import MfaComponent from '@/views/MfaComponent.vue';
import PasswordInputEyeIcons from '@/components/PasswordInputEyeIcons.vue';
import PasswordStrength from '@/components/PasswordStrength.vue';

const auth: AuthHttpApi = new AuthHttpApi();

const configStore = useConfigStore();
const usersStore = useUsersStore();

const router = useRouter();
const route = useRoute();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const showPassword = ref(false);
const showPasswordStrength = ref(false);
const useOTP = ref(true);
const isMFARequired = ref(false);
const isMFAError = ref(false);
const formValid = ref<boolean>(false);

const password = ref('');
const repPassword = ref('');
const passcode = ref('');
const recoveryCode = ref('');
const token = ref<string>('');

const form = ref<VForm | null>(null);
const repPasswordField = ref<VTextField | null>(null);

const passMaxLength = computed<number>(() => configStore.state.config.passwordMaximumLength);
const passMinLength = computed<number>(() => configStore.state.config.passwordMinimumLength);
const badPasswords = computed<Set<string>>(() => usersStore.state.badPasswords);
const liveCheckBadPassword = computed<boolean>(() => configStore.state.config.liveCheckBadPasswords);

const passwordRules = computed<ValidationRule<string>[]>(() => {
    const rules = [
        RequiredRule,
        (value: string) => value.length < passMinLength.value || value.length > passMaxLength.value
            ? `Password must be between ${passMinLength.value} and ${passMaxLength.value} characters`
            : true,
    ];
    if (liveCheckBadPassword.value) rules.push(GoodPasswordRule);

    return rules;
});

const repeatPasswordRules = computed<ValidationRule<string>[]>(() => [
    ...passwordRules.value,
    (value: string) => {
        if (password.value !== value) {
            return 'Passwords do not match';
        }
        return true;
    },
]);

/**
 * Validates input fields and requests password reset.
 */
function onResetClick(): void {
    form.value?.validate();
    if (!formValid.value) {
        return;
    }

    withLoading(async () => {
        try {
            await auth.resetPassword(token.value, password.value, passcode.value.trim(), recoveryCode.value.trim());
            notify.success('Password reset successfully');
            await router.push(ROUTES.Login.path);
        } catch (error) {
            isLoading.value = false;

            if (error instanceof ErrorMFARequired) {
                if (isMFARequired.value) isMFAError.value = true;
                isMFARequired.value = true;
                return;
            }

            if (error instanceof ErrorTokenExpired) {
                await router.push({
                    name: ROUTES.ForgotPassword.name,
                    query: { expired: 'true' },
                });
                return;
            }

            if (isMFARequired.value) {
                if (error instanceof ErrorTooManyAttempts) notify.notifyError(error);

                isMFAError.value = true;
                return;
            }

            notify.notifyError(error);
        }
    });
}

onBeforeMount(() => {
    if (liveCheckBadPassword.value && badPasswords.value.size === 0) {
        usersStore.getBadPasswords().catch(() => {});
    }
});

/**
 * Lifecycle hook after initial render.
 * Initializes recovery token from route param
 * and redirects to login page if token doesn't exist.
 */
onMounted(() => {
    if (route.query.token) {
        token.value = route.query.token.toString();
    } else {
        router.push(ROUTES.Login.path);
    }
});

watch(password, () => {
    if (repPassword.value) {
        repPasswordField.value?.validate();
    }
});
</script>
