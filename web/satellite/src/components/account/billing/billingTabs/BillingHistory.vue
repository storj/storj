// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="billing-history">
        <h1 class="billing-history__title">
            Billing History
        </h1>

        <v-table class="billing-history__table">
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

import { PaymentsHistoryItem, PaymentsHistoryItemType } from '@/types/payments';
import { PAYMENTS_ACTIONS } from '@/store/modules/payments';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify, useStore } from '@/utils/hooks';

import BillingHistoryHeader from '@/components/account/billing/billingTabs/BillingHistoryHeader.vue';
import BillingHistoryItem from '@/components/account/billing/billingTabs/BillingHistoryItem.vue';
import VTable from '@/components/common/VTable.vue';

const store = useStore();
const notify = useNotify();

async function fetchHistory(): Promise<void> {
    try {
        await store.dispatch(PAYMENTS_ACTIONS.GET_PAYMENTS_HISTORY);
    } catch (error) {
        await notify.error(error.message, AnalyticsErrorEventSource.BILLING_HISTORY_TAB);
    }
}

const historyItems = computed((): PaymentsHistoryItem[] => {
    return store.state.paymentsModule.paymentsHistory.filter((item: PaymentsHistoryItem) => {
        return item.status !== 'draft' && item.status !== '' && (item.type === PaymentsHistoryItemType.Invoice || item.type === PaymentsHistoryItemType.Charge);
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
