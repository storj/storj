// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="d-flex justify-space-between my-4">
        <h3>History</h3>
        <v-btn-toggle
            v-model="exact"
            mandatory
            border
            inset
            rounded="lg"
            class="pa-1 bg-surface"
        >
            <v-btn :loading="isLoading" :value="false">All</v-btn>
            <v-btn :loading="isLoading" :value="true">
                User Related
            </v-btn>
        </v-btn-toggle>
    </div>
    <v-card variant="flat" :border="true" rounded="xlg">
        <v-data-table
            :sort-by="sortBy"
            :headers="headers"
            :items="history"
            :search="search"
            :loading="isLoading"
            class="border-0"
            item-key="id"
            density="comfortable"
            show-expand
            hover
        >
            <template #top>
                <v-text-field
                    v-model="search" label="Search" :prepend-inner-icon="Search" single-line variant="solo-filled" flat
                    hide-details clearable density="compact" rounded="lg" class="mx-2 mt-2 mb-2"
                />
            </template>
            <template #expanded-row="{ columns, item }">
                <tr>
                    <td :colspan="columns.length">
                        <v-alert class="mx-n3" rounded="0">
                            <template #title>Changes Details</template>
                            <p v-for="(change, key) in item.changes" :key="key">
                                <span class="text-high-emphasis">{{ key }}:</span> <span class="text-decoration-line-through">{{ castChange(change)[0] }}</span> &rarr; {{ castChange(change)[1] }}
                            </p>
                        </v-alert>
                    </td>
                </tr>
            </template>

            <template #item.operation="{ item }">
                {{ cleanupOperation(item.operation) }}
            </template>

            <template #item.projectID="{ item }">
                <v-chip v-if="item.projectID" variant="tonal" size="small" rounded="lg" @click="goToProject(item.projectID)">
                    {{ item.projectID }}
                </v-chip>
                <template v-else>
                    —
                </template>
            </template>

            <template #item.bucketName="{ item }">
                <v-chip v-if="item.bucketName" variant="tonal" size="small" rounded="lg">
                    {{ item.bucketName }}
                </v-chip>
                <template v-else>
                    —
                </template>
            </template>

            <template #item.timestamp="{ item }">
                <span class="text-no-wrap">
                    {{ dateFns.format(item.timestamp, 'fullDateTime') }}
                </span>
            </template>
        </v-data-table>
    </v-card>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { VAlert, VBtn, VBtnToggle, VCard, VTextField, VDataTable, VChip } from 'vuetify/components';
import { Search } from 'lucide-vue-next';
import { useDate } from 'vuetify';
import { useRouter } from 'vue-router';

import { DataTableHeader, SortItem } from '@/types/common';
import { ChangeLog, UserAccount } from '@/api/client.gen';
import { useUsersStore } from '@/store/users';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { ROUTES } from '@/router';

const usersStore = useUsersStore();

const dateFns = useDate();
const router = useRouter();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const props = defineProps<{
    account: UserAccount;
}>();

const history = ref<ChangeLog[]>([]);
const exact = ref<boolean>(true);
const search = ref<string>('');
const sortBy: SortItem[] = [{ key: 'name', order: 'asc' }];

const headers = computed<DataTableHeader[]>(() => {
    const h = [
        { title: 'Date', key: 'timestamp' },
        { title: 'Operation', key: 'operation' },
        { title: 'Project ID', key: 'projectID' },
        { title: 'Bucket Name', key: 'bucketName' },
        { title: 'Admin', key: 'adminEmail' },
        { title: '', key: 'data-table-expand' },
    ];
    if (exact.value) {
        return h.filter(header => header.key !== 'projectID' && header.key !== 'bucketName');
    }
    return h;
});

function goToProject(projectID: string) {
    router.push({
        name: ROUTES.AccountProject.name,
        params: { userID: props.account?.id, projectID },
    });
}

function cleanupOperation(operation: string): string {
    const op = operation.replace(/_/g, ' ');
    return op.charAt(0).toUpperCase() + op.slice(1);
}

function castChange(change: unknown): unknown[] {
    return change as unknown[];
}

function fetchHistory() {
    withLoading(async () => {
        try {
            history.value = await usersStore.getHistory(props.account.id, exact.value);
        } catch (error) {
            notify.error(error);
        }
    });
}

watch(exact, () => fetchHistory(), { immediate: true });

watch(() => props.account, () => fetchHistory());
</script>
