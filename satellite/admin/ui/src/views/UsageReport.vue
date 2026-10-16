// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container fluid>
        <v-card
            title="Usage Report"
            subtitle="Usage and charges of the projects with usage in a month, by product."
            variant="flat" rounded="xlg" border
        >
            <v-data-table
                v-model:expanded="expanded"
                :headers="headers"
                :items="report?.items ?? []"
                :item-value="itemKey"
                :loading="isLoading"
                :items-per-page="-1"
                no-data-text="No usage in this month"
                show-expand
                class="border-0"
            >
                <template #top>
                    <v-row dense align="center" class="mx-2 my-2">
                        <v-col cols="12" sm="4">
                            <v-select
                                v-model="period"
                                :disabled="isLoading"
                                :items="periodItems"
                                label="Month"
                                variant="solo-filled" flat hide-details density="compact" rounded="lg"
                                @update:model-value="onPeriodChange"
                            />
                        </v-col>
                        <v-spacer />
                        <v-col cols="auto">
                            <v-btn
                                :disabled="isLoading"
                                :prepend-icon="FileSpreadsheet"
                                color="primary"
                                variant="flat"
                                rounded="lg"
                                @click="downloadCSV"
                            >
                                Download CSV
                            </v-btn>
                        </v-col>
                    </v-row>
                </template>

                <template #item.userEmail="{ item }">
                    <v-btn
                        v-tooltip="item.userEmail"
                        variant="outlined"
                        color="default"
                        density="compact"
                        @click="goToUser(item)"
                    >
                        <template #prepend>
                            <v-icon :icon="User" />
                        </template>
                        <template #default>
                            <span class="text-truncate" style="max-width: 200px;">{{ item.userEmail }}</span>
                        </template>
                    </v-btn>
                </template>

                <template #item.projectName="{ item }">
                    <a
                        v-tooltip="item.projectID"
                        class="link d-inline-block text-truncate"
                        style="max-width: 150px;"
                        @click="goToProject(item)"
                    >
                        {{ item.projectName }}
                    </a>
                </template>

                <template #item.productName="{ item }">
                    <span class="text-no-wrap">
                        {{ item.productName }}
                        <v-chip v-if="hasDiscount(item)" color="success" variant="tonal" size="small" class="ml-1 font-weight-medium">
                            -{{ item.discountPercent }}%
                        </v-chip>
                    </span>
                </template>

                <template #item.storageGBMonth="{ item }">
                    {{ formatUsage(item.storageGBMonth) }}
                </template>

                <template #item.egressGB="{ item }">
                    {{ formatUsage(item.egressGB) }}
                </template>

                <template #item.minObjectSizeGBMonth="{ item }">
                    {{ formatUsage(item.minObjectSizeGBMonth) }}
                </template>

                <template #item.minRetentionGBMonth="{ item }">
                    {{ formatUsage(item.minRetentionGBMonth) }}
                </template>

                <template #item.total.standardCents="{ item }">
                    {{ centsToDollars(item.total.standardCents) }}
                </template>

                <template #item.total.discountCents="{ item }">
                    {{ formatDiscount(item.total.discountCents) }}
                </template>

                <template #item.total.billableCents="{ item }">
                    <span class="font-weight-bold">{{ centsToDollars(item.total.billableCents) }}</span>
                </template>

                <template #expanded-row="{ columns, item }">
                    <tr>
                        <td :colspan="columns.length" class="pa-4">
                            <v-table density="compact" class="border rounded-lg">
                                <thead>
                                    <tr>
                                        <th>Usage</th>
                                        <th class="text-end">Amount</th>
                                        <th class="text-end">Standard Price</th>
                                        <th class="text-end">Discount</th>
                                        <th class="text-end">Billable Price</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    <tr v-for="row in breakdown(item)" :key="row.name">
                                        <td>{{ row.name }}</td>
                                        <td class="text-end">{{ row.amount }}</td>
                                        <td class="text-end">{{ centsToDollars(row.charge.standardCents) }}</td>
                                        <td class="text-end">{{ formatDiscount(row.charge.discountCents) }}</td>
                                        <td class="text-end">{{ centsToDollars(row.charge.billableCents) }}</td>
                                    </tr>
                                </tbody>
                            </v-table>
                        </td>
                    </tr>
                </template>

                <template #bottom>
                    <div class="d-flex align-center justify-end flex-wrap ga-4 px-4 py-2 text-body-2">
                        <div class="d-flex align-center ga-2">
                            <span>Projects per page</span>
                            <v-select
                                v-model="limit"
                                :disabled="isLoading"
                                :items="itemsPerPageOptions"
                                variant="outlined" density="compact" hide-details
                                style="max-width: 100px;"
                                @update:model-value="resetPages"
                            />
                        </div>
                        <span>Page {{ cursors.length }}</span>
                        <div class="d-flex ga-1">
                            <v-btn
                                :icon="ChevronLeft"
                                :disabled="isLoading || cursors.length <= 1"
                                aria-label="Previous page"
                                variant="text" density="comfortable"
                                @click="previousPage"
                            />
                            <v-btn
                                :icon="ChevronRight"
                                :disabled="isLoading || !report?.nextCursor"
                                aria-label="Next page"
                                variant="text" density="comfortable"
                                @click="nextPage"
                            />
                        </div>
                    </div>
                </template>
            </v-data-table>
        </v-card>
    </v-container>
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import {
    VBtn,
    VCard,
    VChip,
    VCol,
    VContainer,
    VDataTable,
    VIcon,
    VRow,
    VSelect,
    VSpacer,
    VTable,
} from 'vuetify/components';
import { ChevronLeft, ChevronRight, FileSpreadsheet, User } from 'lucide-vue-next';

import { UsageReportCharge, UsageReportItem, UsageReportPage } from '@/api/client.gen';
import { ROUTES } from '@/router';
import { useReportsStore } from '@/store/reports';
import { useNotify } from '@/composables/useNotify';
import { useLoading } from '@/composables/useLoading';
import { DataTableHeader } from '@/types/common';
import { centsToDollars } from '@/utils/strings';
import { Download } from '@/utils/download';

type BreakdownRow = {
    name: string;
    amount: string;
    charge: UsageReportCharge;
};

// the bandwidth rollups older than the default rollup-archive.archive-age are archived, so only the
// months that started within it can be picked.
const ROLLUP_ARCHIVE_AGE_DAYS = 90;

const reportsStore = useReportsStore();
const notify = useNotify();
const router = useRouter();

const { isLoading, withLoading } = useLoading();

const report = ref<UsageReportPage>();
const expanded = ref<string[]>([]);
const limit = ref<number>(25);
// the cursors of the pages up to the current one; the first page's cursor is empty.
const cursors = ref<string[]>(['']);

const periodItems = getPeriodItems();
// the previous month is the latest one with complete usage.
const period = ref<string>(periodItems[Math.min(1, periodItems.length - 1)].value);

const headers: DataTableHeader[] = [
    { title: 'Email', key: 'userEmail', sortable: false },
    { title: 'Project', key: 'projectName', sortable: false },
    { title: 'Product', key: 'productName', sortable: false },
    { title: 'Storage (GB-mo)', key: 'storageGBMonth', align: 'end', sortable: false },
    { title: 'Egress (GB)', key: 'egressGB', align: 'end', sortable: false },
    { title: 'Min. Object Size (GB-mo)', key: 'minObjectSizeGBMonth', align: 'end', sortable: false },
    { title: 'Min. Retention (GB-mo)', key: 'minRetentionGBMonth', align: 'end', sortable: false },
    { title: 'Standard', key: 'total.standardCents', align: 'end', sortable: false },
    { title: 'Discount', key: 'total.discountCents', align: 'end', sortable: false },
    { title: 'Billable', key: 'total.billableCents', align: 'end', sortable: false },
    { title: '', key: 'data-table-expand' },
];

const itemsPerPageOptions = [
    { value: 10, title: '10' },
    { value: 25, title: '25' },
    { value: 50, title: '50' },
    { value: 100, title: '100' },
];

const usageFormatter = new Intl.NumberFormat('en-US', { maximumFractionDigits: 2 });

/**
 * Returns the months that can be picked, the current one first, in YYYY-MM format.
 */
function getPeriodItems(): { title: string, value: string }[] {
    const now = new Date();
    const oldest = now.getTime() - ROLLUP_ARCHIVE_AGE_DAYS * 24 * 60 * 60 * 1000;
    const items: { title: string, value: string }[] = [];
    for (let i = 0; ; i++) {
        const month = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() - i, 1));
        // the current month is always offered.
        if (i > 0 && month.getTime() < oldest) break;
        items.push({
            title: month.toLocaleString('en-US', { month: 'long', year: 'numeric', timeZone: 'UTC' }),
            value: month.toISOString().substring(0, 7),
        });
    }
    return items;
}

function itemKey(item: UsageReportItem): string {
    return `${item.projectID}-${item.productID}`;
}

function hasDiscount(item: UsageReportItem): boolean {
    return Number(item.discountPercent) > 0;
}

function formatUsage(value: number): string {
    return usageFormatter.format(value);
}

function formatDiscount(cents: number): string {
    return cents ? `-${centsToDollars(cents)}` : centsToDollars(0);
}

/**
 * Returns the usage and the charges of each type of usage of the item.
 */
function breakdown(item: UsageReportItem): BreakdownRow[] {
    return [
        { name: 'Storage', amount: `${formatUsage(item.storageGBMonth)} GB-Month`, charge: item.storage },
        { name: 'Egress', amount: `${formatUsage(item.egressGB)} GB`, charge: item.egress },
        { name: 'Segments', amount: `${formatUsage(item.segmentMonth)} Segment-Month`, charge: item.segment },
        { name: 'Minimum Object Size', amount: `${formatUsage(item.minObjectSizeGBMonth)} GB-Month`, charge: item.minObjectSize },
        { name: 'Minimum Retention', amount: `${formatUsage(item.minRetentionGBMonth)} GB-Month`, charge: item.minRetention },
    ];
}

/**
 * Returns to the first page when the month changes.
 */
function onPeriodChange(): void {
    resetPages();
}

/**
 * Returns to the first page.
 */
function resetPages(): void {
    cursors.value = [''];
}

/**
 * Goes to the next page.
 */
function nextPage(): void {
    const next = report.value?.nextCursor;
    if (isLoading.value || !next) return;
    cursors.value = [...cursors.value, next];
}

/**
 * Goes back to the previous page.
 */
function previousPage(): void {
    if (isLoading.value || cursors.value.length <= 1) return;
    cursors.value = cursors.value.slice(0, -1);
}

/**
 * Fetches the page of the usage report.
 */
function fetchReport(): void {
    withLoading(async () => {
        try {
            report.value = await reportsStore.getUsageReport(period.value, limit.value, cursors.value[cursors.value.length - 1]);
            expanded.value = [];
        } catch (error) {
            notify.error(`Error getting the usage report. ${error.message}`);
        }
    });
}

function downloadCSV(): void {
    Download.fileByLink(reportsStore.getUsageReportCSVLink(period.value));
    notify.success('Usage report download started successfully.');
}

function goToUser(item: UsageReportItem): void {
    router.push({ name: ROUTES.Account.name, params: { userID: item.userID } });
}

function goToProject(item: UsageReportItem): void {
    router.push({ name: ROUTES.AccountProject.name, params: { userID: item.userID, projectID: item.projectID } });
}

watch([period, limit, cursors], fetchReport);

onMounted(fetchReport);
</script>
