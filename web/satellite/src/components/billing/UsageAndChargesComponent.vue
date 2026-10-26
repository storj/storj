// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-row>
        <v-col cols="12">
            <v-card title="Costs per project" :subtitle class="pa-2">
                <v-card-item v-if="projectIds.length">
                    <product-usage-and-charges-item-component v-for="projectID of projectIds" :key="projectID" :project-i-d="projectID" />
                </v-card-item>
            </v-card>
        </v-col>
        <v-col v-if="minimumStorageCharges.length" cols="12">
            <v-card title="Minimum Storage" subtitle="Adjusts the storage costs of all your projects to a product's minimum storage and overage price." class="pa-2">
                <v-card-item>
                    <div v-for="charge in minimumStorageCharges" :key="charge.productID">
                        <h4 class="my-2">{{ charge.productName }}</h4>
                        <v-table density="comfortable" class="border rounded-lg">
                            <thead>
                                <tr>
                                    <th class="text-left">
                                        Resource
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
                                <tr v-if="charge.storage > 0">
                                    <td>
                                        <p>Minimum {{ Size.toBase10String(charge.minimumStorageBytes) }} Storage Adjustment</p>
                                    </td>
                                    <td class="d-none d-sm-table-cell">
                                        <p>{{ getGBMonthFormatted(charge.storage) }} Gigabyte-month</p>
                                    </td>
                                    <td class="text-right">
                                        <p>{{ centsToDollars(charge.storagePrice) }}</p>
                                    </td>
                                </tr>
                                <tr v-if="charge.overageStorage > 0">
                                    <td>
                                        <p>Storage Overage (+{{ getOverageExtraPrice(charge) }} per Gigabyte-Month)</p>
                                    </td>
                                    <td class="d-none d-sm-table-cell">
                                        <p>{{ getGBMonthFormatted(charge.overageStorage) }} Gigabyte-month</p>
                                    </td>
                                    <td class="text-right">
                                        <p>{{ centsToDollars(charge.overagePrice) }}</p>
                                    </td>
                                </tr>
                                <tr v-if="charge.discountPrice > 0">
                                    <td>
                                        <p>Discount ({{ charge.discountPercent }}%)</p>
                                    </td>
                                    <td class="d-none d-sm-table-cell" />
                                    <td class="text-right">
                                        <p>-{{ centsToDollars(charge.discountPrice) }}</p>
                                    </td>
                                </tr>
                            </tbody>
                        </v-table>
                    </div>
                </v-card-item>
            </v-card>
        </v-col>
    </v-row>
</template>

<script setup lang="ts">
import {
    VRow,
    VCol,
    VCard,
    VCardItem,
    VTable,
} from 'vuetify/components';
import { computed } from 'vue';

import { CENTS_MB_TO_DOLLARS_GB_SHIFT, centsToDollars, decimalShift, formatPrice } from '@/utils/strings';
import type { MinimumStorageCharge } from '@/types/payments';
import { Size } from '@/utils/bytesSize';
import { useBillingStore } from '@/store/modules/billingStore';

import ProductUsageAndChargesItemComponent from '@/components/billing/ProductUsageAndChargesItemComponent.vue';

/**
 * HOURS_IN_MONTH constant shows amount of hours in 30-day month.
 */
const HOURS_IN_MONTH = 720;
const BYTES_IN_GB = 1000000000;

const props = defineProps<{
    projectIds: string[],
}>();

const billingStore = useBillingStore();

const subtitle = computed(() => {
    if (props.projectIds.length) {
        return 'View usage and download detailed report for every project.';
    }

    return 'No usage data available.';
});

/**
 * The charges that top the storage of all projects up to the minimum storage of products.
 */
const minimumStorageCharges = computed<MinimumStorageCharge[]>(() => billingStore.state.productCharges.minimumStorageCharges);

/**
 * Returns formatted storage in byte-hours in GB x month dimension.
 */
function getGBMonthFormatted(storage: number): string {
    return (storage / HOURS_IN_MONTH / BYTES_IN_GB).toFixed(2);
}

/**
 * Returns how much more than the storage price storage above the minimum costs per GB.
 */
function getOverageExtraPrice(charge: MinimumStorageCharge): string {
    return formatPrice(decimalShift(charge.overageExtraCents, CENTS_MB_TO_DOLLARS_GB_SHIFT));
}
</script>
