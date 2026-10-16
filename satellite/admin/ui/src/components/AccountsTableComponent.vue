// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-data-table-server
        :headers="headers"
        :items="accountsPage?.items ?? []"
        :loading="isLoading"
        :items-length="accountsPage?.totalCount ?? 0"
        :page="cursor.page"
        :items-per-page="cursor.limit"
        :items-per-page-options="itemsPerPageOptions"
        :sort-by="sortBy"
        must-sort
        items-per-page-text="Accounts per page"
        no-data-text="No accounts found"
        class="border-0"
        @update:page="onUpdatePage"
        @update:items-per-page="onUpdateLimit"
        @update:sort-by="onUpdateSortBy"
    >
        <template #top>
            <v-row dense class="mx-2 my-2">
                <v-col cols="12" sm="4">
                    <v-select
                        v-model="cursor.status"
                        :disabled="isLoading"
                        :items="usersStore.state.userStatuses"
                        item-title="name"
                        item-value="value"
                        label="Status"
                        variant="solo-filled" flat hide-details clearable density="compact" rounded="lg"
                        @update:model-value="onFilterChange"
                    />
                </v-col>
                <v-col cols="12" sm="4">
                    <v-select
                        v-model="cursor.kind"
                        :disabled="isLoading"
                        :items="usersStore.state.userKinds"
                        item-title="name"
                        item-value="value"
                        label="Kind"
                        variant="solo-filled" flat hide-details clearable density="compact" rounded="lg"
                        @update:model-value="onFilterChange"
                    />
                </v-col>
                <v-col cols="12" sm="4">
                    <v-select
                        v-model="cursor.freezeStatus"
                        :disabled="isLoading"
                        :items="freezeStatusItems"
                        label="Freeze Status"
                        variant="solo-filled" flat hide-details clearable density="compact" rounded="lg"
                        @update:model-value="onFilterChange"
                    />
                </v-col>
            </v-row>
        </template>

        <template #item.email="{ item }">
            <v-btn
                v-tooltip="item.email"
                variant="outlined"
                color="default"
                density="compact"
                @click="goToUser(item)"
            >
                <template #prepend>
                    <v-icon :icon="User" />
                </template>
                <template #default>
                    <span class="text-truncate" style="max-width: 200px;">{{ item.email }}</span>
                </template>
            </v-btn>
        </template>

        <template #item.fullName="{ item }">
            <span v-tooltip="item.fullName" class="d-inline-block text-truncate" style="max-width: 150px;">{{ item.fullName }}</span>
        </template>

        <template #item.kind="{ item }">
            <v-chip :color="userKindColor(item)" variant="tonal" size="small" class="font-weight-medium">
                {{ item.kind.name }}
            </v-chip>
        </template>

        <template #item.status="{ item }">
            <div class="d-flex ga-1">
                <v-chip :color="userStatusColor(item.status)" variant="tonal" size="small" class="font-weight-medium">
                    {{ item.status.name }}
                </v-chip>
                <v-chip v-if="item.freezeStatus" color="error" variant="tonal" size="small" class="font-weight-medium">
                    {{ item.freezeStatus.name }}
                </v-chip>
            </div>
        </template>

        <template #item.tenantID="{ item }">
            <v-chip v-if="item.tenantID" color="warning" variant="tonal" size="small" class="font-weight-medium">
                <span v-tooltip="item.tenantID" class="text-truncate" style="max-width: 150px;">{{ item.tenantID }}</span>
            </v-chip>
        </template>

        <template #item.storageUsed="{ item }">
            <span class="text-no-wrap">{{ sizeToBase10String(item.storageUsed) }}</span>
        </template>

        <template #item.bandwidthUsed="{ item }">
            <span class="text-no-wrap">{{ sizeToBase10String(item.bandwidthUsed) }}</span>
        </template>

        <template #item.createdAt="{ item }">
            <span class="text-no-wrap">
                {{ formatDate(item.createdAt) }}
            </span>
        </template>
    </v-data-table-server>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import { VBtn, VChip, VCol, VDataTableServer, VIcon, VRow, VSelect } from 'vuetify/components';
import { User } from 'lucide-vue-next';
import { useDate } from 'vuetify';

import { AccountListItem, AccountsPage } from '@/api/client.gen';
import { ROUTES } from '@/router';
import { useAppStore } from '@/store/app';
import { useUsersStore } from '@/store/users';
import { useNotify } from '@/composables/useNotify';
import { useLoading } from '@/composables/useLoading';
import { DataTableHeader, SortItem } from '@/types/common';
import {
    AccountsListCursor,
    AccountsOrderBy,
    AccountsOrderDirection,
    FREEZE_STATUS_ANY,
    FREEZE_STATUS_NONE,
    userKindColor,
    userStatusColor,
} from '@/types/user';
import { sizeToBase10String } from '@/utils/memory';

const appStore = useAppStore();
const usersStore = useUsersStore();
const notify = useNotify();
const router = useRouter();
const date = useDate();

const { isLoading, withLoading } = useLoading();
const accountsPage = ref<AccountsPage>();

// the cursor is kept in the store, so the page, filters and order survive navigating to an account and back.
const cursor = computed<AccountsListCursor>(() => usersStore.state.listCursor);

const headers = computed<DataTableHeader[]>(() => {
    const h: DataTableHeader[] = [
        { title: 'Email', key: 'email' },
        { title: 'Name', key: 'fullName' },
        { title: 'Kind', key: 'kind', sortable: false },
        { title: 'Status', key: 'status', sortable: false },
        { title: 'Tenant', key: 'tenantID', sortable: false },
        { title: 'Storage', key: 'storageUsed' },
        { title: 'Download This Month', key: 'bandwidthUsed' },
        { title: 'Date Created', key: 'createdAt' },
    ];
    // a tenant-scoped admin only sees the accounts of its tenant.
    if (appStore.state.settings?.console?.tenantScope) {
        return h.filter(header => header.key !== 'tenantID');
    }
    return h;
});

const itemsPerPageOptions = [
    { value: 10, title: '10' },
    { value: 25, title: '25' },
    { value: 50, title: '50' },
    { value: 100, title: '100' },
];

const orderByHeaderKey: Record<string, AccountsOrderBy> = {
    email: AccountsOrderBy.Email,
    fullName: AccountsOrderBy.Name,
    storageUsed: AccountsOrderBy.Storage,
    bandwidthUsed: AccountsOrderBy.Bandwidth,
    createdAt: AccountsOrderBy.CreatedAt,
};

const sortBy = computed<SortItem[]>(() => {
    const key = Object.keys(orderByHeaderKey).find(k => orderByHeaderKey[k] === cursor.value.order) ?? 'createdAt';
    return [{ key, order: cursor.value.direction }];
});

const freezeStatusItems = computed(() => [
    { title: 'Not Frozen', value: FREEZE_STATUS_NONE },
    { title: 'Any Freeze', value: FREEZE_STATUS_ANY },
    ...usersStore.state.freezeTypes.map(type => ({ title: type.name, value: type.value.toString() })),
]);

/**
 * Returns to the first page when a filter changes.
 */
function onFilterChange(): void {
    cursor.value.page = 1;
}

/**
 * Handles update table page event.
 */
function onUpdatePage(page: number): void {
    if (isLoading.value) return;
    cursor.value.page = page;
}

/**
 * Handles update table rows limit event.
 */
function onUpdateLimit(limit: number): void {
    if (isLoading.value) return;
    cursor.value.limit = limit;
    cursor.value.page = 1;
}

/**
 * Handles update table sorting event.
 */
function onUpdateSortBy(sortBy: SortItem[]): void {
    if (isLoading.value || !sortBy.length) return;
    const order = orderByHeaderKey[sortBy[0].key] ?? AccountsOrderBy.CreatedAt;
    let direction: AccountsOrderDirection = sortBy[0].order === 'asc' ? 'asc' : 'desc';
    // the largest usage is shown first when a usage column is selected.
    if (order !== cursor.value.order && (order === AccountsOrderBy.Storage || order === AccountsOrderBy.Bandwidth)) {
        direction = 'desc';
    }
    cursor.value.order = order;
    cursor.value.direction = direction;
    cursor.value.page = 1;
}

/**
 * Fetches the page of accounts of the cursor.
 */
function fetchAccounts(): void {
    withLoading(async () => {
        try {
            accountsPage.value = await usersStore.listUsers(cursor.value);
        } catch (error) {
            notify.error(`Error listing accounts. ${error.message}`);
        }
    });
}

function formatDate(dateString: string): string {
    return `${date.format(dateString, 'shortDate')}, ${date.format(dateString, 'year')}`;
}

function goToUser(user: AccountListItem): void {
    router.push({ name: ROUTES.Account.name, params: { userID: user.id } });
}

watch(cursor, fetchAccounts, { deep: true });

onMounted(() => {
    fetchAccounts();

    Promise.all([
        usersStore.getUserStatuses(),
        usersStore.getUserKinds(),
        usersStore.getAccountFreezeTypes(),
    ]).catch(error => notify.error(`Error loading account filters. ${error.message}`));
});
</script>
