// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card variant="flat" :border="true" rounded="xlg">
        <v-text-field
            v-model="search" label="Search" prepend-inner-icon="mdi-magnify" single-line variant="solo-filled" flat
            hide-details clearable density="compact" rounded="lg" class="mx-2 mt-2"
        />

        <v-data-table
            v-model="selected" :sort-by="sortBy" :headers="headers" :items="files" :search="search"
            class="elevation-1" density="comfortable" hover
        >
            <template #item.projectid="{ item }">
                <div class="text-no-wrap">
                    <v-btn
                        variant="outlined" color="default" size="small" class="mr-1 text-caption" density="comfortable" icon
                        width="24" height="24"
                    >
                        <!--                        <ProjectActionsMenu />-->
                        <v-icon icon="mdi-dots-horizontal" />
                    </v-btn>
                    <v-chip
                        variant="text" color="default" size="small" router-link to="/project-details"
                        class="font-weight-medium pl-1 ml-1"
                    >
                        <template #prepend>
                            <svg class="mr-2" width="24" height="24" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
                                <rect x="0.5" y="0.5" width="31" height="31" rx="10" stroke="currentColor" stroke-opacity="0.2" />
                                <path
                                    d="M16.2231 7.08668L16.2547 7.10399L23.4149 11.2391C23.6543 11.3774 23.7829 11.6116 23.8006 11.8529L23.8021 11.8809L23.8027 11.9121V20.1078C23.8027 20.3739 23.6664 20.6205 23.4432 20.7624L23.4136 20.7803L16.2533 24.8968C16.0234 25.029 15.7426 25.0342 15.5088 24.9125L15.4772 24.8951L8.38642 20.7787C8.15725 20.6457 8.01254 20.4054 8.00088 20.1422L8 20.1078L8.00026 11.8975L8 11.8738C8.00141 11.6177 8.12975 11.3687 8.35943 11.2228L8.38748 11.2058L15.4783 7.10425C15.697 6.97771 15.9622 6.96636 16.1893 7.07023L16.2231 7.08668ZM22.251 13.2549L16.6424 16.4939V22.8832L22.251 19.6588V13.2549ZM9.55175 13.2614V19.6611L15.0908 22.8766V16.4916L9.55175 13.2614ZM15.8669 8.67182L10.2916 11.8967L15.8686 15.149L21.4755 11.9109L15.8669 8.67182Z"
                                    fill="currentColor"
                                />
                            </svg>
                        </template>
                        {{ item.projectid }}
                    </v-chip>
                </div>
            </template>

            <template #item.name="{ item }">
                {{ item.name }}
            </template>

            <template #item.email="{ item }">
                <v-chip variant="tonal" color="default" size="small" rounded="lg" @click="setSearch(item.email)">
                    {{ item.email }}
                </v-chip>
            </template>

            <template #item.agent="{ item }">
                <v-chip variant="tonal" color="default" size="small" rounded="lg" @click="setSearch(item.agent)">
                    {{ item.agent }}
                </v-chip>
            </template>

            <template #item.placement="{ item }">
                <v-chip variant="tonal" color="default" size="small" rounded="lg" @click="setSearch(item.placement)">
                    {{ item.placement }}
                </v-chip>
            </template>

            <template #item.storagepercent="{ item }">
                <v-chip
                    variant="tonal" :color="getPercentColor(item.storagepercent)" size="small" rounded="lg"
                    class="font-weight-bold"
                >
                    {{ item.storagepercent }}&percnt;
                </v-chip>
            </template>

            <template #item.downloadpercent="{ item }">
                <v-chip
                    variant="tonal" :color="getPercentColor(item.downloadpercent)" size="small" rounded="lg"
                    class="font-weight-bold"
                >
                    {{ item.downloadpercent }}&percnt;
                </v-chip>
            </template>

            <template #item.segmentpercent="{ item }">
                <v-tooltip text="430,000 / 1,000,000">
                    <template #activator="{ props }">
                        <v-chip
                            v-bind="props" variant="tonal" :color="getPercentColor(item.segmentpercent)" size="small"
                            rounded="lg" class="font-weight-bold"
                        >
                            {{ item.segmentpercent }}&percnt;
                        </v-chip>
                    </template>
                </v-tooltip>
            </template>

            <template #item.date="{ item }">
                <span class="text-no-wrap">
                    {{ item.date }}
                </span>
            </template>
        </v-data-table>
    </v-card>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import {
    VCard,
    VTextField,
    VBtn,
    VIcon,
    VChip,
    VTooltip,
    VDataTable,
} from 'vuetify/components';

import { DataTableHeader, SortItem } from '@/types/common';

const search = ref<string>('');
const selected = ref<string[]>([]);
const sortBy: SortItem[] = [{ key: 'name', order: 'asc' }];

const headers: DataTableHeader[] = [
    { title: 'Project ID', key: 'projectid', align: 'start' },
    // { title: 'Name', key: 'name'},
    { title: 'Account', key: 'email' },
    { title: 'Storage Used', key: 'storagepercent' },
    { title: 'Storage Used', key: 'storageused' },
    { title: 'Storage Limit', key: 'storagelimit' },
    { title: 'Download Used', key: 'downloadpercent' },
    { title: 'Download Used', key: 'downloadused' },
    { title: 'Download Limit', key: 'downloadlimit' },
    { title: 'Segments Used', key: 'segmentpercent' },
    { title: 'Value', key: 'agent' },
    { title: 'Placement', key: 'placement' },
    { title: 'Created', key: 'date' },
];
const files = [
    {
        name: 'My First Project',
        email: 'vduke@gmail.com',
        projectid: 'F82SR21Q284JF',
        storageused: '24 TB',
        storagelimit: '30 TB',
        storagepercent: '80',
        downloadused: '7 TB',
        downloadlimit: '100 TB',
        segmentpercent: '20',
        downloadpercent: '7',
        placement: 'Global',
        agent: 'Test Agent',
        date: '02 Mar 2023',
    },
    {
        name: 'Personal Project',
        email: 'knowles@aurora.io',
        projectid: '284JFF82SR21Q',
        storageused: '150 TB',
        storagelimit: '300 TB',
        storagepercent: '50',
        downloadused: '100 TB',
        downloadlimit: '100 TB',
        downloadpercent: '100',
        segmentpercent: '43',
        placement: 'Global',
        agent: 'Agent',
        date: '21 Apr 2023',
    },
    {
        name: 'Invitation Project',
        email: 'sctrevis@gmail.com',
        projectid: 'R21Q284JFF82S',
        storageused: '99 TB',
        storagelimit: '100 TB',
        storagepercent: '99',
        downloadused: '85 TB',
        downloadlimit: '100 TB',
        segmentpercent: '83',
        downloadpercent: '85',
        placement: 'Global',
        agent: 'Random',
        date: '24 Mar 2023',
    },
    {
        name: 'Videos',
        email: 'vduke@gmail.com',
        projectid: '482SR21Q223JA',
        storageused: '24 TB',
        storagelimit: '30 TB',
        storagepercent: '80',
        downloadused: '7 TB',
        downloadlimit: '100 TB',
        segmentpercent: '20',
        downloadpercent: '7',
        placement: 'Global',
        agent: 'Test Agent',
        date: '11 Mar 2023',
    },
    {
        name: 'App',
        email: 'vduke@gmail.com',
        projectid: '56F82SR21Q284',
        storageused: '150 TB',
        storagelimit: '300 TB',
        storagepercent: '50',
        downloadused: '100 TB',
        downloadlimit: '100 TB',
        downloadpercent: '100',
        segmentpercent: '43',
        placement: 'Global',
        agent: 'Test Agent',
        date: '11 Mar 2023',
    },
    {
        name: 'Backup',
        email: 'knowles@aurora.io',
        projectid: '624QXF42SR20S',
        storageused: '99 TB',
        storagelimit: '100 TB',
        storagepercent: '99',
        downloadused: '85 TB',
        downloadlimit: '100 TB',
        segmentpercent: '83',
        downloadpercent: '85',
        placement: 'Global',
        agent: 'Agent',
        date: '21 Apr 2023',
    },
    {
        name: 'My Project',
        email: 'sctrevis@gmail.com',
        projectid: 'P33Q284JFF8FF',
        storageused: '24 TB',
        storagelimit: '30 TB',
        storagepercent: '80',
        downloadused: '7 TB',
        downloadlimit: '100 TB',
        segmentpercent: '20',
        downloadpercent: '7',
        placement: 'Global',
        agent: 'Random',
        date: '24 Mar 2023',
    },
    {
        name: 'Sync',
        email: 'sctrevis@gmail.com',
        projectid: 'W22S284JFF8ZZ',
        storageused: '150 TB',
        storagelimit: '300 TB',
        storagepercent: '50',
        downloadused: '100 TB',
        downloadlimit: '100 TB',
        downloadpercent: '100',
        segmentpercent: '43',
        placement: 'Global',
        agent: 'Random',
        date: '24 Mar 2023',
    },
    {
        name: 'Backupss',
        email: 'destiny@gmail.com',
        projectid: '2SFX284JFF8TS',
        storageused: '99 TB',
        storagelimit: '100 TB',
        storagepercent: '99',
        downloadused: '85 TB',
        downloadlimit: '100 TB',
        segmentpercent: '83',
        downloadpercent: '85',
        placement: 'Global',
        agent: 'Random',
        date: '24 Mar 2023',
    },
    {
        name: 'Destiny',
        email: 'destiny@gmail.com',
        projectid: 'FGXZ484IF42TM',
        storageused: '24 TB',
        storagelimit: '30 TB',
        storagepercent: '80',
        downloadused: '7 TB',
        downloadlimit: '100 TB',
        segmentpercent: '20',
        downloadpercent: '7',
        placement: 'Global',
        agent: 'Random',
        date: '29 Mar 2023',
    },
];

function setSearch(searchText: string) {
    search.value = searchText;
}

function getPercentColor(p: number | string) {
    let percent = 0;
    if (typeof p === 'string')
        percent = parseInt(p, 10);
    else
        percent = p;
    if (percent >= 99) {
        return 'error';
    } else if (percent >= 80) {
        return 'warning';
    } else {
        return 'success';
    }
}
</script>
