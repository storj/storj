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
            <div class="payments-area__container__new-payments">
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
                    <StripeCardInput
                        ref="stripeCardInput"
                        class="add-card-area__stripe stripe_input"
                        :on-stripe-response-callback="addCard"
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
                <CreditCardImage class="card-icon-default" />
                <div class="edit_payment_method__container__close-cross-container-default" @click="onCloseClickDefault">
                    <CloseCrossIcon class="close-icon" />
                </div>
                <div class="edit_payment_method__header">Select Default Card</div>
                <form v-for="card in creditCards" :key="card.id">
                    <div class="change-default-input-container">
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
                    </div>
                </form>
                <div class="default-card-button" @click="updatePaymentMethod">
                    Update Default Card
                </div>
            </div>
            <div v-if="isRemovePaymentMethodsModalOpen" class="edit_payment_method__container">
                <CreditCardImage class="card-icon" />
                <div class="edit_payment_method__container__close-cross-container" @click="onCloseClick">
                    <CloseCrossIcon class="close-icon" />
                </div>
                <div class="edit_payment_method__header">Remove Credit Card</div>
                <div v-if="!cardBeingEdited.isDefault" class="edit_payment_method__header-subtext">This is not your default payment card.</div>
                <div v-if="cardBeingEdited.isDefault" class="edit_payment_method__header-subtext-default">This is your default payment card.</div>
                <div class="edit_payment_method__header-change-default" @click="changeDefault">
                    <a class="edit-card-text">Edit default card -></a>
                </div>
                <div
                    class="remove-card-button"
                    @click="removePaymentMethod"
                    @mouseover="deleteHover = true"
                    @mouseleave="deleteHover = false"
                >
                    <Trash v-if="deleteHover === false" />
                    <Trash v-if="deleteHover === true" class="red-trash" />
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
                    :limit="pageSize"
                    :total-page-count="pageCount"
                    :total-items-count="transactionCount"
                    :on-page-click-callback="paginationController"
                >
                    <template #head>
                        <SortingHeader @sortFunction="sortFunction" />
                    </template>
                    <template #body>
                        <token-transaction-item
                            v-for="item in displayedHistory"
                            :key="item.id"
                            :item="item"
                        />
                    </template>
                </v-table>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import QRCode from 'qrcode';

import {
    CreditCard,
    Wallet,
    NativePaymentHistoryItem,
} from '@/types/payments';
import { USER_ACTIONS } from '@/store/modules/users';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { RouteConfig } from '@/router';
import { MetaUtils } from '@/utils/meta';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';

import VButton from '@/components/common/VButton.vue';
import VLoader from '@/components/common/VLoader.vue';
import CreditCardContainer from '@/components/account/billing/billingTabs/CreditCardContainer.vue';
import StripeCardInput from '@/components/account/billing/paymentMethods/StripeCardInput.vue';
import SortingHeader from '@/components/account/billing/billingTabs/SortingHeader.vue';
import AddTokenCard from '@/components/account/billing/paymentMethods/AddTokenCard.vue';
import AddTokenCardNative from '@/components/account/billing/paymentMethods/AddTokenCardNative.vue';
import TokenTransactionItem from '@/components/account/billing/paymentMethods/TokenTransactionItem.vue';
import VTable from '@/components/common/VTable.vue';

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
const {
    ADD_CREDIT_CARD,
    GET_CREDIT_CARDS,
    REMOVE_CARD,
    MAKE_CARD_DEFAULT,
    GET_NATIVE_PAYMENTS_HISTORY,
} = PAYMENTS_ACTIONS;

// @vue/component
@Component({
    components: {
        VTable,
        AmericanExpressIcon,
        DiscoverIcon,
        JCBIcon,
        MastercardIcon,
        UnionPayIcon,
        VisaIcon,
        VButton,
        TokenTransactionItem,
        SortingHeader,
        CloseCrossIcon,
        CreditCardImage,
        StripeCardInput,
        DinersIcon,
        Trash,
        CreditCardContainer,
        AddTokenCard,
        AddTokenCardNative,
        VLoader,
    },
})
export default class PaymentMethods extends Vue {
    public nativeTokenPaymentsEnabled = MetaUtils.getMetaContent('native-token-payments-enabled') === 'true';

    /**
     * controls token inputs and transaction table
     */
    public showTransactions = false;
    public displayedHistory: NativePaymentHistoryItem[] = [];
    public transactionCount = 0;
    public nativePayIsLoading = false;
    public pageSize = 10;

    /**
     * controls card inputs
     */
    public deleteHover = false;
    public isLoading = false;
    public cardBeingEdited: CardEdited = {};
    public isAddingPayment = false;
    public isChangeDefaultPaymentModalOpen = false;
    public defaultCreditCardSelection = '';
    public isRemovePaymentMethodsModalOpen = false;
    public $refs!: {
        stripeCardInput: StripeCardInput & StripeForm;
        canvas: HTMLCanvasElement;
    };

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    public mounted(): void {
        if (this.$route.params.action === 'token history') {
            this.showTransactionsTable();
        }
    }

    private get wallet(): Wallet {
        return this.$store.state.paymentsModule.wallet;
    }

    public onCopyAddressClick(): void {
        this.$copyText(this.wallet.address);
        this.$notify.success('Address copied to your clipboard');
    }

    public async fetchHistory(): Promise<void> {
        this.nativePayIsLoading = true;
        try {
            await this.$store.dispatch(GET_NATIVE_PAYMENTS_HISTORY);
            this.transactionCount = this.nativePaymentHistoryItems.length;
            this.displayedHistory = this.nativePaymentHistoryItems.slice(0, this.pageSize);
        } catch (error) {
            await this.$notify.error(error.message, AnalyticsErrorEventSource.BILLING_PAYMENT_METHODS_TAB);
        } finally {
            this.nativePayIsLoading = false;
        }
    }

    public async hideTransactionsTable(): Promise<void> {
        this.showTransactions = false;
    }

    public async showTransactionsTable(): Promise<void> {
        await this.fetchHistory();
        this.showTransactions = true;
        await Vue.nextTick();
        await this.prepQRCode();
    }

    public async prepQRCode() {
        try {
            await QRCode.toCanvas(this.$refs.canvas, this.wallet.address);
        } catch (error) {
            await this.$notify.error(error.message, AnalyticsErrorEventSource.BILLING_PAYMENT_METHODS_TAB);
        }
    }

    public async updatePaymentMethod(): Promise<void> {
        try {
            await this.$store.dispatch(MAKE_CARD_DEFAULT, this.defaultCreditCardSelection);
            await this.$notify.success('Default payment card updated');
            this.isChangeDefaultPaymentModalOpen = false;
        } catch (error) {
            await this.$notify.error(error.message, AnalyticsErrorEventSource.BILLING_PAYMENT_METHODS_TAB);
        }
    }

    public async removePaymentMethod(): Promise<void> {
        if (!this.cardBeingEdited.isDefault) {
            try {
                await this.$store.dispatch(REMOVE_CARD, this.cardBeingEdited.id);
                this.analytics.eventTriggered(AnalyticsEvent.CREDIT_CARD_REMOVED);
                await this.$notify.success('Credit card removed');
            } catch (error) {
                await this.$notify.error(error.message, AnalyticsErrorEventSource.BILLING_PAYMENT_METHODS_TAB);
            }
            this.isRemovePaymentMethodsModalOpen = false;

        } else {
            this.$notify.error('You cannot delete the default payment method.', AnalyticsErrorEventSource.BILLING_PAYMENT_METHODS_TAB);
        }
    }

    public changeDefault() {
        this.isChangeDefaultPaymentModalOpen = true;
        this.isRemovePaymentMethodsModalOpen = false;
    }

    public closeAddPayment() {
        this.isAddingPayment = false;
    }

    public get creditCards(): CreditCard[] {
        return this.$store.state.paymentsModule.creditCards;
    }

    public async addCard(token: string): Promise<void> {
        this.$emit('toggleIsLoading');
        try {
            await this.$store.dispatch(ADD_CREDIT_CARD, token);

            // We fetch User one more time to update their Paid Tier status.
            await this.$store.dispatch(USER_ACTIONS.GET);
        } catch (error) {
            await this.$notify.error(error.message, AnalyticsErrorEventSource.BILLING_PAYMENT_METHODS_TAB);

            this.$emit('toggleIsLoading');

            return;
        }

        await this.$notify.success('Card successfully added');
        try {
            await this.$store.dispatch(GET_CREDIT_CARDS);
        } catch (error) {
            await this.$notify.error(error.message, AnalyticsErrorEventSource.BILLING_PAYMENT_METHODS_TAB);
            this.$emit('toggleIsLoading');
        }

        this.$emit('toggleIsLoading');
        this.$emit('toggleIsLoaded');

        setTimeout(() => {
            this.$emit('cancel');
            this.$emit('toggleIsLoaded');

            setTimeout(() => {
                if (!this.userHasOwnProject) {
                    this.$router.push(RouteConfig.CreateProject.path);
                }
            }, 500);
        }, 2000);
    }

    public async onConfirmAddStripe(): Promise<void> {
        if (this.isLoading) return;
        this.isLoading = true;
        await this.$refs.stripeCardInput.onSubmit().then(() => {this.isLoading = false;});
        this.analytics.eventTriggered(AnalyticsEvent.CREDIT_CARD_ADDED_FROM_BILLING);
    }

    public addPaymentMethodHandler() {
        this.analytics.eventTriggered(AnalyticsEvent.ADD_NEW_PAYMENT_METHOD_CLICKED);
        this.isAddingPayment = true;
    }

    public removePaymentMethodHandler(creditCard: CreditCard) {
        this.cardBeingEdited = {
            id: creditCard.id,
            isDefault: creditCard.isDefault,
        };
        this.isRemovePaymentMethodsModalOpen = true;
    }

    public onCloseClick() {
        this.isRemovePaymentMethodsModalOpen = false;
    }

    public onCloseClickDefault() {
        this.isChangeDefaultPaymentModalOpen = false;
    }

    /**
     * Indicates if user has own project.
     */
    private get userHasOwnProject(): boolean {
        return this.$store.getters.projectsCount > 0;
    }

    /**
     * controls sorting the transaction table
     */
    public sortFunction(key) {
        switch (key) {
        case 'date-ascending':
            this.nativePaymentHistoryItems.sort((a, b) => {return a.timestamp.getTime() - b.timestamp.getTime();});
            break;
        case 'date-descending':
            this.nativePaymentHistoryItems.sort((a, b) => {return b.timestamp.getTime() - a.timestamp.getTime();});
            break;
        case 'amount-ascending':
            this.nativePaymentHistoryItems.sort((a, b) => {return a.amount.value - b.amount.value;});
            break;
        case 'amount-descending':
            this.nativePaymentHistoryItems.sort((a, b) => {return b.amount.value - a.amount.value;});
            break;
        case 'status-ascending':
            this.nativePaymentHistoryItems.sort((a, b) => {
                if (a.status < b.status) {return -1;}
                if (a.status > b.status) {return 1;}
                return 0;});
            break;
        case 'status-descending':
            this.nativePaymentHistoryItems.sort((a, b) => {
                if (b.status < a.status) {return -1;}
                if (b.status > a.status) {return 1;}
                return 0;});
            break;
        }
        this.displayedHistory = this.nativePaymentHistoryItems.slice(0, 10);
    }

    /**
     * controls transaction table pagination
     */
    public paginationController(i): void {
        this.displayedHistory = this.nativePaymentHistoryItems.slice((i - 1) * this.pageSize, ((i - 1) * this.pageSize) + this.pageSize);
    }

    public get pageCount(): number {
        return Math.ceil(this.transactionCount / this.pageSize);
    }

    /**
     * Returns deposit history items.
     */
    public get nativePaymentHistoryItems(): NativePaymentHistoryItem[] {
        return this.$store.state.paymentsModule.nativePaymentsHistory;
    }
}
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
    margin: auto;
    display: $flex;
    flex-direction: row;
    align-items: flex-start;
    padding: 16px;
    gap: 10px;
    width: 300px;
    height: 10px;
    border: 1px solid var(--c-grey-4);
    border-radius: 8px;
    margin-top: 7px;
}

.change-default-input {
    margin-left: auto;
    background: #fff;
    border: 1px solid var(--c-grey-4);
    border-radius: 24px;
}

.default-card-button {
    margin-top: 20px;
    margin-bottom: 20px;
    cursor: pointer;
    margin-left: 112px;
    display: $flex;
    grid-column: 1;
    grid-row: 5;
    width: 132px;
    height: 24px;
    padding: 16px;
    gap: 8px;
    background: var(--c-blue-3);
    box-shadow: 0 0 1px rgb(9 28 69 / 80%);
    border-radius: 8px;
    font-family: 'font_bold', sans-serif;
    font-size: 14px;
    line-height: 24px;
    align-items: $align;
    letter-spacing: -0.02em;
    color: white;

    &:hover {
        background-color: #0059d0;
    }
}

.remove-card-button {
    cursor: pointer;
    margin-left: 130px;
    margin-top: 15px;
    margin-bottom: 21px;
    grid-column: 1;
    grid-row: 5;
    width: 111px;
    height: 24px;
    padding: 16px;
    gap: 8px;
    background: #fff;
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

.card-icon {
    margin-top: 20px;
    margin-left: 168px;
    grid-column: 1;
    grid-row: 1;
}

.card-icon-default {
    margin-top: 35px;
    margin-bottom: 10px;
    margin-left: 168px;
}

.change-default-modal-container {
    width: 400px;
    background: #f5f6fa;
    border-radius: 6px;
}

.edit_payment_method {
    position: fixed;
    top: 0;
    bottom: 0;
    left: 0;
    right: 0;
    z-index: 100;
    background: rgb(27 37 51 / 75%);
    display: $flex;
    align-items: $align;
    justify-content: $align;

    &__header-change-default {
        margin-top: -6px;
        margin-left: 141px;
        grid-column: 1;
        grid-row: 4;
    }

    &__header {
        grid-column: 1;
        grid-row: 2;
        font-family: 'font_bold', sans-serif;
        font-size: 24px;
        line-height: 31px;
        text-align: $align;
        letter-spacing: -0.02em;
        color: #1b2533;
        align-self: center;
    }

    &__header-subtext {
        grid-column: 1;
        grid-row: 3;
        font-family: 'font_regular', sans-serif;
        font-size: 14px;
        line-height: 20px;
        text-align: $align;
        color: var(--c-grey-6);
    }

    &__header-subtext-default {
        margin-left: 94px;
        font-family: 'font_regular', sans-serif;
        font-size: 14px;
        line-height: 20px;
        color: var(--c-grey-6);
    }

    &__container {
        display: grid;
        grid-template-columns: auto;
        grid-template-rows: 1fr 60px 30px auto auto;
        width: 400px;
        background: #f5f6fa;
        border-radius: 6px;

        &__close-cross-container {
            margin-top: 22px;
            margin-left: 357px;
            grid-column: 1;
            grid-row: 1;
            height: 24px;
            width: 24px;
            cursor: pointer;

            &:hover .close-cross-svg-path {
                fill: #2683ff;
            }
        }

        &__close-cross-container-default {
            position: absolute;
            margin-top: -58px;
            margin-left: 357px;
            grid-column: 1;
            grid-row: 1;
            height: 24px;
            width: 24px;
            cursor: pointer;

            &:hover .close-cross-svg-path {
                fill: #2683ff;
            }
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
            height: 203px;
            padding: 24px;
            box-sizing: border-box;
            border: 2px dashed var(--c-grey-5);
            border-radius: 10px;
            display: flex;
            align-items: center;
            justify-content: center;

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

                    @media screen and (max-width: 375px) {
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

            &__address {
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
