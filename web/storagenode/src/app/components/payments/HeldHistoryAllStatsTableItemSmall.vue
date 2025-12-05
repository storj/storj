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
                    <p class="held-history-table-container--small__item__held-info__item__label">First Contact</p>
                    <p class="held-history-table-container--small__item__held-info__item__value">{{ heldHistoryItem.joinedAt.toISOString().split('T')[0] }}</p>
                </div>
                <div class="held-history-table-container--small__item__held-info__item">
                    <p class="held-history-table-container--small__item__held-info__item__label">Held Total</p>
                    <p class="held-history-table-container--small__item__held-info__item__value">{{ centsToDollars(heldHistoryItem.totalHeld) }}</p>
                </div>
                <div class="held-history-table-container--small__item__held-info__item">
                    <p class="held-history-table-container--small__item__held-info__item__label">Held Returned</p>
                    <p class="held-history-table-container--small__item__held-info__item__value">{{ centsToDollars(heldHistoryItem.totalDisposed) }}</p>
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

// Deliberately not scoped to allow styles to cascade to other components.
<style lang="scss">
.held-history-table-container--small__item {
    padding: 12px;
    width: calc(100% - 24px);

    &__satellite-info {
        display: flex;
        align-items: center;
        justify-content: space-between;

        &__name {
            font-family: 'font_regular', sans-serif;
            font-size: 14px;
            color: var(--regular-text-color);
            max-width: calc(100% - 40px);
            overflow-wrap: anywhere;
        }

        &__months {
            font-family: 'font_regular', sans-serif;
            font-size: 11px;
            color: #9b9db1;
            margin-top: 3px;
        }

        &__button {
            display: flex;
            align-items: center;
            justify-content: center;
            width: 30px;
            height: 30px;
            min-width: 30px;
            min-height: 30px;
            background: var(--expand-button-background-color);
            border-radius: 3px;
            cursor: pointer;
        }
    }

    &__held-info {
        margin-top: 16px;

        &__item {
            display: flex;
            align-items: center;
            justify-content: space-between;
            font-size: 12px;
            line-height: 12px;
            margin-bottom: 10px;

            &__label {
                font-family: 'font_medium', sans-serif;
                color: #909bad;
            }

            &__value {
                font-family: 'font_regular', sans-serif;
                color: var(--regular-text-color);
            }
        }
    }
}

.table-icon {
    display: flex;
    align-items: center;
    justify-content: center;
    max-width: 100%;
    max-height: 100%;
    width: 100%;
    height: 100%;
}

.fade-enter-active,
.fade-leave-active {
    transition: opacity 0.5s;
}

.fade-enter,
.fade-leave-to /* .fade-leave-active below version 2.1.8 */ {
    opacity: 0;
}
</style>
