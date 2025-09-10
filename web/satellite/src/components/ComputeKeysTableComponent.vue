// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card variant="outlined" :border="true" rounded="xlg">
        <v-text-field
            v-model="search"
            label="Search"
            prepend-inner-icon="mdi-magnify"
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
            :items="accesses"
            :search="search"
            item-value="name"
            show-select
            hover
        >
            <template #item.name="{ item }">
                <v-list-item class="font-weight-bold pl-0">
                    {{ item.name }}
                </v-list-item>
            </template>
            <template #item.fingerprint="{ item }">
                <v-chip variant="tonal" size="small" rounded="xl" class="font-weight-bold">
                    {{ item.fingerprint }}
                </v-chip>
            </template>
            <template #item.actions>
                <v-btn
                    size="small"
                    variant="outlined"
                    color="default"
                >
                    Remove
                </v-btn>
            </template>
        </v-data-table>
    </v-card>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import {
    VCard,
    VTextField,
    VDataTable,
    VListItem,
    VChip,
    VBtn,
} from 'vuetify/components';

import { DataTableHeader } from '@/types/common';

const headers: DataTableHeader[] = [
    {
        title: 'Name',
        align: 'start',
        key: 'name',
    },
    { title: 'Fingerprint', key: 'fingerprint' },
    { title: 'Type', key: 'type' },
    { title: 'Date Created', key: 'date' },
    { title: '', key: 'actions', align: 'end', sortable: false },
];
const accesses = [
    {
        name: 'Test',
        fingerprint: 'SHA256:AbCdEfGhIjKlMnOpQrStUvWxYz1234567890',
        date: '02 Mar 2023',
        type: 'Generated',
    },
    {
        name: 'New Key',
        fingerprint: 'SHA256:BcDeFgHiJkLmNoPqRsTuVwXyZ1234567890a',
        date: '03 Mar 2023',
        type: 'Generated',
    },
    {
        name: '2415',
        fingerprint: 'SHA256:CdEfGhIjKlMnOpQrStUvWxYz1234567890ab',
        date: '04 Mar 2023',
        type: 'Uploaded',
    },
];

const search = ref<string>('');
const selected = ref([]);
</script>
