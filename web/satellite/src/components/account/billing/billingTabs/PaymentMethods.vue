// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payments-area">
        <div class="payments-area__top-container">
            <h1 class="payments-area__title">Payment Methods{{showTransactions? ' > Storj Tokens':null}}</h1>
            <div 
                class="payments-area__container"
                v-if="!showTransactions"
            >
                <div
                    class="payments-area__container__token"
                >
                    <div
                        class="payments-area__container__token__large-icon-container"
                    >
                        <div class="payments-area__container__token__large-icon">
                            <StorjLarge />
                        </div>
                    </div>
                    <div class="payments-area__container__token__base"
                    v-if="!showAddFunds"
                    >
                        <div class="payments-area__container__token__base__small-icon">   <StorjSmall />
                        </div>
                        <div class="payments-area__container__token__base__confirmation-container">
                            <p class="payments-area__container__token__base__confirmation-container__label">STORJ Token Deposit</p>
                            <span :class="`payments-area__container__token__base__confirmation-container__circle-icon ${depositStatus}`">
                                &#9679;
                            </span>
                            <span class="payments-area__container__token__base__confirmation-container__text">
                                <span>
                                    {{ depositStatus }}
                                </span>
                            </span>
                        </div>
                        <div class="payments-area__container__token__base__balance-container">
                            <p class="payments-area__container__token__base__balance-container__label">Total Balance</p>
                            <span class="payments-area__container__token__base__balance-container__text">USD ${{ balanceAmount }}</span>
                        </div>
                        <VButton
                            label='See transactions'
                            width="120px"
                            height="30px"
                            is-transparent="true"
                            font-size="13px"
                            class="payments-area__container__token__base__transaction-button"
                            :onPress="toggleTransactionsTable"
                        />
                        <VButton
                            label='Add funds'
                            font-size="13px"
                            width="80px"
                            height="30px"
                            class="payments-area__container__token__base__funds-button"
                            :onPress="toggleShowAddFunds"
                        />
                    </div>
                    <div
                        class="payments-area__container__token__add-funds"
                        v-if="showAddFunds"
                    >
                        <h3
                            class="payments-area__container__token__add-funds__title"
                        >STORJ Token</h3>
                        <p class="payments-area__container__token__add-funds__label">Deposit STORJ Tokens via Coin Payments:</p>
                        <TokenDepositSelection2
                            class="payments-area__container__token__add-funds__dropdown"
                            :payment-options="paymentOptions"
                            @onChangeTokenValue="onChangeTokenValue"
                        />
                        <VButton
                            class="payments-area__container__token__add-funds__button"
                            label="Continue to CoinPayments"
                            width="150px"
                            height="30px"
                            font-size="11px"
                            :on-press="onConfirmAddSTORJ"
                        />
                    </div>
                </div>
                <div v-for="card in creditCards" :key="card.id" class="payments-area__container__cards" >
                    <CreditCardContainer
                        :credit-card="card"
                        @remove="removePaymentMethodHandler"
                    />
                </div>
                <div class="payments-area__container__new-payments">
                    <div v-if="!isAddingPayment" class="payments-area__container__new-payments__text-area">
                        <span class="payments-area__container__new-payments__text-area__plus-icon">+&nbsp;</span>
                        <span 
                            class="payments-area__container__new-payments__text-area__text"
                            @click="addPaymentMethodHandler"
                        >Add New Payment Method</span>
                    </div>
                    <div v-if="isAddingPayment">
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
                            <img
                                v-if="isLoading"
                                class="payment-loading-image"
                                src="@/../static/images/account/billing/loading.gif"
                                alt="loading gif"
                            >
                            <SuccessImage
                                v-if="isLoaded"
                                class="payment-loaded-image"
                            />
                            <span class="add_card_button_text">Add Credit Card</span>
                        </div>
                    </div>
                </div>
            </div>
            
            <div v-if="isRemovePaymentMethodsModalOpen || isChangeDefaultPaymentModalOpen" class="edit_payment_method">
                <!-- Change Default Card Modal -->
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
                <!-- Remove Credit Card Modal -->
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
                        <RedTrash v-if="deleteHover === true" />
                        Remove
                    </div>
                </div>
            </div>
            <div v-if="showTransactions">
                <div class="payments-area__container__transactions">
                    <SortingHeader2
                        @sortFunction='sortFunction'
                    />
                    <token-transaction-item
                        v-for="item in displayedHistory"
                        :key="item.id"
                        :billing-item="item"
                    />
                    <div class="divider"></div>
                    <div class="pagination">
                        <div class="pagination__total">
                            <p>
                                {{transactionCount}} transactions found
                            </p>
                        </div>
                        <div class="pagination__right-container">
                            <div class="pagination__right-container__count">
                                <span
                                    v-if="transactionCount > 10 && paginationLocation.end !== transactionCount"
                                >
                                   {{paginationLocation.start + 1}} - {{paginationLocation.end}} of {{transactionCount}}
                                </span>
                                <span
                                    v-else
                                >
                                   {{paginationLocation.start + 1}} - {{transactionCount}} of {{transactionCount}}
                                </span>
                            </div>
                            <div class="pagination__right-container__buttons"
                                v-if="transactionCount > 10"
                            >
                                <ArrowIcon
                                    class="pagination__right-container__buttons__left"
                                    v-if="paginationLocation.start > 0"
                                    @click="paginationController(-10)"
                                />
                                <ArrowIcon
                                    class="pagination__right-container__buttons__right"    
                                    v-if="paginationLocation.end < transactionCount - 1"
                                    @click="paginationController(10)"
                                />

                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import VLoader from '@/components/common/VLoader.vue';
import VButton from '@/components/common/VButton.vue';
import VList from '@/components/common/VList.vue';
import SortingHeader2 from '@/components/account/billing/depositAndBillingHistory/SortingHeader2.vue';

import TokenDepositSelection2 from '@/components/account/billing/paymentMethods/TokenDepositSelection2.vue';
import TokenTransactionItem from '@/components/account/billing/paymentMethods/TokenTransactionItem.vue';

import StorjSmall from '@/../static/images/billing/storj-icon-small.svg';
import StorjLarge from '@/../static/images/billing/storj-icon-large.svg';
import ArrowIcon from '@/../static/images/common/arrowRight.svg'
import CloseCrossIcon from '@/../static/images/common/closeCross.svg';
import StripeCardInput from '@/components/account/billing/paymentMethods/StripeCardInput.vue';
import SuccessImage from '@/../static/images/account/billing/success.svg';

import AmericanExpressIcon from '@/../static/images/payments/cardIcons/smallamericanexpress.svg';
import DinersIcon from '@/../static/images/payments/cardIcons/smalldinersclub.svg';
import DiscoverIcon from '@/../static/images/payments/cardIcons/discover.svg';
import JCBIcon from '@/../static/images/payments/cardIcons/smalljcb.svg';
import MastercardIcon from '@/../static/images/payments/cardIcons/smallmastercard.svg';
import UnionPayIcon from '@/../static/images/payments/cardIcons/smallunionpay.svg';
import VisaIcon from '@/../static/images/payments/cardIcons/smallvisa.svg';

import Trash from '@/../static/images/account/billing/trash.svg';
import RedTrash from '@/../static/images/account/billing/redtrash.svg';

import CreditCardImage from '@/../static/images/billing/credit-card.svg';
import CreditCardContainer from '@/components/account/billing/billingTabs/CreditCardContainer.vue';

import { CreditCard } from '@/types/payments';
import { USER_ACTIONS } from '@/store/modules/users';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PaymentsHistoryItem, PaymentsHistoryItemType, PaymentAmountOption } from '@/types/payments';
import { SortDirection } from '@/types/common';
import { RouteConfig } from '@/router';

interface StripeForm {
    onSubmit(): Promise<void>;
}
const {
    ADD_CREDIT_CARD,
    GET_CREDIT_CARDS,
    REMOVE_CARD,
    MAKE_CARD_DEFAULT,
    MAKE_TOKEN_DEPOSIT,
    GET_PAYMENTS_HISTORY,
} = PAYMENTS_ACTIONS;

// @vue/component
@Component({
    components: {
        AmericanExpressIcon,
        DiscoverIcon,
        JCBIcon,
        MastercardIcon,
        UnionPayIcon,
        VisaIcon,
        VLoader,
        VButton,
        VList,
        StorjSmall,
        StorjLarge,
        TokenTransactionItem,
        TokenDepositSelection2,
        SortingHeader2,
        ArrowIcon,
        CloseCrossIcon,
        CreditCardImage,
        StripeCardInput,
        DinersIcon,
        SuccessImage,
        Trash,
        RedTrash,
        CreditCardContainer
    },
})
export default class paymentsArea extends Vue {

    //controls token inputs
    public depositStatus: string = 'Confirmed';
    public balanceAmount: number = 0.00;
    public showTransactions: boolean = false;
    public showAddFunds: boolean = false;
    private readonly DEFAULT_TOKEN_DEPOSIT_VALUE = 10; // in dollars.
    private readonly MAX_TOKEN_AMOUNT = 1000000; // in dollars.
    private tokenDepositValue: number = this.DEFAULT_TOKEN_DEPOSIT_VALUE;
    public paginationLocation: {start: number, end: number} = {start: 0, end: 10};
    public tokenHistory: {amount: number, start: Date, status: string,}[] = []
    public displayedHistory: {}[] = [];
    public transactionCount: number = 0;

    // controls card inputs
    public deleteHover = false;
    public cardBeingEdited: any = {};
    public isAddingPayment = false;
    public isChangeDefaultPaymentModalOpen = false;
    public defaultCreditCardSelection = "";
    public isRemovePaymentMethodsModalOpen = false;
    public testData = [{},{},{}];
    public isAddCardClicked = false;
    public $refs!: {
        stripeCardInput: StripeCardInput & StripeForm;
    };

    public async mounted() {
        try {
            await this.$store.dispatch(GET_PAYMENTS_HISTORY);
        } catch (error) {
            await this.$notify.error(error.message);
        }

        let array = (this.$store.state.paymentsModule.paymentsHistory.filter((item: PaymentsHistoryItem) => {
            return item.type === PaymentsHistoryItemType.Transaction || item.type === PaymentsHistoryItemType.DepositBonus;
        }));
        this.tokenHistory = array;
        this.transactionCount = array.length
        this.displayedHistory = array.slice(0,10)
        console.log(array.length)
    }

    public toggleTransactionsTable(): void {
        this.showTransactions = !this.showTransactions;
    }

    public toggleShowAddFunds(): void {
        this.showAddFunds = !this.showAddFunds ;
    }

    /**
     * Returns TokenTransactionItem item component.
     */
    public get itemComponent(): typeof TokenTransactionItem {
        return TokenTransactionItem;
    }

    public async updatePaymentMethod() {
        try {
            await this.$store.dispatch(MAKE_CARD_DEFAULT, this.defaultCreditCardSelection);
            await this.$notify.success('Default payment card updated');
            this.isChangeDefaultPaymentModalOpen = false;
        } catch (error) {
            await this.$notify.error(error.message);
        }
    }

    public async removePaymentMethod() {
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
            this.$notify.error("You cannot delete the default payment method.");
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
        await this.$refs.stripeCardInput.onSubmit();
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

    // controls sorting the table
    public sortFunction(key) {
        this.paginationLocation = {start: 0, end: 10}
        this.displayedHistory = this.tokenHistory.slice(0,10)
        switch (key) {
            case 'date-ascending':
                this.tokenHistory.sort((a,b) => {return a.start.getTime() - b.start.getTime()});
                break;
            case 'date-descending':
                this.tokenHistory.sort((a,b) => {return b.start.getTime() - a.start.getTime()});
                break;
            case 'amount-ascending':
                this.tokenHistory.sort((a,b) => {return a.amount - b.amount});
                break;
            case 'amount-descending':
                this.tokenHistory.sort((a,b) => {return b.amount - a.amount});
                break;
            case 'status-ascending':
                this.tokenHistory.sort((a, b) => {
                    if (a.status < b.status) {return -1;}
                    if (a.status > b.status) {return 1;}
                    return 0});
                break;
            case 'status-descending':
                this.tokenHistory.sort((a, b) => {
                    if (b.status < a.status) {return -1;}
                    if (b.status > a.status) {return 1;}
                    return 0});
                break;
        }
    }

    //controls pagination
    public paginationController(i): void {
        let diff = this.transactionCount - this.paginationLocation.start
        if (this.paginationLocation.start + i >= 0 && this.paginationLocation.end + i <= this.transactionCount && this.paginationLocation.end !== this.transactionCount){
            this.paginationLocation = {
                start: this.paginationLocation.start + i,
                end: this.paginationLocation.end + i
            }
        } else if (this.paginationLocation.start + i < 0 ) {
            this.paginationLocation = {
                start: 0,
                end: 10
            }
        } else if(this.paginationLocation.end + i > this.transactionCount) {
            this.paginationLocation = {
                start: this.paginationLocation.start + i,
                end: this.transactionCount
            }
        }   else if(this.paginationLocation.end === this.transactionCount) {
            this.paginationLocation = {
                start: this.paginationLocation.start + i,
                end: this.transactionCount - (diff)
            }
        }

        this.displayedHistory = this.tokenHistory.slice(this.paginationLocation.start, this.paginationLocation.end)
    }

    /**
     * Returns deposit history items.
     */
    public get depositHistoryItems(): PaymentsHistoryItem[] {
        return this.$store.state.paymentsModule.paymentsHistory.filter((item: PaymentsHistoryItem) => {
            return item.type === PaymentsHistoryItemType.Transaction || item.type === PaymentsHistoryItemType.DepositBonus;
        });
    }

    /**
     * Set of default payment options.
     */
    public paymentOptions: PaymentAmountOption[] = [
        new PaymentAmountOption(10, `USD $10`),
        new PaymentAmountOption(20, `USD $20`),
        new PaymentAmountOption(50, `USD $50`),
        new PaymentAmountOption(100, `USD $100`),
        new PaymentAmountOption(1000, `USD $1000`),
    ];

    /**
     * onConfirmAddSTORJ checks if amount is valid.
     * If so processes token payment and returns state to default.
     */
    public async onConfirmAddSTORJ(): Promise<void> {

        if (!this.isDepositValueValid) return;

        try {
            const tokenResponse = await this.$store.dispatch(MAKE_TOKEN_DEPOSIT, this.tokenDepositValue * 100);
            await this.$notify.success(`Successfully created new deposit transaction! \nAddress:${tokenResponse.address} \nAmount:${tokenResponse.amount}`);
            const depositWindow = window.open(tokenResponse.link, '_blank');
            if (depositWindow) {
                depositWindow.focus();
            }
        } catch (error) {
            await this.$notify.error(error.message);
            this.$emit('toggleIsLoading');
        }

        this.tokenDepositValue = this.DEFAULT_TOKEN_DEPOSIT_VALUE;
        try {
            await this.$store.dispatch(GET_PAYMENTS_HISTORY);
        } catch (error) {
            await this.$notify.error(error.message);
            this.$emit('toggleIsLoading');
        }
    }

    /**
     * Event for changing token deposit value.
     */
    public onChangeTokenValue(value: number): void {
        this.tokenDepositValue = value;
    }

    /**
     * Indicates if user has own project.
     */
    private get userHasOwnProject(): boolean {
        return this.$store.getters.projectsCount > 0;
    }

    /**
     * Indicates if deposit value is valid.
     */
    private get isDepositValueValid(): boolean {
        switch (true) {
        case (this.tokenDepositValue < this.DEFAULT_TOKEN_DEPOSIT_VALUE || this.tokenDepositValue >= this.MAX_TOKEN_AMOUNT) && !this.userHasOwnProject:
            this.$notify.error('First deposit amount must be more than $10 and less than $1000000');
            this.setDefault();

            return false;
        case this.tokenDepositValue >= this.MAX_TOKEN_AMOUNT || this.tokenDepositValue === 0:
            this.$notify.error('Deposit amount must be more than $0 and less than $1000000');
            this.setDefault();

            return false;
        default:
            return true;
        }
    }

    /**
     * Sets adding payment method state to default.
     */
    private setDefault(): void {
        this.tokenDepositValue = this.DEFAULT_TOKEN_DEPOSIT_VALUE;
    }

}
</script>

<style scoped lang="scss">

    .Pending {
        color: #FFA800;
    }

    .Confirmed {
        color: #00ac26;
    }

    .Rejected {
        color: #ac1a00;
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
        color: #0149FF;
    }

    .change-default-input-container {
        margin: auto;
        display: flex;
        flex-direction: row;
        align-items: flex-start;
        padding: 16px;
        gap: 10px;
        width: 300px;
        height: 10px;
        /* background: #e6edf7; */
        border: 1px solid #C8D3DE;
        border-radius: 8px;
        margin-top: 7px;
    }   
    

    .change-default-input {
        margin-left: auto;
        background: #FFFFFF;
        border: 1px solid #C8D3DE;
        border-radius: 24px;
    }

    .default-card-button {
        margin-top: 20px;
        margin-bottom: 20px;
        cursor: pointer;
        margin-left: 112px;
        display: flex;
        grid-column: 1;
        grid-row: 5;
        width: 132px;
        height: 24px;
        align-items: center;
        padding: 16px;
        gap: 8px;
        background: #0149FF;
        box-shadow: 0px 0px 1px rgba(9, 28, 69, 0.8);
        border-radius: 8px;
        font-family: sans-serif;
        font-style: normal;
        font-weight: 700;
        font-size: 14px;
        line-height: 24px;
        display: flex;
        align-items: center;
        letter-spacing: -0.02em;
        color: white;

        &:hover {
            background-color: #0059d0;
        }   
    }

     @keyframes example {
            from {
                color: #56606d;
                border: 1px solid #D8DEE3;
            }
            to {
                border: 1px solid #e30011!important;
                color: #e30011;
            }
        }


    .remove-card-button {
        cursor: pointer;
        animation: example;
        animation-duration: 4s;
        margin-left: 130px;
        margin-top: 15px;
        margin-bottom: 21px;
        display: flex;
        grid-column: 1;
        grid-row: 5;
        width: 111px;
        height: 24px;
        align-items: center;
        padding: 16px;
        gap: 8px;
        background: #FFFFFF;
        border: 1px solid #D8DEE3;
        box-shadow: 0px 0px 3px rgba(0, 0, 0, 0.08);
        border-radius: 8px;
        font-family: sans-serif;
        font-style: normal;
        font-weight: 700;
        font-size: 14px;
        line-height: 24px;
        display: flex;
        align-items: center;
        letter-spacing: -0.02em;
        color: #56606D;

       
        &:hover {
            border: 1px solid #e30011!important;
            color: #e30011;
        }
        
    }

    .payments-area__container__new-payments {
        padding: 18px;
    }

    .close-add-payment {
        position: absolute;
        margin-left: 208px;
    }

    .card-icon { 
        margin-top: 10px;
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
        // width: 546px;
        position: fixed;
        top: 0;
        bottom: 0;
        left: 0;
        right: 0;
        z-index: 100;
        background: rgb(27 37 51 / 75%);
        display: flex;
        align-items: center;
        justify-content: center;

        &__header-change-default {
            margin-top: -6px;
            margin-left: 141px;
            grid-column: 1;
            grid-row: 4
        }

        &__header {
            grid-column: 1;
            grid-row: 2;
            font-family: sans-serif;
            font-style: normal;
            font-weight: 800;
            font-size: 24px;
            line-height: 31px;
            /* or 129% */

            text-align: center;
            letter-spacing: -0.02em;

            color: #1B2533;
        }
        &__header-subtext {
            grid-column: 1;
            grid-row: 3;
            font-family: sans-serif;
            font-style: normal;
            font-weight: 400;
            font-size: 14px;
            line-height: 20px;
            text-align: center;
            color: #56606D;
        }

        &__header-subtext-default {
            margin-left: 94px;
            font-family: sans-serif;
            font-style: normal;
            font-weight: 400;
            font-size: 14px;
            line-height: 20px;
            color: #56606D;
        }

        &__container {
            display: grid;
            grid-template-columns: auto;
            grid-template-rows: 1fr 30px 30px auto auto;
            width: 400px;
            background: #f5f6fa;
            border-radius: 6px;
            // justify-content: center;
           
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
        background-color: #0149FF;
        font-family: 'font_medium', sans-serif;
        font-size: 16px;
        line-height: 23px;
        color: #fff;
        user-select: none;
        transition: top 0.5s ease-in-out;

        &:hover {
            background-color: #0059d0;
        }
    }
    
    .active-discount {
        background: #dffff7;
        color: #00ac26;
    }

    .add_card_button_text {
        margin-top: 4px;
        margin-left: 9px;
        font-family: font-medium, sans-serif;
        font-style: normal;
        font-weight: 700;
        font-size: 13px;
        line-height: 29px;
        /* identical to box height, or 154% */

        display: flex;
        align-items: center;
        letter-spacing: -0.02em;
        
    }

    .inactive-discount {
        background: #ffe1df;
        color: #ac1a00;
    }

    .divider {
        height: 1px;
        width: calc(100% + 30px);
        background-color: #E5E7EB;
        align-self: center;
    }

    .stripe_input {
        grid-row: 3;
        grid-column: 1;
        width: 240px;
    }

    .payments-area {

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
            color: #56606D;
        }


        &__title {
            font-family: sans-serif;
            font-size: 24px;
            margin: 20px 0;
        }

        &__container {
            display: flex;
            flex-wrap: wrap;
            &__cards {
                width: 227px;
                height: 126px;
                padding: 20px;
                background: #FFFFFF;
                box-shadow: 0px 0px 20px rgba(0, 0, 0, 0.04);
                border-radius: 10px;
                margin: 0 10px 10px 0;   
            }

             &__token {
                border-radius: 10px;
                width: 227px;
                height: 126px;
                margin: 0 10px 10px 0;
                padding: 20px;
                box-shadow: 0 0 20px rgb(0 0 0 / 4%);
                background: #fff;
                position: relative;
                &__large-icon-container{
                    position: absolute;
                    top: 0px;
                    right: 0px;
                    height: 120px;
                    width: 120px;
                    z-index: 1;
                    border-radius: 10px;
                    overflow: hidden;
                }
                &__large-icon{
                    position: absolute;
                    top: -25px;
                    right: -24px;
                }
                &__base{
                    display: grid;
                    grid-template-columns: 1.5fr 1fr;
                    grid-template-rows: 4fr 1fr 1fr;
                    overflow: hidden;
                    font-family: sans-serif;
                    z-index: 5;
                    position: relative;
                    height: 100%;
                    width: 100%;
                    &__small-icon{
                        grid-column: 1;
                        grid-row: 1;
                        height: 30px;
                        width: 40px;
                        background-color: #E6EDF7;
                        border-radius: 5px;
                        display: flex;
                        justify-content: center;
                        align-items: center;
                    }
                    &__confirmation-container {
                        grid-column: 1;
                        grid-row: 2;
                        display: grid;
                        grid-template-columns: 1fr 6fr;
                        grid-template-rows: 1fr 1fr;
                        &__label{
                            font-size: 12px;
                            font-weight: 700;
                            color: #56606D;
                            grid-column: 1/ span 2;
                            grid-row: 1;
                            margin: auto 0 0 0;
                        }
                        &__circle-icon{
                            grid-column: 1;
                            grid-row: 2;
                            margin: auto;
                        }
                        &__text{
                            font-size: 16px;
                            font-weight: 700;
                            grid-column: 2;
                            grid-row: 2;
                            margin: auto 0;
                        }
                    }
                    &__balance-container {
                        grid-column: 2;
                        grid-row: 2;
                        display: grid;
                        grid-template-rows: 1fr 1fr;
                        &__label{
                            font-size: 12px;
                            font-weight: 700;
                            color: #56606D;
                            grid-row: 1;
                            margin: auto 0 0 0;
                        }
                        &__text{
                            font-size: 16px;
                            font-weight: 700;
                            grid-row: 2;
                            margin: auto 0;
                        }
                    }
                    &__transaction-button{
                        grid-column: 1;
                        grid-row: 4;
                    }
                    &__funds-button{
                        grid-column: 2;
                        grid-row: 4;
                    }
                }
                &__add-funds {
                    display: flex;
                    flex-direction: column;
                    z-index: 5;
                    position: relative;
                    height: 100%;
                    width: 100%;

                    &__title{
                        font-family: sans-serif;
                    }
                    &__label{
                        font-family: sans-serif;
                        color: #56606D;
                        font-size: 11px;
                        margin-top: 5px;
                    }
                    &__dropdown{
                        margin-top: 10px;
                    }
                    &__button{
                        margin-top: 10px;
                    }
                }
                &__confirmation-container {
                    grid-column: 1;
                    grid-row: 2;
                    z-index: 3;
                    display: grid;
                    grid-template-columns: 1fr 6fr;
                    grid-template-rows: 1fr 1fr;
                    &__label{
                        font-size: 12px;
                        font-weight: 700;
                        color: #56606D;
                        grid-column: 1/ span 2;
                        grid-row: 1;
                        margin: auto 0 0 0;
                    }
                    &__circle-icon{
                        grid-column: 1;
                        grid-row: 2;
                        margin: auto;
                    }
                    &__text{
                        font-size: 16px;
                        font-weight: 700;
                        grid-column: 2;
                        grid-row: 2;
                        margin: auto 0;
                    }
                }
                &__balance-container {
                    grid-column: 2;
                    grid-row: 2;
                    z-index: 3;
                    display: grid;
                    grid-template-rows: 1fr 1fr;
                    &__label{
                        font-size: 12px;
                        font-weight: 700;
                        color: #56606D;
                        grid-row: 1;
                        margin: auto 0 0 0;
                    }
                    &__text{
                        font-size: 16px;
                        font-weight: 700;
                        grid-row: 2;
                        margin: auto 0;
                    }
                }
                &__transaction-button{
                    grid-column: 1;
                    grid-row: 4;
                    z-index: 3;
                }
                &__funds-button{
                    grid-column: 2;
                    grid-row: 4;
                    z-index: 3;
                }
            }

            &__new-payments {
                width: 227px;
                height: 126px;
                padding: 18px;
                display: grid !important;
                grid-template-columns:  6fr;
                grid-template-rows: 1fr 1fr 1fr 1fr;
                border: 2px dashed #929fb1;
                border-radius: 10px;
                width: 227px;
                height: 126px;
                padding: 18px;
                display: flex;
                align-items: center;
                justify-content: center;
                cursor: pointer;

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
                box-shadow: 0px 0px 20px rgba(0, 0, 0, 0.04);
                padding: 0 15px;
                display: flex;
                flex-direction: column;
            }
        }
    }
    .pagination {
        display: flex;
        justify-content: space-between;
        font-family: sans-serif;
        padding: 15px 0;
        color: #6B7280;
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
                    padding: 1px 0 0 0;
                }
                &__right {
                    cursor: pointer;
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