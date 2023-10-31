// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payments-area">
        <div class="payments-area__top-container">
            <h1 class="payments-area__title">
                <span class="payments-area__title__back" @click="hideTransactionsTable">Payment Methods</span>{{ showTransactions? ' > Storj Tokens':null }}
            </h1>
        </div>
        <div v-if="!showTransactions" class="payments-area__container">
            <v-loader
                v-if="nativePayIsLoading"
            />
            <add-token-card-native
                v-else-if="nativeTokenPaymentsEnabled"
                :parent-init-loading="parentInitLoading"
                @showTransactions="showTransactionsTable"
            />
            <add-token-card
                v-else
                :total-count="transactionCount"
            />
            <div v-for="card in creditCards" :key="card.id" class="payments-area__container__cards">
                <CreditCardContainer
                    :credit-card="card"
                    @remove="removePaymentMethodHandler"
                />
            </div>
            <div class="payments-area__container__new-payments" :class="{ 'white-background': isAddingPayment }">
                <div v-if="!isAddingPayment" class="payments-area__container__new-payments__text-area">
                    <span>+&nbsp;</span>
                    <span
                        class="payments-area__container__new-payments__text-area__text"
                        @click="addPaymentMethodHandler"
                    >Add New Payment Method</span>
                </div>
                <div v-else class="payments-area__container__new-payments__add-card">
                    <div class="close-add-payment" @click="closeAddPayment">
                        <CloseCrossIcon />
                    </div>
                    <div class="payments-area__create-header">Credit Card</div>
                    <div class="payments-area__create-subheader">Add Card Info</div>
                    <StripeCardElement
                        ref="stripeCardInput"
                        class="add-card-area__stripe stripe_input"
                        @pm-created="addCard"
                    />
                    <VButton
                        class="add-card-button"
                        label="Add Credit Card"
                        width="115px"
                        height="30px"
                        font-size="13px"
                        :on-press="onConfirmAddStripe"
                        :is-disabled="isLoading"
                    />
                </div>
            </div>
        </div>

        <div v-if="isRemovePaymentMethodsModalOpen || isChangeDefaultPaymentModalOpen" class="edit_payment_method">
            <div v-if="isChangeDefaultPaymentModalOpen" class="change-default-modal-container">
                <CreditCardImage />
                <div class="edit_payment_method__container__close-cross-container-default" @click="onCloseClickDefault">
                    <CloseCrossIcon class="close-icon" />
                </div>
                <div class="edit_payment_method__header">Select Default Card</div>
                <label v-for="card in creditCards" :key="card.id" :for="card.id" class="change-default-input-container">
                    <AmericanExpressIcon v-if="card.brand === 'amex' " class="cardIcons" />
                    <DiscoverIcon v-if="card.brand === 'discover' " class="cardIcons" />
                    <JCBIcon v-if="card.brand === 'jcb' " class="cardIcons jcb-icon" />
                    <MastercardIcon v-if="card.brand === 'mastercard' " class="cardIcons mastercard-icon" />
                    <UnionPayIcon v-if="card.brand === 'unionpay' " class="cardIcons union-icon" />
                    <VisaIcon v-if="card.brand === 'visa' " class="cardIcons" />
                    <DinersIcon v-if="card.brand === 'diners' " class="cardIcons diners-icon" />
                    <img src="@/../static/images/payments/cardStars.png" alt="Hidden card digits stars image" class="payment-methods-container__card-container__info-area__info-container__image">
                    {{ card.last4 }}
                    <input
                        :id="card.id"
                        v-model="defaultCreditCardSelection"
                        :value="card.id"
                        class="change-default-input"
                        type="radio"
                        name="defaultCreditCardSelection"
                    >
                </label>
                <div class="default-card-button" @click="updatePaymentMethod">
                    Update Default Card
                </div>
            </div>
            <div v-if="isRemovePaymentMethodsModalOpen" class="edit_payment_method__container">
                <CreditCardImage />
                <div class="edit_payment_method__container__close-cross-container" @click="onCloseClick">
                    <CloseCrossIcon class="close-icon" />
                </div>
                <div class="edit_payment_method__header">Remove Credit Card</div>
                <div v-if="cardBeingEdited.isDefault" class="edit_payment_method__header-subtext-default">This is your default payment card.<br>It can't be deleted.</div>
                <div v-else class="edit_payment_method__header-subtext">This is not your default payment card.</div>
                <div class="edit_payment_method__header-change-default" @click="changeDefault">
                    <a class="edit-card-text">Edit default card -></a>
                </div>
                <div
                    v-if="!cardBeingEdited.isDefault"
                    class="remove-card-button"
                    @click="removePaymentMethod"
                    @mouseover="deleteHover = true"
                    @mouseleave="deleteHover = false"
                >
                    <Trash v-if="!deleteHover" />
                    <Trash v-if="deleteHover" class="red-trash" />
                    Remove
                </div>
            </div>
        </div>
        <div v-if="showTransactions">
            <div class="payments-area__address-card">
                <div class="payments-area__address-card__left">
                    <canvas ref="canvas" class="payments-area__address-card__left__canvas" />
                    <div class="payments-area__address-card__left__balance">
                        <p class="payments-area__address-card__left__balance__label">
                            Available Balance (USD)
                        </p>
                        <p class="payments-area__address-card__left__balance__value">
                            {{ wallet.balance.value }}
                        </p>
                    </div>
                </div>

                <div class="payments-area__address-card__right">
                    <div class="payments-area__address-card__right__address">
                        <p class="payments-area__address-card__right__address__label">
                            Deposit Address
                        </p>
                        <p class="payments-area__address-card__right__address__value">
                            {{ wallet.address }}
                        </p>
                    </div>
                    <div class="payments-area__address-card__right__copy">
                        <VButton
                            class="modal__address__copy-button"
                            label="Copy Address"
                            width="8.6rem"
                            height="2.5rem"
                            font-size="0.9rem"
                            icon="copy"
                            :on-press="onCopyAddressClick"
                        />
                    </div>
                </div>
            </div>

            <div class="payments-area__transactions-area">
                <h2 class="payments-area__transactions-area__title">Transactions</h2>
                <v-table
                    class="payments-area__transactions-area__table"
                    items-label="transactions"
                    :limit="DEFAULT_PAGE_LIMIT"
                    :total-page-count="pageCount"
                    :total-items-count="transactionCount"
                    :on-page-change="paginationController"
                >
                    <template #head>
                        <SortingHeader @sortFunction="sortFunction" />
                    </template>
                    <template #body>
                        <token-transaction-item
                            v-for="(item, index) in displayedHistory"
                            :key="index"
                            :item="item"
                        />
                    </template>
                </v-table>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue';
import QRCode from 'qrcode';
import { useRoute, useRouter } from 'vue-router';

import {
    CreditCard,
    Wallet,
    NativePaymentHistoryItem,
} from '@/types/payments';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useUsersStore } from '@/store/modules/usersStore';
import { useBillingStore } from '@/store/modules/billingStore';
import { useAppStore } from '@/store/modules/appStore';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { useConfigStore } from '@/store/modules/configStore';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useCreateProjectClickHandler } from '@/composables/useCreateProjectClickHandler';
import { useLoading } from '@/composables/useLoading';

import VButton from '@/components/common/VButton.vue';
import VLoader from '@/components/common/VLoader.vue';
import CreditCardContainer from '@/components/account/billing/billingTabs/CreditCardContainer.vue';
import SortingHeader from '@/components/account/billing/billingTabs/SortingHeader.vue';
import AddTokenCard from '@/components/account/billing/paymentMethods/AddTokenCard.vue';
import AddTokenCardNative from '@/components/account/billing/paymentMethods/AddTokenCardNative.vue';
import TokenTransactionItem from '@/components/account/billing/paymentMethods/TokenTransactionItem.vue';
import VTable from '@/components/common/VTable.vue';
import StripeCardElement from '@/components/account/billing/paymentMethods/StripeCardElement.vue';

import CloseCrossIcon from '@/../static/images/common/closeCross.svg';
import AmericanExpressIcon from '@/../static/images/payments/cardIcons/smallamericanexpress.svg';
import DinersIcon from '@/../static/images/payments/cardIcons/smalldinersclub.svg';
import DiscoverIcon from '@/../static/images/payments/cardIcons/discover.svg';
import JCBIcon from '@/../static/images/payments/cardIcons/smalljcb.svg';
import MastercardIcon from '@/../static/images/payments/cardIcons/smallmastercard.svg';
import UnionPayIcon from '@/../static/images/payments/cardIcons/smallunionpay.svg';
import VisaIcon from '@/../static/images/payments/cardIcons/smallvisa.svg';
import Trash from '@/../static/images/account/billing/trash.svg';
import CreditCardImage from '@/../static/images/billing/credit-card.svg';

interface StripeForm {
    onSubmit(): Promise<void>;
}

interface CardEdited {
    id?: string,
    isDefault?: boolean
}

const analyticsStore = useAnalyticsStore();
const configStore = useConfigStore();
const billingStore = useBillingStore();
const usersStore = useUsersStore();
const appStore = useAppStore();
const projectsStore = useProjectsStore();

const { handleCreateProjectClick } = useCreateProjectClickHandler();
const { isLoading: parentInitLoading, withLoading } = useLoading();
const notify = useNotify();
const router = useRouter();
const route = useRoute();

const showTransactions = ref<boolean>(false);
const nativePayIsLoading = ref<boolean>(false);
const deleteHover = ref<boolean>(false);
const isLoading = ref<boolean>(false);
const isAddingPayment = ref<boolean>(false);
const isChangeDefaultPaymentModalOpen = ref<boolean>(false);
const isRemovePaymentMethodsModalOpen = ref<boolean>(false);
const displayedHistory = ref<NativePaymentHistoryItem[]>([]);
const transactionCount = ref<number>(0);
const defaultCreditCardSelection = ref<string>('');
const cardBeingEdited = ref<CardEdited>({});
const stripeCardInput = ref<typeof StripeCardInput & StripeForm>();
const canvas = ref<HTMLCanvasElement>();

const pageCount = computed((): number => {
    return Math.ceil(transactionCount.value / DEFAULT_PAGE_LIMIT);
});

/**
 * Indicates whether native token payments are enabled.
 */
const nativeTokenPaymentsEnabled = computed((): boolean => {
    return configStore.state.config.nativeTokenPaymentsEnabled;
});

/**
 * Returns deposit history items.
 */
const nativePaymentHistoryItems = computed((): NativePaymentHistoryItem[] => {
    return billingStore.state.nativePaymentsHistory as NativePaymentHistoryItem[];
});

/**
 * Returns wallet entity from store.
 */
const wallet = computed((): Wallet => {
    return billingStore.state.wallet as Wallet;
});

/**
 * Indicates if user has own project.
 */
const userHasOwnProject = computed((): boolean => {
    return projectsStore.projectsCount(usersStore.state.user.id) > 0;
});

const creditCards = computed((): CreditCard[] => {
    return billingStore.state.creditCards;
});

function onCopyAddressClick(): void {
    navigator.clipboard.writeText(wallet.value.address);
    notify.success('Address copied to your clipboard');
}

async function fetchHistory(): Promise<void> {
    nativePayIsLoading.value = true;

    try {
        await billingStore.getNativePaymentsHistory();
        transactionCount.value = nativePaymentHistoryItems.value.length;
        displayedHistory.value = nativePaymentHistoryItems.value.slice(0, DEFAULT_PAGE_LIMIT);
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.BILLING_PAYMENT_METHODS_TAB);
    } finally {
        nativePayIsLoading.value = false;
    }
}

async function hideTransactionsTable(): Promise<void> {
    showTransactions.value = false;
}

async function showTransactionsTable(): Promise<void> {
    await fetchHistory();
    showTransactions.value = true;
    nextTick(async () => {
        await prepQRCode();
    });
}

async function prepQRCode(): Promise<void> {
    try {
        await QRCode.toCanvas(canvas.value, wallet.value.address);
    } catch (error) {
        notify.error(error.message, AnalyticsErrorEventSource.BILLING_PAYMENT_METHODS_TAB);
    }
}

async function updatePaymentMethod(): Promise<void> {
    if (!defaultCreditCardSelection.value) {
        notify.error('Default card is not selected', AnalyticsErrorEventSource.BILLING_PAYMENT_METHODS_TAB);
        return;
    }

    const card = creditCards.value.find(c => c.id === defaultCreditCardSelection.value);
    if (card && card.isDefault) {
        notify.error('Chosen card is already default', AnalyticsErrorEventSource.BILLING_PAYMENT_METHODS_TAB);
        return;
    }

    try {
        await billingStore.makeCardDefault(defaultCreditCardSelection.value);
        notify.success('Default payment card updated');
        isChangeDefaultPaymentModalOpen.value = false;
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.BILLING_PAYMENT_METHODS_TAB);
    }
}

async function removePaymentMethod(): Promise<void> {
    if (!cardBeingEdited.value.isDefault) {
        if (!cardBeingEdited.value.id) {
            return;
        }

        try {
            await billingStore.removeCreditCard(cardBeingEdited.value.id);
            analyticsStore.eventTriggered(AnalyticsEvent.CREDIT_CARD_REMOVED);
            notify.success('Credit card removed');
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.BILLING_PAYMENT_METHODS_TAB);
        }

        isRemovePaymentMethodsModalOpen.value = false;
    } else {
        notify.error('You cannot delete the default payment method.', AnalyticsErrorEventSource.BILLING_PAYMENT_METHODS_TAB);
    }
}

function changeDefault(): void {
    isChangeDefaultPaymentModalOpen.value = true;
    isRemovePaymentMethodsModalOpen.value = false;
}

function closeAddPayment(): void {
    isAddingPayment.value = false;
}

async function addCard(pmID: string): Promise<void> {
    try {
        await billingStore.addCardByPaymentMethodID(pmID);
        // We fetch User one more time to update their Paid Tier status.
        await usersStore.getUser();
    } catch (error) {
        isLoading.value = false;
        notify.notifyError(error, AnalyticsErrorEventSource.BILLING_PAYMENT_METHODS_TAB);
        return;
    }

    notify.success('Card successfully added');
    try {
        await billingStore.getCreditCards();
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.BILLING_PAYMENT_METHODS_TAB);
    }

    if (!userHasOwnProject.value) {
        handleCreateProjectClick();
    }
}

async function onConfirmAddStripe(): Promise<void> {
    if (isLoading.value || !stripeCardInput.value) return;

    isLoading.value = true;
    await stripeCardInput.value.onSubmit().then(() => {isLoading.value = false;});
    analyticsStore.eventTriggered(AnalyticsEvent.CREDIT_CARD_ADDED_FROM_BILLING);
}

function addPaymentMethodHandler(): void {
    analyticsStore.eventTriggered(AnalyticsEvent.ADD_NEW_PAYMENT_METHOD_CLICKED);

    if (!usersStore.state.user.paidTier) {
        appStore.updateActiveModal(MODALS.upgradeAccount);
        return;
    }

    isAddingPayment.value = true;
}

function removePaymentMethodHandler(creditCard: CreditCard): void {
    cardBeingEdited.value = {
        id: creditCard.id,
        isDefault: creditCard.isDefault,
    };
    isRemovePaymentMethodsModalOpen.value = true;
}

function onCloseClick(): void {
    isRemovePaymentMethodsModalOpen.value = false;
}

function onCloseClickDefault(): void {
    isChangeDefaultPaymentModalOpen.value = false;
}

/**
 * controls sorting the transaction table
 */
function sortFunction(key: string): void {
    switch (key) {
    case 'date-ascending':
        nativePaymentHistoryItems.value.sort((a, b) => {return a.timestamp.getTime() - b.timestamp.getTime();});
        break;
    case 'date-descending':
        nativePaymentHistoryItems.value.sort((a, b) => {return b.timestamp.getTime() - a.timestamp.getTime();});
        break;
    case 'amount-ascending':
        nativePaymentHistoryItems.value.sort((a, b) => {return a.amount.value - b.amount.value;});
        break;
    case 'amount-descending':
        nativePaymentHistoryItems.value.sort((a, b) => {return b.amount.value - a.amount.value;});
        break;
    case 'status-ascending':
        nativePaymentHistoryItems.value.sort((a, b) => {
            if (a.status < b.status) return -1;
            if (a.status > b.status) return 1;

            return 0;
        });
        break;
    case 'status-descending':
        nativePaymentHistoryItems.value.sort((a, b) => {
            if (b.status < a.status) return -1;
            if (b.status > a.status) return 1;

            return 0;
        });
    }

    displayedHistory.value = nativePaymentHistoryItems.value.slice(0, 10);
}

/**
 * controls transaction table pagination
 */
function paginationController(page: number, limit: number): void {
    displayedHistory.value = nativePaymentHistoryItems.value.slice((page - 1) * limit, ((page - 1) * limit) + limit);
}

onMounted(async (): Promise<void> => {
    defaultCreditCardSelection.value = creditCards.value.find(c => c.isDefault)?.id ?? '';

    if (route.query.action === 'token history') {
        showTransactionsTable();
    }

    await withLoading(async () => {
        try {
            await Promise.all([
                billingStore.getWallet(),
                billingStore.getCreditCards(),
            ]);
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.BILLING_PAYMENT_METHODS_TAB);
        }
    });
});
</script>

<style scoped lang="scss">
$flex: flex;
$align: center;

:deep(.loader) {
    width: 40px;
    height: 40px;
    padding: 81.5px 154px;
}

.divider {
    height: 1px;
    width: calc(100% + 30px);
    background-color: #e5e7eb;
    align-self: center;
}

.union-icon {
    margin-top: -6px;
}

.jcb-icon {
    margin-top: -10px;
}

.mastercard-icon {
    margin-top: -10px;
}

.diners-icon {
    margin-top: -10px;
}

.cardIcons {
    flex: none;
}

.edit-card-text {
    color: var(--c-blue-3);
    font-family: 'font_regular', sans-serif;
}

.red-trash {

    :deep(path) {
        fill: #ac1a00;
    }
}

.change-default-input-container {
    font-family: 'font_regular', sans-serif;
    display: $flex;
    align-items: center;
    padding: 16px;
    gap: 10px;
    width: 100%;
    box-sizing: border-box;
    border: 1px solid var(--c-grey-4);
    border-radius: 8px;
    margin: 7px auto auto;
    cursor: pointer;
}

.change-default-input {
    margin-left: auto;
    background: #fff;
    border: 1px solid var(--c-grey-4);
    border-radius: 24px;
}

.default-card-button {
    margin-top: 20px;
    cursor: pointer;
    height: 24px;
    padding: 16px;
    background: var(--c-blue-3);
    box-shadow: 0 0 1px rgb(9 28 69 / 80%);
    border-radius: 8px;
    font-family: 'font_bold', sans-serif;
    font-size: 14px;
    line-height: 24px;
    letter-spacing: -0.02em;
    color: white;

    &:hover {
        background-color: #0059d0;
    }
}

.remove-card-button {
    cursor: pointer;
    padding: 16px;
    background: #fff;
    gap: 8px;
    border: 1px solid var(--c-grey-3);
    box-shadow: 0 0 3px rgb(0 0 0 / 8%);
    border-radius: 8px;
    font-family: 'font_bold', sans-serif;
    font-size: 14px;
    line-height: 24px;
    display: $flex;
    align-items: $align;
    letter-spacing: -0.02em;
    color: var(--c-grey-6);

    &:hover {
        border: 1px solid #e30011 !important;
        color: #e30011;
    }
}

.close-add-payment {
    position: absolute;
    right: 0;
    top: 0;
    cursor: pointer;
}

.change-default-modal-container {
    display: flex;
    flex-direction: column;
    align-items: center;
    width: 320px;
    padding: 20px;
    box-sizing: border-box;
    background: var(--c-white);
    border-radius: 6px;
    position: relative;
}

.edit_payment_method {
    position: fixed;
    inset: 0;
    z-index: 100;
    background: rgb(27 37 51 / 75%);
    display: $flex;
    align-items: $align;
    justify-content: $align;

    &__header-change-default {
        margin: 16px 0;
    }

    &__header {
        font-family: 'font_bold', sans-serif;
        font-size: 24px;
        line-height: 31px;
        text-align: $align;
        letter-spacing: -0.02em;
        color: #1b2533;
        align-self: center;
        margin-top: 16px;
    }

    &__header-subtext,
    &__header-subtext-default {
        font-family: 'font_regular', sans-serif;
        font-size: 14px;
        line-height: 20px;
        text-align: $align;
        color: var(--c-grey-6);
    }

    &__container {
        display: flex;
        flex-direction: column;
        align-items: center;
        width: 320px;
        padding: 20px;
        box-sizing: border-box;
        background: var(--c-white);
        border-radius: 6px;
        position: relative;

        &__close-cross-container,
        &__close-cross-container-default {
            position: absolute;
            top: 20px;
            right: 20px;
            cursor: pointer;
        }
    }
}

.add-card-button {
    margin-top: 2px;
}

.active-discount {
    background: var(--c-green-1);
    color: var(--c-green-5);
}

.inactive-discount {
    background: #ffe1df;
    color: #ac1a00;
}

.active-status {
    background: var(--c-green-5);
}

.inactive-status {
    background: #ac1a00;
}

.stripe_input {
    grid-row: 3;
    grid-column: 1;
    width: 100%;
}

.payments-area {

    &__top-container {
        display: flex;
        justify-content: space-between;
        align-items: center;
    }

    &__create-header {
        grid-row: 1;
        grid-column: 1;
        font-family: 'font_bold', sans-serif;
        font-size: 18px;
        line-height: 27px;
    }

    &__create-subheader {
        grid-row: 2;
        grid-column: 1;
        font-family: 'font_regular', sans-serif;
        font-size: 14px;
        line-height: 20px;
        color: var(--c-grey-6);
    }

    &__title {
        font-family: 'font_regular', sans-serif;
        font-size: 24px;
        margin: 20px 0;

        &__back {
            cursor: pointer;

            &:hover {
                color: #000000bd;
            }
        }
    }

    &__container {
        display: flex;
        flex-wrap: wrap;
        gap: 10px;

        &__cards {
            width: 348px;
            height: 203px;
            padding: 24px;
            box-sizing: border-box;
            background: #fff;
            box-shadow: 0 0 20px rgb(0 0 0 / 4%);
            border-radius: 10px;
        }

        &__new-payments {
            width: 348px;
            min-height: 203px;
            padding: 24px;
            box-sizing: border-box;
            border: 2px dashed var(--c-grey-5);
            border-radius: 10px;
            display: flex;
            align-items: center;
            justify-content: center;

            &.white-background {
                background: var(--c-white);
            }

            &__text-area {
                display: flex;
                align-items: center;
                font-size: 16px;
                font-family: 'font_regular', sans-serif;
                color: var(--c-blue-3);
                cursor: pointer;

                &__text {
                    text-decoration: underline;
                    text-underline-position: under;
                }
            }

            &__add-card {
                position: relative;
                width: 100%;
                height: 100%;
                display: flex;
                flex-direction: column;
                justify-content: space-between;
            }
        }

        &__transactions {
            margin: 20px 0;
            background-color: #fff;
            border-radius: 10px;
            box-shadow: 0 0 20px rgb(0 0 0 / 4%);
            padding: 0 15px;
            display: flex;
            flex-direction: column;
        }
    }

    .pagination {
        display: flex;
        justify-content: space-between;
        font-family: 'font_regular', sans-serif;
        padding: 15px 0;
        color: #6b7280;

        &__right-container {
            display: flex;
            width: 150px;
            justify-content: space-between;

            &__buttons {
                display: flex;
                justify-content: space-between;
                align-items: center;
                width: 25%;

                &__left {
                    transform: rotate(180deg);
                    cursor: pointer;
                    padding: 1px 0 0;
                }

                &__right {
                    cursor: pointer;
                }
            }
        }
    }

    &__address-card {
        display: flex;
        justify-content: space-between;
        flex-wrap: wrap;
        background: #fff;
        box-shadow: 0 0 20px rgb(0 0 0 / 4%);
        border-radius: 0.6rem;
        padding: 1rem 1.5rem;
        font-family: 'font_regular', sans-serif;

        &__left {
            display: flex;
            align-items: center;
            gap: 1.5rem;

            &__canvas {
                height: 4rem !important;
                width: 4rem !important;
            }

            &__balance {
                display: flex;
                flex-direction: column;
                justify-content: center;
                gap: 0.3rem;

                &__label {
                    font-size: 0.9rem;
                    color: rgb(0 0 0 / 75%);
                }

                &__value {
                    font-family: 'font_bold', sans-serif;
                    white-space: nowrap;
                    text-overflow: ellipsis;
                    overflow: hidden;

                    @media screen and (width <= 375px) {
                        width: 16rem;
                    }
                }
            }
        }

        &__right {
            width: 60%;
            display: flex;
            justify-content: space-between;
            align-items: center;
            flex-wrap: wrap;
            gap: 0.3rem;

            @media screen and (width <= 650px) {
                width: 100%;
            }

            &__address {
                display: flex;
                flex-direction: column;
                justify-content: center;
                width: 100%;
                gap: 0.3rem;

                &__label {
                    font-size: 0.9rem;
                    color: rgb(0 0 0 / 75%);
                }

                &__value {
                    font-family: 'font_bold', sans-serif;
                    overflow: hidden;
                    text-overflow: ellipsis;
                }
            }
        }
    }

    &__transactions-area {
        margin-top: 1.5rem;
        display: flex;
        flex-direction: column;
        align-items: flex-start;
        gap: 1.5rem;

        &__title {
            font-family: 'font_regular', sans-serif;
            font-size: 1.5rem;
            line-height: 1.8rem;
        }

        &__table {
            width: 100%;
        }
    }
}

@mixin reset-list {
    margin: 0;
    padding: 0;
    list-style: none;
}

@mixin horizontal-list {
    @include reset-list;

    li {
        display: inline-block;
        margin: {
            left: -2px;
            right: 2em;
        }
    }
}

nav ul {
    @include horizontal-list;
}
</style>
