// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <div class="selected-container" v-if="!isCustomAmount">
            <div id="paymentSelectButton" class="selected-container__label-container" @click="toggleSelection">
                <p class="selected-container__label-container__label">{{current.label}}</p>
                <div class="selected-container__label-container__svg">
                    <svg width="14" height="8" viewBox="0 0 14 8" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <path fill-rule="evenodd" clip-rule="evenodd" d="M0.372773 0.338888C0.869804 -0.112963 1.67565 -0.112963 2.17268 0.338888L7 4.72741L11.8273 0.338888C12.3243 -0.112963 13.1302 -0.112963 13.6272 0.338888C14.1243 0.790739 14.1243 1.52333 13.6272 1.97519L7 8L0.372773 1.97519C-0.124258 1.52333 -0.124258 0.790739 0.372773 0.338888Z" fill="#2683FF"/>
                    </svg>
                </div>
            </div>
            <div id="paymentSelect" class="options-container" v-if="isSelectionShown">
                <div
                    class="options-container__item"
                    v-for="option in paymentOptions"
                    :key="option.label"
                    @click.prevent="select(option)">

                    <div class="options-container__item__svg" v-if="option.value === current.value">
                        <svg width="15" height="13" viewBox="0 0 15 13" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <path d="M14.0928 3.02746C14.6603 2.4239 14.631 1.4746 14.0275 0.907152C13.4239 0.339699 12.4746 0.368972 11.9072 0.972536L14.0928 3.02746ZM4.53846 11L3.44613 12.028C3.72968 12.3293 4.12509 12.5001 4.53884 12.5C4.95258 12.4999 5.34791 12.3289 5.63131 12.0275L4.53846 11ZM3.09234 7.27469C2.52458 6.67141 1.57527 6.64261 0.971991 7.21036C0.36871 7.77812 0.339911 8.72743 0.907664 9.33071L3.09234 7.27469ZM11.9072 0.972536L3.44561 9.97254L5.63131 12.0275L14.0928 3.02746L11.9072 0.972536ZM5.6308 9.97199L3.09234 7.27469L0.907664 9.33071L3.44613 12.028L5.6308 9.97199Z" fill="#2683FF"/>
                        </svg>
                    </div>
                    <p class="options-container__item__label">{{option.label}}</p>
                </div>
                <div class="options-container__custom-container" @click.prevent="toggleCustomAmount">Custom Amount</div>
            </div>
        </div>
        <label class="label" v-if="isCustomAmount">
            <input class="custom-input" type="number" placeholder="Enter Amount" v-model="customAmount">
            <div class="input-svg" @click="toggleCustomAmount">
                <svg width="14" height="8" viewBox="0 0 14 8" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path fill-rule="evenodd" clip-rule="evenodd" d="M0.372773 0.338888C0.869804 -0.112963 1.67565 -0.112963 2.17268 0.338888L7 4.72741L11.8273 0.338888C12.3243 -0.112963 13.1302 -0.112963 13.6272 0.338888C14.1243 0.790739 14.1243 1.52333 13.6272 1.97519L7 8L0.372773 1.97519C-0.124258 1.52333 -0.124258 0.790739 0.372773 0.338888Z" fill="#2683FF"/>
                </svg>
            </div>
        </label>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { PaymentAmountOption } from '@/types/payments';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';

@Component
export default class StorjInput extends Vue {
    public paymentOptions: PaymentAmountOption[] = [
        new PaymentAmountOption(20, `US $20 (+5 Bonus)`),
        new PaymentAmountOption(5, `US $5`),
        new PaymentAmountOption(10, `US $10 (+2 Bonus)`),
        new PaymentAmountOption(100, `US $100 (+20 Bonus)`),
        new PaymentAmountOption(1000, `US $1000 (+200 Bonus)`),
    ];

    public current: PaymentAmountOption = new PaymentAmountOption(20, `US $20 (+$5 Bonus)`);
    public customAmount: number = 0;
    public isCustomAmount = false;

    public get isSelectionShown(): boolean {
        return this.$store.state.appStateModule.appState.isPaymentSelectionShown;
    }

    public toggleSelection(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_PAYMENT_SELECTION);
    }

    public toggleCustomAmount(): void {
        this.isCustomAmount = !this.isCustomAmount;
    }

    public select(value: PaymentAmountOption): void {
        this.current = value;
        this.toggleSelection();
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
        padding: 0 36px 0 20px;
        font-family: font_medium;
        font-size: 16px;
        line-height: 28px;
        color: #354049;
    }

    .custom-input::-webkit-inner-spin-button,
    .custom-input::-webkit-outer-spin-button {
        -webkit-appearance: none;
        margin: 0;
    }

    .label {
        position: relative;
    }

    .input-svg {
        position: absolute;
        top: 50%;
        right: 20px;
        transform: translate(0, -50%);
        cursor: pointer;
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
            padding: 0 20px;
            width: calc(100% - 40px);
            height: 100%;

            &__label {
                font-family: font_medium;
                font-size: 16px;
                line-height: 28px;
                color: #354049;
                margin: 0;
            }

            &__svg {
                cursor: pointer;
            }
        }
    }

    .options-container {
        width: 256px;
        position: absolute;
        height: auto;
        font-family: font_medium;
        font-size: 16px;
        line-height: 48px;
        color: #354049;
        background-color: white;
        z-index: 102;
        margin-right: 10px;
        border-radius: 12px;
        top: calc(100% + 10px);
        box-shadow: 0 4px 4px rgba(0, 0, 0, 0.25);

        &__custom-container {
            border-bottom-left-radius: 12px;
            border-bottom-right-radius: 12px;
            padding: 0 20px;
            cursor: pointer;

            &:hover {
                background-color: #F2F2F6;
            }
        }

        &__item {
            display: flex;
            align-items: center;
            padding: 0 20px;
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
                background-color: #F2F2F6;
            }

            &.selected {
                background-color: red;
            }
        }
    }
</style>
