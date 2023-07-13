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
            :items="files"
            :search="search"
            class="elevation-1"
            item-key="path"
        >
            <template #item.name="{ item }">
                <v-list-item class="rounded-lg font-weight-bold pl-1" link router-link to="/dashboard">
                    <template #prepend>
                        <img src="../assets/icon-project-tonal.svg" alt="Bucket" class="mr-3">
                        <!-- Project Icon -->
                        <!-- <v-chip :color="getColor(item.raw.role)" variant="tonal" size="small" class="ml-2 mr-2" rounded>
                            <svg width="16" height="16" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
                                <path fill="currentColor" d="M10.0567 1.05318L10.0849 1.06832L17.4768 5.33727C17.6249 5.42279 17.7084 5.5642 17.7274 5.71277L17.7301 5.73906L17.7314 5.76543L17.7316 14.2407C17.7316 14.4124 17.6452 14.5718 17.5032 14.6657L17.476 14.6826L10.084 18.9322C9.93533 19.0176 9.75438 19.0223 9.60224 18.9463L9.57407 18.9311L2.25387 14.6815C2.10601 14.5956 2.01167 14.4418 2.00108 14.2726L2 14.2407V5.77863L2.00023 5.75974C1.99504 5.58748 2.07759 5.41781 2.22993 5.31793L2.25457 5.30275L9.57482 1.06849C9.71403 0.987969 9.88182 0.978444 10.0278 1.03993L10.0567 1.05318ZM16.7123 6.6615L10.3397 10.3418V17.6094L16.7123 13.9458V6.6615ZM3.01944 6.66573V13.947L9.32037 17.605V10.3402L3.01944 6.66573ZM9.83034 2.09828L3.49475 5.76287L9.83122 9.45833L16.2029 5.7786L9.83034 2.09828Z"/>
                            </svg>
                        </v-chip> -->
                    </template>
                    {{ item.columns.name }}
                </v-list-item>
            </template>

            <template #item.role="{ item }">
                <v-chip :color="getColor(item.raw.role)" rounded="xl" size="small" class="font-weight-bold">
                    {{ item.raw.role }}
                </v-chip>
            </template>

            <template #item.actions="{ item }">
                <v-btn size="small" link router-link to="/dashboard">
                    {{ item.raw.actions }}
                </v-btn>
            </template>
        </v-data-table>
    </v-card>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { VCard, VTextField, VListItem, VChip, VBtn } from 'vuetify/components';
import { VDataTable } from 'vuetify/labs/components';

const search = ref<string>('');
const selected = ref([]);

const sortBy = [{ key: 'name', order: 'asc' }];
const headers = [
    {
        title: 'Project',
        align: 'start',
        key: 'name',
    },
    { title: 'Role', key:'role' },
    { title: 'Members', key: 'size' },
    { title: 'Date Added', key: 'date' },
    { title: 'Actions', key: 'actions' },
];
const files = [
    {
        name: 'My First Project',
        role: 'Owner',
        size: '1',
        date: '02 Mar 2023',
        actions: 'Open Project',
    },
    {
        name: 'Storj Labs',
        role: 'Member',
        size: '23',
        date: '21 Apr 2023',
        actions: 'Open Project',
    },
    {
        name: 'Invitation Project',
        role: 'Invited',
        size: '15',
        date: '24 Mar 2023',
        actions: 'Join Project',
    },
];

function getColor(role: string): string {
    if (role === 'Owner') return 'purple2';
    if (role === 'Invited') return 'warning';
    return 'green';
}
</script>
