// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog v-model="model" transition="fade-transition" max-width="1000" scrollable>
        <v-card rounded="xlg" title="STORJ Transactions" subtitle="Deposits and bonuses for this account">
            <template #append>
                <v-btn :icon="X" variant="text" size="small" color="default" @click="model = false" />
            </template>

            <v-divider />

            <v-data-table
                :sort-by="sortBy"
                :headers="headers"
                :items="rows"
                :search="search"
                :loading="isLoading"
                class="border-0"
                item-key="id"
                density="comfortable"
                hover
            >
                <template #top>
                    <v-text-field
                        v-model="search"
                        label="Search"
                        :prepend-inner-icon="Search"
                        single-line
                        variant="solo-filled"
                        flat
                        hide-details
                        clearable
                        density="compact"
                        rounded="lg"
                        class="mx-2 mt-2 mb-2"
                    />
                </template>

                <template #item.timestamp="{ item }">
                    <span class="text-no-wrap">
                        {{ dateFns.format(item.timestamp, 'fullDateTime') }}
                    </span>
                </template>

                <template #item.type="{ item }">
                    <v-chip variant="tonal" color="primary" size="small" rounded="lg">
                        {{ transactionTypeLabel(item.type) }}
                    </v-chip>
                </template>

                <template #item.usd="{ item }">
                    <span class="font-weight-bold text-no-wrap">
                        {{ formatUSD(item.usd) }}
                        <v-tooltip v-if="item.usdExact" activator="parent" location="top">
                            {{ item.usdExact }}
                        </v-tooltip>
                    </span>
                </template>

                <template #item.status="{ item }">
                    <v-chip :color="statusColor(item.status)" variant="tonal" size="small" rounded="lg">
                        {{ capitalize(item.status) }}
                    </v-chip>
                </template>

                <template #item.link="{ item }">
                    <a
                        v-if="item.link"
                        :href="item.link"
                        target="_blank"
                        rel="noopener noreferrer"
                        class="link"
                    >View</a>
                    <span v-else class="text-disabled">-</span>
                </template>

                <template #no-data>
                    <div class="text-center py-8">
                        <v-icon :icon="Coins" size="48" class="text-disabled mb-4" />
                        <p class="text-h6 text-disabled">No transactions found</p>
                        <p class="text-body-2 text-disabled">
                            This user has no STORJ token transactions yet.
                        </p>
                    </div>
                </template>
            </v-data-table>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { VBtn, VCard, VChip, VDataTable, VDialog, VDivider, VIcon, VTextField, VTooltip } from 'vuetify/components';
import { Coins, Search, X } from 'lucide-vue-next';
import { useDate } from 'vuetify';

import { TokenTransaction } from '@/api/client.gen';
import { DataTableHeader, SortItem } from '@/types/common';
import { useBillingStore } from '@/store/billing';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { formatPrice } from '@/utils/strings';

const model = defineModel<boolean>({ required: true });

const props = defineProps<{
    userId: string;
}>();

const dateFns = useDate();
const billingStore = useBillingStore();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const search = ref<string>('');
const transactions = ref<TokenTransaction[]>([]);

type TransactionRow = TokenTransaction & {
    // usd is the amount the row displays, as a number, so the column sorts numerically and on
    // the same field it renders.
    usd: number;
    // usdExact is the unrounded amount, set only when rounding to cents loses precision.
    usdExact: string;
};

/**
 * Returns the transactions with their displayed amount resolved.
 *
 * Coinpayments deposits may be partially paid, so what was actually received is the meaningful
 * figure for them.
 */
const rows = computed<TransactionRow[]>(() => transactions.value.map(tx => {
    const raw = tx.type === 'coinpayments' ? tx.received : tx.amount;
    const usd = parseFloat(raw) || 0;

    return {
        ...tx,
        usd,
        // Compare numerically: the API trims trailing zeros, so "12.5" and "12.50" are equal.
        usdExact: parseFloat(usd.toFixed(2)) === usd ? '' : formatPrice(raw),
    };
}));

// Timestamps are ISO 8601 strings, so the default string sort is already chronological.
const sortBy = ref<SortItem[]>([{ key: 'timestamp', order: 'desc' }]);

const headers = computed<DataTableHeader[]>(() => [
    { title: 'Date', key: 'timestamp', align: 'start', sortable: true },
    { title: 'Transaction', key: 'type', sortable: true },
    { title: 'Amount (USD)', key: 'usd', sortable: true },
    { title: 'Status', key: 'status', sortable: true },
    { title: 'Details', key: 'link', sortable: false },
]);

function capitalize(value: string): string {
    if (!value) return '';
    return value.charAt(0).toUpperCase() + value.slice(1);
}

/**
 * Returns a human readable label for a transaction source.
 */
function transactionTypeLabel(type: string): string {
    switch (type) {
    case 'storjscan': return 'STORJ Deposit';
    case 'storjscanbonus': return 'STORJ Bonus';
    case 'coinpayments': return 'STORJ Coinpayments';
    default: return `STORJ ${capitalize(type)}`;
    }
}

/**
 * Formats a USD amount, rounded to cents like the customer console does.
 *
 * On-chain amounts come from the API as USDollarsMicro and can carry up to six decimals.
 */
function formatUSD(usd: number): string {
    return formatPrice(usd.toFixed(2));
}

/**
 * Returns the chip color for a transaction status.
 *
 * The statuses reachable here are "confirmed"/"pending" for on-chain payments,
 * "complete"/"pending"/"failed" for bonus credits, and "pending"/"paid"/"completed"/"cancelled"
 * for coinpayments deposits.
 */
function statusColor(status: string): string {
    switch (status) {
    case 'confirmed':
    case 'complete':
    case 'completed':
    case 'paid':
        return 'success';
    case 'pending':
        return 'warning';
    case 'cancelled':
    case 'failed':
        return 'error';
    default:
        return 'default';
    }
}

async function fetchTransactions(): Promise<void> {
    await withLoading(async () => {
        try {
            transactions.value = await billingStore.getTokenTransactions(props.userId);
        } catch (error) {
            notify.error(`Failed to get STORJ token transactions. ${error.message}`);
        }
    });
}

watch([model, () => props.userId], ([shown]) => {
    if (!shown) return;
    search.value = '';
    fetchTransactions();
}, { immediate: true });
</script>
