// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card variant="flat" :border="true" rounded="xlg">
        <v-text-field
            v-model="search" label="Search" prepend-inner-icon="mdi-magnify" single-line variant="solo-filled" flat
            hide-details clearable density="compact" rounded="lg" class="mx-2 mt-2"
        />

        <v-data-table
            v-model="selected" v-model:sort-by="sortBy" :headers="headers" :items="files" :search="search"
            class="elevation-1" item-key="path" density="comfortable" show-expand hover @item-click="handleItemClick"
        >
            <template #expanded-row="{ columns, item }">
                <tr>
                    <td :colspan="columns.length">
                        More info about {{ item.raw.name }} change.
                    </td>
                </tr>
            </template>

            <template #item.operation="{ item }">
                <v-chip variant="tonal" size="small" rounded="lg" @click="setSearch(item.raw.operation)">
                    {{ item.raw.operation }}
                </v-chip>
            </template>

            <template #item.name="{ item }">
                <v-list-item class="rounded-lg pl-1" link router-link to="/dashboard">
                    {{ item.columns.name }}
                </v-list-item>
            </template>

            <template #item.email="{ item }">
                <v-chip variant="tonal" size="small" rounded="lg" @click="setSearch(item.raw.email)">
                    {{ item.raw.email }}
                </v-chip>
            </template>

            <template #item.date="{ item }">
                <span class="text-no-wrap">
                    {{ item.raw.date }}
                </span>
            </template>
        </v-data-table>
    </v-card>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { VCard, VTextField, VChip, VListItem } from 'vuetify/components';
import { VDataTable } from 'vuetify/labs/components';

const search = ref<string>('');
const selected = ref<string[]>([]);
const sortBy = ref([{ key: 'date', order: 'asc' }]);

const headers = [
    { title: 'Date', key: 'date' },
    { title: 'Change', key: 'name' },
    { title: 'Operation', key: 'operation' },
    { title: 'Project ID', key: 'projectID' },
    { title: 'Bucket', key: 'bucket' },
    { title: 'Updated', key: 'updated' },
    { title: 'Previous', key: 'previous' },
    { title: 'Admin', key: 'email' },
    { title: '', key: 'data-table-expand' },
];
const files = [
    {
        name: 'Project',
        operation: 'Limits',
        email: 'vduke@gmail.com',
        projectID: 'F82SR21Q284JF',
        bucket: 'All',
        updated: '300TB',
        previous: '100TB',
        date: '02 Mar 2023',
    },
    {
        name: 'Account',
        operation: 'Coupon',
        email: 'knowles@aurora.io',
        projectID: '',
        bucket: 'All',
        updated: '30TB',
        previous: 'Free Tier',
        date: '21 Apr 2023',
    },
];

function setSearch(searchText: string) {
    search.value = searchText;
}
</script>
