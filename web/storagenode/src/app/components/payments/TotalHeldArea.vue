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
                <p class="total-held-area__united-info-area__item__amount">{{ totalHeldAndPaid.held | centsToDollars }}</p>
            </div>
            <div class="total-held-area__united-info-area__item align-end">
                <p class="total-held-area__united-info-area__item__label">Total Held Returned</p>
                <p class="total-held-area__united-info-area__item__amount">{{ totalHeldAndPaid.disposed | centsToDollars }}</p>
            </div>
        </div>
        <div class="total-held-area__info-area">
            <SingleInfo width="100%" label="Held Amount Rate" :value="heldPercentage + '%'" />
            <SingleInfo width="100%" label="Total Held Amount" :value="totalHeldAndPaid.held | centsToDollars" />
            <SingleInfo width="100%" label="Total Held Returned" :value="totalHeldAndPaid.disposed | centsToDollars" />
        </div>
    </section>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import SingleInfo from '@/app/components/payments/SingleInfo.vue';

import { TotalHeldAndPaid } from '@/storagenode/payouts/payouts';

@Component({
    components: {
        SingleInfo,
    },
})
export default class TotalPayoutArea extends Vue {
    public get totalHeldAndPaid(): TotalHeldAndPaid {
        return this.$store.state.payoutModule.totalHeldAndPaid;
    }

    public get heldPercentage(): string {
        return this.$store.state.payoutModule.heldPercentage;
    }
}
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

    @media screen and (max-width: 780px) {

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
