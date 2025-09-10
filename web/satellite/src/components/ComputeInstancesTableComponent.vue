// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card variant="outlined" border rounded="xlg">
        <v-text-field
            v-model="search"
            label="Search"
            :prepend-inner-icon="Search"
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
            :items="instances"
            :search="search"
            item-value="title"
            hover
        >
            <template #item.title="{ item }">
                <v-list-item class="font-weight-bold pl-0" density="compact">
                    <template #prepend>
                        <v-icon :icon="Computer" color="primary" :size="18" class="mr-2" />
                    </template>
                    {{ item.title }}
                </v-list-item>
            </template>

            <template #item.name="{ item }">
                <v-list-item class="pl-0">
                    {{ item.gpu }}
                </v-list-item>
                <v-list-item class="pl-0 mt-n4 text-caption text-medium-emphasis">
                    {{ item.cpu }} | {{ item.ram }} | {{ item.storage }}
                </v-list-item>
            </template>

            <template #item.status="{ item }">
                <div class="text-no-wrap">
                    <v-chip
                        variant="tonal"
                        :color="getStatusColor(item.status)"
                        size="small"
                        rounded-lg
                        class="font-weight-bold"
                    >
                        <template #prepend>
                            <v-icon :icon="getStatusIcon(item.status)" size="small" class="mr-2" />
                        </template>
                        {{ item.status }}
                    </v-chip>
                </div>
            </template>

            <template #item.actions>
                <v-btn
                    variant="outlined"
                    color="default"
                    class="mr-1 text-caption"
                    density="comfortable"
                    :append-icon="ArrowRight"
                    router
                >
                    View
                </v-btn>
                <v-menu>
                    <template #activator="{ props }">
                        <v-btn :icon="EllipsisVertical" v-bind="props" variant="text" />
                    </template>
                    <v-list>
                        <v-list-item>
                            <v-list-item-title>Start</v-list-item-title>
                        </v-list-item>
                        <v-list-item>
                            <v-list-item-title>Stop</v-list-item-title>
                        </v-list-item>
                        <v-list-item>
                            <v-list-item-title>Restart</v-list-item-title>
                        </v-list-item>
                    </v-list>
                </v-menu>
            </template>
        </v-data-table>
    </v-card>
</template>

<script setup lang="ts">
import { FunctionalComponent, ref } from 'vue';
import {
    VCard,
    VDataTable,
    VTextField,
    VListItem,
    VChip,
    VIcon,
    VBtn,
    VMenu,
    VList,
    VListItemTitle,
} from 'vuetify/components';
import {
    ArrowRight,
    Computer,
    EllipsisVertical,
    Search,
    CheckCircle,
    StopCircle,
    Cog,
    HelpCircle,
} from 'lucide-vue-next';

import { DataTableHeader } from '@/types/common';

const headers: DataTableHeader[] = [
    { title: 'Name', key: 'title' },
    { title: 'Status', key: 'status' },
    { title: 'Configuration', key: 'name' },
    { title: 'Location', key: 'location' },
    { title: '', key: 'actions', align: 'end', sortable: false },
];
const instances = [
    {
        title: 'AI-VM-1',
        gpu: '6x NVIDIA H100 SXM5 80GB',
        cpu: 'Intel Xeon Platinum 8470 (52 vCPUs)',
        storage: '6140 GB NVMe SSD',
        ram: '488 GB',
        location: 'Norway',
        status: 'Online',
    },
    {
        title: 'Production-VM-1',
        gpu: '4x NVIDIA H100 SXM5 80GB',
        cpu: 'Intel Xeon Platinum 8470 (32 vCPUs)',
        storage: '888 GB NVMe SSD',
        ram: '320 GB',
        location: 'Canada',
        status: 'Online',
    },
    {
        title: 'Test-VM-1',
        gpu: '10x NVIDIA H100 SXM5 80GB',
        cpu: 'Intel Xeon Platinum 8470 (80 vCPUs)',
        storage: '3240 GB NVMe SSD',
        ram: '820 GB',
        location: 'USA',
        status: 'Offline',
    },
    {
        title: 'Dev-VM-4',
        gpu: '1x NVIDIA A100',
        cpu: 'AMD EPYC 7763 (16 vCPUs)',
        storage: '512 GB NVMe SSD',
        ram: '128 GB',
        location: 'Germany',
        status: 'Building',
    },
];

const search = ref<string>('');
const selected = ref([]);

function getStatusColor(status: string): string {
    if (status === 'Online') return 'success';
    if (status === 'Offline') return 'default';
    if (status === 'Building') return 'info';
    return 'default';
}

function getStatusIcon(status: string): FunctionalComponent {
    if (status === 'Online') return CheckCircle;
    if (status === 'Offline') return StopCircle;
    if (status === 'Building') return Cog;
    return HelpCircle;
}
</script>

<style scoped lang="scss">
.v-list-item :deep(.v-list-item__prepend .v-list-item__spacer) {
    display: none;
}
</style>
