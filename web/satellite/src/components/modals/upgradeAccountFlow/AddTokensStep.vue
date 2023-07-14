// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <UpgradeAccountWrapper :title="title">
        <template #content>
            <div class="add-tokens">
                <p class="add-tokens__info">
                    Send more than $10 in STORJ Tokens to the following deposit address to upgrade to a Pro account.
                    Your account will be upgraded after your transaction receives {{ neededConfirmations }} confirmations.
                    If your account is not automatically upgraded, please fill out this
                    <a
                        class="add-tokens__info__link"
                        href="https://supportdcs.storj.io/hc/en-us/requests/new?ticket_form_id=360000683212"
                        target="_blank"
                        rel="noopener noreferrer"
                    >limit increase request form</a>.
                </p>
                <canvas ref="canvas" />
                <div class="add-tokens__label">
                    <h2 class="add-tokens__label__text">Deposit Address</h2>
                    <VInfo class="add-tokens__label__info">
                        <template #icon>
                            <InfoIcon />
                        </template>
                        <template #message>
                            <p class="add-tokens__label__info__msg">
                                This is a Storj deposit address generated just for you.
                                <a
                                    class="add-tokens__label__info__msg__link"
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
                <div class="add-tokens__address">
                    <p class="add-tokens__address__value">{{ wallet.address }}</p>
                    <VButton
                        class="add-tokens__address__copy-button"
                        label="Copy"
                        width="84px"
                        height="32px"
                        font-size="12px"
                        border-radius="8px"
                        icon="copy"
                        :on-press="onCopyAddressClick"
                    />
                </div>
                <div class="add-tokens__divider" />
                <AddTokensStepBanner
                    :is-default="viewState === ViewState.Default"
                    :is-pending="viewState === ViewState.Pending"
                    :is-success="viewState === ViewState.Success"
                    :pending-payments="pendingPayments"
                />
            </div>
        </template>
    </UpgradeAccountWrapper>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import QRCode from 'qrcode';

import { useBillingStore } from '@/store/modules/billingStore';
import { useConfigStore } from '@/store/modules/configStore';
import { useNotify } from '@/utils/hooks';
import { PaymentStatus, PaymentWithConfirmations, Wallet } from '@/types/payments';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

import UpgradeAccountWrapper from '@/components/modals/upgradeAccountFlow/UpgradeAccountWrapper.vue';
import VButton from '@/components/common/VButton.vue';
import VInfo from '@/components/common/VInfo.vue';
import AddTokensStepBanner from '@/components/modals/upgradeAccountFlow/AddTokensStepBanner.vue';

import InfoIcon from '@/../static/images/payments/infoIcon.svg';

enum ViewState {
    Default,
    Pending,
    Success,
}

const configStore = useConfigStore();
const billingStore = useBillingStore();
const notify = useNotify();

const canvas = ref<HTMLCanvasElement>();
const intervalID = ref<NodeJS.Timer>();
const viewState = ref<ViewState>(ViewState.Default);

/**
 * Returns wallet from store.
 */
const wallet = computed((): Wallet => {
    return billingStore.state.wallet as Wallet;
});

/**
 * Returns needed transaction confirmations from config store.
 */
const neededConfirmations = computed((): number => {
    return configStore.state.config.neededTransactionConfirmations;
});

/**
 * Returns pending payments from store.
 */
const pendingPayments = computed((): PaymentWithConfirmations[] => {
    return billingStore.state.pendingPaymentsWithConfirmations;
});

/**
 * Returns title based on payment statuses.
 */
const title = computed((): string => {
    switch (viewState.value) {
    case ViewState.Pending:
        return 'Transaction pending...';
    case ViewState.Success:
        return 'Transaction Successful';
    default:
        return 'Add STORJ Tokens';
    }
});

/**
 * Copies address to user's clipboard.
 */
function onCopyAddressClick(): void {
    navigator.clipboard.writeText(wallet.value.address);
    notify.success('Address copied to your clipboard');
}

/**
 * Sets current view state depending on payment statuses.
 */
function setViewState(): void {
    switch (true) {
    case pendingPayments.value.some(p => p.status === PaymentStatus.Pending):
        viewState.value = ViewState.Pending;
        break;
    case pendingPayments.value.some(p => p.status === PaymentStatus.Confirmed):
        viewState.value = ViewState.Success;
        break;
    default:
        viewState.value = ViewState.Default;
    }
}

watch(() => pendingPayments.value, () => {
    setViewState();
}, { deep: true });

/**
 * Mounted lifecycle hook after initial render.
 * Renders QR code.
 */
onMounted(async (): Promise<void> => {
    setViewState();

    intervalID.value = setInterval(async () => {
        try {
            await billingStore.getPaymentsWithConfirmations();
        } catch { /* empty */ }
    }, 20000); // get payments every 20 seconds.

    if (!canvas.value) {
        return;
    }

    try {
        await QRCode.toCanvas(canvas.value, wallet.value.address, { width: 124 });
    } catch (error) {
        notify.error(error.message, AnalyticsErrorEventSource.UPGRADE_ACCOUNT_MODAL);
    }
});

onBeforeUnmount(() => {
    clearInterval(intervalID.value);

    if (viewState.value === ViewState.Success) {
        billingStore.clearPendingPayments();
    }
});
</script>

<style scoped lang="scss">
.add-tokens {
    max-width: 482px;
    font-family: 'font_regular', sans-serif;

    @media screen and (width <= 600px) {
        max-width: 350px;
    }

    @media screen and (width <= 470px) {
        max-width: 280px;
    }

    &__info {
        font-size: 14px;
        line-height: 20px;
        color: var(--c-blue-6);
        margin-bottom: 16px;
        text-align: left;

        &__link {
            color: var(--c-blue-3);
            text-decoration: underline;
            text-underline-position: under;

            &:visited {
                color: var(--c-blue-3);
            }
        }
    }

    &__label {
        display: flex;
        align-items: center;
        align-self: flex-start;
        margin-top: 16px;

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
                color: var(--c-white);

                &__link {
                    font-size: 12px;
                    line-height: 18px;
                    color: var(--c-white);
                    text-decoration: underline !important;

                    &:visited {
                        color: var(--c-white);
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
        margin: 8px 0 16px;
        width: 100%;

        &__value {
            font-size: 13px;
            line-height: 20px;
            color: var(--c-black);
            white-space: nowrap;
            text-overflow: ellipsis;
            overflow: hidden;
        }

        &__copy-button {
            margin-left: 10px;
            min-width: 84px;
        }
    }

    &__divider {
        width: 100%;
        height: 1px;
        margin-top: 16px;
        background-color: var(--c-grey-2);
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
