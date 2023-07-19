// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="billing-history">
        <h1 class="billing-history__title">
            Billing History
        </h1>

        <v-table :total-items-count="historyItems.length" class="billing-history__table">
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
import { computed, onMounted } from 'vue';

import {
    PaymentsHistoryItem,
    PaymentsHistoryItemStatus,
    PaymentsHistoryItemType,
} from '@/types/payments';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useBillingStore } from '@/store/modules/billingStore';

import BillingHistoryHeader
    from '@/components/account/billing/billingTabs/BillingHistoryHeader.vue';
import BillingHistoryItem
    from '@/components/account/billing/billingTabs/BillingHistoryItem.vue';
import VTable from '@/components/common/VTable.vue';

const billingStore = useBillingStore();
const notify = useNotify();

async function fetchHistory(): Promise<void> {
    try {
        await billingStore.getPaymentsHistory();
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.BILLING_HISTORY_TAB);
    }
}

const historyItems = computed((): PaymentsHistoryItem[] => {
    return billingStore.state.paymentsHistory.filter((item: PaymentsHistoryItem) => {
        return item.status !== PaymentsHistoryItemStatus.Draft && item.status !== PaymentsHistoryItemStatus.Empty
            && (item.type === PaymentsHistoryItemType.Invoice || item.type === PaymentsHistoryItemType.Charge);
    });
});

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
