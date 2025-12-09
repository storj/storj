// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <base-table v-if="nodePayoutsSummary.length">
        <template #head>
            <thead>
                <tr>
                    <th class="align-left" @click="sortBy('nodeName')">NODE{{ sortByKey === 'nodeName' ? sortArrow : '' }}</th>
                    <th @click="sortBy('held')">HELD{{ sortByKey === 'held' ? sortArrow : '' }}</th>
                    <th @click="sortBy('paid')">PAID{{ sortByKey === 'paid' ? sortArrow : '' }}</th>
                    <th class="options" />
                </tr>
            </thead>
        </template>
        <template #body>
            <tbody>
                <payouts-summary-item v-for="payoutSummary in sortedNodePayoutsSummary" :key="payoutSummary.nodeId" :payouts-summary="payoutSummary" />
            </tbody>
        </template>
    </base-table>
</template>

<script setup lang="ts">
import { computed, onBeforeMount, ref } from 'vue';

import { NodePayoutsSummary } from '@/payouts';

import BaseTable from '@/app/components/common/BaseTable.vue';
import PayoutsSummaryItem from '@/app/components/payouts/tables/payoutSummary/PayoutsSummaryItem.vue';

const props = withDefaults(defineProps<{
    nodePayoutsSummary?: NodePayoutsSummary[];
}>(), {
    nodePayoutsSummary: () => [],
});

const sortByKey = ref<string>('');
const sortDirection = ref<string>('asc');

const sortArrow = computed<string>(() => sortDirection.value === 'asc' ? ' ↑' : ' ↓');
const sortedNodePayoutsSummary = computed<NodePayoutsSummary[]>(() => {
    const key = sortByKey.value;
    const direction = sortDirection.value === 'asc' ? 1 : -1;
    if (key === '') return props.nodePayoutsSummary;
    return props.nodePayoutsSummary.slice().sort((a, b) => {
        if (a[key] < b[key]) return -direction;
        if (a[key] > b[key]) return direction;
        return 0;
    });
});

function sortBy(key: string) {
    if (sortByKey.value === key) {
        if (sortDirection.value === 'asc') {
            sortDirection.value = 'desc';
        } else {
            // Disable sorting after three clicks (flow: asc -> desc -> disable -> asc -> ...)
            sortByKey.value = '';
        }
    } else {
        sortByKey.value = key;
        sortDirection.value = 'asc';
    }

    localStorage.setItem('payoutSortByKey', sortByKey.value);
    localStorage.setItem('payoutSortDirection', sortDirection.value);
}

onBeforeMount(() => {
    const savedSortByKey = localStorage.getItem('payoutSortByKey');
    const savedSortDirection = localStorage.getItem('payoutSortDirection');
    if (savedSortByKey) {
        sortByKey.value = savedSortByKey;
    }
    if (savedSortDirection) {
        sortDirection.value = savedSortDirection;
    }
});
</script>

<style scoped lang="scss">
    th {
        user-select: none; /* Diable user selecting the headers for sort selection */
    }
</style>
