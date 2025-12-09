// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="payouts">
        <h1 class="payouts__title">Payouts</h1>
        <div class="payouts__content-area">
            <div class="payouts__left-area">
                <div class="payouts__left-area__dropdowns">
                    <satellite-selection-dropdown />
                    <payout-period-calendar-button :period="payoutsStore.periodString" />
                </div>
                <payouts-summary-table
                    v-if="summary.nodeSummary"
                    class="payouts__left-area__table"
                    :node-payouts-summary="summary.nodeSummary"
                />
            </div>
            <div class="payouts__right-area">
                <details-area
                    :total-earned="summary.totalEarned"
                    :total-held="summary.totalHeld"
                    :total-paid="summary.totalPaid"
                    :period="payoutsStore.periodString"
                />
                <balance-area
                    :current-month-estimation="totalExpectations.currentMonthEstimation"
                    :undistributed="totalExpectations.undistributed"
                />
                <!--                <payout-history-block />-->
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue';

import { UnauthorizedError } from '@/api';
import { usePayoutsStore } from '@/app/store/payoutsStore';

import SatelliteSelectionDropdown from '@/app/components/common/SatelliteSelectionDropdown.vue';
import BalanceArea from '@/app/components/payouts/BalanceArea.vue';
import DetailsArea from '@/app/components/payouts/DetailsArea.vue';
import PayoutPeriodCalendarButton from '@/app/components/payouts/PayoutPeriodCalendarButton.vue';
import PayoutsSummaryTable from '@/app/components/payouts/tables/payoutSummary/PayoutsSummaryTable.vue';

const payoutsStore = usePayoutsStore();

const summary = computed(() => payoutsStore.state.summary);
const totalExpectations = computed(() => payoutsStore.state.totalExpectations);

onMounted(async () => {
    try {
        await payoutsStore.summary();
    } catch (error) {
        if (error instanceof UnauthorizedError) {
            // TODO: redirect to login screen.
        }

        // TODO: notify error
    }

    try {
        await payoutsStore.expectations();
    } catch (error) {
        if (error instanceof UnauthorizedError) {
            // TODO: redirect to login screen.
        }

        // TODO: notify error
    }
});
</script>

<style lang="scss" scoped>
    .payouts {
        box-sizing: border-box;
        padding: 60px;
        overflow-y: auto;
        height: calc(100vh - 60px);
        background-color: var(--v-background-base);

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 32px;
            color: var(--v-header-base);
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
