// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card class="mb-2">
        <v-expansion-panels>
            <v-expansion-panel min-height="64">
                <v-expansion-panel-title>
                    <v-row justify="space-between" align="center">
                        <v-col cols="auto" class="pr-2">
                            <div class="d-flex align-center">
                                <img src="@/assets/icon-project-tonal.svg" alt="Project" class="mr-2" style="min-width: 24px;">
                                <span class="font-weight-bold text-body-2 text-truncate">{{ projectName }}</span>
                            </div>
                        </v-col>
                        <v-col cols="auto" class="text-end ml-auto">
                            <div class="d-flex align-center justify-end">
                                <span class="d-none d-sm-inline text-body-2 text-medium-emphasis">
                                    Estimated Total &nbsp;
                                </span>
                                <span class="font-weight-bold">
                                    {{ centsToDollars(productCharges.getProjectPrice(projectID)) }}
                                </span>
                            </div>
                        </v-col>
                    </v-row>
                </v-expansion-panel-title>
                <v-expansion-panel-text>
                    <div v-for="[productID, charge] in charges" :key="projectID + productID">
                        <h4 class="my-2">{{ getProductName(productID) }}</h4>
                        <v-table density="comfortable" class="border rounded-lg">
                            <thead>
                                <tr>
                                    <th class="text-left">
                                        Resource
                                    </th>
                                    <th class="text-left d-none d-md-table-cell">
                                        Period
                                    </th>
                                    <th class="text-left d-none d-sm-table-cell">
                                        Usage
                                    </th>
                                    <th class="text-right">
                                        Cost
                                    </th>
                                </tr>
                            </thead>
                            <tbody>
                                <tr>
                                    <td>
                                        <p>Storage <span class="d-none d-md-inline">({{ getStoragePrice(productID) }} per Gigabyte-Month)</span></p>
                                    </td>
                                    <td class="d-none d-md-table-cell">
                                        <p>{{ getPeriod(charge) }}</p>
                                    </td>
                                    <td class="d-none d-sm-table-cell">
                                        <p>{{ getStorageFormatted(charge) }} Gigabyte-month</p>
                                    </td>
                                    <td class="text-right">
                                        <p>{{ centsToDollars(charge.storagePrice) }}</p>
                                    </td>
                                </tr>

                                <template v-if="charge.priceModel.egressOverageMode">
                                    <tr v-if="charge.includedEgress > 0">
                                        <td>
                                            <p>Included Egress <span class="d-none d-md-inline">({{ charge.priceModel.egressDiscountRatio }}X of storage)</span></p>
                                        </td>
                                        <td class="d-none d-md-table-cell">
                                            <p>{{ getPeriod(charge) }}</p>
                                        </td>
                                        <td class="d-none d-sm-table-cell">
                                            <p>{{ getEgressAmountAndDimension(charge.includedEgress) }}</p>
                                        </td>
                                        <td class="text-right">
                                            <p>$0</p>
                                        </td>
                                    </tr>

                                    <tr v-if="charge.egress > 0">
                                        <td>
                                            <p>Overage Egress <span class="d-none d-md-inline">({{ getEgressPrice(productID) }} per GB)</span></p>
                                        </td>
                                        <td class="d-none d-md-table-cell">
                                            <p>{{ getPeriod(charge) }}</p>
                                        </td>
                                        <td class="d-none d-sm-table-cell">
                                            <p>{{ getEgressAmountAndDimension(charge.egress) }}</p>
                                        </td>
                                        <td class="text-right">
                                            <p>{{ centsToDollars(charge.egressPrice) }}</p>
                                        </td>
                                    </tr>
                                </template>

                                <tr v-else>
                                    <td>
                                        <p>Egress <span class="d-none d-md-inline">({{ getEgressPrice(productID) }} per GB)</span></p>
                                    </td>
                                    <td class="d-none d-md-table-cell">
                                        <p>{{ getPeriod(charge) }}</p>
                                    </td>
                                    <td class="d-none d-sm-table-cell">
                                        <p>{{ getEgressAmountAndDimension(charge.egress) }}</p>
                                    </td>
                                    <td class="text-right">
                                        <p>{{ centsToDollars(charge.egressPrice) }}</p>
                                    </td>
                                </tr>

                                <tr v-if="getSegmentPrice(productID)">
                                    <td>
                                        <p>Segments <span class="d-none d-md-inline">({{ getSegmentPrice(productID) }} per Segment-Month)</span></p>
                                    </td>
                                    <td class="d-none d-md-table-cell">
                                        <p>{{ getPeriod(charge) }}</p>
                                    </td>
                                    <td class="d-none d-sm-table-cell">
                                        <p>{{ getSegmentCountFormatted(charge) }} Segment-month</p>
                                    </td>
                                    <td class="text-right">
                                        <p>{{ centsToDollars(charge.segmentPrice) }}</p>
                                    </td>
                                </tr>

                                <tr v-if="parseFloat(charge.priceModel.smallObjectFeeCents) > 0">
                                    <td>
                                        <p>Minimum Object Size Remainder</p>
                                    </td>
                                    <td class="d-none d-md-table-cell">
                                        <p>{{ getPeriod(charge) }}</p>
                                    </td>
                                    <td class="d-none d-sm-table-cell">
                                        <p>MB-month</p>
                                    </td>
                                    <td class="text-right">
                                        <p>$0</p>
                                    </td>
                                </tr>

                                <tr v-if="parseFloat(charge.priceModel.minimumRetentionFeeCents) > 0">
                                    <td>
                                        <p>Minimum Storage Retention Remainder</p>
                                    </td>
                                    <td class="d-none d-md-table-cell">
                                        <p>{{ getPeriod(charge) }}</p>
                                    </td>
                                    <td class="d-none d-sm-table-cell">
                                        <p>MB-month</p>
                                    </td>
                                    <td class="text-right">
                                        <p>$0</p>
                                    </td>
                                </tr>
                            </tbody>
                        </v-table>
                    </div>
                    <v-btn v-if="project" :prepend-icon="Calendar" class="mt-2">
                        <detailed-usage-report-dialog :project />
                        Detailed Project Report
                    </v-btn>
                </v-expansion-panel-text>
            </v-expansion-panel>
        </v-expansion-panels>
    </v-card>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import {
    VBtn,
    VCard,
    VCol,
    VExpansionPanel,
    VExpansionPanels,
    VExpansionPanelText,
    VExpansionPanelTitle,
    VRow,
    VTable,
} from 'vuetify/components';
import { Calendar } from 'lucide-vue-next';

import { CENTS_MB_TO_DOLLARS_GB_SHIFT, centsToDollars, decimalShift, formatPrice } from '@/utils/strings';
import { ProductCharge, ProductCharges, ProjectCharge, UsagePriceModel } from '@/types/payments';
import { Project } from '@/types/projects';
import { Size } from '@/utils/bytesSize';
import { SHORT_MONTHS_NAMES } from '@/utils/constants/date';
import { useBillingStore } from '@/store/modules/billingStore';
import { useProjectsStore } from '@/store/modules/projectsStore';

import DetailedUsageReportDialog from '@/components/dialogs/DetailedUsageReportDialog.vue';

/**
 * HOURS_IN_MONTH constant shows amount of hours in 30-day month.
 */
const HOURS_IN_MONTH = 720;

const props = withDefaults(defineProps<{
    /**
     * The ID of the project for which to show the usage and charge information.
     */
    projectID?: string;
}>(), {
    projectID: '',
});

const billingStore = useBillingStore();
const projectsStore = useProjectsStore();

/**
 * An array of tuples containing the partner name and usage charge for the specified project ID.
 */
const charges = computed((): [productID: number, charge: ProductCharge][] => {
    const arr = productCharges.value.toArray();
    arr.sort(([product1], [product2]) => Number(product1) - Number(product2));
    const tuple = arr.find(tuple => tuple[0] === props.projectID);
    return tuple ? tuple[1] : [];
});

/**
 * project returns project with ID props.projectID.
 */
const project = computed<Project | null>(() => {
    return projectsStore.state.projects.find(project => project.id === props.projectID) || null;
});

/**
 * projectName returns project name.
 */
const projectName = computed((): string => {
    return project.value?.name || '';
});

/**
 * Returns product usage price model from store.
 */
const productCharges = computed<ProductCharges>(() => billingStore.state.productCharges as ProductCharges);

/**
 * Returns product name by product ID.
 */
function getProductName(productID: number): string {
    return productCharges.value.getProductName(props.projectID, productID) ?? '';
}

/**
 * Returns product price model by product ID.
 */
function getPriceModel(productID: number): UsagePriceModel {
    return productCharges.value.getUsagePriceModel(props.projectID, productID) || billingStore.state.usagePriceModel;
}

/**
 * Returns string of date range.
 */
function getPeriod(charge: ProjectCharge): string {
    const since = `${SHORT_MONTHS_NAMES[charge.since.getUTCMonth()]} ${charge.since.getUTCDate()}`;
    const before = `${SHORT_MONTHS_NAMES[charge.before.getUTCMonth()]} ${charge.before.getUTCDate()}`;

    return `${since} - ${before}`;
}

/**
 * Returns formatted egress depending on amount of bytes.
 */
function egressFormatted(egress: number): Size {
    return new Size(egress, 2);
}

/**
 * Returns formatted storage used in GB x month dimension.
 */
function getStorageFormatted(charge: ProjectCharge): string {
    const bytesInGB = 1000000000;

    return (charge.storage / HOURS_IN_MONTH / bytesInGB).toFixed(2);
}

/**
 * Returns formatted segment count in segment x month dimension.
 */
function getSegmentCountFormatted(charge: ProjectCharge): string {
    return (charge.segmentCount / HOURS_IN_MONTH).toFixed(2);
}

/**
 * Returns storage price per GB.
 */
function getStoragePrice(productID: number): string {
    return formatPrice(decimalShift(getPriceModel(productID).storageMBMonthCents, CENTS_MB_TO_DOLLARS_GB_SHIFT));
}

/**
 * Returns egress price per GB.
 */
function getEgressPrice(productID: number): string {
    return formatPrice(decimalShift(getPriceModel(productID).egressMBCents, CENTS_MB_TO_DOLLARS_GB_SHIFT));
}

/**
 * Returns segment price.
 */
function getSegmentPrice(productID: number): string {
    const price = getPriceModel(productID).segmentMonthCents;
    if (!price) return '';
    if (parseInt(price, 10) === 0) return '';
    return formatPrice(decimalShift(price, 2));
}

/**
 * Returns string of egress amount and dimension.
 */
function getEgressAmountAndDimension(egress: number): string {
    const formatted = egressFormatted(egress);
    return `${formatted.formattedBytes} ${formatted.label}`;
}
</script>
