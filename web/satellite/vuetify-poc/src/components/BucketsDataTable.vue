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
            :items="buckets"
            :search="search"
            class="elevation-1"
            show-select
        >
            <template #item.name="{ item }">
                <v-list-item class="rounded-lg font-weight-bold pl-1" link router-link to="/bucket">
                    <template #prepend>
                        <img src="../assets/icon-bucket-tonal.svg" alt="Bucket" class="mr-3">
                    </template>
                    {{ item.columns.name }}
                </v-list-item>
            </template>
        </v-data-table>
    </v-card>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { VCard, VTextField, VListItem } from 'vuetify/components';
import { VDataTable } from 'vuetify/labs/components';

const props = defineProps<{
    headers: unknown[],
    buckets: unknown[],
}>();

const search = ref<string>('');
const selected = ref([]);

const sortBy = [{ key: 'date', order: 'asc' }];
</script>
