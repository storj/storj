// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card variant="outlined" border rounded="xlg">
        <v-text-field
            v-model="search"
            label="Search"
            :prepend-inner-icon="Search"
            single-line
            variant="solo-filled"
            flat
            hide-details
            clearable
            density="comfortable"
            rounded="lg"
            class="mx-2 mt-2"
        />

        <v-data-table
            v-model="selected"
            :headers="headers"
            :items="instanceTypes"
            :search="search"
            item-value="name"
            hover
        >
            <template #item.name="{ item }">
                <v-list-item class="font-weight-bold pl-0 mt-1">
                    {{ item.gpu }}
                </v-list-item>
                <v-list-item class="pl-0 mt-n4 mb-1 text-medium-emphasis">
                    {{ item.cpu }} | {{ item.ram }} | {{ item.storage }}
                </v-list-item>
            </template>

            <template #item.price="{ item }">
                <p class="font-weight-bold">
                    {{ item.price }}
                </p>
            </template>

            <template #item.actions>
                <v-btn
                    color="primary"
                    class="mr-1 text-caption"
                    density="comfortable"
                    :append-icon="ArrowRight"
                >
                    Configure
                </v-btn>
            </template>
        </v-data-table>
    </v-card>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import {
    VBtn,
    VCard,
    VDataTable,
    VListItem,
    VTextField,
} from 'vuetify/components';
import { Search, ArrowRight } from 'lucide-vue-next';

import { DataTableHeader } from '@/types/common';

const headers: DataTableHeader[] = [
    {
        title: 'Instance Type',
        align: 'start',
        key: 'name',
    },
    { title: 'Network Speed', key: 'speed' },
    { title: 'Location', key: 'location' },
    { title: 'Price', key: 'price' },
    { title: '', key: 'actions', align: 'end', sortable: false },
];
const instanceTypes = [
    {
        gpu: '6x NVIDIA H100 SXM5 80GB',
        cpu: 'Intel Xeon Platinum 8470',
        speed: '10 Gbps',
        storage: '6140 GB NVMe SSD',
        ram: '488 GB',
        location: 'Norway',
        price: '$3.31 / hr',
    },
    {
        gpu: '4x NVIDIA H100 SXM5 80GB',
        cpu: 'Intel Xeon Platinum 8470',
        speed: '10 Gbps',
        storage: '888 GB NVMe SSD',
        ram: '320 GB',
        location: 'Canada',
        price: '$3.29 / hr',
    },
    {
        gpu: '10x NVIDIA H100 SXM5 80GB',
        cpu: 'Intel Xeon Platinum 8470',
        speed: '10 Gbps',
        storage: '3240 GB NVMe SSD',
        ram: '820 GB',
        location: 'USA',
        price: '$3.31 / hr',
    },
];

const search = ref<string>('');
const selected = ref([]);
</script>
