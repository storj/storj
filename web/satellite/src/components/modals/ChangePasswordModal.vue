// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="change-password">
                <div class="change-password__row">
                    <ChangePasswordIcon />
                    <h2 class="change-password__row__label">Change Password</h2>
                </div>
                <VInput
                    class="full-input"
                    label="Old Password"
                    placeholder="Old Password"
                    is-password
                    :error="oldPasswordError"
                    @setData="setOldPassword"
                />
                <div class="password-input">
                    <VInput
                        class="full-input"
                        label="New Password"
                        placeholder="New Password"
                        is-password
                        :error="newPasswordError"
                        @setData="setNewPassword"
                        @showPasswordStrength="showPasswordStrength"
                        @hidePasswordStrength="hidePasswordStrength"
                    />
                    <PasswordStrength
                        :password-string="newPassword"
                        :is-shown="isPasswordStrengthShown"
                    />
                </div>
                <VInput
                    class="full-input"
                    label="Confirm Password"
                    placeholder="Confirm Password"
                    is-password
                    :error="confirmationPasswordError"
                    @setData="setPasswordConfirmation"
                />
                <div class="change-password__buttons">
                    <VButton
                        label="Cancel"
                        width="100%"
                        height="48px"
                        :on-press="closeModal"
                        :is-transparent="true"
                    />
                    <VButton
                        label="Update"
                        width="100%"
                        height="48px"
                        :on-press="onUpdateClick"
                    />
                </div>
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { useRouter } from 'vue-router';

import { AuthHttpApi } from '@/api/auth';
import { RouteConfig } from '@/types/router';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useConfigStore } from '@/store/modules/configStore';
import { useAppStore } from '@/store/modules/appStore';

import PasswordStrength from '@/components/common/PasswordStrength.vue';
import VInput from '@/components/common/VInput.vue';
import VButton from '@/components/common/VButton.vue';
import VModal from '@/components/common/VModal.vue';

import ChangePasswordIcon from '@/../static/images/account/changePasswordPopup/changePassword.svg';

const configStore = useConfigStore();
const appStore = useAppStore();
const notify = useNotify();
const router = useRouter();

const DELAY_BEFORE_REDIRECT = 2000; // 2 sec
const auth: AuthHttpApi = new AuthHttpApi();
const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

const oldPassword = ref<string>('');
const newPassword = ref<string>('');
const confirmationPassword = ref<string>('');
const oldPasswordError = ref<string>('');
const newPasswordError = ref<string>('');
const confirmationPasswordError = ref<string>('');
const isPasswordStrengthShown = ref<boolean>(false);

/**
 * Enables password strength info container.
 */
function showPasswordStrength(): void {
    isPasswordStrengthShown.value = true;
}

/**
 * Disables password strength info container.
 */
function hidePasswordStrength(): void {
    isPasswordStrengthShown.value = false;
}

/**
 * Sets old password from input.
 */
function setOldPassword(value: string): void {
    oldPassword.value = value;
    oldPasswordError.value = '';
}

/**
 * Sets new password from input.
 */
function setNewPassword(value: string): void {
    newPassword.value = value;
    newPasswordError.value = '';
}

/**
 * Sets password confirmation from input.
 */
function setPasswordConfirmation(value: string): void {
    confirmationPassword.value = value;
    confirmationPasswordError.value = '';
}

/**
 * Validates inputs and if everything are correct tries to change password and close popup.
 */
async function onUpdateClick(): Promise<void> {
    let hasError = false;
    if (oldPassword.value.length < 6) {
        oldPasswordError.value = 'Invalid old password. Must be 6 or more characters';
        hasError = true;
    }

    const config = configStore.state.config;

    if (newPassword.value.length < config.passwordMinimumLength) {
        newPasswordError.value = `Invalid password. Use ${config.passwordMinimumLength} or more characters`;
        hasError = true;
    }

    if (newPassword.value.length > config.passwordMaximumLength) {
        newPasswordError.value = `Invalid password. Use ${config.passwordMaximumLength} or fewer characters`;
        hasError = true;
    }

    if (!confirmationPassword.value) {
        confirmationPasswordError.value = 'Password required';
        hasError = true;
    }

    if (newPassword.value !== confirmationPassword.value) {
        confirmationPasswordError.value = 'Password doesn\'t match new one';
        hasError = true;
    }

    if (hasError) {
        analytics.errorEventTriggered(AnalyticsErrorEventSource.CHANGE_PASSWORD_MODAL);
        return;
    }

    try {
        await auth.changePassword(oldPassword.value, newPassword.value);
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.CHANGE_PASSWORD_MODAL);

        return;
    }

    try {
        await auth.logout();

        setTimeout(() => {
            router.push(RouteConfig.Login.path);
        }, DELAY_BEFORE_REDIRECT);
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.CHANGE_PASSWORD_MODAL);
    }

    analytics.eventTriggered(AnalyticsEvent.PASSWORD_CHANGED);
    await notify.success('Password successfully changed!');
    closeModal();
}

/**
 * Closes popup.
 */
function closeModal(): void {
    appStore.removeActiveModal();
}
</script>

<style scoped lang="scss">
    .change-password {
        background-color: #fff;
        border-radius: 6px;
        display: flex;
        flex-direction: column;
        padding: 48px;
        box-sizing: border-box;
        min-width: 550px;

        @media screen and (width <= 600px) {
            min-width: 475px;
            padding: 48px 24px;
        }

        @media screen and (width <= 530px) {
            min-width: 420px;
        }

        @media screen and (width <= 470px) {
            min-width: unset;
        }

        &__row {
            display: flex;
            align-items: center;
            margin-bottom: 20px;

            @media screen and (width <= 600px) {

                svg {
                    display: none;
                }
            }

            &__label {
                font-family: 'font_bold', sans-serif;
                font-size: 32px;
                line-height: 60px;
                color: #384b65;
                margin: 0 0 0 32px;

                @media screen and (width <= 600px) {
                    font-size: 24px;
                    line-height: 28px;
                    margin: 0;
                }
            }
        }

        &__buttons {
            width: 100%;
            display: flex;
            align-items: center;
            margin-top: 32px;
            column-gap: 20px;

            @media screen and (width <= 600px) {
                flex-direction: column-reverse;
                column-gap: unset;
                row-gap: 10px;
                margin-top: 15px;
            }
        }
    }

    .password-input {
        position: relative;
        width: 100%;
    }

    .full-input {
        margin-bottom: 15px;
    }

    @media screen and (width <= 600px) {

        :deep(.password-strength-container) {
            width: unset;
            height: unset;
        }

        :deep(.password-strength-container__header) {
            flex-direction: column;
            align-items: flex-start;
        }

        :deep(.password-strength-container__rule-area__rule) {
            text-align: left;
        }
    }
</style>
