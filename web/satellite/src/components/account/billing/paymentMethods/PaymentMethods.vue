// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payment-methods-area">
        <div class="payment-methods-area__top-container">
            <h1 class="payment-methods-area__title text">Payment Methods</h1>
            <div class="payment-methods-area__button-area">
                <div class="payment-methods-area__button-area__default-buttons" v-if="isDefaultState">
                    <VButton
                        class="button"
                        label="Add STORJ"
                        width="123px"
                        height="48px"
                        :on-press="onAddSTORJ"
                    />
                    <VButton
                        class="button"
                        label="Add Card"
                        width="123px"
                        height="48px"
                        :on-press="onAddCard"
                    />
                </div>
                <div class="payment-methods-area__button-area__cancel" v-if="!isDefaultState" @click="onCancel">
                    <p class="payment-methods-area__button-area__cancel__text">Cancel</p>
                </div>
            </div>
        </div>
        <PaymentsBonus
            v-if="isDefaultBonusBannerShown"
            :any-credit-cards="false"
            class="payment-methods-area__bonus"
        />
        <PaymentsBonus
            v-else
            :any-credit-cards="true"
            class="payment-methods-area__bonus"
        />
        <div class="payment-methods-area__adding-container storj" v-if="isAddingStorjState">
            <div class="storj-container">
                <p class="storj-container__label">Deposit STORJ Tokens via Coin Payments</p>
                <TokenDepositSelection class="form" @onChangeTokenValue="onChangeTokenValue"/>
            </div>
            <VButton
                label="Continue to Coin Payments"
                width="251px"
                height="48px"
                :on-press="onConfirmAddSTORJ"
            />
        </div>
        <div class="payment-methods-area__adding-container card" v-if="isAddingCardState">
            <p class="payment-methods-area__adding-container__label">Add Credit or Debit Card</p>
            <StripeCardInput
                class="payment-methods-area__adding-container__stripe"
                ref="stripeCardInput"
                :on-stripe-response-callback="addCard"
            />
            <VButton
                label="Add card"
                width="123px"
                height="48px"
                :on-press="onConfirmAddStripe"
            />
        </div>
        <div class="payment-methods-area__existing-cards-container">
            <CardComponent
                v-for="card in creditCards"
                :key="card.id"
                :credit-card="card"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import CardComponent from '@/components/account/billing/paymentMethods/CardComponent.vue';
import PaymentsBonus from '@/components/account/billing/paymentMethods/PaymentsBonus.vue';
import StripeCardInput from '@/components/account/billing/paymentMethods/StripeCardInput.vue';
import TokenDepositSelection from '@/components/account/billing/paymentMethods/TokenDepositSelection.vue';
import VButton from '@/components/common/VButton.vue';

import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { CreditCard } from '@/types/payments';
import { SegmentEvent } from '@/utils/constants/analyticsEventNames';
import { PaymentMethodsBlockState } from '@/utils/constants/billingEnums';

const {
    ADD_CREDIT_CARD,
    GET_CREDIT_CARDS,
    MAKE_TOKEN_DEPOSIT,
    GET_BILLING_HISTORY,
} = PAYMENTS_ACTIONS;

interface StripeForm {
    onSubmit(): Promise<void>;
}

@Component({
    components: {
        VButton,
        CardComponent,
        TokenDepositSelection,
        StripeCardInput,
        PaymentsBonus,
    },
})
export default class PaymentMethods extends Vue {
    private areaState: number = PaymentMethodsBlockState.DEFAULT;
    private isLoading: boolean = false;
    private readonly DEFAULT_TOKEN_DEPOSIT_VALUE = 20;
    private readonly MAX_TOKEN_AMOUNT_IN_DOLLARS = 1000000;
    private tokenDepositValue: number = this.DEFAULT_TOKEN_DEPOSIT_VALUE;

    public mounted() {
        try {
            this.$segment.track(SegmentEvent.PAYMENT_METHODS_VIEWED, {
                project_id: this.$store.getters.selectedProject.id,
            });
            this.$store.dispatch(GET_CREDIT_CARDS);
        } catch (error) {
            this.$notify.error(error.message);
        }
    }

    public $refs!: {
        stripeCardInput: StripeCardInput & StripeForm;
    };

    public get creditCards(): CreditCard[] {
        return this.$store.state.paymentsModule.creditCards;
    }

    public get isDefaultState(): boolean {
        return this.areaState === PaymentMethodsBlockState.DEFAULT;
    }
    public get isAddingStorjState(): boolean {
        return this.areaState === PaymentMethodsBlockState.ADDING_STORJ;
    }
    public get isAddingCardState(): boolean {
        return this.areaState === PaymentMethodsBlockState.ADDING_CARD;
    }

    public get isDefaultBonusBannerShown(): boolean {
        return this.isDefaultState && this.noCreditCards;
    }

    public get noCreditCards(): boolean {
        return this.$store.state.paymentsModule.creditCards.length === 0;
    }

    public onChangeTokenValue(value: number): void {
        this.tokenDepositValue = value;
    }

    public onAddSTORJ(): void {
        this.areaState = PaymentMethodsBlockState.ADDING_STORJ;

        return;
    }
    public onAddCard(): void {
        this.areaState = PaymentMethodsBlockState.ADDING_CARD;

        return;
    }
    public onCancel(): void {
        this.areaState = PaymentMethodsBlockState.DEFAULT;
        this.tokenDepositValue = this.DEFAULT_TOKEN_DEPOSIT_VALUE;

        return;
    }

    /**
     * onConfirmAddSTORJ checks if amount is valid and if so process token
     * payment and return state to default
     */
    public async onConfirmAddSTORJ(): Promise<void> {
        if (this.tokenDepositValue >= this.MAX_TOKEN_AMOUNT_IN_DOLLARS || this.tokenDepositValue === 0) {
            await this.$notify.error('Deposit amount must be more than 0 and less than 1000000');
            this.tokenDepositValue = this.DEFAULT_TOKEN_DEPOSIT_VALUE;
            this.areaState = PaymentMethodsBlockState.DEFAULT;

            return;
        }

        try {
            const tokenResponse = await this.$store.dispatch(MAKE_TOKEN_DEPOSIT, this.tokenDepositValue * 100);
            await this.$notify.success(`Successfully created new deposit transaction! \nAddress:${tokenResponse.address} \nAmount:${tokenResponse.amount}`);
            const depositWindow = window.open(tokenResponse.link, '_blank');
            if (depositWindow) {
                depositWindow.focus();
            }
        } catch (error) {
            await this.$notify.error(error.message);
        }

        this.$segment.track(SegmentEvent.PAYMENT_METHOD_ADDED, {
            project_id: this.$store.getters.selectedProject.id,
        });

        this.tokenDepositValue = this.DEFAULT_TOKEN_DEPOSIT_VALUE;
        try {
            await this.$store.dispatch(GET_BILLING_HISTORY);
        } catch (error) {
            await this.$notify.error(error.message);
        }

        this.areaState = PaymentMethodsBlockState.DEFAULT;
    }

    public async onConfirmAddStripe(): Promise<void> {
        await this.$refs.stripeCardInput.onSubmit();

        try {
            await this.$store.dispatch(GET_BILLING_HISTORY);
        } catch (error) {
            await this.$notify.error(error.message);
        }

        this.$segment.track(SegmentEvent.PAYMENT_METHOD_ADDED, {
            project_id: this.$store.getters.selectedProject.id,
        });
    }

    public async addCard(token: string) {
        if (this.isLoading) {
            return;
        }

        this.isLoading = true;

        try {
            await this.$store.dispatch(ADD_CREDIT_CARD, token);
        } catch (error) {
            await this.$notify.error(error.message);

            this.isLoading = false;

            return;
        }

        await this.$notify.success('Card successfully added');
        this.$segment.track(SegmentEvent.PAYMENT_METHOD_ADDED, {
            project_id: this.$store.getters.selectedProject.id,
        });
        try {
            await this.$store.dispatch(GET_CREDIT_CARDS);
        } catch (error) {
            await this.$notify.error(error.message);
            this.isLoading = false;
        }

        this.areaState = PaymentMethodsBlockState.DEFAULT;

        this.isLoading = false;
    }
}
</script>

<style scoped lang="scss">
    .text {
        margin: 0;
        color: #354049;
    }

    .form {
        width: 60%;
    }

    .button {

        &:last-of-type {
            margin-left: 23px;
        }

        &:hover {
            background-color: #0059d0;
            box-shadow: none;
        }
    }

    .payment-methods-area {
        padding: 40px;
        margin-bottom: 32px;
        background-color: #fff;
        border-radius: 8px;
        font-family: 'font_regular', sans-serif;

        &__top-container {
            display: flex;
            align-items: center;
            justify-content: space-between;
        }

        &__bonus {
            margin-top: 50px;
        }

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 32px;
            line-height: 48px;
            user-select: none;
        }

        &__button-area {
            display: flex;
            align-items: center;
            max-height: 48px;

            &__default-buttons {
                display: flex;
            }

            &__cancel {

                &__text {
                    font-family: 'font_medium', sans-serif;
                    font-size: 16px;
                    text-decoration: underline;
                    color: #354049;
                    opacity: 0.7;
                    cursor: pointer;
                }
            }
        }

        &__adding-container {
            margin-top: 44px;
            display: flex;
            max-height: 52px;
            justify-content: space-between;
            align-items: center;

            &__label {
                font-family: 'font_medium', sans-serif;
                font-size: 21px;
            }

            &__stripe {
                width: 60%;
                min-width: 400px;
            }
        }

        &__existing-cards-container {
            position: relative;
            padding-top: 12px;
            width: 100%;
        }
    }

    .storj-container {
        display: flex;
        align-items: center;

        &__label {
            margin-right: 30px;
        }
    }
</style>
