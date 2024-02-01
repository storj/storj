// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-row>
        <v-col>
            <h4>Costs per project</h4>
        </v-col>
    </v-row>

    <v-row>
        <v-col>
            <usage-and-charges-item-component v-for="projectID of projectIds" :key="projectID" :project-id="projectID" />
        </v-col>
    </v-row>
</template>

<script setup lang="ts">
import {
    VRow,
    VCol,
} from 'vuetify/components';

import UsageAndChargesItemComponent from '@/components/billing/UsageAndChargesItemComponent.vue';

const props = defineProps<{
    projectIds: string[],
}>();
</script>
