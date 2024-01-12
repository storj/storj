// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card border rounded="xlg">
        <v-data-table-server
            :loading="isLoading"
            :headers="headers"
            :items="historyItems"
            :must-sort="false"
            no-data-text="No results found"
            hover
        >
            <template #item.amount="{ item }">
                <span>
                    {{ centsToDollars(item.amount) }}
                </span>
            </template>
            <template #item.formattedStart="{ item }">
                <span class="font-weight-bold">
                    {{ item.formattedStart }}
                </span>
            </template>
            <template #item.formattedStatus="{ item }">
                <v-chip :color="getColor(item.formattedStatus)" variant="tonal" size="small" rounded="xl" class="font-weight-bold">
                    {{ item.formattedStatus }}
                </v-chip>
            </template>
            <template #item.link="{ item }">
                <v-btn v-if="item.link" variant="flat" size="small" :href="item.link">
                    Download
                </v-btn>
            </template>

            <template #bottom>
                <div class="v-data-table-footer">
                    <v-row justify="end" align="center" class="pa-2">
                        <v-col cols="auto">
                            <span class="caption">Items per page:</span>
                        </v-col>
                        <v-col cols="auto">
                            <v-select
                                v-model="limit"
                                density="compact"
                                :items="pageSizes"
                                variant="outlined"
                                hide-details
                                @update:model-value="sizeChanged"
                            />
                        </v-col>
                        <v-col cols="auto">
                            <v-btn-group density="compact">
                                <v-btn :disabled="!historyPage.hasPrevious" :icon="mdiChevronLeft" @click="previousClicked" />
                                <v-btn :disabled="!historyPage.hasNext" :icon="mdiChevronRight" @click="nextClicked" />
                            </v-btn-group>
                        </v-col>
                    </v-row>
                </div>
            </template>
        </v-data-table-server>
    </v-card>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import {
    VBtn,
    VBtnGroup,
    VCard,
    VChip,
    VCol,
    VRow,
    VSelect,
    VDataTableServer,
} from 'vuetify/components';
import { mdiChevronLeft, mdiChevronRight } from '@mdi/js';

import { centsToDollars } from '@/utils/strings';
import { useBillingStore } from '@/store/modules/billingStore';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { PaymentHistoryPage, PaymentsHistoryItem } from '@/types/payments';
import { useLoading } from '@/composables/useLoading';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';

const billingStore = useBillingStore();
const notify = useNotify();

const { isLoading, withLoading } = useLoading();

const limit = ref(DEFAULT_PAGE_LIMIT);
const headers = [
    { title: 'Date', key: 'formattedStart', sortable: false },
    { title: 'Amount', key: 'amount', sortable: false },
    { title: 'Status', key: 'formattedStatus', sortable: false },
    { title: '', key: 'link', sortable: false, width: 0 },
];
const pageSizes = [DEFAULT_PAGE_LIMIT, 25, 50, 100];

const historyPage = computed((): PaymentHistoryPage => {
    return billingStore.state.paymentsHistory;
});

const historyItems = computed((): PaymentsHistoryItem[] => {
    return historyPage.value.items;
});

function getColor(status: string): string {
    if (status === 'Paid') return 'success';
    if (status === 'Open') return 'warning';
    if (status === 'Pending') return 'warning';
    return 'error';
}

function fetchHistory(endingBefore = '', startingAfter = ''): void {
    withLoading(async () => {
        try {
            await billingStore.getPaymentsHistory({
                limit: limit.value,
                startingAfter,
                endingBefore,
            });
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.BILLING_HISTORY_TAB);
        }
    });
}

async function nextClicked(): Promise<void> {
    const length = historyItems.value.length;
    if (!historyPage.value.hasNext || !length) {
        return;
    }
    fetchHistory('', historyItems.value[length - 1].id);
}

async function previousClicked(): Promise<void> {
    if (!historyPage.value.hasPrevious || !historyItems.value.length) {
        return;
    }
    fetchHistory(historyItems.value[0].id, '');
}

async function sizeChanged(size: number) {
    limit.value = size;
    fetchHistory();
}

onMounted(() => {
    fetchHistory();
});
</script>