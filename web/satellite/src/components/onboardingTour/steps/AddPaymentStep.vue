// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payment-step">
        <h1 class="payment-step__title">Get Started with 50 GB Free</h1>
        <p class="payment-step__sub-title">
            Adding a payment method ensures your project won’t be interrupted after your <b>free</b> credit is used.
        </p>
        <div class="payment-step__methods-container">
            <div class="payment-step__methods-container__title-area">
                <h2 class="payment-step__methods-container__title-area__title">Payment Method</h2>
                <div class="payment-step__methods-container__title-area__options-area">
                    <span
                        class="payment-step__methods-container__title-area__options-area__token"
                        @click="setAddStorjState"
                        :class="{ selected: isAddStorjState }"
                    >
                        STORJ Token
                    </span>
                    <span
                        class="payment-step__methods-container__title-area__options-area__card"
                        @click="setAddCardState"
                        :class="{ selected: isAddCardState }"
                    >
                        Card
                    </span>
                </div>
            </div>
            <div class="payment-step__methods-container__blur" v-if="isLoading"/>
        </div>
        <AddCardState
            v-if="isAddCardState"
            @toggleIsLoading="toggleIsLoading"
            @setProjectState="setProjectState"
        />
        <AddStorjState
            v-if="isAddStorjState"
            @toggleIsLoading="toggleIsLoading"
            @setProjectState="setProjectState"
        />
        <h1 class="payment-step__title second-title">Transparent Monthly Pricing</h1>
        <p class="payment-step__sub-title">
            Pay only for the storage and bandwidth you use.
        </p>
        <div class="payment-step__pricing-modal">
            <div class="payment-step__pricing-modal__item">
                <div class="payment-step__pricing-modal__item__left-side">
                    <img src="@/../static/images/onboardingTour/cloud.png" alt="cloud image">
                    <span class="payment-step__pricing-modal__item__left-side__title">Storage</span>
                </div>
                <div class="payment-step__pricing-modal__item__right-side">
                    <b class="payment-step__pricing-modal__item__right-side__price">$0.01</b>
                    <span class="payment-step__pricing-modal__item__left-side__dimension">/GB</span>
                </div>
            </div>
            <div class="payment-step__pricing-modal__item download-item">
                <div class="payment-step__pricing-modal__item__left-side">
                    <img src="@/../static/images/onboardingTour/arrow-down.png" alt="arrow image">
                    <span class="payment-step__pricing-modal__item__left-side__title">Download</span>
                </div>
                <div class="payment-step__pricing-modal__item__right-side">
                    <b class="payment-step__pricing-modal__item__right-side__price">$0.045</b>
                    <span class="payment-step__pricing-modal__item__left-side__dimension">/GB</span>
                </div>
            </div>
            <div class="payment-step__pricing-modal__item">
                <div class="payment-step__pricing-modal__item__left-side">
                    <img src="@/../static/images/onboardingTour/squares.png" alt="squares image">
                    <span class="payment-step__pricing-modal__item__left-side__title">Per Object</span>
                </div>
                <div class="payment-step__pricing-modal__item__right-side">
                    <b class="payment-step__pricing-modal__item__right-side__price">$0.0000022</b>
                    <span class="payment-step__pricing-modal__item__left-side__dimension">/OBJECT</span>
                </div>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import AddCardState from '@/components/onboardingTour/steps/paymentStates/AddCardState.vue';
import AddStorjState from '@/components/onboardingTour/steps/paymentStates/AddStorjState.vue';

import { AddingPaymentState } from '@/utils/constants/onboardingTourEnums';

@Component({
    components: {
        AddStorjState,
        AddCardState,
    },
})

export default class AddPaymentStep extends Vue {
    public areaState: number = AddingPaymentState.ADD_CARD;
    public isLoading: boolean = false;

    /**
     * Lifecycle hook after initial render.
     * Sets area to needed state.
     */
    public mounted(): void {
        if (this.$store.getters.isTransactionProcessing || this.$store.getters.isBalancePositive) {
            this.setAddStorjState();
        }
    }

    /**
     * Indicates if area is in adding card state.
     */
    public get isAddCardState(): boolean {
        return this.areaState === AddingPaymentState.ADD_CARD;
    }

    /**
     * Indicates if area is in adding tokens state.
     */
    public get isAddStorjState(): boolean {
        return this.areaState === AddingPaymentState.ADD_STORJ;
    }

    /**
     * Sets area to adding card state.
     */
    public setAddCardState(): void {
        this.areaState = AddingPaymentState.ADD_CARD;
    }

    /**
     * Sets area to adding tokens state.
     */
    public setAddStorjState(): void {
        this.areaState = AddingPaymentState.ADD_STORJ;
    }

    /**
     * Toggles area's loading state.
     */
    public toggleIsLoading(): void {
        this.isLoading = !this.isLoading;
    }

    /**
     * Sets tour area to creating project state.
     */
    public setProjectState(): void {
        this.$emit('setProjectState');
    }
}
</script>

<style scoped lang="scss">
    h1,
    h2,
    p {
        margin: 0;
    }

    .payment-step {
        font-family: 'font_regular', sans-serif;
        margin-top: 75px;
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: space-between;
        padding: 0 140px 200px 140px;
        position: relative;

        &__title {
            font-size: 32px;
            line-height: 39px;
            color: #1b2533;
            margin-bottom: 25px;
        }

        &__sub-title {
            font-size: 16px;
            line-height: 19px;
            color: #354049;
            margin-bottom: 35px;
            text-align: center;
            word-break: break-word;
        }

        &__methods-container {
            padding: 30px 45px 10px 45px;
            width: calc(100% - 90px);
            border-radius: 8px 8px 0 0;
            background-color: #fff;
            position: relative;

            &__title-area {
                display: flex;
                align-items: center;
                justify-content: space-between;

                &__title {
                    font-family: 'font_medium', sans-serif;
                    font-size: 22px;
                    line-height: 27px;
                    color: #354049;
                }

                &__options-area {
                    display: flex;
                    align-items: flex-start;
                    justify-content: space-between;
                    min-height: 21px;

                    &__token,
                    &__card {
                        font-size: 14px;
                        line-height: 18px;
                        color: #a9b5c1;
                        text-align: center;
                        cursor: pointer;
                    }

                    &__token {
                        min-width: 110px;
                    }

                    &__card {
                        margin-left: 10px;
                        min-width: 65px;
                    }
                }
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

        &__pricing-modal {
            width: calc(100% - 80px);
            padding: 20px 40px;
            background-color: #fff;
            border-radius: 8px;

            &__item {
                display: flex;
                align-items: center;
                justify-content: space-between;
                padding: 20px 0;
                width: 100%;

                &__left-side {
                    display: flex;
                    align-items: center;
                    justify-content: flex-start;

                    &__title {
                        margin-left: 20px;
                        font-family: 'font_medium', sans-serif;
                        font-size: 18px;
                        line-height: 20px;
                        color: #000;
                    }
                }

                &__right-side {
                    display: flex;
                    align-items: center;

                    &__price {
                        font-family: 'font_bold', sans-serif;
                        font-size: 18px;
                        line-height: 20px;
                        color: #000;
                    }

                    &__dimension {
                        font-size: 14px;
                        line-height: 20px;
                        color: #384b65;
                    }
                }
            }
        }
    }

    .selected {
        color: #2582ff;
        border-bottom: 3px solid #2582ff;
    }

    .second-title {
        margin-top: 30px;
    }

    .download-item {
        border-top: 1px solid #afb7c1;
        border-bottom: 1px solid #afb7c1;
    }

    @media screen and (max-width: 1550px) {

        .payment-step {
            padding: 0 70px 200px 70px;
        }
    }

    @media screen and (max-width: 800px) {

        .payment-step {
            padding: 0 25px 200px 25px;
        }
    }
</style>