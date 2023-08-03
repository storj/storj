// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="enable-mfa">
                <h1 class="enable-mfa__title">Two-Factor Authentication</h1>
                <p v-if="isScan" class="enable-mfa__subtitle">
                    Scan this QR code in your favorite TOTP app to get started.
                </p>
                <p v-if="isEnable" class="enable-mfa__subtitle max-width">
                    Enter the authentication code generated in your TOTP app to confirm your account is connected.
                </p>
                <p v-if="isCodes" class="enable-mfa__subtitle">
                    Save recovery codes.
                </p>
                <div v-if="isScan" class="enable-mfa__scan">
                    <h2 class="enable-mfa__scan__title">Scan this QR Code</h2>
                    <p class="enable-mfa__scan__subtitle">Scan the following QR code in your OTP app.</p>
                    <div class="enable-mfa__scan__qr">
                        <canvas ref="canvas" class="enable-mfa__scan__qr__canvas" />
                    </div>
                    <p class="enable-mfa__scan__subtitle">Unable to scan? Use the following code instead:</p>
                    <p class="enable-mfa__scan__secret">{{ userMFASecret }}</p>
                </div>
                <div v-if="isEnable" class="enable-mfa__confirm">
                    <h2 class="enable-mfa__confirm__title">Confirm Authentication Code</h2>
                    <ConfirmMFAInput :on-input="onConfirmInput" :is-error="isError" />
                </div>
                <div v-if="isCodes" class="enable-mfa__codes">
                    <h2 class="enable-mfa__codes__title max-width">
                        Please save these codes somewhere to be able to recover access to your account.
                    </h2>
                    <p
                        v-for="(code, index) in userMFARecoveryCodes"
                        :key="index"
                    >
                        {{ code }}
                    </p>
                </div>
                <div class="enable-mfa__buttons">
                    <VButton
                        v-if="!isCodes"
                        label="Cancel"
                        width="100%"
                        height="44px"
                        :is-white="true"
                        :on-press="closeModal"
                    />
                    <VButton
                        v-if="isScan"
                        label="Continue"
                        width="100%"
                        height="44px"
                        :on-press="showEnable"
                    />
                    <VButton
                        v-if="isEnable"
                        label="Enable"
                        width="100%"
                        height="44px"
                        :on-press="enable"
                        :is-disabled="!confirmPasscode || isLoading"
                    />
                    <VButton
                        v-if="isCodes"
                        label="Done"
                        width="100%"
                        height="44px"
                        :on-press="closeModal"
                    />
                </div>
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import QRCode from 'qrcode';
import { computed, onMounted, ref } from 'vue';

import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useUsersStore } from '@/store/modules/usersStore';
import { useAppStore } from '@/store/modules/appStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';

import ConfirmMFAInput from '@/components/account/mfa/ConfirmMFAInput.vue';
import VButton from '@/components/common/VButton.vue';
import VModal from '@/components/common/VModal.vue';

const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();
const appStore = useAppStore();
const usersStore = useUsersStore();
const notify = useNotify();

const isScan = ref<boolean>(true);
const isEnable = ref<boolean>(false);
const isCodes = ref<boolean>(false);
const isError = ref<boolean>(false);
const isLoading = ref<boolean>(false);
const confirmPasscode = ref<string>('');
const canvas = ref<HTMLCanvasElement>();

/**
 * Returns satellite name from store.
 */
const satellite = computed((): string => {
    return configStore.state.config.satelliteName;
});

/**
 * Returns pre-generated MFA secret from store.
 */
const userMFASecret = computed((): string => {
    return usersStore.state.userMFASecret;
});

/**
 * Returns user MFA recovery codes from store.
 */
const userMFARecoveryCodes = computed((): string[] => {
    return usersStore.state.userMFARecoveryCodes;
});

const qrLink = `otpauth://totp/${encodeURIComponent(usersStore.state.user.email)}?secret=${userMFASecret.value}&issuer=${encodeURIComponent(`STORJ ${satellite.value}`)}&algorithm=SHA1&digits=6&period=30`;

/**
 * Toggles view to Enable MFA state.
 */
function showEnable(): void {
    isScan.value = false;
    isEnable.value = true;
}

/**
 * Closes enable MFA modal.
 */
function closeModal(): void {
    appStore.removeActiveModal();
}

/**
 * Toggles view to MFA Recovery Codes state.
 */
async function showCodes(): Promise<void> {
    try {
        await usersStore.generateUserMFARecoveryCodes();
        isEnable.value = false;
        isCodes.value = true;
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.ENABLE_MFA_MODAL);
    }
}

/**
 * Sets confirmation passcode value from input.
 */
function onConfirmInput(value: string): void {
    isError.value = false;
    confirmPasscode.value = value;
}

/**
 * Enables user MFA and sets view to Recovery Codes state.
 */
async function enable(): Promise<void> {
    if (!confirmPasscode.value || isLoading.value || isError.value) return;

    isLoading.value = true;

    try {
        await usersStore.enableUserMFA(confirmPasscode.value);
        await usersStore.getUser();
        await showCodes();

        analyticsStore.eventTriggered(AnalyticsEvent.MFA_ENABLED);
        notify.success('MFA was enabled successfully');
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.ENABLE_MFA_MODAL);
        isError.value = true;
    }

    isLoading.value = false;
}

/**
 * Mounted lifecycle hook after initial render.
 * Renders QR code.
 */
onMounted(async (): Promise<void> => {
    try {
        await QRCode.toCanvas(canvas.value, qrLink);
    } catch (error) {
        notify.error(error.message, AnalyticsErrorEventSource.ENABLE_MFA_MODAL);
    }
});
</script>

<style scoped lang="scss">
    .enable-mfa {
        padding: 60px;
        background: #fff;
        border-radius: 6px;
        display: flex;
        flex-direction: column;
        align-items: center;
        font-family: 'font_regular', sans-serif;

        @media screen and (width <= 550px) {
            padding: 48px 24px;
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 28px;
            line-height: 34px;
            text-align: center;
            color: #000;
            margin: 0 0 30px;

            @media screen and (width <= 550px) {
                font-size: 24px;
                line-height: 28px;
                margin-bottom: 15px;
            }
        }

        &__subtitle {
            font-size: 16px;
            line-height: 21px;
            text-align: center;
            color: #000;
            margin: 0 0 45px;

            @media screen and (width <= 550px) {
                font-size: 14px;
                line-height: 18px;
                margin-bottom: 20px;
            }
        }

        &__scan {
            padding: 25px;
            background: #f5f6fa;
            border-radius: 6px;
            display: flex;
            flex-direction: column;
            align-items: center;
            width: calc(100% - 50px);

            @media screen and (width <= 550px) {
                padding: 15px;
                width: calc(100% - 30px);
            }

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 16px;
                line-height: 19px;
                text-align: center;
                color: #000;
                margin: 0 0 30px;

                @media screen and (width <= 550px) {
                    margin-bottom: 15px;
                }
            }

            &__subtitle {
                font-size: 14px;
                line-height: 25px;
                text-align: center;
                color: #000;
            }

            &__qr {
                margin: 30px 0;
                background: #fff;
                border-radius: 6px;
                padding: 10px;

                &__canvas {
                    height: 200px !important;
                    width: 200px !important;

                    @media screen and (width <= 550px) {
                        height: unset !important;
                        width: 100% !important;
                    }
                }
            }

            &__secret {
                margin: 5px 0 0;
                font-family: 'font_medium', sans-serif;
                font-size: 14px;
                line-height: 25px;
                text-align: center;
                color: #000;
                overflow-wrap: anywhere;
            }
        }

        &__confirm,
        &__codes {
            padding: 25px;
            background: #f5f6fa;
            border-radius: 6px;
            width: calc(100% - 50px);
            display: flex;
            flex-direction: column;
            align-items: center;

            &__title {
                font-size: 16px;
                line-height: 19px;
                text-align: center;
                color: #000;
                margin-bottom: 20px;
            }
        }

        &__buttons {
            display: flex;
            align-items: center;
            width: 100%;
            margin-top: 30px;
            column-gap: 20px;

            @media screen and (width <= 550px) {
                flex-direction: column-reverse;
                column-gap: unset;
                row-gap: 10px;
                margin-top: 20px;
            }
        }
    }

    .max-width {
        max-width: 485px;
    }
</style>
