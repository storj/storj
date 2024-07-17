// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card variant="outlined" border rounded="xlg">
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
            :sort-by="sortBy"
            :headers="headers"
            :items="domains"
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
        </v-data-table>
    </v-card>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import {
    VDataTable,
    VCard,
    VTextField,
    VListItem,
} from 'vuetify/components';

import { Domain } from '@/types/domains';

type SortItem = {
    key: keyof Domain;
    order: boolean | 'asc' | 'desc';
}

const search = ref<string>('');
const selected = ref<Domain[]>([]);
const sortBy = ref<SortItem[] | undefined>([{ key: 'createdAt', order: 'asc' }]);
const domains = ref<Domain[]>([]);

const headers = [
    {
        title: 'Name',
        align: 'start',
        key: 'name',
    },
    { title: 'Date Created', align: '', key: 'date' },
];
</script>
