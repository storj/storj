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
            class="elevation-1" item-key="path" density="comfortable" hover @item-click="handleItemClick"
        >
            <template #item.name="{ item }">
                <div class="text-no-wrap">
                    <v-btn
                        variant="outlined" color="default" size="small" class="mr-1 text-caption" density="comfortable" icon
                        width="24" height="24"
                    >
                        <BucketActionsMenu />
                        <v-icon icon="mdi-dots-horizontal" />
                    </v-btn>
                    <v-chip variant="text" size="small" router-link to="/bucket-details" class="font-weight-bold pl-1 ml-1">
                        <template #prepend>
                            <svg class="mr-2" width="24" height="24" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
                                <rect width="32" height="32" rx="10" />
                                <rect x="0.5" y="0.5" width="31" height="31" rx="7.5" stroke="currentColor" stroke-opacity="0.2" />
                                <path
                                    d="M23.3549 14.5467C24.762 15.9538 24.7748 18.227 23.3778 19.624C22.8117 20.1901 22.1018 20.5247 21.3639 20.6289L21.1106 22.0638C21.092 23.1897 18.7878 24.1 15.9481 24.1C13.1254 24.1 10.8319 23.2006 10.7863 22.0841L10.7858 22.0638L8.84466 11.066C8.82281 10.9882 8.80883 10.9095 8.80304 10.8299L8.8 10.8122L8.8019 10.8123C8.80063 10.7903 8.8 10.7682 8.8 10.746C8.8 9.17422 12.0003 7.90002 15.9481 7.90002C19.8959 7.90002 23.0962 9.17422 23.0962 10.746C23.0962 10.7682 23.0955 10.7903 23.0943 10.8123L23.0962 10.8122L23.093 10.8311C23.0872 10.9098 23.0734 10.9876 23.0519 11.0645L22.5749 13.7666L23.3549 14.5467ZM21.2962 12.6344C19.9867 13.2218 18.076 13.592 15.9481 13.592C13.8203 13.592 11.9096 13.2219 10.6001 12.6344L12.0072 20.6077L12.2373 21.8286L12.2586 21.8452C12.3789 21.9354 12.5652 22.0371 12.807 22.1351L12.8561 22.1546C13.6355 22.4594 14.7462 22.6439 15.9481 22.6439C17.1569 22.6439 18.2733 22.4573 19.0528 22.1497C19.3337 22.0388 19.5431 21.9223 19.6661 21.8231L19.6761 21.8148L19.9019 20.5348C19.3338 20.3787 18.7955 20.0812 18.3429 19.6429L18.3004 19.6011L15.3749 16.6756C15.0906 16.3913 15.0906 15.9303 15.3749 15.646C15.6523 15.3686 16.0978 15.3618 16.3834 15.6257L16.4045 15.646L19.33 18.5715C19.5717 18.8132 19.8555 18.9861 20.1569 19.0901L21.2962 12.6344ZM22.2661 15.517L21.6408 19.0597C21.8989 18.9575 22.1402 18.8024 22.3482 18.5944C23.1641 17.7784 23.1664 16.4494 22.355 15.6065L22.3253 15.5763L22.2661 15.517ZM15.9481 9.35612C14.2013 9.35612 12.5813 9.62893 11.4322 10.0864C10.9385 10.283 10.5712 10.4995 10.3598 10.6985C10.3463 10.7112 10.334 10.7232 10.3228 10.7347L10.3122 10.7459L10.3314 10.7661L10.3598 10.7936C10.5712 10.9926 10.9385 11.2091 11.4322 11.4056C12.5813 11.8631 14.2013 12.1359 15.9481 12.1359C17.6949 12.1359 19.3149 11.8631 20.4639 11.4056C20.9577 11.2091 21.325 10.9926 21.5364 10.7936C21.5499 10.7809 21.5622 10.7688 21.5733 10.7574L21.5841 10.7459L21.5647 10.726L21.5364 10.6985C21.325 10.4995 20.9577 10.283 20.4639 10.0864C19.3149 9.62893 17.6949 9.35612 15.9481 9.35612Z"
                                    fill="currentColor"
                                />
                            </svg>
                        </template>
                        {{ item.raw.name }}
                    </v-chip>
                </div>
            </template>

            <template #item.placement="{ item }">
                <v-chip variant="tonal" color="default" size="small" rounded="lg" class="text-capitalize">
                    {{ item.raw.placement }}
                </v-chip>
            </template>

            <template #item.agent="{ item }">
                <v-chip variant="tonal" color="default" size="small" rounded="lg" @click="setSearch(item.raw.agent)">
                    {{ item.raw.agent }}
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

<script lang="ts">
import { VCard, VTextField, VBtn, VIcon, VChip } from 'vuetify/components';
import { VDataTable } from 'vuetify/labs/components';

import BucketActionsMenu from '@/components/BucketActionsMenu.vue';

export default {
    name: 'BucketsTableComponent',
    components: {
        VCard,
        VTextField,
        VBtn,
        VIcon,
        VChip,
        VDataTable,
        BucketActionsMenu,
    },
    data() {
        return {
            // search in the table
            search: '',
            selected: [],
            sortBy: [{ key: 'name', order: 'asc' }],
            headers: [
                { title: 'Bucket', key: 'name' },
                { title: 'Storage', key: 'storage' },
                { title: 'Download', key: 'download' },
                { title: 'Segments', key: 'segments' },
                { title: 'Placement', key: 'placement' },
                { title: 'Value', key: 'agent' },
                { title: 'Created', key: 'date' },
            ],
            files: [
                {
                    name: 'First',
                    placement: 'global',
                    bucketid: '1Q284JF',
                    storage: '300TB',
                    download: '100TB',
                    segments: '23,456',
                    agent: 'Test Agent',
                    date: '02 Mar 2023',
                },
                {
                    name: 'Personal',
                    placement: 'global',
                    bucketid: '82SR21Q',
                    storage: '30TB',
                    download: '10TB',
                    segments: '123,456',
                    agent: 'Agent',
                    date: '21 Apr 2023',
                },
                {
                    name: 'Invitation',
                    placement: 'global',
                    bucketid: '4JFF82S',
                    storage: '500TB',
                    download: '200TB',
                    segments: '456',
                    agent: 'Random',
                    date: '24 Mar 2023',
                },
                {
                    name: 'Videos',
                    placement: 'global',
                    bucketid: '1Q223JA',
                    storage: '300TB',
                    download: '100TB',
                    segments: '3,456',
                    agent: 'Test Agent',
                    date: '11 Mar 2023',
                },
                {
                    name: 'App',
                    placement: 'global',
                    bucketid: 'R21Q284',
                    storage: '300TB',
                    download: '100TB',
                    segments: '56',
                    agent: 'Test Agent',
                    date: '11 Mar 2023',
                },
                {
                    name: 'Backup',
                    placement: 'global',
                    bucketid: '42SR20S',
                    storage: '30TB',
                    download: '10TB',
                    segments: '1,456',
                    agent: 'Agent',
                    date: '21 Apr 2023',
                },
                {
                    name: 'My Bucket',
                    placement: 'global',
                    bucketid: '4JFF8FF',
                    storage: '500TB',
                    download: '200TB',
                    segments: '6',
                    agent: 'Random',
                    date: '24 Mar 2023',
                },
                {
                    name: 'Sync',
                    placement: 'global',
                    bucketid: '4JFF8ZZ',
                    storage: '500TB',
                    download: '200TB',
                    segments: '3,123,456',
                    agent: 'Random',
                    date: '24 Mar 2023',
                },
                {
                    name: 'Backupss',
                    placement: 'global',
                    bucketid: '4JFF8TS',
                    storage: '500TB',
                    download: '200TB',
                    segments: '10,123,456',
                    agent: 'Random',
                    date: '24 Mar 2023',
                },
                {
                    name: 'Destiny',
                    placement: 'global',
                    bucketid: '4IF42TM',
                    storage: '500TB',
                    download: '200TB',
                    segments: '3,456',
                    agent: 'Random',
                    date: '29 Mar 2023',
                },
            ],
        };
    },
    methods: {
        setSearch(searchText) {
            this.search = searchText;
        },
    },
};
</script>
