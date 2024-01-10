// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card variant="flat" :border="true" rounded="xlg">
        <v-text-field
            v-model="search" label="Search" prepend-inner-icon="mdi-magnify" single-line variant="solo-filled" flat
            hide-details clearable density="compact" rounded="lg" class="mx-2 mt-2"
        />

        <v-data-table
            v-model="selected"
            v-model:sort-by="sortBy"
            :headers="headers"
            :items="projects"
            :search="search"
            class="elevation-1"
            density="comfortable"
            hover
        >
            <template #item.name="{ item }: ProjectTableSlotProps">
                <div class="text-no-wrap">
                    <v-btn
                        variant="outlined" color="default" size="small" class="mr-1 text-caption" density="comfortable" icon
                        width="24" height="24"
                    >
                        <ProjectActionsMenu />
                        <v-icon icon="mdi-dots-horizontal" />
                    </v-btn>
                    <v-chip
                        variant="text" color="default" size="small"
                        class="font-weight-bold pl-1 ml-1"
                        @click="selectProject(item.raw.id)"
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
                        {{ item.raw.name }}
                    </v-chip>
                </div>
            </template>

            <template #item.storage.percent="{ item }: ProjectTableSlotProps">
                <v-chip
                    v-if="item.raw.storage.percent !== null"
                    variant="tonal"
                    :color="getPercentColor(item.raw.storage.percent)"
                    size="small"
                    rounded="lg"
                    class="font-weight-bold"
                >
                    {{ item.raw.storage.percent }}&percnt;
                </v-chip>
                <v-icon v-else icon="mdi-alert-circle-outline" color="error" />
            </template>

            <template #item.storage.used="{ item }: ProjectTableSlotProps">
                <template v-if="item.raw.storage.used !== null">
                    {{ sizeToBase10String(item.raw.storage.used) }}
                </template>
                <v-icon v-else icon="mdi-alert-circle-outline" color="error" />
            </template>

            <template #item.storage.limit="{ item }: ProjectTableSlotProps">
                {{ sizeToBase10String(item.raw.storage.limit) }}
            </template>

            <template #item.download.percent="{ item }: ProjectTableSlotProps">
                <v-chip
                    variant="tonal"
                    :color="getPercentColor(item.raw.download.percent)"
                    size="small"
                    rounded="lg"
                    class="font-weight-bold"
                >
                    {{ item.raw.download.percent }}&percnt;
                </v-chip>
            </template>

            <template #item.download.used="{ item }: ProjectTableSlotProps">
                {{ sizeToBase10String(item.raw.download.used) }}
            </template>

            <template #item.download.limit="{ item }: ProjectTableSlotProps">
                {{ sizeToBase10String(item.raw.download.limit) }}
            </template>

            <template #item.segment.percent="{ item }: ProjectTableSlotProps">
                <v-tooltip>
                    {{ item.raw.segment.used !== null ? item.raw.segment.used.toLocaleString() + '/' : 'Limit:' }}
                    {{ item.raw.segment.limit.toLocaleString() }}
                    <template #activator="{ props }">
                        <v-chip
                            v-if="item.raw.segment.percent !== null"
                            v-bind="props"
                            variant="tonal"
                            :color="getPercentColor(item.raw.segment.percent)"
                            size="small"
                            rounded="lg"
                            class="font-weight-bold"
                        >
                            {{ item.raw.segment.percent }}&percnt;
                        </v-chip>
                        <v-icon v-else icon="mdi-alert-circle-outline" color="error" v-bind="props" />
                    </template>
                </v-tooltip>
            </template>

            <template #item.id="{ item }: ProjectTableSlotProps">
                <div class="text-caption text-no-wrap text-uppercase">{{ item.raw.id }}</div>
            </template>

            <!--
            <template #item.agent="{ item }">
                <v-chip variant="tonal" color="default" size="small" rounded="lg" @click="search = item.raw.agent">
                    {{ item.raw.agent }}
                </v-chip>
            </template>

            <template #item.date="{ item }">
                <span class="text-no-wrap">
                    {{ item.raw.date }}
                </span>
            </template>
            -->
        </v-data-table>
    </v-card>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { useRouter } from 'vue-router';
import { VCard, VTextField, VBtn, VIcon, VTooltip, VChip } from 'vuetify/components';
import { VDataTable } from 'vuetify/labs/components';

import { useAppStore } from '@/store/app';
import { sizeToBase10String } from '@/utils/memory';

import ProjectActionsMenu from '@/components/ProjectActionsMenu.vue';

type UsageStats = {
    used: number | null;
    limit: number;
    percent: number | null;
};

type RequiredUsageStats = {
    [K in keyof UsageStats]: NonNullable<UsageStats[K]>;
};

type ProjectTableItem = {
    id: string;
    name: string;
    storage: UsageStats;
    download: RequiredUsageStats;
    segment: UsageStats;
};

type ProjectTableSlotProps = { item: { raw: ProjectTableItem } };

const search = ref<string>('');
const selected = ref<string[]>([]);
const sortBy = ref([{ key: 'name', order: 'asc' }]);
const router = useRouter();

const headers = [
    { title: 'Name', key: 'name' },
    { title: 'Storage Used', key: 'storage.percent' },
    { title: 'Storage Used', key: 'storage.used' },
    { title: 'Storage Limit', key: 'storage.limit' },
    { title: 'Download Used', key: 'download.percent' },
    { title: 'Download Used', key: 'download.used' },
    { title: 'Download Limit', key: 'download.limit' },
    { title: 'Segments Used', key: 'segment.percent' },
    { title: 'Project ID', key: 'id', align: 'start' },
    // { title: 'Value Attribution', key: 'agent' },
    // { title: 'Date Created', key: 'date' },
];

const appStore = useAppStore();

/**
 * Returns the user's project usage data.
 */
const projects = computed<ProjectTableItem[]>(() => {
    function makeUsageStats(used: number, limit: number): RequiredUsageStats;
    function makeUsageStats(used: number | null, limit: number): UsageStats;
    function makeUsageStats(used: number | null, limit: number) {
        return {
            used,
            limit,
            percent: used !== null ? Math.round(used * 100 / limit) : null,
        };
    }

    const projects = appStore.state.userAccount?.projects;
    if (!projects || !projects.length) {
        return [];
    }

    return projects.map<ProjectTableItem>(project => ({
        id: project.id,
        name: project.name,
        storage: makeUsageStats(project.storageUsed, project.storageLimit),
        download: makeUsageStats(project.bandwidthUsed, project.bandwidthLimit),
        segment: makeUsageStats(project.segmentUsed, project.segmentLimit),
    }));
});

/**
* Selects the project and navigates to the project dashboard.
*/
async function selectProject(id: string): Promise<void> {
    await appStore.selectProject(id);
    router.push('/project-details');
}

function getPercentColor(percent: number) {
    if (percent >= 99) {
        return 'error';
    } else if (percent >= 80) {
        return 'warning';
    } else {
        return 'success';
    }
}
</script>
