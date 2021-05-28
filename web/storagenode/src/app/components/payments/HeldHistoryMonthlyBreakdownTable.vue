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
            <div v-for="item in allSatellitesHeldHistory" class="held-history-table-container--large__info-area" :key="item.satelliteID">
                <div class="justify-start column-1">
                    <p class="held-history-table-container--large__info-area__text">{{ item.satelliteName }}</p>
                    <p class="held-history-table-container--large__info-area__months">{{ item.monthsWithNode }} month</p>
                </div>
                <div class="column justify-end column-2">
                    <p class="held-history-table-container--large__info-area__text">{{ item.holdForFirstPeriod |
                        centsToDollars }}</p>
                </div>
                <div class="column justify-end column-3">
                    <p class="held-history-table-container--large__info-area__text">{{ item.holdForSecondPeriod |
                        centsToDollars }}</p>
                </div>
                <div class="column justify-end column-4">
                    <p class="held-history-table-container--large__info-area__text">{{ item.holdForThirdPeriod |
                        centsToDollars }}</p>
                </div>
            </div>
        </div>
        <div class="held-history-table-container--small">
            <HeldHistoryMonthlyBreakdownTableItemSmall
                v-for="item in allSatellitesHeldHistory"
                :held-history-item="item"
                :key="item.satelliteID"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import BaseHeldHistoryTable from '@/app/components/payments/BaseHeldHistoryTable.vue';
import HeldHistoryMonthlyBreakdownTableItemSmall from '@/app/components/payments/HeldHistoryMonthlyBreakdownTableItemSmall.vue';

import { SatelliteHeldHistory } from '@/storagenode/payouts/payouts';

@Component({
    components: {
        HeldHistoryMonthlyBreakdownTableItemSmall,
    },
})
export default class HeldHistoryMonthlyBreakdownTable extends BaseHeldHistoryTable {
    /**
     * Returns list of satellite held history items by periods from store.
     */
    public get allSatellitesHeldHistory(): SatelliteHeldHistory[] {
        return this.$store.state.payoutModule.heldHistory;
    }
}
</script>
