// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-row>
        <v-col>
            <v-card title="Costs per project" subtitle="View usage and download detailed report for every project." class="pa-2">
                <v-card-item>
                    <usage-and-charges-item-component v-for="projectID of projectIds" :key="projectID" :project-id="projectID" />
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

import UsageAndChargesItemComponent from '@/components/billing/UsageAndChargesItemComponent.vue';

defineProps<{
    projectIds: string[],
}>();
</script>
