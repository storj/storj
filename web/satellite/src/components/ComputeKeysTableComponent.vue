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
            :items="keys"
            :search="search"
            :loading="isLoading"
            no-data-text="No results found"
            hover
        >
            <template #item.name="{ item }">
                <v-list-item class="font-weight-bold pl-0">
                    {{ item.name }}
                </v-list-item>
            </template>
            <template #item.publicKey="{ item }">
                <v-chip variant="tonal" size="small" rounded="xl" :title="item.publicKey" class="font-weight-bold ellipsis">
                    {{ item.publicKey }}
                </v-chip>
            </template>
            <template #item.created="{ item }">
                <span class="text-no-wrap">
                    {{ Time.formattedDate(item.created) }}
                </span>
            </template>
            <template #item.actions="{ item }">
                <v-btn
                    size="small"
                    variant="outlined"
                    color="default"
                    @click="onDelete(item)"
                >
                    Remove
                </v-btn>
            </template>
        </v-data-table>
    </v-card>

    <delete-compute-s-s-h-key-dialog v-model="isDeleteDialog" :ssh-key="keyToDelete" />
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import {
    VCard,
    VTextField,
    VDataTable,
    VListItem,
    VChip,
    VBtn,
} from 'vuetify/components';
import { Search } from 'lucide-vue-next';

import { DataTableHeader } from '@/types/common';
import { useComputeStore } from '@/store/modules/computeStore';
import { SSHKey } from '@/types/compute';
import { Time } from '@/utils/time';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';

import DeleteComputeSSHKeyDialog from '@/components/dialogs/DeleteComputeSSHKeyDialog.vue';

const computeStore = useComputeStore();

const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const headers: DataTableHeader[] = [
    {
        title: 'Name',
        align: 'start',
        key: 'name',
    },
    { title: 'Public Key', key: 'publicKey', maxWidth: '600px' },
    { title: 'Date Created', key: 'created' },
    { title: '', key: 'actions', align: 'end', sortable: false },
];

const isDeleteDialog = ref<boolean>(false);
const keyToDelete = ref<SSHKey>(new SSHKey());
const search = ref<string>('');

const keys = computed<SSHKey[]>(() => computeStore.state.sshKeys);

function fetch(): void {
    withLoading(async () => {
        try {
            await computeStore.getSSHKeys();
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.COMPUTE_SSH_KEYS_TABLE);
        }
    });
}

function onDelete(key: SSHKey): void {
    keyToDelete.value = key;
    isDeleteDialog.value = true;
}

onMounted(() => {
    fetch();
});
</script>

<style scoped lang="scss">
:deep(.v-chip__content) {
    display: inline-block !important;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}
</style>
