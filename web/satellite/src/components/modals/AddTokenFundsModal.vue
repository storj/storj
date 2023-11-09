// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal v-if="wallet.address" :on-close="closeModal">
        <template #content>
            <div class="modal">
                <h1 class="modal__title" aria-roledescription="modal-title">
                    Add STORJ Tokens
                </h1>
                <p class="modal__info">
                    Send STORJ Tokens to the following deposit address to credit your Storj account:
                </p>
                <div class="modal__qr">
                    <canvas ref="canvas" class="modal__qr__canvas" />
                </div>
                <div class="modal__label">
                    <h2 class="modal__label__text">Deposit Address</h2>
                    <VInfo class="modal__label__info">
                        <template #icon>
                            <InfoIcon />
                        </template>
                        <template #message>
                            <p class="modal__label__info__msg">
                                This is a Storj deposit address generated just for you.
                                <a
                                    class="modal__label__info__msg__link"
                                    href=""
                                    target="_blank"
                                    rel="noopener noreferrer"
                                >
                                    Learn more
                                </a>
                            </p>
                        </template>
                    </VInfo>
                </div>
                <div class="modal__address">
                    <p class="modal__address__value">{{ wallet.address }}</p>
                    <VButton
                        class="modal__address__copy-button"
                        label="Copy"
                        width="84px"
                        height="32px"
                        font-size="13px"
                        icon="copy"
                        :on-press="onCopyAddressClick"
                    />
                </div>
                <VButton
                    width="194px"
                    height="48px"
                    border-radius="8px"
                    label="Done"
                    font-size="14px"
                    :on-press="closeModal"
                />
                <div class="modal__footer">
                    <h2 class="modal__footer__title">Send only STORJ tokens via Layer 1 transaction to this deposit address.</h2>
                    <div class="modal__footer__msg">
                        <p>
                            Sending anything else than STORJ token will result in the loss of your deposit.
                        </p>
                        <p>
                            Please note that zkSync transactions are not yet supported.
                        </p>
                    </div>
                </div>
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import QRCode from 'qrcode';

import { Wallet } from '@/types/payments';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useBillingStore } from '@/store/modules/billingStore';
import { useAppStore } from '@/store/modules/appStore';

import VButton from '@/components/common/VButton.vue';
import VModal from '@/components/common/VModal.vue';
import VInfo from '@/components/common/VInfo.vue';

import InfoIcon from '@/../static/images/payments/infoIcon.svg';

const appStore = useAppStore();
const billingStore = useBillingStore();
const notify = useNotify();

const canvas = ref<HTMLCanvasElement>();

/**
 * Returns wallet from store.
 */
const wallet = computed((): Wallet => {
    return billingStore.state.wallet as Wallet;
});

/**
 * Closes create project prompt modal.
 */
function closeModal(): void {
    appStore.removeActiveModal();
}

/**
 * Copies address to user's clipboard.
 */
function onCopyAddressClick(): void {
    navigator.clipboard.writeText(wallet.value.address);
    notify.success('Address copied to your clipboard');
}

/**
 * Mounted lifecycle hook after initial render.
 * Fetches wallet if necessary and renders QR code.
 */
onMounted(async (): Promise<void> => {
    if (!canvas.value) {
        return;
    }

    try {
        await QRCode.toCanvas(canvas.value, wallet.value.address);
    } catch (error) {
        notify.error(error.message, AnalyticsErrorEventSource.ADD_TOKEN_FUNDS_MODAL);
    }
});
</script>

<style scoped lang="scss">
    .modal {
        width: 560px;
        padding: 48px 0 0;
        display: flex;
        align-items: center;
        flex-direction: column;
        font-family: 'font_regular', sans-serif;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 24px;
            line-height: 31px;
            letter-spacing: -0.02em;
            color: #1b2533;
        }

        &__info {
            font-family: 'font_regular', sans-serif;
            font-size: 16px;
            line-height: 24px;
            text-align: center;
            color: #000;
            margin: 15px 0;
            max-width: 410px;
        }

        &__loader {
            margin-bottom: 20px;
        }

        &__label {
            display: flex;
            align-items: center;
            align-self: flex-start;
            padding: 0 32px;

            &__text {
                font-size: 14px;
                line-height: 20px;
                color: var(--c-grey-6);
                margin-right: 9px;
                font-family: 'font_medium', sans-serif;
            }

            &__info {
                cursor: pointer;
                max-height: 16px;

                &__msg {
                    font-size: 12px;
                    line-height: 18px;
                    text-align: center;
                    color: #fff;

                    &__link {
                        font-size: 12px;
                        line-height: 18px;
                        color: #fff;
                        text-decoration: underline !important;

                        &:visited {
                            color: #fff;
                        }
                    }
                }
            }
        }

        &__address {
            box-sizing: border-box;
            display: flex;
            align-items: center;
            justify-content: space-between;
            border: 1px solid var(--c-grey-4);
            border-radius: 8px;
            padding: 10px 15px;
            width: calc(100% - 64px);
            max-width: 482px;
            margin: 8px 0 15px;

            &__value {
                font-size: 13px;
                line-height: 20px;
                color: #000;
                white-space: nowrap;
                text-overflow: ellipsis;
                overflow: hidden;
            }

            &__copy-button {
                margin-left: 10px;
            }
        }

        &__footer {
            margin-top: 26px;
            background-color: #fec;
            padding: 12px 28px;
            border-radius: 0 0 10px 10px;
            width: calc(100% - 56px);

            &__title {
                font-family: 'font_medium', sans-serif;
                font-size: 14px;
                line-height: 20px;
            }

            &__msg {
                font-size: 12px;
                line-height: 16px;
            }
        }
    }

    :deep(.info__box) {
        width: 214px;
        left: calc(50% - 107px);
        top: calc(100% - 80px);
        cursor: default;
        filter: none;
        transform: rotate(-180deg);
    }

    :deep(.info__box__message) {
        background: var(--c-grey-6);
        border-radius: 4px;
        padding: 10px 8px;
        transform: rotate(-180deg);
    }

    :deep(.info__box__arrow) {
        background: var(--c-grey-6);
        width: 10px;
        height: 10px;
        margin-bottom: -3px;
    }
</style>
