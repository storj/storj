// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payment-methods-area">
        <div class="payment-methods-area__functional-area" :class="functionalAreaClassName">
            <div class="payment-methods-area__functional-area__top-container">
                <h1 class="payment-methods-area__functional-area__title">Payment Method</h1>
                <div class="payment-methods-area__functional-area__button-area">
                    <div class="payment-methods-area__functional-area__button-area__default-buttons" v-if="!areAddButtonsClicked">
                        <VButton
                            class="add-storj-button"
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
            <AddStorjForm
                ref="addStorj"
                v-if="isAddingStorjState"
                :is-loading="isLoading"
                @toggleIsLoading="toggleIsLoading"
                @cancel="onCancel"
            />
            <AddCardForm
                ref="addCard"
                v-if="isAddingCardState"
                @toggleIsLoading="toggleIsLoading"
                @toggleIsLoaded="toggleIsLoaded"
                @cancel="onCancel"
            />
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

import AddCardForm from '@/components/account/billing/paymentMethods/AddCardForm.vue';
import AddStorjForm from '@/components/account/billing/paymentMethods/AddStorjForm.vue';
import CardComponent from '@/components/account/billing/paymentMethods/CardComponent.vue';
import PaymentsBonus from '@/components/account/billing/paymentMethods/PaymentsBonus.vue';
import VButton from '@/components/common/VButton.vue';

import LockImage from '@/../static/images/account/billing/lock.svg';
import SuccessImage from '@/../static/images/account/billing/success.svg';

import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { CreditCard } from '@/types/payments';
import { SegmentEvent } from '@/utils/constants/analyticsEventNames';
import { PaymentMethodsBlockState } from '@/utils/constants/billingEnums';

const {
    GET_CREDIT_CARDS,
    GET_BILLING_HISTORY,
    GET_BALANCE,
} = PAYMENTS_ACTIONS;

interface AddCardConfirm {
    onConfirmAddStripe(): Promise<void>;
}

@Component({
    components: {
        AddStorjForm,
        AddCardForm,
        VButton,
        CardComponent,
        PaymentsBonus,
        LockImage,
        SuccessImage,
    },
})
export default class PaymentMethods extends Vue {
    private areaState: number = PaymentMethodsBlockState.DEFAULT;

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
        addCard: AddCardForm & AddCardConfirm;
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
        return this.isDefaultState && !this.$store.getters.canUserCreateFirstProject;
    }

    /**
     * Indicates if any of credit cards is attached to account.
     */
    public get noCreditCards(): boolean {
        return this.$store.state.paymentsModule.creditCards.length === 0;
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
            await this.onConfirmAddCard();

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
    }

    /**
     * Toggles loading state.
     */
    public toggleIsLoading(): void {
        this.isLoading = !this.isLoading;
    }

    /**
     * Toggles loaded state.
     */
    public toggleIsLoaded(): void {
        this.isLoaded = !this.isLoaded;
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
    private async onConfirmAddCard(): Promise<void> {
        await this.$refs.addCard.onConfirmAddStripe();
    }
}
</script>

<style scoped lang="scss">
    .add-card-button {
        display: flex;
        justify-content: center;
        align-items: center;
        width: 125px;
        height: 50px;
        position: absolute;
        top: 0;
        right: 0;
        cursor: pointer;
        border-radius: 6px;
        background-color: #2683ff;
        font-family: 'font_medium', sans-serif;
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
                margin: 0;
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
