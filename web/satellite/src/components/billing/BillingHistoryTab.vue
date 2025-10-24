// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-data-table-server
        :loading="isLoading"
        :headers="headers"
        :items="historyItems"
        :items-length="historyItems.length"
        :must-sort="false"
        no-data-text="No results found"
        hover
    >
        <template #item.amount="{ item }">
            <span>
                {{ centsToDollars(item.amount) }}
            </span>
        </template>
        <template #item.period="{ item }">
            <span class="font-weight-bold">
                {{ item.period }}
            </span>
        </template>
        <template #item.formattedStatus="{ item }">
            <v-chip :color="getColor(item.formattedStatus)" variant="tonal" size="small" class="font-weight-bold">
                {{ item.formattedStatus }}
            </v-chip>
        </template>
        <template #item.link="{ item }">
            <div class="d-flex flex-wrap ga-1">
                <v-btn
                    v-if="item.link"
                    :prepend-icon="DownloadIcon"
                    variant="outlined"
                    color="default"
                    size="small"
                    :href="item.link"
                >
                    Invoice
                </v-btn>
                <v-btn
                    v-if="item.link"
                    :prepend-icon="DownloadIcon"
                    variant="outlined"
                    color="default"
                    size="small"
                    @click="downloadUsageReport(item)"
                >
                    Usage report
                </v-btn>
            </div>
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
                            <v-btn :disabled="!historyPage.hasPrevious" :icon="ChevronLeft" @click="previousClicked" />
                            <v-btn :disabled="!historyPage.hasNext" :icon="ChevronRight" @click="nextClicked" />
                        </v-btn-group>
                    </v-col>
                </v-row>
            </div>
        </template>
    </v-data-table-server>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import {
    VBtn,
    VBtnGroup,
    VChip,
    VCol,
    VRow,
    VSelect,
    VDataTableServer,
} from 'vuetify/components';
import { ChevronLeft, ChevronRight, DownloadIcon } from 'lucide-vue-next';

import { centsToDollars } from '@/utils/strings';
import { useBillingStore } from '@/store/modules/billingStore';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { PaymentHistoryPage, PaymentsHistoryItem } from '@/types/payments';
import { useLoading } from '@/composables/useLoading';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';
import { DataTableHeader } from '@/types/common';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { Download } from '@/utils/download';

const billingStore = useBillingStore();
const projectsStore = useProjectsStore();

const notify = useNotify();

const { isLoading, withLoading } = useLoading();

const limit = ref(DEFAULT_PAGE_LIMIT);
const headers: DataTableHeader[] = [
    { title: 'Usage Period', key: 'period', sortable: false },
    { title: 'Amount', key: 'amount', sortable: false },
    { title: 'Status', key: 'formattedStatus', sortable: false },
    { title: '', key: 'link', sortable: false, width: 250, align: 'end' },
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

function downloadUsageReport(item: PaymentsHistoryItem): void {
    try {
        const link = projectsStore.getUsageReportLink(item.start, item.end, true, true);
        Download.fileByLink(link);
        notify.success('Usage report download started successfully.');
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.BILLING_HISTORY_TAB);
    }
}

onMounted(() => {
    fetchHistory();
});
</script>
