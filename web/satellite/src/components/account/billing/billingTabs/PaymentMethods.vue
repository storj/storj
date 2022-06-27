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
            
                <div
                    class="payments-area__container__cards"
                >
                    
                </div>
                <div 
                    class="payments-area__container__new-payments"
                >
                    <div class="payments-area__container__new-payments__text-area">
                        <span class="payments-area__container__new-payments__text-area__plus-icon">+&nbsp;</span>
                        <span class="payments-area__container__new-payments__text-area__text">Add New Payment Method</span>
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
                <div class="">
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

import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PaymentsHistoryItem, PaymentsHistoryItemType, PaymentAmountOption } from '@/types/payments';
import { SortDirection } from '@/types/common';
import { RouteConfig } from '@/router';

const {
    MAKE_TOKEN_DEPOSIT,
    GET_PAYMENTS_HISTORY,
} = PAYMENTS_ACTIONS;

// @vue/component
@Component({
    components: {
        VLoader,
        VButton,
        VList,
        StorjSmall,
        StorjLarge,
        TokenTransactionItem,
        TokenDepositSelection2,
        SortingHeader2,
        ArrowIcon,
    },
})
export default class paymentsArea extends Vue {
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

    .divider {
        height: 1px;
        width: calc(100% + 30px);
        background-color: #E5E7EB;
        align-self: center;
    }

    .payments-area {

        &__title {
            font-family: sans-serif;
            font-size: 24px;
            margin: 20px 0;
        }

        &__container {
            display: flex;
            flex-wrap: wrap;
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
            }

            &__new-payments {
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
</style>