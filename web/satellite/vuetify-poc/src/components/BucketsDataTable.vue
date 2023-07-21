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
                <div>
                    <v-btn
                        class="rounded-lg w-100 pl-1 pr-4 justify-start font-weight-bold"
                        variant="text"
                        height="40"
                        color="default"
                        to="/bucket"
                    >
                        <img src="../assets/icon-bucket-tonal.svg" alt="Bucket" class="mr-3">
                        {{ item.raw.name }}
                    </v-btn>
                </div>
            </template>
        </v-data-table>
    </v-card>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { VCard, VTextField, VBtn } from 'vuetify/components';
import { VDataTable } from 'vuetify/labs/components';

const props = defineProps<{
    headers: unknown[],
    buckets: unknown[],
}>();

const search = ref<string>('');
const selected = ref([]);

const sortBy = [{ key: 'date', order: 'asc' }];
</script>
