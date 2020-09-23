// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="held-history-table-container--small__item">
        <div class="held-history-table-container--small__item__satellite-info">
            <div>
                <p class="held-history-table-container--small__item__satellite-info__name">{{ heldHistoryItem.satelliteName }}</p>
                <p class="held-history-table-container--small__item__satellite-info__months">{{ heldHistoryItem.monthsWithNode }} month</p>
            </div>
            <div class="held-history-table-container--small__item__satellite-info__button">
                <div class="icon hide" @click="hide" v-if="isExpanded">
                    <blue-hide-icon></blue-hide-icon>
                </div>
                <div class="icon expand" @click="expand" v-else>
                    <blue-expand-icon></blue-expand-icon>
                </div>
            </div>
        </div>
        <transition name="fade">
            <div class="held-history-table-container--small__item__held-info" v-if="isExpanded">
                <div class="held-history-table-container--small__item__held-info__item">
                    <p class="held-history-table-container--small__item__held-info__item__label">Month 1-3</p>
                    <p class="held-history-table-container--small__item__held-info__item__value">{{
                        heldHistoryItem.holdForFirstPeriod | centsToDollars }}</p>
                </div>
                <div class="held-history-table-container--small__item__held-info__item">
                    <p class="held-history-table-container--small__item__held-info__item__label">Month 4-6</p>
                    <p class="held-history-table-container--small__item__held-info__item__value">{{
                        heldHistoryItem.holdForSecondPeriod | centsToDollars }}</p>
                </div>
                <div class="held-history-table-container--small__item__held-info__item">
                    <p class="held-history-table-container--small__item__held-info__item__label">Month 7-9</p>
                    <p class="held-history-table-container--small__item__held-info__item__value">{{
                        heldHistoryItem.holdForThirdPeriod | centsToDollars }}</p>
                </div>
            </div>
        </transition>
    </div>
</template>

<script lang="ts">
import { Component, Prop } from 'vue-property-decorator';

import BaseSmallHeldHistoryTable from '@/app/components/payments/BaseSmallHeldHistoryTable.vue';

import { SatelliteHeldHistory } from '@/storagenode/payouts/payouts';

@Component
export default class HeldHistoryMonthlyBreakdownTableSmall extends BaseSmallHeldHistoryTable {
    @Prop({default: () => new SatelliteHeldHistory()})
    public readonly heldHistoryItem: SatelliteHeldHistory;
}
</script>
