// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payment-methods-area">
        <div class="payment-methods-area__functional-area" :class="functionalAreaClassName">
            <div class="payment-methods-area__functional-area__top-container">
                <h1 class="payment-methods-area__functional-area__title text">Payment Method</h1>
                <div class="payment-methods-area__functional-area__button-area">
                    <div class="payment-methods-area__functional-area__button-area__default-buttons" v-if="!areAddButtonsClicked">
                        <VButton
                            class="button"
                            label="Add STORJ"
                            width="123px"
                            height="48px"
                            is-blue-white="true"
                            :on-press="onAddSTORJ"
                        />
                    </div>
                    <div class="payment-methods-area__functional-area__button-area__cancel" v-else @click="onCancel">
                        <p class="payment-methods-area__functional-area__button-area__cancel__text">Cancel</p>
                    </div>
                </div>
            </div>
            <PaymentsBonus
                v-if="isDefaultBonusBannerShown && !isAddCardClicked"
                :any-credit-cards="false"
                class="payment-methods-area__functional-area__bonus"
            />
            <PaymentsBonus
                v-else-if="!isDefaultBonusBannerShown && !isAddCardClicked"
                :any-credit-cards="true"
                class="payment-methods-area__functional-area__bonus"
            />
            <div class="payment-methods-area__functional-area__adding-container" v-if="isAddingStorjState">
                <div class="storj-container">
                    <p class="storj-container__label">Deposit STORJ Tokens via Coin Payments</p>
                    <TokenDepositSelection class="form" @onChangeTokenValue="onChangeTokenValue"/>
                </div>
                <div class="payment-methods-area__functional-area__adding-container__submit-area">
                    <img
                        v-if="isLoading"
                        class="payment-loading-image"
                        src="@/../static/images/account/billing/loading.gif"
                        alt="loading gif"
                    >
                    <VButton
                        class="confirm-add-storj-button"
                        label="Continue to Coin Payments"
                        width="251px"
                        height="48px"
                        :on-press="onConfirmAddSTORJ"
                        :is-disabled="isLoading"
                    />
                </div>
            </div>
            <div class="payment-methods-area__functional-area__adding-container" v-if="isAddingCardState">
                <p class="payment-methods-area__functional-area__adding-container__label">Add Credit or Debit Card</p>
                <StripeCardInput
                    class="payment-methods-area__functional-area__adding-container__stripe"
                    ref="stripeCardInput"
                    :on-stripe-response-callback="addCard"
                />
                <div class="payment-methods-area__functional-area__adding-container__submit-area"/>
            </div>
            <div
                v-if="!isAddStorjClicked"
                class="add-card-button"
                :class="{ 'button-moved': isAddCardClicked }"
                @click="onAddCard"
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
                <span>{{ addingCCButtonLabel }}</span>
            </div>
        </div>
        <div class="payment-methods-area__security-info-container" v-if="isAddingCardState && noCreditCards">
            <LockImage/>
            <span class="payment-methods-area__security-info-container__info">
                Your card is secured by Stripe through TLS and AES-256 encryption. Your information is secure.
            </span>
        </div>
        <div class="payment-methods-area__existing-cards-container" v-if="!noCreditCards">
            <CardComponent
                v-for="card in creditCards"
                :key="card.id"
                :credit-card="card"
            />
        </div>
        <div class="payment-methods-area__blur" v-if="isLoading || isLoaded"/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import CardComponent from '@/components/account/billing/paymentMethods/CardComponent.vue';
import PaymentsBonus from '@/components/account/billing/paymentMethods/PaymentsBonus.vue';
import StripeCardInput from '@/components/account/billing/paymentMethods/StripeCardInput.vue';
import TokenDepositSelection from '@/components/account/billing/paymentMethods/TokenDepositSelection.vue';
import VButton from '@/components/common/VButton.vue';

import LockImage from '@/../static/images/account/billing/lock.svg';
import SuccessImage from '@/../static/images/account/billing/success.svg';

import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { CreditCard } from '@/types/payments';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { SegmentEvent } from '@/utils/constants/analyticsEventNames';
import { PaymentMethodsBlockState } from '@/utils/constants/billingEnums';
import { ProjectOwning } from '@/utils/projectOwning';

const {
    ADD_CREDIT_CARD,
    GET_CREDIT_CARDS,
    MAKE_TOKEN_DEPOSIT,
    GET_BILLING_HISTORY,
    GET_BALANCE,
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
        LockImage,
        SuccessImage,
    },
})
export default class PaymentMethods extends Vue {
    private areaState: number = PaymentMethodsBlockState.DEFAULT;
    private readonly DEFAULT_TOKEN_DEPOSIT_VALUE = 50; // in dollars.
    private readonly MAX_TOKEN_AMOUNT = 1000000; // in dollars.
    private tokenDepositValue: number = this.DEFAULT_TOKEN_DEPOSIT_VALUE;

    public isLoading: boolean = false;
    public isLoaded: boolean = false;
    public isAddCardClicked: boolean = false;
    public isAddStorjClicked: boolean = false;

    /**
     * Lifecycle hook after initial render where credit cards are fetched.
     */
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

    /**
     * Returns list of credit cards from store.
     */
    public get creditCards(): CreditCard[] {
        return this.$store.state.paymentsModule.creditCards;
    }

    /**
     * Sets class with specific styles for functional area.
     */
    public get functionalAreaClassName(): string {
        switch (true) {
            case this.isAddCardClicked:
                return 'reduced';
            case this.isAddStorjClicked:
                return 'extended';
            default:
                return '';
        }
    }

    /**
     * Indicates if adding buttons were clicked.
     */
    public get areAddButtonsClicked(): boolean {
        return this.isAddCardClicked || this.isAddStorjClicked;
    }

    /**
     * Sets button label depending on loading state.
     */
    public get addingCCButtonLabel(): string {
        switch (true) {
            case this.isLoading:
                return 'Adding';
            case this.isLoaded:
                return 'Added!';
            default:
                return 'Add Card';
        }
    }

    /**
     * Indicates if adding tokens in progress.
     */
    public get isAddingStorjState(): boolean {
        return this.areaState === PaymentMethodsBlockState.ADDING_STORJ;
    }

    /**
     * Indicates if adding card in progress.
     */
    public get isAddingCardState(): boolean {
        return this.areaState === PaymentMethodsBlockState.ADDING_CARD;
    }

    /**
     * Indicates if free credits or percentage bonus banner is shown.
     */
    public get isDefaultBonusBannerShown(): boolean {
        return this.isDefaultState && !this.$store.getters.isBonusCouponApplied;
    }

    /**
     * Indicates if any of credit cards is attached to account.
     */
    public get noCreditCards(): boolean {
        return this.$store.state.paymentsModule.creditCards.length === 0;
    }

    /**
     * Event for changing token deposit value.
     */
    public onChangeTokenValue(value: number): void {
        this.tokenDepositValue = value;
    }

    /**
     * Changes area state to adding tokens state.
     */
    public onAddSTORJ(): void {
        this.isAddStorjClicked = true;
        setTimeout(() => {
            this.areaState = PaymentMethodsBlockState.ADDING_STORJ;
        }, 500);
    }

    /**
     * Changes area state to adding card state and proceeds adding card process.
     */
    public async onAddCard(): Promise<void> {
        if (this.isAddCardClicked) {
            await this.onConfirmAddStripe();

            return;
        }

        this.isAddCardClicked = true;
        setTimeout(() => {
            this.areaState = PaymentMethodsBlockState.ADDING_CARD;
        }, 500);
    }

    /**
     * Changes area state and token deposit value to default.
     */
    public onCancel(): void {
        if (this.isLoading || this.isDefaultState) return;

        this.isAddCardClicked = false;
        this.isAddStorjClicked = false;
        this.areaState = PaymentMethodsBlockState.DEFAULT;
        this.tokenDepositValue = this.DEFAULT_TOKEN_DEPOSIT_VALUE;
    }

    /**
     * onConfirmAddSTORJ checks if amount is valid and if so process token.
     * payment and return state to default
     */
    public async onConfirmAddSTORJ(): Promise<void> {
        if (this.isLoading) return;

        this.isLoading = true;

        if ((this.tokenDepositValue < 50 || this.tokenDepositValue >= this.MAX_TOKEN_AMOUNT) && !new ProjectOwning(this.$store).userHasOwnProject()) {
            await this.$notify.error('First deposit amount must be more than 50 and less than 1000000');
            this.tokenDepositValue = this.DEFAULT_TOKEN_DEPOSIT_VALUE;
            this.areaState = PaymentMethodsBlockState.DEFAULT;
            this.isAddStorjClicked = false;
            this.isLoading = false;

            return;
        }

        if (this.tokenDepositValue >= this.MAX_TOKEN_AMOUNT || this.tokenDepositValue === 0) {
            await this.$notify.error('Deposit amount must be more than 0 and less than 1000000');
            this.tokenDepositValue = this.DEFAULT_TOKEN_DEPOSIT_VALUE;
            this.areaState = PaymentMethodsBlockState.DEFAULT;
            this.isAddStorjClicked = false;
            this.isLoading = false;

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
            this.isLoading = false;
        }

        this.$segment.track(SegmentEvent.PAYMENT_METHOD_ADDED, {
            project_id: this.$store.getters.selectedProject.id,
        });

        this.tokenDepositValue = this.DEFAULT_TOKEN_DEPOSIT_VALUE;
        try {
            await this.$store.dispatch(GET_BILLING_HISTORY);
        } catch (error) {
            await this.$notify.error(error.message);
            this.isLoading = false;
        }

        this.areaState = PaymentMethodsBlockState.DEFAULT;
        this.isAddStorjClicked = false;
        this.isLoading = false;
    }

    /**
     * Adds card after Stripe confirmation.
     *
     * @param token from Stripe
     */
    public async addCard(token: string) {
        if (this.isLoading) return;

        this.isLoading = true;

        try {
            await this.$store.dispatch(ADD_CREDIT_CARD, token);
            await this.$store.dispatch(GET_BILLING_HISTORY);
            await this.$store.dispatch(GET_BALANCE);
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

        this.isLoading = false;
        this.isLoaded = true;

        if (!new ProjectOwning(this.$store).userHasOwnProject()) {
            await this.$store.dispatch(APP_STATE_ACTIONS.SHOW_CREATE_PROJECT_BUTTON);
        }

        setTimeout(() => {
            this.onCancel();
            this.isLoaded = false;

            setTimeout(() => {
                this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_CONTENT_BLUR);
            }, 500);
        }, 2000);
    }

    /**
     * Indicates if no adding card nor tokens in progress.
     */
    private get isDefaultState(): boolean {
        return this.areaState === PaymentMethodsBlockState.DEFAULT;
    }

    /**
     * Provides card information to Stripe.
     */
    private async onConfirmAddStripe(): Promise<void> {
        await this.$refs.stripeCardInput.onSubmit();

        this.$segment.track(SegmentEvent.PAYMENT_METHOD_ADDED, {
            project_id: this.$store.getters.selectedProject.id,
        });
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

    .add-card-button {
        display: flex;
        justify-content: center;
        align-items: center;
        width: 123px;
        height: 48px;
        position: absolute;
        top: 0;
        right: 0;
        cursor: pointer;
        border-radius: 6px;
        background-color: #2683ff;
        font-family: 'font_bold', sans-serif;
        font-size: 16px;
        line-height: 23px;
        color: #fff;
        user-select: none;
        -webkit-transition: top 0.5s ease-in-out;
        -moz-transition: top 0.5s ease-in-out;
        -o-transition: top 0.5s ease-in-out;
        transition: top 0.5s ease-in-out;

        &:hover {
            background-color: #0059d0;
        }
    }

    .button-moved {
        top: 95px;
    }

    .payment-methods-area {
        position: relative;
        padding: 40px;
        margin-bottom: 32px;
        background-color: #fff;
        border-radius: 8px;
        font-family: 'font_regular', sans-serif;

        &__functional-area {
            position: relative;
            height: 192px;
            -webkit-transition: all 0.3s ease-in-out;
            -moz-transition: all 0.3s ease-in-out;
            -o-transition: all 0.3s ease-in-out;
            transition: all 0.3s ease-in-out;

            &__top-container {
                display: flex;
                align-items: center;
                justify-content: space-between;
            }

            &__bonus {
                margin-top: 20px;
            }

            &__title {
                font-family: 'font_bold', sans-serif;
                font-size: 28px;
                line-height: 42px;
                color: #384b65;
            }

            &__button-area {
                display: flex;
                align-items: center;
                max-height: 48px;

                &__default-buttons {
                    display: flex;
                    justify-content: flex-start;
                    width: 269px;
                }

                &__cancel {

                    &__text {
                        font-family: 'font_medium', sans-serif;
                        font-size: 16px;
                        text-decoration: underline;
                        color: #354049;
                        opacity: 0.7;
                        cursor: pointer;
                        user-select: none;
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

                &__submit-area {
                    display: flex;
                    align-items: center;
                    min-width: 135px;
                }
            }
        }

        &__security-info-container {
            position: absolute;
            left: 0;
            top: 215px;
            width: 100%;
            height: 50px;
            display: flex;
            align-items: center;
            justify-content: center;
            background-color: #cef0e3;
            border-radius: 0 0 8px 8px;

            &__info {
                font-size: 15px;
                line-height: 18px;
                color: #34bf89;
            }
        }

        &__existing-cards-container {
            position: relative;
            margin-top: 45px;
            width: 100%;
        }

        &__blur {
            position: absolute;
            top: 0;
            left: 0;
            height: 100%;
            width: 100%;
            border-radius: 8px;
            background-color: rgba(229, 229, 229, 0.2);
            z-index: 100;
        }
    }

    .storj-container {
        display: flex;
        align-items: center;

        &__label {
            margin-right: 30px;
        }
    }

    .payment-loading-image {
        width: 18px;
        height: 18px;
        margin-right: 5px;
    }

    .payment-loaded-image {
        margin-right: 5px;
    }

    .extended {
        height: 300px;
    }

    .reduced {
        height: 170px;
    }
</style>
