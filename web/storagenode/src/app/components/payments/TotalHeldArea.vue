// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <section class="total-held-area">
        <div class="total-held-area__united-info-area">
            <div class="total-held-area__united-info-area__item">
                <p class="total-held-area__united-info-area__item__label">Held Amount Rate</p>
                <p class="total-held-area__united-info-area__item__amount">{{ heldPercentage }}%</p>
            </div>
            <div class="total-held-area__united-info-area__item align-center">
                <p class="total-held-area__united-info-area__item__label">Total Held Amount</p>
                <p class="total-held-area__united-info-area__item__amount">{{ centsToDollars(totalPayments.held) }}</p>
            </div>
            <div class="total-held-area__united-info-area__item align-end">
                <p class="total-held-area__united-info-area__item__label">Total Held Returned</p>
                <p class="total-held-area__united-info-area__item__amount">{{ centsToDollars(totalPayments.disposed) }}</p>
            </div>
        </div>
        <div class="total-held-area__info-area">
            <SingleInfo width="100%" label="Held Amount Rate" :value="heldPercentage + '%'" />
            <SingleInfo width="100%" label="Total Held Amount" :value="centsToDollars(totalPayments.held)" />
            <SingleInfo width="100%" label="Total Held Returned" :value="centsToDollars(totalPayments.disposed)" />
        </div>
    </section>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { centsToDollars } from '@/app/utils/payout';
import { usePayoutStore } from '@/app/store/modules/payoutStore';

import SingleInfo from '@/app/components/payments/SingleInfo.vue';

const payoutStore = usePayoutStore();

const totalPayments = computed(() => {
    return payoutStore.state.totalPayments;
});

const heldPercentage = computed(() => {
    return payoutStore.state.heldPercentage;
});
</script>

<style scoped lang="scss">
    .total-held-area {
        width: 100%;

        &__united-info-area {
            width: calc(100% - 60px);
            padding: 24px 30px;
            display: flex;
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

        .total-held-area {

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
