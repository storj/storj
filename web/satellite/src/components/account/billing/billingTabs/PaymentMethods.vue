// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payments-area">
        <div class="payments-area__top-container">
            <h1 class="payments-area__title">
                Payment Methods{{ showTransactions? ' > Storj Tokens':null }}
            </h1>
            <VButton
                v-if="showTransactions"
                label="Add Funds with CoinPayments"
                font-size="13px"
                height="40px"
                width="220px"
                :on-press="showAddFundsCard"
            />
        </div>
        <div v-if="!showTransactions" class="payments-area__container">
            <add-token-card-native v-if="nativeTokenPaymentsEnabled" />
            <template v-else>
                <v-loader
                    v-if="!tokensAreLoaded"
                />
                <div v-else-if="!showAddFunds">
                    <balance-token-card
                        v-for="item in mostRecentTransaction"
                        :key="item.id"
                        :v-if="tokensAreLoaded"
                        :billing-item="item"
                        :show-add-funds="showAddFunds"
                        @showTransactions="toggleTransactionsTable"
                        @toggleShowAddFunds="toggleShowAddFunds"
                    />
                </div>
                <div v-else>
                    <add-token-card
                        :total-count="transactionCount"
                        @toggleShowAddFunds="toggleShowAddFunds"
                        @fetchHistory="addTokenHelper"
                    />
                </div>
            </template>
            <div v-for="card in creditCards" :key="card.id" class="payments-area__container__cards">
                <CreditCardContainer
                    :credit-card="card"
                    @remove="removePaymentMethodHandler"
                />
            </div>
            <div class="payments-area__container__new-payments">
                <v-loader v-if="isLoading" class="payments-area__container__new-payments__payment-loading-image" />
                <div v-else-if="!isAddingPayment" class="payments-area__container__new-payments__text-area">
                    <span class="payments-area__container__new-payments__text-area__plus-icon">+&nbsp;</span>
                    <span
                        class="payments-area__container__new-payments__text-area__text"
                        @click="addPaymentMethodHandler"
                    >Add New Payment Method</span>
                </div>
                <div v-else-if="isAddingPayment">
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
                    <div
                        v-if="!isAddCardClicked"
                        class="add-card-button"
                        @click="onConfirmAddStripe"
                    >
                        <span class="add-card-button__text">Add Credit Card</span>
                    </div>
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
            <div class="payments-area__container__transactions">
                <SortingHeader2
                    @sortFunction="sortFunction"
                />
                <token-transaction-item
                    v-for="item in displayedHistory"
                    :key="item.id"
                    :billing-item="item"
                />
                <div class="divider" />
                <div class="pagination">
                    <div class="pagination__total">
                        <p>
                            {{ transactionCount }} transactions found
                        </p>
                    </div>
                    <div class="pagination__right-container">
                        <div
                            v-if="transactionCount > 0"
                            class="pagination__right-container__count"
                        >
                            <span v-if="transactionCount > 10 && paginationLocation.end !== transactionCount">
                                {{ paginationLocation.start + 1 }} - {{ paginationLocation.end }} of {{ transactionCount }}
                            </span>
                            <span v-else>
                                {{ paginationLocation.start + 1 }} - {{ transactionCount }} of {{ transactionCount }}
                            </span>
                        </div>
                        <div
                            v-if="transactionCount > 10"
                            class="pagination__right-container__buttons"
                        >
                            <ArrowIcon
                                class="pagination__right-container__buttons__left"
                                @click="paginationController(-10)"
                            />
                            <ArrowIcon
                                v-if="paginationLocation.end < transactionCount - 1"
                                class="pagination__right-container__buttons__right"
                                @click="paginationController(10)"
                            />
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { CreditCard , PaymentsHistoryItem, PaymentsHistoryItemType } from '@/types/payments';
import { USER_ACTIONS } from '@/store/modules/users';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { RouteConfig } from '@/router';
import { MetaUtils } from '@/utils/meta';

import VButton from '@/components/common/VButton.vue';
import VLoader from '@/components/common/VLoader.vue';
import CreditCardContainer from '@/components/account/billing/billingTabs/CreditCardContainer.vue';
import StripeCardInput from '@/components/account/billing/paymentMethods/StripeCardInput.vue';
import SortingHeader2 from '@/components/account/billing/depositAndBillingHistory/SortingHeader2.vue';
import BalanceTokenCard from '@/components/account/billing/paymentMethods/BalanceTokenCard.vue';
import AddTokenCard from '@/components/account/billing/paymentMethods/AddTokenCard.vue';
import AddTokenCardNative from '@/components/account/billing/paymentMethods/AddTokenCardNative.vue';
import TokenTransactionItem from '@/components/account/billing/paymentMethods/TokenTransactionItem.vue';

import ArrowIcon from '@/../static/images/common/arrowRight.svg';
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
    id?: number,
    isDefault?: boolean
}
const {
    ADD_CREDIT_CARD,
    GET_CREDIT_CARDS,
    REMOVE_CARD,
    MAKE_CARD_DEFAULT,
    GET_PAYMENTS_HISTORY,
} = PAYMENTS_ACTIONS;

const paginationStartNumber = 0;
const paginationEndNumber = 10;

// @vue/component
@Component({
    components: {
        AmericanExpressIcon,
        DiscoverIcon,
        JCBIcon,
        MastercardIcon,
        UnionPayIcon,
        VisaIcon,
        VButton,
        TokenTransactionItem,
        SortingHeader2,
        ArrowIcon,
        CloseCrossIcon,
        CreditCardImage,
        StripeCardInput,
        DinersIcon,
        Trash,
        CreditCardContainer,
        BalanceTokenCard,
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
    public showAddFunds = false;
    public mostRecentTransaction: Record<string, unknown>[] = [];
    public paginationLocation: {start: number, end: number} = { start: paginationStartNumber, end: paginationEndNumber };
    public tokenHistory: {amount: number, start: Date, status: string,}[] = [];
    public displayedHistory: Record<string, unknown>[] = [];
    public transactionCount = 0;
    public tokensAreLoaded = false;
    public reloadKey = 0;

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
    public isAddCardClicked = false;
    public $refs!: {
        stripeCardInput: StripeCardInput & StripeForm;
    };

    public beforeMount() {
        this.fetchHistory();
    }

    public addTokenHelper(): void {
        this.fetchHistory();
        this.toggleShowAddFunds();
    }

    public async fetchHistory(): Promise<void> {
        this.tokensAreLoaded = false;
        try {
            await this.$store.dispatch(GET_PAYMENTS_HISTORY);
            this.fetchHelper(this.depositHistoryItems);
            this.reloadKey = this.reloadKey + 1;
        } catch (error) {
            await this.$notify.error(error.message);
        }
    }

    public fetchHelper(tokenArray): void {
        this.mostRecentTransaction = [tokenArray[0]];
        this.tokenHistory = tokenArray;
        this.transactionCount = tokenArray.length;
        this.displayedHistory = tokenArray.slice(0,10);
        this.tokensAreLoaded = true;
        if (this.transactionCount > 0){
            this.showAddFunds = false;
        } else {
            this.showAddFunds = true;
        }
    }

    public toggleShowAddFunds(): void {
        this.showAddFunds = !this.showAddFunds;
    }

    public showAddFundsCard(): void {
        this.showTransactions = false;
        this.showAddFunds = true;
    }

    public toggleTransactionsTable(): void {
        this.showAddFunds = true;
        this.showTransactions = !this.showTransactions;
    }

    /**
     * Returns TokenTransactionItem item component.
     */
    public get itemComponent(): typeof TokenTransactionItem {
        return TokenTransactionItem;
    }

    public async updatePaymentMethod(): Promise<void> {
        try {
            await this.$store.dispatch(MAKE_CARD_DEFAULT, this.defaultCreditCardSelection);
            await this.$notify.success('Default payment card updated');
            this.isChangeDefaultPaymentModalOpen = false;
        } catch (error) {
            await this.$notify.error(error.message);
        }
    }

    public async removePaymentMethod(): Promise<void> {
        if (!this.cardBeingEdited.isDefault) {
            try {
                await this.$store.dispatch(REMOVE_CARD, this.cardBeingEdited.id);
                await this.$notify.success('Credit card removed');
            } catch (error) {
                await this.$notify.error(error.message);
            }
            this.isRemovePaymentMethodsModalOpen = false;

        }
        else {
            this.$notify.error('You cannot delete the default payment method.');
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
            await this.$notify.error(error.message);

            this.$emit('toggleIsLoading');

            return;
        }

        await this.$notify.success('Card successfully added');
        try {
            await this.$store.dispatch(GET_CREDIT_CARDS);
        } catch (error) {
            await this.$notify.error(error.message);
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
        this.isLoading = true;
        await this.$refs.stripeCardInput.onSubmit().then(() => {this.isLoading = false;});
    }

    public addPaymentMethodHandler() {
        this.isAddingPayment = true;
    }

    public removePaymentMethodHandler(creditCard) {
        this.cardBeingEdited = creditCard;
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
        this.paginationLocation = { start: 0, end: 10 };
        this.displayedHistory = this.tokenHistory.slice(0,10);
        switch (key) {
        case 'date-ascending':
            this.tokenHistory.sort((a,b) => {return a.start.getTime() - b.start.getTime();});
            break;
        case 'date-descending':
            this.tokenHistory.sort((a,b) => {return b.start.getTime() - a.start.getTime();});
            break;
        case 'amount-ascending':
            this.tokenHistory.sort((a,b) => {return a.amount - b.amount;});
            break;
        case 'amount-descending':
            this.tokenHistory.sort((a,b) => {return b.amount - a.amount;});
            break;
        case 'status-ascending':
            this.tokenHistory.sort((a, b) => {
                if (a.status < b.status) {return -1;}
                if (a.status > b.status) {return 1;}
                return 0;});
            break;
        case 'status-descending':
            this.tokenHistory.sort((a, b) => {
                if (b.status < a.status) {return -1;}
                if (b.status > a.status) {return 1;}
                return 0;});
            break;
        }
    }

    /**
     * controls transaction table pagination
     */
    public paginationController(i): void {
        let diff = this.transactionCount - this.paginationLocation.start;
        if (this.paginationLocation.start + i >= 0 && this.paginationLocation.end + i <= this.transactionCount && this.paginationLocation.end !== this.transactionCount){
            this.paginationLocation = {
                start: this.paginationLocation.start + i,
                end: this.paginationLocation.end + i,
            };
        } else if (this.paginationLocation.start + i < 0 ) {
            this.paginationLocation = {
                start: 0,
                end: 10,
            };
        } else if (this.paginationLocation.end + i > this.transactionCount) {
            this.paginationLocation = {
                start: this.paginationLocation.start + i,
                end: this.transactionCount,
            };
        }   else if (this.paginationLocation.end === this.transactionCount) {
            this.paginationLocation = {
                start: this.paginationLocation.start + i,
                end: this.transactionCount - (diff),
            };
        }

        this.displayedHistory = this.tokenHistory.slice(this.paginationLocation.start, this.paginationLocation.end);
    }

    /**
     * Returns deposit history items.
     */
    public get depositHistoryItems(): PaymentsHistoryItem[] {
        return this.$store.state.paymentsModule.paymentsHistory.filter((item: PaymentsHistoryItem) => {
            return item.type === PaymentsHistoryItemType.Transaction || item.type === PaymentsHistoryItemType.DepositBonus;
        });
    }
}
</script>

<style scoped lang="scss">
$flex: flex;
$align: center;

:deep(.loader) {
    width: auto;
    padding: 63px 114px;
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
    color: #0149ff;
    font-family: sans-serif;
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
    border: 1px solid #c8d3de;
    border-radius: 8px;
    margin-top: 7px;
}

.change-default-input {
    margin-left: auto;
    background: #fff;
    border: 1px solid #c8d3de;
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
    background: #0149ff;
    box-shadow: 0 0 1px rgb(9 28 69 / 80%);
    border-radius: 8px;
    font-family: sans-serif;
    font-style: normal;
    font-weight: 700;
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
    border: 1px solid #d8dee3;
    box-shadow: 0 0 3px rgb(0 0 0 / 8%);
    border-radius: 8px;
    font-family: sans-serif;
    font-style: normal;
    font-weight: 700;
    font-size: 14px;
    line-height: 24px;
    display: $flex;
    align-items: $align;
    letter-spacing: -0.02em;
    color: #56606d;

    &:hover {
        border: 1px solid #e30011 !important;
        color: #e30011;
    }
}

.close-add-payment {
    position: absolute;
    margin-left: 208px;
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
        font-family: sans-serif;
        font-style: normal;
        font-weight: 800;
        font-size: 24px;
        line-height: 31px;
        text-align: $align;
        letter-spacing: -0.02em;
        color: #1b2533;
    }

    &__header-subtext {
        grid-column: 1;
        grid-row: 3;
        font-family: sans-serif;
        font-style: normal;
        font-weight: 400;
        font-size: 14px;
        line-height: 20px;
        text-align: $align;
        color: #56606d;
    }

    &__header-subtext-default {
        margin-left: 94px;
        font-family: sans-serif;
        font-style: normal;
        font-weight: 400;
        font-size: 14px;
        line-height: 20px;
        color: #56606d;
    }

    &__container {
        display: grid;
        grid-template-columns: auto;
        grid-template-rows: 1fr 30px 30px auto auto;
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
    grid-row: 4;
    grid-column: 1;
    width: 115px;
    height: 30px;
    margin-top: 2px;
    cursor: pointer;
    border-radius: 6px;
    background-color: #0149ff;
    font-family: 'font_medium', sans-serif;
    font-size: 16px;
    line-height: 23px;
    color: #fff;
    user-select: none;
    transition: top 0.5s ease-in-out;

    &:hover {
        background-color: #0059d0;
    }

    &__text {
        margin-top: 4px;
        margin-left: 9px;
        font-family: 'font-medium', sans-serif;
        font-style: normal;
        font-weight: 700;
        font-size: 13px;
        line-height: 29px;
        display: $flex;
        align-items: $align;
        letter-spacing: -0.02em;
    }
}

.active-discount {
    background: #dffff7;
    color: #00ac26;
}

.inactive-discount {
    background: #ffe1df;
    color: #ac1a00;
}

.active-status {
    background: #00ac26;
}

.inactive-status {
    background: #ac1a00;
}

.stripe_input {
    grid-row: 3;
    grid-column: 1;
    width: 240px;
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
        font-family: sans-serif;
        font-style: normal;
        font-weight: 700;
        font-size: 18px;
        line-height: 27px;
    }

    &__create-subheader {
        grid-row: 2;
        grid-column: 1;
        font-family: sans-serif;
        font-style: normal;
        font-weight: 400;
        font-size: 14px;
        line-height: 20px;
        color: #56606d;
    }

    &__title {
        font-family: sans-serif;
        font-size: 24px;
        margin: 20px 0;
    }

    &__container {
        display: flex;
        flex-wrap: wrap;
        gap: 10px;
        justify-content: space-between;

        &__cards {
            width: 227px;
            height: 126px;
            padding: 20px;
            background: #fff;
            box-shadow: 0 0 20px rgb(0 0 0 / 4%);
            border-radius: 10px;
            margin: 0 10px 10px 0;
        }

        &__new-payments {
            width: 227px;
            height: 126px;
            padding: 18px;
            display: grid !important;
            grid-template-columns: 6fr;
            grid-template-rows: 1fr 1fr 1fr 1fr;
            border: 2px dashed #929fb1;
            border-radius: 10px;
            cursor: pointer;

            &__payment-loading-image {
                padding: 40px;
            }

            &__text-area {
                display: flex;
                align-items: center;
                justify-content: center;

                &__plus-icon {
                    color: #0149ff;
                    font-family: sans-serif;
                    font-size: 24px;
                }

                &__text {
                    color: #0149ff;
                    font-family: sans-serif;
                    font-size: 16px;
                    text-decoration: underline;
                }
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
        font-family: sans-serif;
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
