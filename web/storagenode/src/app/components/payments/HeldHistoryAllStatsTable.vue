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
                    <p class="held-history-table-container--large__labels-area__text">First Contact</p>
                </div>
                <div class="column justify-end column-3">
                    <p class="held-history-table-container--large__labels-area__text">Held Total</p>
                </div>
                <div class="column justify-end column-4">
                    <p class="held-history-table-container--large__labels-area__text">Held Returned</p>
                </div>
            </div>
            <div v-for="item in allSatellitesHeldHistory" :key="item.satelliteID" class="held-history-table-container--large__info-area">
                <div class="justify-start column-1">
                    <p class="held-history-table-container--large__info-area__text">{{ item.satelliteName }}</p>
                    <p class="held-history-table-container--large__info-area__months">{{ item.monthsWithNode }} month</p>
                </div>
                <div class="column justify-end column-2">
                    <p class="held-history-table-container--large__info-area__text">{{ item.joinedAt.toISOString().split('T')[0] }}</p>
                </div>
                <div class="column justify-end column-3">
                    <p class="held-history-table-container--large__info-area__text">{{ centsToDollars(item.totalHeld) }}</p>
                </div>
                <div class="column justify-end column-4">
                    <p class="held-history-table-container--large__info-area__text">{{ centsToDollars(item.totalDisposed) }}</p>
                </div>
            </div>
        </div>
        <div class="held-history-table-container--small">
            <HeldHistoryAllStatsTableItemSmall
                v-for="item in allSatellitesHeldHistory"
                :key="item.satelliteID"
                :held-history-item="item"
            />
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

import { SatelliteHeldHistory } from '@/storagenode/payouts/payouts';
import { centsToDollars } from '@/app/utils/payout';
import { usePayoutStore } from '@/app/store/modules/payoutStore';

import HeldHistoryAllStatsTableItemSmall from '@/app/components/payments/HeldHistoryAllStatsTableItemSmall.vue';

const payoutStore = usePayoutStore();

const allSatellitesHeldHistory = computed<SatelliteHeldHistory[]>(() => {
    return payoutStore.state.heldHistory as SatelliteHeldHistory[];
});
</script>

// Deliberately not scoped to allow styles to cascade to other components.
<style lang="scss">
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
        border-bottom: 1px solid rgb(169 181 193 / 30%);

        &:last-of-type {
            border-bottom: none;
        }

        &__text {
            font-family: 'font_regular', sans-serif;
            font-size: 14px;
            color: var(--regular-text-color);
            max-width: 100%;
            overflow-wrap: anywhere;
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

@media screen and (width <= 720px) {

    .column-1 {
        width: 31%;
    }

    .column-2,
    .column-3,
    .column-4 {
        width: 23%;
    }
}

@media screen and (width <= 600px) {

    .held-history-table-container--large {
        display: none;
    }

    .held-history-table-container--small {
        display: block;
    }
}
</style>
