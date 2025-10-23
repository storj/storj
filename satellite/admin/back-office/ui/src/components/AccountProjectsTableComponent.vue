// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card variant="flat" :border="true" rounded="xlg">
        <v-data-table
            v-model="selected"
            :sort-by="sortBy"
            :headers="headers"
            :items="projects"
            :search="search"
            class="border-0"
            density="comfortable"
            hover
        >
            <template #top>
                <v-text-field
                    v-model="search" label="Search" :prepend-inner-icon="Search" single-line variant="solo-filled" flat
                    hide-details clearable density="compact" rounded="lg" class="mx-2 mt-2 mb-2"
                />
            </template>
            <template #item.name="{ item }: ProjectTableSlotProps">
                <div class="text-no-wrap">
                    <v-btn
                        variant="outlined" color="default" size="small" class="mr-1 text-caption" density="comfortable" icon
                        width="24" height="24"
                    >
                        <ProjectActionsMenu
                            :project-id="item.id" :owner="item.owner"
                            @update-limits="onUpdateLimitsClicked"
                        />
                        <v-icon :icon="MoreHorizontal" />
                    </v-btn>
                    <v-chip
                        variant="text" color="default" size="small"
                        class="font-weight-bold pl-1 ml-1"
                        @click="selectProject(item.id)"
                    >
                        <template #prepend>
                            <v-icon :icon="Box" size="16" class="mr-1" />
                        </template>
                        {{ item.name }}
                    </v-chip>
                </div>
            </template>

            <template #item.storage.percent="{ item }: ProjectTableSlotProps">
                <v-chip
                    v-if="item.storage.percent !== null"
                    variant="tonal"
                    :color="getPercentColor(item.storage.percent)"
                    size="small"
                    rounded="lg"
                    class="font-weight-bold"
                >
                    {{ item.storage.percent }}&percnt;
                </v-chip>
                <v-icon v-else :icon="AlertCircle" color="error" />
            </template>

            <template #item.storage.used="{ item }: ProjectTableSlotProps">
                <template v-if="item.storage.used !== null">
                    {{ sizeToBase10String(item.storage.used) }}
                </template>
                <v-icon v-else :icon="AlertCircle" color="error" />
            </template>

            <template #item.storage.limit="{ item }: ProjectTableSlotProps">
                {{ sizeToBase10String(item.storage.limit) }}
            </template>

            <template #item.download.percent="{ item }: ProjectTableSlotProps">
                <v-chip
                    variant="tonal"
                    :color="getPercentColor(item.download.percent)"
                    size="small"
                    rounded="lg"
                    class="font-weight-bold"
                >
                    {{ item.download.percent }}&percnt;
                </v-chip>
            </template>

            <template #item.download.used="{ item }: ProjectTableSlotProps">
                {{ sizeToBase10String(item.download.used) }}
            </template>

            <template #item.download.limit="{ item }: ProjectTableSlotProps">
                {{ sizeToBase10String(item.download.limit) }}
            </template>

            <template #item.segment.percent="{ item }: ProjectTableSlotProps">
                <v-tooltip>
                    {{ item.segment.used !== null ? item.segment.used.toLocaleString() + '/' : 'Limit:' }}
                    {{ item.segment.limit.toLocaleString() }}
                    <template #activator="{ props: activatorProps }">
                        <v-chip
                            v-if="item.segment.percent !== null"
                            v-bind="activatorProps"
                            variant="tonal"
                            :color="getPercentColor(item.segment.percent)"
                            size="small"
                            rounded="lg"
                            class="font-weight-bold"
                        >
                            {{ item.segment.percent }}&percnt;
                        </v-chip>
                        <v-icon v-else :icon="AlertCircle" color="error" v-bind="activatorProps" />
                    </template>
                </v-tooltip>
            </template>

            <template #item.id="{ item }: ProjectTableSlotProps">
                <div class="text-caption text-no-wrap text-uppercase">{{ item.id }}</div>
            </template>

            <!--
            <template #item.agent="{ item }">
                <v-chip variant="tonal" color="default" size="small" rounded="lg" @click="search = item.agent">
                    {{ item.agent }}
                </v-chip>
            </template>

            <template #item.date="{ item }">
                <span class="text-no-wrap">
                    {{ item.date }}
                </span>
            </template>
            -->
        </v-data-table>
    </v-card>

    <ProjectUpdateLimitsDialog
        v-if="projectToUpdate && featureFlags.project.updateLimits"
        v-model="updateLimitsDialog"
        :project="projectToUpdate"
    />
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import { VCard, VTextField, VBtn, VIcon, VDataTable, VTooltip, VChip } from 'vuetify/components';
import { AlertCircle, Box, MoreHorizontal, Search } from 'lucide-vue-next';

import { useAppStore } from '@/store/app';
import { sizeToBase10String } from '@/utils/memory';
import { DataTableHeader, SortItem } from '@/types/common';
import { Project, User, UserAccount } from '@/api/client.gen';
import { useProjectsStore } from '@/store/projects';
import { ROUTES } from '@/router';

import ProjectActionsMenu from '@/components/ProjectActionsMenu.vue';
import ProjectUpdateLimitsDialog from '@/components/ProjectUpdateLimitsDialog.vue';

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
    owner: User;
};

type ProjectTableSlotProps = { item: ProjectTableItem  };

const appStore = useAppStore();
const projectsStore = useProjectsStore();
const router = useRouter();

const search = ref<string>('');
const selected = ref<string[]>([]);
const projectToUpdate = ref<Project>();
const updateLimitsDialog = ref<boolean>(false);

const sortBy: SortItem[] = [{ key: 'name', order: 'asc' }];

const headers: DataTableHeader[] = [
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

const props = defineProps<{
    account: UserAccount;
}>();

const featureFlags = computed(() => appStore.state.settings.admin.features);

/**
 * Returns the user's project usage data.
 */
const projects = computed<ProjectTableItem[]>(() => {
    function makeUsageStats(used: number, limit: number): RequiredUsageStats;
    function makeUsageStats(used: number | null, limit: number): UsageStats;
    function makeUsageStats(used: number | null, limit: number) {
        const normalizedUsed = used ?? 0;
        let percent: number;
        if (limit === 0 || normalizedUsed > limit) {
            percent = 100;
        } else {
            percent = Math.round(normalizedUsed * 100 / limit);
        }

        return {
            used,
            limit,
            percent,
        };
    }

    const projects = props.account.projects;
    if (!projects || !projects.length) {
        return [];
    }

    return projects.map<ProjectTableItem>(project => ({
        id: project.id,
        name: project.name,
        storage: makeUsageStats(project.storageUsed, project.storageLimit),
        download: makeUsageStats(project.bandwidthUsed, project.bandwidthLimit),
        segment: makeUsageStats(project.segmentUsed, project.segmentLimit),
        owner: props.account,
    }));
});

async function onUpdateLimitsClicked(projectId: string) {
    projectToUpdate.value = await projectsStore.getProject(projectId);
    updateLimitsDialog.value = true;
}

/**
* Selects the project and navigates to the project dashboard.
*/
function selectProject(id: string):void {
    router.push({
        name: ROUTES.AccountProject.name,
        params: { userID: props.account?.id, projectID: id },
    });
}

function getPercentColor(percent: number) {
    if (percent > 80) {
        return 'error';
    } else if (percent > 60) {
        return 'warning';
    } else {
        return 'success';
    }
}

watch(updateLimitsDialog, async (shown) => {
    if (shown) return;

    // wait for the dialog to close
    await new Promise(resolve => setTimeout(resolve, 300));
    projectToUpdate.value = undefined;
});
</script>
