// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card variant="flat" :border="true" class="rounded-xlg">
        <v-text-field
            v-model="search"
            label="Search"
            prepend-inner-icon="mdi-magnify"
            single-line
            hide-details
        />

        <v-data-table
            v-model="selected"
            :sort-by="sortBy"
            :headers="headers"
            :items="accesses"
            :search="search"
            class="elevation-1"
            show-select
            hover
        >
            <template #item.name="{ item }">
                <span class="font-weight-bold">
                    {{ item.raw.name }}
                </span>
            </template>
            <template #item.status="{ item }">
                <v-chip :color="item.raw.status == 'Active' ? 'success' : 'warning'" variant="tonal" size="small" rounded="xl" class="font-weight-bold">
                    {{ item.raw.status }}
                </v-chip>
            </template>
        </v-data-table>
    </v-card>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { VCard, VTextField, VChip } from 'vuetify/components';
import { VDataTable } from 'vuetify/labs/components';

const search = ref<string>('');
const selected = ref([]);

const sortBy = [{ key: 'date', order: 'asc' }];
const headers = [
    {
        title: 'Name',
        align: 'start',
        key: 'name',
    },
    { title: 'Type', key: 'type' },
    { title: 'Status', key: 'status' },
    { title: 'Permissions', key: 'permissions' },
    { title: 'Date Created', key: 'date' },
];
const accesses = [
    {
        name: 'Backup',
        date: '02 Mar 2023',
        type: 'Access Grant',
        permissions: 'All',
        status: 'Active',
    },
    {
        name: 'S3 Test',
        date: '03 Mar 2023',
        type: 'S3 Credentials',
        permissions: 'Read, Write',
        status: 'Expired',
    },
    {
        name: 'CLI Demo',
        date: '04 Mar 2023',
        type: 'CLI Access',
        permissions: 'Read, Write, List',
        status: 'Active',
    },
    {
        name: 'Sharing',
        date: '08 Mar 2023',
        type: 'Access Grant',
        permissions: 'Read, Delete',
        status: 'Active',
    },
    {
        name: 'Sync Int',
        date: '12 Mar 2023',
        type: 'S3 Credentials',
        permissions: 'All',
        status: 'Expired',
    },
];
</script>
