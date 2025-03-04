// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-text-field
        v-model="search"
        label="Search"
        :prepend-inner-icon="Search"
        single-line
        variant="solo-filled"
        flat
        hide-details
        clearable
        density="comfortable"
        class="mb-4"
    />
    <v-data-table
        :sort-by="sortBy"
        :loading="isLoading"
        :items-length="nativePaymentHistoryItems.length"
        :headers="headers"
        :items="nativePaymentHistoryItems"
        :items-per-page-options="tableSizeOptions(nativePaymentHistoryItems.length)"
        :search="search"
        :custom-key-sort="customSortFns"
        no-data-text="No results found"
    >
        <template #item.timestamp="{ item }">
            <p class="font-weight-bold">
                {{ Time.formattedDate(item.timestamp) }}
            </p>
            <p>
                {{ item.timestamp.toLocaleTimeString('en-US', { hour: 'numeric', minute: 'numeric' }) }}
            </p>
        </template>
        <template #item.type="{ item }">
            <p class="font-weight-bold">
                {{ item.type }}
            </p>
        </template>
        <template #item.amount="{ item }">
            <p class="font-weight-bold text-success">
                {{ item.amount }}
            </p>
        </template>
        <template #item.status="{ item }">
            <v-chip :color="getColor(item.status)" variant="tonal" size="small" class="font-weight-bold">
                {{ item.status }}
            </v-chip>
        </template>
        <template #item.link="{ item }">
            <a v-if="!item.type.includes('bonus')" :href="item.link" target="_blank" rel="noopener noreferrer" class="link">View</a>
        </template>
    </v-data-table>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import {
    VChip,
    VTextField,
    VDataTable,
} from 'vuetify/components';
import { Search } from 'lucide-vue-next';

import { Time } from '@/utils/time';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { useBillingStore } from '@/store/modules/billingStore';
import { DataTableHeader, SortItem, tableSizeOptions } from '@/types/common';

type DisplayedItem = {
    id: string;
    type: string;
    amount: string;
    status: string;
    link: string;
    timestamp: Date;
};

const { isLoading, withLoading } = useLoading();
const notify = useNotify();
const billingStore = useBillingStore();

const sortBy = ref<SortItem[]>([{ key: 'timestamp', order: 'desc' }]);
const search = ref<string>('');

// eslint-disable-next-line @typescript-eslint/no-explicit-any
type DataTableCompareFunction<T = any> = (a: T, b: T) => number;
const customSortFns: Record<string, DataTableCompareFunction> = {
    timestamp: customTimestampSort,
    amount: customAmountSort,
};

const headers: DataTableHeader[] = [
    {
        title: 'Date',
        align: 'start',
        key: 'timestamp',
    },
    { title: 'Transaction', key: 'type', sortable: false },
    { title: 'Amount(USD)', key: 'amount' },
    { title: 'Status', key: 'status' },
    { title: 'Details', key: 'link', sortable: false },
];

/**
 * Returns deposit history items.
 */
const nativePaymentHistoryItems = computed((): DisplayedItem[] => {
    // We concatenate and format some values only to make sorting and search work correctly.
    return billingStore.state.nativePaymentsHistory.map(p => {
        return {
            id: p.id,
            type: `STORJ ${p.type.charAt(0).toUpperCase()}${p.type.slice(1)}`,
            amount: `+ ${p.formattedAmount}`,
            status: `${p.status.charAt(0).toUpperCase()}${p.status.slice(1)}`,
            link: p.link,
            timestamp: p.timestamp,
        };
    });
});

/**
 * Sets chip color based on status value.
 * @param status
 */
function getColor(status: string): string {
    if (status === 'Confirmed' || status === 'Completed' || status === 'Complete') return 'success';
    if (status === 'Pending') return 'warning';
    return 'error';
}

/**
 * Fetches token transaction history.
 */
async function fetchHistory(): Promise<void> {
    await withLoading(async () => {
        try {
            await billingStore.getNativePaymentsHistory();
        } catch (error) {
            notify.notifyError(error.message, AnalyticsErrorEventSource.BILLING_PAYMENT_METHODS_TAB);
        }
    });
}

/**
 * Custom sorting function for timestamp column.
 * @param a timestamp value of type Date
 * @param b timestamp value of type Date
 */
function customTimestampSort(a: Date, b: Date): number {
    return a.getTime() - b.getTime();
}

/**
 * Custom sorting function for amount column.
 * @param a formatted string of amount value
 * @param b formatted string of amount value
 */
function customAmountSort(a: string, b: string): number {
    const valueA = parseFloat(a.substring(3));
    const valueB = parseFloat(b.substring(3));
    return valueA - valueB;
}

onMounted(() => {
    fetchHistory();
});
</script>
