// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <section class="total-payout-area">
        <div class="total-payout-area__united-info-area">
            <div class="total-payout-area__united-info-area__item">
                <p class="total-payout-area__united-info-area__item__label">Current Month Earnings</p>
                <p class="total-payout-area__united-info-area__item__amount">{{ centsToDollars(currentEarnings) }}</p>
            </div>
            <div class="total-payout-area__united-info-area__item align-center">
                <p class="total-payout-area__united-info-area__item__label">Total Earned</p>
                <p class="total-payout-area__united-info-area__item__amount">{{ centsToDollars(totalEarnings) }}</p>
            </div>
            <div class="total-payout-area__united-info-area__item align-end">
                <p class="total-payout-area__united-info-area__item__label">Total Held Amount</p>
                <p class="total-payout-area__united-info-area__item__amount">{{ centsToDollars(totalHeld) }}</p>
            </div>
        </div>
        <div class="total-payout-area__info-area">
            <SingleInfo width="100%" label="Current Month Earnings" :value="centsToDollars(currentEarnings)" />
            <SingleInfo width="100%" label="Total Earnings" :value="centsToDollars(totalEarnings)" />
            <SingleInfo width="100%" label="Total Held Amount" :value="centsToDollars(totalHeld)" />
        </div>
    </section>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { centsToDollars } from '@/app/utils/payout';
import { usePayoutStore } from '@/app/store/modules/payoutStore';

import SingleInfo from '@/app/components/payments/SingleInfo.vue';

const payoutStore = usePayoutStore();

const totalEarnings = computed(() => {
    return payoutStore.state.totalPayments.paid;
});

const totalHeld = computed(() => {
    return payoutStore.state.totalPayments.held;
});

const currentEarnings = computed(() => {
    return payoutStore.state.currentMonthEarnings;
});
</script>

<style scoped lang="scss">
    .total-payout-area {

        &__united-info-area {
            width: calc(100% - 60px);
            padding: 24px 30px;
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: space-between;
            background: var(--block-background-color);
            border: 1px solid var(--block-border-color);
            border-radius: 10px;

            &__item {
                display: flex;
                flex-direction: column;
                align-items: flex-start;
                color: var(--regular-text-color);

                &__label {
                    font-family: 'font_regular', sans-serif;
                    font-size: 14px;
                    line-height: 20px;
                }

                &__amount {
                    font-family: 'font_medium', sans-serif;
                    font-size: 20px;
                    line-height: 20px;
                    margin-top: 12px;
                }
            }
        }

        &__info-area {
            display: none;
            align-items: center;
            justify-content: space-between;
        }
    }

    .align-center {
        align-items: center;
    }

    .align-end {
        align-items: flex-end;
    }

    @media screen and (width <= 780px) {

        .total-payout-area {

            &__united-info-area {
                display: none;
            }

            &__info-area {
                display: flex;
                flex-direction: column;

                .info-container {
                    width: 100% !important;
                    margin-bottom: 12px;
                }
            }
        }
    }
</style>
