// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="billing-history">
        <h1 class="billing-history__title">
            Billing History
        </h1>

        <v-table
            simple-pagination
            :total-items-count="historyItems.length"
            class="billing-history__table"
            :on-next-clicked="nextClicked"
            :on-previous-clicked="previousClicked"
            :on-page-size-changed="sizeChanged"
        >
            <template #head>
                <BillingHistoryHeader />
            </template>
            <template #body>
                <BillingHistoryItem
                    v-for="item in historyItems"
                    :key="item.id"
                    :item="item"
                />
            </template>
        </v-table>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';

import {
    PaymentsHistoryItem,
    PaymentsHistoryItemStatus,
    PaymentHistoryPage,
} from '@/types/payments';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useBillingStore } from '@/store/modules/billingStore';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';

import BillingHistoryHeader
    from '@/components/account/billing/billingTabs/BillingHistoryHeader.vue';
import BillingHistoryItem
    from '@/components/account/billing/billingTabs/BillingHistoryItem.vue';
import VTable from '@/components/common/VTable.vue';

const billingStore = useBillingStore();
const notify = useNotify();

const limit = ref(DEFAULT_PAGE_LIMIT);

const historyPage = computed((): PaymentHistoryPage => {
    return billingStore.state.paymentsHistory;
});

const historyItems = computed((): PaymentsHistoryItem[] => {
    return historyPage.value.items;
});

async function fetchHistory(endingBefore = '', startingAfter = ''): Promise<void> {
    try {
        await billingStore.getPaymentsHistory({
            limit: limit.value,
            startingAfter,
            endingBefore,
        });
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.BILLING_HISTORY_TAB);
    }
}

async function sizeChanged(size: number) {
    limit.value = size;
    await fetchHistory();
}

async function nextClicked(): Promise<void> {
    const length = historyItems.value.length;
    if (!historyPage.value.hasNext || !length) {
        return;
    }
    await fetchHistory('', historyItems.value[length - 1].id);
}

async function previousClicked(): Promise<void> {
    if (!historyPage.value.hasPrevious || !historyItems.value.length) {
        return;
    }
    await fetchHistory(historyItems.value[0].id, '');
}

onMounted(() => {
    fetchHistory();
});
</script>

<style scoped lang="scss">
    .billing-history {
        margin-top: 2rem;

        &__title {
            font-family: 'font_regular', sans-serif;
            font-size: 1.5rem;
        }

        &__table {
            margin-top: 1.5rem;
        }
    }
</style>
