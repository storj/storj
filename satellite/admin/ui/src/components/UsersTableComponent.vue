// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card variant="flat" :border="true" rounded="xlg">
        <v-text-field
            v-model="search" label="Search" prepend-inner-icon="mdi-magnify" single-line variant="solo-filled" flat
            hide-details clearable density="compact" rounded="lg" class="mx-2 mt-2"
        />

        <v-data-table
            v-model="selected" :sort-by="sortBy" :headers="headers" :items="users" :search="search"
            density="comfortable" hover
        >
            <template #item.email="{ item }">
                <div class="text-no-wrap">
                    <v-btn
                        variant="outlined" color="default" size="small" class="mr-1 text-caption" density="comfortable" icon
                        width="24" height="24"
                    >
                        <!--                        <AccountActionsMenu />-->
                        <v-icon icon="mdi-dots-horizontal" />
                    </v-btn>
                    <v-chip
                        variant="text" color="default" size="small" router-link to="/account-details"
                        class="font-weight-bold pl-1 ml-1"
                    >
                        <template #prepend>
                            <svg class="mr-2" width="24" height="24" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
                                <rect x="0.5" y="0.5" width="31" height="31" rx="10" stroke="currentColor" stroke-opacity="0.2" />
                                <path
                                    d="M16.0695 8.34998C18.7078 8.34998 20.8466 10.4888 20.8466 13.1271C20.8466 14.7102 20.0765 16.1134 18.8905 16.9826C21.2698 17.8565 23.1437 19.7789 23.9536 22.1905C23.9786 22.265 24.0026 22.34 24.0256 22.4154L24.0593 22.5289C24.2169 23.0738 23.9029 23.6434 23.3579 23.801C23.2651 23.8278 23.1691 23.8414 23.0725 23.8414H8.91866C8.35607 23.8414 7.89999 23.3853 7.89999 22.8227C7.89999 22.7434 7.90926 22.6644 7.92758 22.5873L7.93965 22.5412C7.97276 22.4261 8.00827 22.3119 8.04612 22.1988C8.86492 19.7523 10.7783 17.8081 13.2039 16.9494C12.0432 16.0781 11.2924 14.6903 11.2924 13.1271C11.2924 10.4888 13.4312 8.34998 16.0695 8.34998ZM16.0013 17.9724C13.1679 17.9724 10.6651 19.7017 9.62223 22.264L9.59178 22.34H22.4107L22.4102 22.3388C21.3965 19.7624 18.9143 18.0092 16.0905 17.973L16.0013 17.9724ZM16.0695 9.85135C14.2604 9.85135 12.7938 11.3179 12.7938 13.1271C12.7938 14.9362 14.2604 16.4028 16.0695 16.4028C17.8786 16.4028 19.3452 14.9362 19.3452 13.1271C19.3452 11.3179 17.8786 9.85135 16.0695 9.85135Z"
                                    fill="currentColor"
                                />
                            </svg>
                        </template>
                        {{ item.email }}
                    </v-chip>
                </div>
            </template>

            <template #item.type="{ item }">
                <v-chip
                    :color="getColor(item.type)" variant="tonal" size="small" rounded="lg" class="font-weight-bold"
                    @click="setSearch(item.type)"
                >
                    {{ item.type }}
                </v-chip>
            </template>

            <template #item.role="{ item }">
                <v-chip variant="tonal" color="default" size="small" @click="setSearch(item.role)">
                    {{ item.role }}
                </v-chip>
            </template>

            <template #item.agent="{ item }">
                <v-chip variant="tonal" color="default" size="small" @click="setSearch(item.agent)">
                    {{ item.agent }}
                </v-chip>
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
import { VCard, VDataTable, VTextField, VBtn, VIcon, VChip } from 'vuetify/components';

import { DataTableHeader, SortItem } from '@/types/common';

const search = ref<string>('');
const selected = ref<string[]>([]);
const sortBy: SortItem[] = [{ key: 'name', order: 'asc' }];

const headers: DataTableHeader[] = [
    { title: 'Account', key: 'email' },
    { title: 'Name', key: 'name' },
    // { title: 'ID', key: 'userid' },
    // { title: 'Type', key: 'type' },
    { title: 'Role', key: 'role' },
    { title: 'Value', key: 'agent' },
    { title: 'Date Added', key: 'date', align: 'start' },
];
const users = [
    {
        name: 'Magnolia Queen',
        date: '12 Mar 2023',
        type: 'Enterprise',
        role: 'Member',
        agent: 'Company',
        email: 'magnolia@queen.com',
        userid: '2521',
    },
    {
        name: 'Irving Thacker',
        date: '30 May 2023',
        type: 'Pro',
        role: 'Owner',
        agent: 'Company',
        email: 'itacker@gmail.com',
        userid: '41',
    },
    {
        name: 'Brianne Deighton',
        date: '20 Mar 2023',
        type: 'Free',
        role: 'Member',
        projects: '1',
        storage: '1,000 TB',
        download: '3,000 TB',
        agent: 'Company',
        email: 'bd@rianne.net',
        userid: '87212',
    },
    {
        name: 'Vince Duke',
        date: '4 Apr 2023',
        type: 'Priority',
        role: 'Member',
        agent: 'User Agent',
        email: 'vduke@gmail.com',
        userid: '53212',
    },
    {
        name: 'Luvenia Breckinridge',
        date: '11 Jan 2023',
        type: 'Free',
        role: 'Member',
        agent: 'User Agent',
        email: 'lbreckinridge42@gmail.com',
        userid: '1244',
    },
    {
        name: 'Georgia Jordan',
        date: '15 Mar 2023',
        type: 'Pro',
        role: 'Member',
        agent: 'Agent',
        email: 'georgia@jordan.net',
        userid: '55524',
    },
];

function getColor(type: string) {
    if (type === 'Enterprise') return 'purple';
    if (type === 'Priority') return 'secondary';
    if (type === 'Pro') return 'success';
    if (type === 'Suspended') return 'warning';
    return 'default';
}

function setSearch(searchText: string) {
    search.value = searchText;
}
</script>