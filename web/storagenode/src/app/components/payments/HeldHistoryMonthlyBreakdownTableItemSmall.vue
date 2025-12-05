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
                <div v-if="isExpanded" class="table-icon hide" @click="hide">
                    <blue-hide-icon />
                </div>
                <div v-else class="table-icon expand" @click="expand">
                    <blue-expand-icon />
                </div>
            </div>
        </div>
        <transition name="fade">
            <div v-if="isExpanded" class="held-history-table-container--small__item__held-info">
                <div class="held-history-table-container--small__item__held-info__item">
                    <p class="held-history-table-container--small__item__held-info__item__label">Month 1-3</p>
                    <p class="held-history-table-container--small__item__held-info__item__value">{{ centsToDollars(heldHistoryItem.holdForFirstPeriod) }}</p>
                </div>
                <div class="held-history-table-container--small__item__held-info__item">
                    <p class="held-history-table-container--small__item__held-info__item__label">Month 4-6</p>
                    <p class="held-history-table-container--small__item__held-info__item__value">{{ centsToDollars(heldHistoryItem.holdForSecondPeriod) }}</p>
                </div>
                <div class="held-history-table-container--small__item__held-info__item">
                    <p class="held-history-table-container--small__item__held-info__item__label">Month 7-9</p>
                    <p class="held-history-table-container--small__item__held-info__item__value">{{ centsToDollars(heldHistoryItem.holdForThirdPeriod) }}</p>
                </div>
            </div>
        </transition>
    </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';

import { SatelliteHeldHistory } from '@/storagenode/payouts/payouts';
import { centsToDollars } from '@/app/utils/payout';

import BlueHideIcon from '@/../static/images/common/BlueMinus.svg';
import BlueExpandIcon from '@/../static/images/common/BluePlus.svg';

withDefaults(defineProps<{
    heldHistoryItem?: SatelliteHeldHistory
}>(), {
    heldHistoryItem: () => new SatelliteHeldHistory(),
});

const isExpanded = ref<boolean>(false);

function expand(): void {
    isExpanded.value = true;
}

function hide(): void {
    isExpanded.value = false;
}
</script>
