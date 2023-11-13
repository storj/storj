// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <th @click="sortFunction('date')">
        <div class="th-content">
            <span>DATE</span>
            <VerticalArrows
                :is-active="arrowController.date"
                :direction="dateSortDirection"
            />
        </div>
    </th>
    <th>
        <div class="th-content">
            <span>TRANSACTION</span>
        </div>
    </th>
    <th @click="sortFunction('amount')">
        <div class="th-content">
            <span>AMOUNT(USD)</span>
            <VerticalArrows
                :is-active="arrowController.amount"
                :direction="amountSortDirection"
            />
        </div>
    </th>
    <th @click="sortFunction('status')">
        <div class="th-content">
            <span>STATUS</span>
            <VerticalArrows
                :is-active="arrowController.status"
                :direction="statusSortDirection"
            />
        </div>
    </th>
    <th class="laptop">
        <div class="th-content">
            <span>DETAILS</span>
        </div>
    </th>
</template>

<script setup lang="ts">
import { ref } from 'vue';

import { SortDirection } from '@/types/common';

import VerticalArrows from '@/components/common/VerticalArrows.vue';

const dateSorting = ref<string>('date-descending');
const amountSorting = ref<string>('amount-descending');
const statusSorting = ref<string>('status-descending');

const emit = defineEmits(['sortFunction']);

const dateSortDirection = ref<SortDirection>(SortDirection.ASCENDING);
const amountSortDirection = ref<SortDirection>(SortDirection.ASCENDING);
const statusSortDirection = ref<SortDirection>(SortDirection.ASCENDING);
const arrowController = ref<{date: boolean, amount: boolean, status: boolean}>({ date: false, amount: false, status: false });

/**
 * sorts table by date
 */
function sortFunction(key): void {
    switch (key) {
    case 'date':
        emit('sortFunction', dateSorting.value);
        dateSorting.value = dateSorting.value === 'date-ascending' ? 'date-descending' : 'date-ascending';
        arrowController.value = { date: true, amount: false, status: false };
        dateSortDirection.value = dateSortDirection.value === SortDirection.DESCENDING ? SortDirection.ASCENDING : SortDirection.DESCENDING;
        break;
    case 'amount':
        emit('sortFunction', amountSorting.value);
        amountSorting.value = amountSorting.value === 'amount-ascending' ? 'amount-descending' : 'amount-ascending';
        arrowController.value = { date: false, amount: true, status: false };
        amountSortDirection.value = amountSortDirection.value === SortDirection.DESCENDING ? SortDirection.ASCENDING : SortDirection.DESCENDING;
        break;
    case 'status':
        emit('sortFunction', statusSorting.value);
        statusSorting.value = statusSorting.value === 'status-ascending' ? 'status-descending' : 'status-ascending';
        arrowController.value = { date: false, amount: false, status: true };
        statusSortDirection.value = statusSortDirection.value === SortDirection.DESCENDING ? SortDirection.ASCENDING : SortDirection.DESCENDING;
        break;
    default:
        break;
    }
}
</script>

<style scoped lang="scss">
.th-content {
    display: flex;
    text-align: left;
}

@media screen and (width <= 1024px) and (width >= 426px) {

    .laptop {
        display: none;
    }
}
</style>
