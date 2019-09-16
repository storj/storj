// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payment-methods-area">
        <div class="payment-methods-area__top-container">
            <h1 class="payment-methods-area__title">Payment Methods</h1>
            <div class="payment-methods-area__button-area">
                <div class="payment-methods-area__button-area__default-buttons" v-if="isDefaultState">
                    <Button
                        class="button"
                        label="Add STORJ"
                        width="123px"
                        height="48px"
                        :onPress="onAddSTORJ"/>
                    <Button
                        class="button"
                        label="Add Card"
                        width="123px"
                        height="48px"
                        :onPress="onAddCard"/>
                </div>
                <div class="payment-methods-area__button-area__cancel" v-if="!isDefaultState" @click="onCancel">
                    <p class="payment-methods-area__button-area__cancel__text">Cancel</p>
                </div>
            </div>
        </div>
        <div class="payment-methods-area__adding-container storj" v-if="isAddingStorjState">
            <div class="storj-container">
                <p>Deposit STORJ Tokens via Coin Payments</p>
                <StorjInput />
            </div>
            <Button
                label="Continue to Coin Payments"
                width="251px"
                height="48px"
                :onPress="onConfirmAddSTORJ"/>
        </div>
        <div class="payment-methods-area__adding-container card" v-if="isAddingCardState">
            <p>Add Credit or Debit Card</p>
            <StripeInput />
            <Button
                label="Add card"
                width="123px"
                height="48px"
                :onPress="onConfirmAddSTORJ"/>
        </div>
        <div class="payment-methods-area__existing-cards-container">
            <CardComponent />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import CardComponent from '@/components/account/billing/CardComponent.vue';
import StorjInput from '@/components/account/billing/StorjInput.vue';
import StripeInput from '@/components/account/billing/StripeInput.vue';
import Button from '@/components/common/Button.vue';

import { PaymentMethodsBlockState } from '@/utils/constants/billingEnums';

@Component({
    components: {
        Button,
        CardComponent,
        StorjInput,
        StripeInput,
    }
})
export default class PaymentMethods extends Vue {
    public areaState: number = PaymentMethodsBlockState.DEFAULT;

    public get isDefaultState(): boolean {
        return this.areaState === PaymentMethodsBlockState.DEFAULT;
    }
    public get isAddingStorjState(): boolean {
        return this.areaState === PaymentMethodsBlockState.ADDING_STORJ;
    }
    public get isAddingCardState(): boolean {
        return this.areaState === PaymentMethodsBlockState.ADDING_CARD;
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

        return;
    }

    public onConfirmAddSTORJ(): void {
        return;
    }
}
</script>

<style scoped lang="scss">
    h1,
    span {
        margin: 0;
        color: #354049;
    }

    form {
        width: 60%;
    }

    .button {

        &:last-of-type {
            margin-left: 23px;
        }

        &:hover {
            background-color: #0059D0;
            box-shadow: none;
        }
    }

    .payment-methods-area {
        padding: 40px;
        margin-bottom: 47px;
        background-color: #FFFFFF;
        border-radius: 8px;
        font-family: 'font_regular';

        &__top-container {
            display: flex;
            display: flex;
            align-items: center;
            justify-content: space-between;
        }

        &__title {
            font-family: 'font_bold';
            font-size: 32px;
            line-height: 48px;
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
                    font-family: 'font_medium';
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

            p {
                font-family: 'font_medium';
                font-size: 21px;
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

        p {
            margin-right: 30px;
        }
    }
</style>
