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
            :headers="headers"
            :items="instances"
            :search="search"
            :loading="isLoading"
            no-data-text="No results found"
            hover
        >
            <template #item.title="{ item }">
                <v-list-item class="font-weight-bold pl-0" density="compact">
                    <template #prepend>
                        <v-icon :icon="Computer" color="primary" :size="18" class="mr-2" />
                    </template>
                    {{ item.name }}
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

            <template #item.created="{ item }">
                <span class="text-no-wrap">
                    {{ Time.formattedDate(item.created) }}
                </span>
            </template>

            <template #item.updated="{ item }">
                <span class="text-no-wrap">
                    {{ Time.formattedDate(item.updated) }}
                </span>
            </template>

            <template #item.ipv4Address="{ item }">
                <span class="text-no-wrap">
                    {{ item.ipv4Address }}
                </span>
            </template>

            <template #item.actions="{ item }">
                <v-btn
                    title="Instance Actions"
                    variant="outlined"
                    color="default"
                    size="small"
                    rounded="md"
                    class="mr-1 text-caption"
                    density="comfortable"
                    icon
                >
                    <v-icon :icon="Ellipsis" />
                    <v-menu activator="parent">
                        <v-list class="pa-1">
                            <v-list-item density="comfortable" link @click="() => viewDetails(item)">
                                <template #prepend>
                                    <component :is="Computer" :size="18" />
                                </template>
                                <v-list-item-title class="ml-3 text-body-2 font-weight-medium">
                                    View Details
                                </v-list-item-title>
                            </v-list-item>
                            <v-list-item density="comfortable" link @click="() => onUpdate(item)">
                                <template #prepend>
                                    <component :is="BoltIcon" :size="18" />
                                </template>
                                <v-list-item-title class="ml-3 text-body-2 font-weight-medium">
                                    Update Type
                                </v-list-item-title>
                            </v-list-item>
                            <v-list-item class="text-error" density="comfortable" link @click="() => onDelete(item)">
                                <template #prepend>
                                    <component :is="Trash2" :size="18" />
                                </template>
                                <v-list-item-title class="ml-3 text-body-2 font-weight-medium">
                                    Remove
                                </v-list-item-title>
                            </v-list-item>
                        </v-list>
                    </v-menu>
                </v-btn>
            </template>
        </v-data-table>
    </v-card>

    <delete-compute-instance-dialog v-model="isDeleteDialog" :instance="instanceToDelete" />
    <compute-instance-details-dialog v-model="isDetailsDialog" :instance="instanceToView" />
    <update-compute-instance-dialog v-model="isUpdateDialog" :instance="instanceToUpdate" />
</template>

<script setup lang="ts">
import { computed, FunctionalComponent, onMounted, ref } from 'vue';
import {
    VCard,
    VDataTable,
    VTextField,
    VListItem,
    VChip,
    VIcon,
    VBtn,
    VList,
    VListItemTitle,
    VMenu,
} from 'vuetify/components';
import {
    Computer,
    Search,
    CheckCircle,
    StopCircle,
    Cog,
    HelpCircle,
    Ellipsis,
    Trash2,
    BoltIcon,
} from 'lucide-vue-next';

import { DataTableHeader } from '@/types/common';
import { useComputeStore } from '@/store/modules/computeStore';
import { Instance } from '@/types/compute';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { Time } from '@/utils/time';

import DeleteComputeInstanceDialog from '@/components/dialogs/DeleteComputeInstanceDialog.vue';
import ComputeInstanceDetailsDialog from '@/components/dialogs/ComputeInstanceDetailsDialog.vue';
import UpdateComputeInstanceDialog from '@/components/dialogs/UpdateComputeInstanceDialog.vue';

const computeStore = useComputeStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const headers: DataTableHeader[] = [
    { title: 'Name', key: 'name' },
    { title: 'Hostname', key: 'hostname' },
    { title: 'Status', key: 'status' },
    { title: 'Address', key: 'ipv4Address' },
    { title: 'Date Updated', key: 'updated' },
    { title: 'Date Created', key: 'created' },
    { title: '', key: 'actions', align: 'end', sortable: false },
];

const search = ref<string>('');
const isDeleteDialog = ref<boolean>(false);
const isDetailsDialog = ref<boolean>(false);
const isUpdateDialog = ref<boolean>(false);
const instanceToDelete = ref<Instance>(new Instance());
const instanceToView = ref<Instance>(new Instance());
const instanceToUpdate = ref<Instance>(new Instance());

const instances = computed<Instance[]>(() => computeStore.state.instances);

function fetch(): void {
    withLoading(async () => {
        try {
            await computeStore.getInstances();
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.COMPUTE_INSTANCES_TABLE);
        }
    });
}

function onDelete(instance: Instance): void {
    instanceToDelete.value = instance;
    isDeleteDialog.value = true;
}

function viewDetails(instance: Instance): void {
    instanceToView.value = instance;
    isDetailsDialog.value = true;
}

function onUpdate(instance: Instance): void {
    instanceToUpdate.value = instance;
    isUpdateDialog.value = true;
}

function getStatusColor(status: string): string {
    if (status === 'complete') return 'success';
    if (status === 'offline') return 'default';
    if (status === 'pending') return 'info';
    return 'default';
}

function getStatusIcon(status: string): FunctionalComponent {
    if (status === 'complete') return CheckCircle;
    if (status === 'offline') return StopCircle;
    if (status === 'pending') return Cog;
    return HelpCircle;
}

onMounted(() => {
    fetch();
});
</script>

<style scoped lang="scss">
.v-list-item :deep(.v-list-item__prepend .v-list-item__spacer) {
    display: none;
}
</style>
