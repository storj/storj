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
            <div v-for="item in allSatellitesHeldHistory" class="held-history-table-container--large__info-area" :key="item.satelliteID">
                <div class="justify-start column-1">
                    <p class="held-history-table-container--large__info-area__text">{{ item.satelliteName }}</p>
                    <p class="held-history-table-container--large__info-area__months">{{ item.monthsWithNode }} month</p>
                </div>
                <div class="column justify-end column-2">
                    <p class="held-history-table-container--large__info-area__text">{{ item.joinedAt.toISOString().split('T')[0] }}</p>
                </div>
                <div class="column justify-end column-3">
                    <p class="held-history-table-container--large__info-area__text">{{ item.totalHeld | centsToDollars }}</p>
                </div>
                <div class="column justify-end column-4">
                    <p class="held-history-table-container--large__info-area__text">{{ item.totalDisposed | centsToDollars }}</p>
                </div>
            </div>
        </div>
        <div class="held-history-table-container--small">
            <HeldHistoryAllStatsTableItemSmall
                v-for="item in allSatellitesHeldHistory"
                :held-history-item="item"
                :key="item.satelliteID"
            />
        </div>
    </div>
</template>

<script lang="ts">
import { Component } from 'vue-property-decorator';

import BaseHeldHistoryTable from '@/app/components/payments/BaseHeldHistoryTable.vue';
import HeldHistoryAllStatsTableItemSmall from '@/app/components/payments/HeldHistoryAllStatsTableItemSmall.vue';

import { SatelliteHeldHistory } from '@/storagenode/payouts/payouts';

@Component({
    components: {
        HeldHistoryAllStatsTableItemSmall,
    },
})
export default class HeldHistoryAllStatsTable extends BaseHeldHistoryTable {
    /**
     * Returns list of satellite held history items by periods from store.
     */
    public get allSatellitesHeldHistory(): SatelliteHeldHistory[] {
        return this.$store.state.payoutModule.heldHistory;
    }
}
</script>
