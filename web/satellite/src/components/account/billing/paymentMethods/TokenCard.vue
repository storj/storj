// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="token">
        <div class="token__large-icon-container">
            <div class="token__large-icon">
                <StorjLarge />
            </div>
        </div>
        <div v-if="!showAddFunds" class="token__base">
            <div class="token__base__small-icon">
                <StorjSmall />
            </div>
            <div class="token__base__confirmation-container">
                <p class="token__base__confirmation-container__label">STORJ Token Deposit</p>
                <span :class="`token__base__confirmation-container__circle-icon ${billingItem.formattedStatus}`">
                    &#9679;
                </span>
                <span class="token__base__confirmation-container__text">
                    <span>
                        {{ billingItem.formattedStatus }}
                    </span>
                </span>
            </div>
            <div class="token__base__balance-container">
                <p class="token__base__balance-container__label">Total Balance</p>
                <span class="token__base__balance-container__text">
                    USD ${{ billingItem.quantity.received.toFixed(2) }}
                </span>
            </div>
            <VButton
                label="See transactions"
                width="120px"
                height="30px"
                is-transparent="true"
                font-size="13px"
                class="token__base__transaction-button"
                :on-press="toggleTransactionsTable"
            />
            <VButton
                label="Add funds"
                font-size="13px"
                width="80px"
                height="30px"
                class="token__base__funds-button"
                :on-press="toggleShowAddFunds"
            />
        </div>
        <div
            v-if="showAddFunds"
            class="token__add-funds"
        >
            <h3
                class="token__add-funds__title"
            >
                STORJ Token
            </h3>
            <p class="token__add-funds__label">Deposit STORJ Tokens via Coin Payments:</p>
            <TokenDepositSelection2
                class="token__add-funds__dropdown"
                :payment-options="paymentOptions"
                @onChangeTokenValue="onChangeTokenValue"
            />
            <div class="token__add-funds__button-container">
                <VButton
                    class="token__add-funds__button"
                    label="Continue to CoinPayments"
                    width="150px"
                    height="30px"
                    font-size="11px"
                    :on-press="onConfirmAddSTORJ"
                />
                <VButton
                    class="token__add-funds__button"
                    label="Back"
                    is-transparent="true"
                    width="50px"
                    height="30px"
                    font-size="11px"
                    :on-press="toggleShowAddFunds"
                />
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Prop, Vue } from 'vue-property-decorator';

import StorjSmall from '@/../static/images/billing/storj-icon-small.svg';
import StorjLarge from '@/../static/images/billing/storj-icon-large.svg';
import VButton from '@/components/common/VButton.vue';
import TokenDepositSelection2 from '@/components/account/billing/paymentMethods/TokenDepositSelection2.vue';

import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { PaymentAmountOption, PaymentsHistoryItem } from '@/types/payments';

interface tokenValue {
    
}

const {
    MAKE_TOKEN_DEPOSIT,
    GET_PAYMENTS_HISTORY,
} = PAYMENTS_ACTIONS;

// @vue/component
@Component({
    components: {
        StorjSmall,
        StorjLarge,
        VButton,
        TokenDepositSelection2,
    },
})
export default class TokenCard extends Vue {
    @Prop({default: false})
    private showAddFunds: boolean;
    @Prop({default: () => new PaymentsHistoryItem()})
    private readonly billingItem: PaymentsHistoryItem;
    private readonly DEFAULT_TOKEN_DEPOSIT_VALUE = 10; // in dollars.
    private readonly MAX_TOKEN_AMOUNT = 1000000; // in dollars.
    private tokenDepositValue: number = this.DEFAULT_TOKEN_DEPOSIT_VALUE;

    public toggleShowAddFunds(): void {
        this.showAddFunds = !this.showAddFunds ;
    }

    public toggleTransactionsTable(): void {
        this.$emit("showTransactions")
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

    /**
     * Indicates if user has own project.
     */
    private get userHasOwnProject(): boolean {
        return this.$store.getters.projectsCount > 0;
    }
}
</script>

<style scoped lang="scss">
    .Confirmed {
        color: #00ac26;
    }

    .Rejected {
        color: #ac1a00;
    }

    .Pending {
        color: #ffa800;
    }

    .token {
        border-radius: 10px;
        width: 227px;
        height: 126px;
        margin: 0 10px 10px 0;
        padding: 20px;
        box-shadow: 0 0 20px rgb(0 0 0 / 4%);
        background: #fff;
        position: relative;

        &__large-icon-container {
            position: absolute;
            top: 0;
            right: 0;
            height: 120px;
            width: 120px;
            z-index: 1;
            border-radius: 10px;
            overflow: hidden;
        }

        &__large-icon {
            position: absolute;
            top: -25px;
            right: -24px;
        }

        &__base {
            display: grid;
            grid-template-columns: 1.5fr 1fr;
            grid-template-rows: 4fr 1fr 1fr;
            overflow: hidden;
            font-family: sans-serif;
            z-index: 5;
            position: relative;
            height: 100%;
            width: 100%;

            &__small-icon {
                grid-column: 1;
                grid-row: 1;
                height: 30px;
                width: 40px;
                background-color: #e6edf7;
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

                &__label {
                    font-size: 12px;
                    font-weight: 700;
                    color: #56606d;
                    grid-column: 1/ span 2;
                    grid-row: 1;
                    margin: auto 0 0;
                }

                &__circle-icon {
                    grid-column: 1;
                    grid-row: 2;
                    margin: auto;
                }

                &__text {
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

                &__label {
                    font-size: 12px;
                    font-weight: 700;
                    color: #56606d;
                    grid-row: 1;
                    margin: auto 0 0;
                }

                &__text {
                    font-size: 16px;
                    font-weight: 700;
                    grid-row: 2;
                    margin: auto 0;
                }
            }

            &__transaction-button {
                grid-column: 1;
                grid-row: 4;
            }

            &__funds-button {
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

            &__title {
                font-family: sans-serif;
            }

            &__label {
                font-family: sans-serif;
                color: #56606d;
                font-size: 11px;
                margin-top: 5px;
            }

            &__dropdown {
                margin-top: 10px;
            }

            &__button-container {
                margin-top: 10px;
                display: flex;
                justify-content: space-between;
            }
        }

        &__confirmation-container {
            grid-column: 1;
            grid-row: 2;
            z-index: 3;
            display: grid;
            grid-template-columns: 1fr 6fr;
            grid-template-rows: 1fr 1fr;

            &__label {
                font-size: 12px;
                font-weight: 700;
                color: #56606d;
                grid-column: 1/ span 2;
                grid-row: 1;
                margin: auto 0 0;
            }

            &__circle-icon {
                grid-column: 1;
                grid-row: 2;
                margin: auto;
            }

            &__text {
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

            &__label {
                font-size: 12px;
                font-weight: 700;
                color: #56606d;
                grid-row: 1;
                margin: auto 0 0;
            }

            &__text {
                font-size: 16px;
                font-weight: 700;
                grid-row: 2;
                margin: auto 0;
            }
        }

        &__transaction-button {
            grid-column: 1;
            grid-row: 4;
            z-index: 3;
        }

        &__funds-button {
            grid-column: 2;
            grid-row: 4;
            z-index: 3;
        }
    }

</style>
