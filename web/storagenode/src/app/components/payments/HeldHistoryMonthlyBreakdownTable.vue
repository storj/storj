// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div>
        <div class="held-history-table-container--large">
            <div class="held-history-table-container--large__labels-area">
                <div class="column justify-start column-1">
                    <p class="held-history-table-container--large__labels-area__text">Satellite</p>
                </div>
                <div class="column justify-end column-2">
                    <p class="held-history-table-container--large__labels-area__text">Month 1-3</p>
                </div>
                <div class="column justify-end column-3">
                    <p class="held-history-table-container--large__labels-area__text">Month 4-6</p>
                </div>
                <div class="column justify-end column-4">
                    <p class="held-history-table-container--large__labels-area__text">Month 7-9</p>
                </div>
            </div>
            <div v-for="item in monthlyBreakdown" class="held-history-table-container--large__info-area" :key="item.satelliteID">
                <div class="justify-start column-1">
                    <p class="held-history-table-container--large__info-area__text">{{ item.satelliteName }}</p>
                    <p class="held-history-table-container--large__info-area__months">{{ item.age }} month</p>
                </div>
                <div class="column justify-end column-2">
                    <p class="held-history-table-container--large__info-area__text">{{ item.firstPeriod | centsToDollars }}</p>
                </div>
                <div class="column justify-end column-3">
                    <p class="held-history-table-container--large__info-area__text">{{ item.secondPeriod | centsToDollars }}</p>
                </div>
                <div class="column justify-end column-4">
                    <p class="held-history-table-container--large__info-area__text">{{ item.thirdPeriod | centsToDollars }}</p>
                </div>
            </div>
        </div>
        <div class="held-history-table-container--small">
            <HeldHistoryMonthlyBreakdownTableSmall
                v-for="item in monthlyBreakdown"
                :held-history-item="item"
                :key="item.satelliteID"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import HeldHistoryMonthlyBreakdownTableSmall from '@/app/components/payments/HeldHistoryMonthlyBreakdownTableSmall.vue';

import { HeldHistoryMonthlyBreakdownItem } from '@/app/types/payout';

@Component({
    components: {
        HeldHistoryMonthlyBreakdownTableSmall,
    },
})
export default class HeldHistoryMonthlyBreakdownTable extends Vue {
    /**
     * Returns list of satellite held history items by periods from store.
     */
    public get monthlyBreakdown(): HeldHistoryMonthlyBreakdownItem[] {
        return this.$store.state.payoutModule.heldHistory.monthlyBreakdown;
    }
}
</script>

<style scoped lang="scss">
    .held-history-table-container--large {

        &__labels-area {
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: space-between;
            margin-top: 17px;
            padding: 0 16px;
            width: calc(100% - 32px);
            height: 36px;
            background: var(--table-header-color);

            &__text {
                font-family: 'font_medium', sans-serif;
                font-size: 14px;
                color: #909bad;
            }
        }

        &__info-area {
            padding: 11px 16px;
            display: flex;
            flex-direction: row;
            align-items: center;
            justify-content: space-between;
            min-height: 34px;
            height: auto;
            border-bottom: 1px solid rgba(169, 181, 193, 0.3);

            &:last-of-type {
                border-bottom: none;
            }

            &__text {
                font-family: 'font_regular', sans-serif;
                font-size: 14px;
                color: var(--regular-text-color);
                max-width: 100%;
                word-break: break-word;
            }

            &__months {
                font-family: 'font_regular', sans-serif;
                font-size: 11px;
                color: #9b9db1;
                margin-top: 3px;
            }
        }
    }

    .held-history-table-container--small {
        display: none;
    }

    .column {
        display: flex;
        flex-direction: row;
        align-items: center;
    }

    .justify-start {
        justify-content: flex-start;
    }

    .justify-end {
        justify-content: flex-end;
    }

    .column-1 {
        width: 37%;
    }

    .column-2,
    .column-3,
    .column-4 {
        width: 21%;
    }

    @media screen and (max-width: 600px) {

        .held-history-table-container--large {
            display: none;
        }

        .held-history-table-container--small {
            display: block;
        }
    }
</style>
