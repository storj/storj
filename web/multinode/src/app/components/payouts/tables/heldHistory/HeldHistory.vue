// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <base-table>
        <template #head>
            <thead>
                <tr>
                    <th class="align-left">SATELLITE</th>
                    <th>MONTH 1-3</th>
                    <th>MONTH 4-6</th>
                    <th>MONTH 7-9</th>
                </tr>
            </thead>
        </template>

        <template #body>
            <tbody>
                <tr v-for="(heldHistoryItem, index) in heldHistory" :key="index" class="table-item">
                    <th class="align-left">
                        <p class="table-item__name">{{ heldHistoryItem.satelliteAddress }}</p>
                        <p class="table-item__months">{{ heldHistoryItem.monthsCount }}</p>
                    </th>
                    <th>{{ Currency.dollarsFromCents(heldHistoryItem.firstQuarter) }}</th>
                    <th>{{ Currency.dollarsFromCents(heldHistoryItem.secondQuarter) }}</th>
                    <th>{{ Currency.dollarsFromCents(heldHistoryItem.thirdQuarter) }}</th>
                </tr>
            </tbody>
        </template>
    </base-table>
</template>

<script setup lang="ts">
import { HeldAmountSummary } from '@/payouts';
import { Currency } from '@/app/utils/currency';

import BaseTable from '@/app/components/common/BaseTable.vue';

defineProps<{
    heldHistory: HeldAmountSummary[];
}>();
</script>

<style lang="scss" scoped>
    .table-item {

        &__name {
            font-family: 'font_regular', sans-serif;
            font-size: 14px;
            color: var(--regular-text-color);
            max-width: calc(100% - 40px);
            overflow-wrap: break-word;
        }

        &__months {
            font-family: 'font_regular', sans-serif;
            font-size: 11px;
            color: #9b9db1;
            margin-top: 3px;
        }
    }
</style>
