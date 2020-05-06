// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="form-container">
        <div class="selected-container" v-if="!isCustomAmount">
            <div id="paymentSelectButton" class="selected-container__label-container" @click="open">
                <p class="selected-container__label-container__label">{{current.label}}</p>
                <div class="selected-container__label-container__svg">
                    <svg width="14" height="8" viewBox="0 0 14 8" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <path fill-rule="evenodd" clip-rule="evenodd" d="M0.372773 0.338888C0.869804 -0.112963 1.67565 -0.112963 2.17268 0.338888L7 4.72741L11.8273 0.338888C12.3243 -0.112963 13.1302 -0.112963 13.6272 0.338888C14.1243 0.790739 14.1243 1.52333 13.6272 1.97519L7 8L0.372773 1.97519C-0.124258 1.52333 -0.124258 0.790739 0.372773 0.338888Z" fill="#2683FF"/>
                    </svg>
                </div>
            </div>
        </div>
        <label class="label" v-if="isCustomAmount">
            <input
                v-number
                class="custom-input"
                placeholder="Enter Amount in USD"
                v-model="customAmount"
                @input="onCustomAmountChange"
            >
            <p class="label__sign" v-if="customAmount">$</p>
            <div class="input-svg" @click.stop="closeCustomAmountSelection">
                <svg width="14" height="8" viewBox="0 0 14 8" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path fill-rule="evenodd" clip-rule="evenodd" d="M0.372773 0.338888C0.869804 -0.112963 1.67565 -0.112963 2.17268 0.338888L7 4.72741L11.8273 0.338888C12.3243 -0.112963 13.1302 -0.112963 13.6272 0.338888C14.1243 0.790739 14.1243 1.52333 13.6272 1.97519L7 8L0.372773 1.97519C-0.124258 1.52333 -0.124258 0.790739 0.372773 0.338888Z" fill="#2683FF"/>
                </svg>
            </div>
        </label>
        <div
            id="paymentSelect"
            class="options-container"
            :class="{ 'top-expand': isExpandingTop }"
            v-if="isSelectionShown"
            v-click-outside="close"
        >
            <div
                class="options-container__item"
                v-for="option in options"
                :key="option.label"
                @click.prevent.stop="select(option)"
            >
                <div class="options-container__item__svg" v-if="isOptionSelected(option)">
                    <svg width="15" height="13" viewBox="0 0 15 13" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <path d="M14.0928 3.02746C14.6603 2.4239 14.631 1.4746 14.0275 0.907152C13.4239 0.339699 12.4746 0.368972 11.9072 0.972536L14.0928 3.02746ZM4.53846 11L3.44613 12.028C3.72968 12.3293 4.12509 12.5001 4.53884 12.5C4.95258 12.4999 5.34791 12.3289 5.63131 12.0275L4.53846 11ZM3.09234 7.27469C2.52458 6.67141 1.57527 6.64261 0.971991 7.21036C0.36871 7.77812 0.339911 8.72743 0.907664 9.33071L3.09234 7.27469ZM11.9072 0.972536L3.44561 9.97254L5.63131 12.0275L14.0928 3.02746L11.9072 0.972536ZM5.6308 9.97199L3.09234 7.27469L0.907664 9.33071L3.44613 12.028L5.6308 9.97199Z" fill="#2683FF"/>
                    </svg>
                </div>
                <p class="options-container__item__label">{{option.label}}</p>
            </div>
            <div class="options-container__custom-container" @click.stop.prevent="openCustomAmountSelection">
                <div class="options-container__item__svg" v-if="isCustomAmount">
                    <svg width="15" height="13" viewBox="0 0 15 13" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <path d="M14.0928 3.02746C14.6603 2.4239 14.631 1.4746 14.0275 0.907152C13.4239 0.339699 12.4746 0.368972 11.9072 0.972536L14.0928 3.02746ZM4.53846 11L3.44613 12.028C3.72968 12.3293 4.12509 12.5001 4.53884 12.5C4.95258 12.4999 5.34791 12.3289 5.63131 12.0275L4.53846 11ZM3.09234 7.27469C2.52458 6.67141 1.57527 6.64261 0.971991 7.21036C0.36871 7.77812 0.339911 8.72743 0.907664 9.33071L3.09234 7.27469ZM11.9072 0.972536L3.44561 9.97254L5.63131 12.0275L14.0928 3.02746L11.9072 0.972536ZM5.6308 9.97199L3.09234 7.27469L0.907664 9.33071L3.44613 12.028L5.6308 9.97199Z" fill="#2683FF"/>
                    </svg>
                </div>
                Custom Amount
            </div>
        </div>
        <div class="payment-selection-blur" v-if="isSelectionShown"/>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { PaymentAmountOption } from '@/types/payments';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { ProjectOwning } from '@/utils/projectOwning';

@Component
export default class TokenDepositSelection extends Vue {
    /**
     * Set of default payment options.
     */
    public paymentOptions: PaymentAmountOption[] = [
        new PaymentAmountOption(50, `USD $50`),
        new PaymentAmountOption(5, `USD $5`),
        new PaymentAmountOption(10, `USD $10`),
        new PaymentAmountOption(20, `USD $20`),
        new PaymentAmountOption(100, `USD $100`),
        new PaymentAmountOption(1000, `USD $1000`),
    ];

    /**
     * Set of payment options for the first ever transaction.
     */
    public initialPaymentOptions: PaymentAmountOption[] = [
        new PaymentAmountOption(50, `USD $50`),
        new PaymentAmountOption(100, `USD $100`),
        new PaymentAmountOption(200, `USD $200`),
        new PaymentAmountOption(500, `USD $500`),
        new PaymentAmountOption(1000, `USD $1000`),
    ];

    /**
     * current selected payment option from default ones.
     */
    public current: PaymentAmountOption = this.paymentOptions[0];
    public customAmount: string = '';
    /**
     * Indicates if custom amount selection state is active.
     */
    public isCustomAmount = false;

    /**
     * Indicates if concrete payment option is currently selected.
     */
    public isOptionSelected(option: PaymentAmountOption): boolean {
        return (option.value === this.current.value) && !this.isCustomAmount;
    }

    /**
     * Indicates if dropdown expands top.
     */
    public get isExpandingTop(): boolean {
        return this.$store.state.paymentsModule.billingHistory.length === 0;
    }

    /**
     * Returns payment options depending on user having his own project.
     */
    public get options(): PaymentAmountOption[] {
        if (!new ProjectOwning(this.$store).userHasOwnProject() && this.noCreditCards) {
            return this.initialPaymentOptions;
        }

        return this.paymentOptions;
    }

    /**
     * isSelectionShown flag that indicate is token amount selection shown.
     */
    public get isSelectionShown(): boolean {
        return this.$store.state.appStateModule.appState.isPaymentSelectionShown;
    }

    /**
     * opens token amount selection.
     */
    public open(): void {
        setTimeout(() => this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_PAYMENT_SELECTION, true), 0);
    }

    /**
     * closes token amount selection.
     */
    public close(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_PAYMENT_SELECTION, false);
    }

    /**
     * onCustomAmountChange input event handle that emits value to parent component.
     */
    public onCustomAmountChange(): void {
        this.$emit('onChangeTokenValue', parseInt(this.customAmount, 10));
    }

    /**
     * Sets view state to custom amount selection.
     */
    public openCustomAmountSelection(): void {
        this.isCustomAmount = true;
        this.close();
        this.$emit('onChangeTokenValue', 0);
    }

    /**
     * Sets view state to default.
     */
    public closeCustomAmountSelection(): void {
        this.open();
        this.$emit('onChangeTokenValue', this.current.value);
    }

    /**
     * select standard value from list and emits it value to parent component.
     */
    public select(option: PaymentAmountOption): void {
        this.isCustomAmount = false;
        this.current = option;
        this.$emit('onChangeTokenValue', option.value);
        this.close();
    }

    /**
     * Indicates if user has no credit cards.
     */
    private get noCreditCards(): boolean {
        return this.$store.state.paymentsModule.creditCards.length === 0;
    }
}
</script>

<style scoped lang="scss">
    .custom-input {
        width: 200px;
        height: 48px;
        border: 1px solid #afb7c1;
        border-radius: 8px;
        background-color: transparent;
        padding: 0 36px 0 25px;
        font-family: 'font_medium', sans-serif;
        font-size: 16px;
        line-height: 19px;
        color: #354049;
        -moz-appearance: textfield;
    }

    .custom-input::-webkit-inner-spin-button,
    .custom-input::-webkit-outer-spin-button {
        -webkit-appearance: none;
        margin: 0;
    }

    .form-container {
        position: relative;
    }

    .label {
        position: relative;
        height: 21px;

        &__sign {
            position: absolute;
            top: 50%;
            left: 15px;
            transform: translate(0, -50%);
            font-family: 'font_bold', sans-serif;
            font-size: 16px;
            line-height: 19px;
            color: #354049;
            margin: 0;
        }
    }

    .input-svg {
        position: absolute;
        top: 49%;
        right: 20px;
        transform: translate(0, -50%);
        cursor: pointer;
        width: 25px;
        height: 25px;
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .selected-container {
        position: relative;
        width: 256px;
        height: 48px;
        border: 1px solid #afb7c1;
        border-radius: 8px;
        background-color: transparent;
        display: flex;
        align-items: center;

        &__label-container {
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 0 25px;
            width: calc(100% - 40px);
            height: 100%;

            &__label {
                font-family: 'font_medium', sans-serif;
                font-size: 16px;
                line-height: 28px;
                color: #354049;
                margin: 0;
            }

            &__svg {
                cursor: pointer;
                min-height: 25px;
            }
        }
    }

    .options-container {
        width: 256px;
        position: absolute;
        height: auto;
        font-family: 'font_medium', sans-serif;
        font-size: 16px;
        line-height: 48px;
        color: #354049;
        background-color: white;
        z-index: 102;
        border-radius: 12px;
        top: 50px;
        box-shadow: 0 4px 4px rgba(0, 0, 0, 0.25);

        &__custom-container {
            display: flex;
            align-items: center;
            justify-content: flex-start;
            border-bottom-left-radius: 12px;
            border-bottom-right-radius: 12px;
            padding: 0 30px;
            cursor: pointer;

            &:hover {
                background-color: #f2f2f6;
            }
        }

        &__item {
            display: flex;
            align-items: center;
            padding: 0 30px;
            cursor: pointer;

            &__svg {
                cursor: pointer;
                margin-right: 10px;
            }

            &__label {
                margin: 0;
            }

            &:first-of-type {
                border-top-left-radius: 12px;
                border-top-right-radius: 12px;
            }

            &:hover {
                background-color: #f2f2f6;
            }

            &.selected {
                background-color: red;
            }
        }
    }

    .payment-selection-blur {
        width: 258px;
        height: 50px;
        position: absolute;
        top: 0;
        left: 0;
    }

    .top-expand {
        top: -290px;
        box-shadow: 0 -1px 2px rgba(0, 0, 0, 0.25);
    }
</style>
