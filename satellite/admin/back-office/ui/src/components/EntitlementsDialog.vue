// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog
        v-model="model"
        transition="fade-transition"
        :width="placementProductMappings.length ? 1200 : 800"
    >
        <v-card rounded="xlg" :title="`${project.name} Entitlements`">
            <template #append>
                <v-btn
                    :icon="X"
                    variant="text" size="small" color="default" @click="model = false"
                />
            </template>

            <div class="pa-6">
                <v-divider>
                    <span class="text-caption">Compute access token</span>
                </v-divider>
                <v-chip class="mt-3">
                    <template v-if="entitlements?.computeAccessToken">***************************</template>
                    <template v-else>Not Set</template>
                </v-chip>

                <v-divider class="my-5">
                    <span class="text-caption">New bucket placements</span>
                </v-divider>

                <div class="d-flex flex-wrap ga-2">
                    <v-chip v-for="placement in entitlements?.newBucketPlacements ?? []" :key="placement">
                        {{ placement }}
                    </v-chip>
                    <v-chip v-if="!entitlements?.newBucketPlacements?.length">Not Set</v-chip>
                </div>

                <v-divider class="my-5">
                    <span class="text-caption">Placement-product mappings</span>
                </v-divider>

                <v-data-table :items="placementProductMappings" :headers="headers">
                    <template #no-data> No mappings set </template>
                    <template #bottom>
                        <div class="v-data-table-footer" />
                    </template>
                </v-data-table>
            </div>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { VBtn, VCard, VChip, VDataTable, VDialog, VDivider } from 'vuetify/components';
import { computed } from 'vue';
import { X } from 'lucide-vue-next';

import { Project, ProjectEntitlements } from '@/api/client.gen';
import { DataTableHeader } from '@/types/common';
import { centsToDollars } from '@/utils/strings';

const props = defineProps<{
    project: Project;
}>();

const model = defineModel<boolean>({ required: true });

const headers: DataTableHeader[] = [
    { title: 'Placement', key: 'placement', sortable: false },
    { title: 'Product Name', key: 'productName', sortable: false },
    { title: 'Egress / MB', key: 'egress', sortable: false, align: 'end' },
    { title: 'Egress Discount Ratio', key: 'egressDiscountRatio', sortable: false, align: 'end' },
    { title: 'Storage / MB / Month', key: 'storage', sortable: false, align: 'end' },
    { title: 'Segment / Month', key: 'segmentMonthCents', sortable: false, align: 'end' },
];

const entitlements = computed<ProjectEntitlements | null>(() => props.project.entitlements);

const placementProductMappings = computed(() => {
    if (!entitlements.value?.placementProductMappings) return [];
    return Object.entries(entitlements.value.placementProductMappings).map(([placement, mapping]) => ({
        placement,
        productName: mapping.productName,
        egress: centsToDollars(mapping.egressMBCents),
        egressDiscountRatio: mapping.egressDiscountRatio,
        storage: centsToDollars(mapping.storageMBMonthCents),
        segmentMonthCents: centsToDollars(mapping.segmentMonthCents),
    }));
});
</script>

<style scoped lang="scss">
:deep(.v-data-table-footer) {
    background: rgb(var(--v-theme-surface)) !important;
    box-shadow: none !important;
}

// These make sure the first and last table headers are rounded
// so they don't clash with the rounded table corners.

:deep(th:last-of-type) {
    border-top-right-radius: 10px;
}

:deep(th:first-of-type) {
    border-top-left-radius: 10px;
}
</style>
