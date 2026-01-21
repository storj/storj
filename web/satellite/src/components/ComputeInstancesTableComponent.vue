// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
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
        class="mb-5"
    />

    <v-data-table
        :headers="headers"
        :items="instances"
        :search="search"
        :loading="isLoading"
        no-data-text="No results found"
        :hover="false"
        @update:current-items="onCurrentItemsUpdate"
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
                    class="font-weight-bold text-capitalize"
                >
                    <template #prepend>
                        <v-icon :icon="getStatusIcon(item.status)" size="small" class="mr-2" />
                    </template>
                    {{ item.status }}
                </v-chip>
            </div>
        </template>

        <template #item.hostname="{ item }">
            <span class="text-no-wrap">
                {{ item.hostname || '-' }}
            </span>
        </template>

        <template #item.running="{ item }">
            <v-progress-circular v-if="item.running === undefined" indeterminate size="small" />
            <v-chip
                v-else
                v-tooltip="'Click to refresh running status'"
                variant="tonal"
                :color="item.running ? 'success' : 'primary'"
                size="small"
                class="text-capitalize font-weight-semibold"
                @click="fetchSingleInstance(item.id)"
            >
                {{ item.running ? 'Yes' : 'No' }}
            </v-chip>
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
                :loading="isLoading"
            >
                <v-icon :icon="Ellipsis" />
                <v-menu activator="parent" @update:model-value="val => onMenuOpen(val, item)">
                    <v-list class="pa-1">
                        <v-progress-linear v-if="isLoading" indeterminate />
                        <v-list-item density="comfortable" link @click="() => viewDetails(item)">
                            <template #prepend>
                                <component :is="MonitorCloud" :size="18" />
                            </template>
                            <v-list-item-title class="ml-3 text-body-2 font-weight-medium">
                                View Details
                            </v-list-item-title>
                        </v-list-item>
                        <v-list-item density="comfortable" link @click="() => onUpdate(item)">
                            <template #prepend>
                                <component :is="MonitorCog" :size="18" />
                            </template>
                            <v-list-item-title class="ml-3 text-body-2 font-weight-medium">
                                Update Type
                            </v-list-item-title>
                        </v-list-item>
                        <template v-if="!isLoading">
                            <template v-if="item.running">
                                <v-list-item density="comfortable" link @click="() => onStopOrRestart(item, InstanceAction.STOP)">
                                    <template #prepend>
                                        <component :is="OctagonPauseIcon" :size="18" />
                                    </template>
                                    <v-list-item-title class="ml-3 text-body-2 font-weight-medium">
                                        Stop
                                    </v-list-item-title>
                                </v-list-item>
                                <v-list-item density="comfortable" link @click="() => onStopOrRestart(item, InstanceAction.RESTART)">
                                    <template #prepend>
                                        <component :is="RotateCcwIcon" :size="18" />
                                    </template>
                                    <v-list-item-title class="ml-3 text-body-2 font-weight-medium">
                                        Restart
                                    </v-list-item-title>
                                </v-list-item>
                            </template>
                            <v-list-item v-else density="comfortable" link @click="() => onStart(item)">
                                <template #prepend>
                                    <component :is="CirclePlayIcon" :size="18" />
                                </template>
                                <v-list-item-title class="ml-3 text-body-2 font-weight-medium">
                                    Start
                                </v-list-item-title>
                            </v-list-item>
                        </template>
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

    <delete-instance-dialog v-model="isDeleteDialog" :instance="instanceToDelete" />
    <instance-details-dialog v-model="isDetailsDialog" :instance="instanceToView" />
    <update-instance-dialog v-model="isUpdateDialog" :instance="instanceToUpdate" />
    <stop-or-restart-instance-dialog v-model="isStopOrRestartDialog" :instance="instanceToStopOrRestart" :action="instanceAction" />
</template>

<script setup lang="ts">
import { computed, FunctionalComponent, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import {
    VBtn,
    VChip,
    VDataTable,
    VIcon,
    VList,
    VListItem,
    VListItemTitle,
    VMenu,
    VProgressLinear,
    VProgressCircular,
    VTextField,
} from 'vuetify/components';
import {
    CheckCircle,
    CirclePlayIcon,
    Cog,
    Computer,
    Ellipsis,
    HelpCircle,
    MonitorCloud,
    MonitorCog,
    OctagonPauseIcon,
    RotateCcwIcon,
    Search,
    StopCircle,
    Trash2,
} from 'lucide-vue-next';

import { DataTableHeader } from '@/types/common';
import { useComputeStore } from '@/store/modules/computeStore';
import { Instance, InstanceAction } from '@/types/compute';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { Time } from '@/utils/time';

import DeleteInstanceDialog from '@/components/dialogs/compute/DeleteInstanceDialog.vue';
import InstanceDetailsDialog from '@/components/dialogs/compute/InstanceDetailsDialog.vue';
import UpdateInstanceDialog from '@/components/dialogs/compute/UpdateInstanceDialog.vue';
import StopOrRestartInstanceDialog from '@/components/dialogs/compute/StopOrRestartInstanceDialog.vue';

const computeStore = useComputeStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const headers: DataTableHeader[] = [
    { title: 'Name', key: 'name' },
    { title: 'Hostname', key: 'hostname' },
    { title: 'Status', key: 'status' },
    { title: 'Is Running?', key: 'running' },
    { title: 'Address', key: 'ipv4Address' },
    { title: 'Date Updated', key: 'updated' },
    { title: 'Date Created', key: 'created' },
    { title: '', key: 'actions', align: 'end', sortable: false },
];

const search = ref<string>('');
const isDeleteDialog = ref<boolean>(false);
const isDetailsDialog = ref<boolean>(false);
const isUpdateDialog = ref<boolean>(false);
const isStopOrRestartDialog = ref<boolean>(false);
const instanceToDelete = ref<Instance>(new Instance());
const instanceToView = ref<Instance>(new Instance());
const instanceToUpdate = ref<Instance>(new Instance());
const instanceToStopOrRestart = ref<Instance>(new Instance());
const instanceAction = ref<InstanceAction>(InstanceAction.STOP);
const pollingIntervalID = ref<NodeJS.Timeout>();
// Set of instance IDs currently being fetched (prevents duplicate requests)
const currentlyFetchingIds = ref<Set<string>>(new Set());

const instances = computed<Instance[]>(() => computeStore.state.instances);
const hasPendingInstances = computed<boolean>(() => instances.value.some(instance => instance.status === 'pending'));

function onMenuOpen(opened: boolean, item: Instance): void {
    if (!opened) return;

    fetchSingleInstance(item.id);
}

function fetchSingleInstance(instanceID: string): void {
    withLoading(async () => {
        try {
            await computeStore.getInstance(instanceID);
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.COMPUTE_INSTANCES_TABLE);
        }
    });
}

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

function onStart(instance: Instance): void {
    withLoading(async () => {
        try {
            await computeStore.startInstance(instance.id);
            notify.success(`Instance start initiated`);
            await computeStore.getInstance(instance.id);
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.COMPUTE_INSTANCES_TABLE);
        }
    });
}

function onStopOrRestart(instance: Instance, action: InstanceAction): void {
    instanceToStopOrRestart.value = instance;
    instanceAction.value = action;
    isStopOrRestartDialog.value = true;
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

interface VDataTableItem<T> {
    raw: T;
}

async function onCurrentItemsUpdate(items: readonly VDataTableItem<Instance>[]): Promise<void> {
    // Only fetch instances that don't have running status and aren't already being fetched.
    const instancesToFetch = items.filter(i => {
        return i.raw.running === undefined && !currentlyFetchingIds.value.has(i.raw.id);
    });

    if (instancesToFetch.length === 0) return;

    // Mark these instances as being fetched.
    instancesToFetch.forEach(i => currentlyFetchingIds.value.add(i.raw.id));

    try {
        const promises = instancesToFetch.map(async (item) => {
            try {
                await computeStore.getInstance(item.raw.id);
            } catch (error) {
                notify.notifyError(error, AnalyticsErrorEventSource.COMPUTE_INSTANCES_TABLE);
            } finally {
                // Remove from the set when done (success or failure).
                currentlyFetchingIds.value.delete(item.raw.id);
            }
        });

        await Promise.all(promises);
    } catch {
        // Ensure we clean up on any unexpected errors.
        instancesToFetch.forEach(i => currentlyFetchingIds.value.delete(i.raw.id));
    }
}

function startPolling(): void {
    if (pollingIntervalID.value) return;

    // Poll every 30 seconds.
    pollingIntervalID.value = setInterval(() => {
        fetch();
    }, 30000);
}

function stopPolling(): void {
    if (!pollingIntervalID.value) return;

    clearInterval(pollingIntervalID.value);
    pollingIntervalID.value = undefined;
}

watch(hasPendingInstances, (hasPending) => {
    if (hasPending) {
        startPolling();
    } else {
        stopPolling();
    }
}, { immediate: true });

onMounted(() => {
    fetch();
});

onBeforeUnmount(() => {
    stopPolling();
    currentlyFetchingIds.value.clear();
});
</script>

<style scoped lang="scss">
.v-list-item :deep(.v-list-item__prepend .v-list-item__spacer) {
    display: none;
}
</style>
