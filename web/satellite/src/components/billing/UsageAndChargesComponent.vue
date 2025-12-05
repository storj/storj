// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-row>
        <v-col>
            <v-card title="Costs per project" :subtitle class="pa-2">
                <v-card-item v-if="projectIds.length">
                    <product-usage-and-charges-item-component v-for="projectID of projectIds" :key="projectID" :project-i-d="projectID" />
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
} from 'vuetify/components';
import { computed } from 'vue';

import ProductUsageAndChargesItemComponent from '@/components/billing/ProductUsageAndChargesItemComponent.vue';

const props = defineProps<{
    projectIds: string[],
}>();

const subtitle = computed(() => {
    if (props.projectIds.length) {
        return 'View usage and download detailed report for every project.';
    }

    return 'No usage data available.';
});
</script>
