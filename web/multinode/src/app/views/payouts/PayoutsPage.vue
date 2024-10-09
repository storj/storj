// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payouts">
        <h1 class="payouts__title">Payouts</h1>
        <div class="payouts__content-area">
            <div class="payouts__left-area">
                <div class="payouts__left-area__dropdowns">
                    <satellite-selection-dropdown />
                    <payout-period-calendar-button :period="period" />
                </div>
                <payouts-summary-table
                    v-if="payouts.summary.nodeSummary"
                    class="payouts__left-area__table"
                    :node-payouts-summary="payouts.summary.nodeSummary"
                />
            </div>
            <div class="payouts__right-area">
                <details-area
                    :total-earned="payouts.summary.totalEarned"
                    :total-held="payouts.summary.totalHeld"
                    :total-paid="payouts.summary.totalPaid"
                    :period="period"
                />
                <balance-area
                    :current-month-estimation="payouts.totalExpectations.currentMonthEstimation"
                    :undistributed="payouts.totalExpectations.undistributed"
                />
                <!--                <payout-history-block />-->
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { UnauthorizedError } from '@/api';
import { PayoutsState } from '@/app/store/payouts';
import { Notify } from '@/app/plugins';

import SatelliteSelectionDropdown from '@/app/components/common/SatelliteSelectionDropdown.vue';
import BalanceArea from '@/app/components/payouts/BalanceArea.vue';
import DetailsArea from '@/app/components/payouts/DetailsArea.vue';
import PayoutPeriodCalendarButton from '@/app/components/payouts/PayoutPeriodCalendarButton.vue';
import PayoutsSummaryTable from '@/app/components/payouts/tables/payoutSummary/PayoutsSummaryTable.vue';

// @vue/component
@Component({
    components: {
        BalanceArea,
        PayoutPeriodCalendarButton,
        DetailsArea,
        PayoutsSummaryTable,
        SatelliteSelectionDropdown,
    },
})
export default class PayoutsPage extends Vue {

    public notify = new Notify();

    public async mounted(): Promise<void> {
        try {
            await this.$store.dispatch('payouts/summary');
        } catch (error: any) {
            if (error instanceof UnauthorizedError) {
                // TODO: redirect to login screen.
            }

            this.notify.error({ message: error.message, title: error.name });

        }

        try {
            await this.$store.dispatch('payouts/expectations');
        } catch (error: any) {
            if (error instanceof UnauthorizedError) {
                // TODO: redirect to login screen.
            }

            this.notify.error({ message: error.message, title: error.name });

        }
    }

    /**
     * payoutsSummary contains payouts state from store.
     */
    public get payouts(): PayoutsState {
        return this.$store.state.payouts;
    }

    /**
     * period selected payout period from store.
     */
    public get period(): string {
        return this.$store.getters['payouts/periodString'];
    }
}
</script>

<style lang="scss" scoped>
    .payouts {
        box-sizing: border-box;
        padding: 60px;
        overflow-y: auto;
        height: calc(100vh - 60px);

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 32px;
            color: var(--c-title);
            margin-bottom: 36px;
        }

        &__content-area {
            display: flex;
            align-items: flex-start;
            justify-content: space-between;
            width: 100%;
        }

        &__left-area {
            width: 65%;
            margin-right: 32px;

            &__dropdowns {
                display: flex;
                align-items: center;
                justify-content: flex-start;

                & > *:first-of-type {
                    margin-right: 20px;
                }
            }

            &__table {
                margin-top: 20px;
            }
        }

        &__right-area {
            width: 35%;

            & > *:not(:first-of-type) {
                margin-top: 20px;
            }
        }
    }
</style>
