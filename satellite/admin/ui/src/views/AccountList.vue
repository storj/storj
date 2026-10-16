// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container fluid>
        <v-card
            :loading="isSearching"
            title="Accounts"
            subtitle="Browse all accounts, or search by ID, email, email domain, name, or Stripe customer ID"
            variant="flat" rounded="xlg" border
        >
            <v-text-field
                v-model="searchTerm" label="Search" :prepend-inner-icon="Search" single-line variant="solo-filled" flat
                hide-details clearable density="compact" rounded="lg" class="mx-4 mb-2"
                @update:model-value="onSearchChange"
            />
            <v-data-table
                v-if="searchTerm"
                :loading="isSearching"
                :headers="searchHeaders"
                :items="searchResults"
                :height="searchResults.length > 10 ? 520 : 'auto'"
                class="border-0 search-results"
                hide-default-header
            >
                <template #no-data>
                    {{ searchTerm.length >= 3 ? 'No results found' : 'Enter a search term of at least 3 characters' }}
                </template>
                <template #loading>
                    Searching...
                </template>
                <template v-if="searchResults.length <= 10" #bottom>
                    <v-container class="v-data-table-footer" />
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
                            <span class="text-truncate" style="max-width: 150px;">{{ item.email }}</span>
                        </template>
                    </v-btn>
                </template>

                <template #item.fullName="{ item }">
                    <v-chip v-if="item.fullName" variant="tonal" color="default" size="small">
                        <span v-tooltip="item.fullName" class="text-truncate" style="max-width: 150px;">{{ item.fullName }}</span>
                    </v-chip>
                </template>

                <template #item.kind="{ item }">
                    <v-chip :color="userKindColor(item)" variant="tonal" size="small" class="font-weight-medium">
                        {{ item.kind.name }}
                    </v-chip>
                </template>

                <template #item.status="{ item }">
                    <v-chip :color="userStatusColor(item.status)" variant="tonal" size="small" class="font-weight-medium">
                        {{ item.status.name }}
                    </v-chip>
                </template>

                <template #item.createdAt="{ item }">
                    <span class="text-no-wrap">
                        Created on {{ formatDate(item.createdAt) }}
                    </span>
                </template>

                <template #item.tenantID="{ item }">
                    <v-chip v-if="item.tenantID" color="warning" variant="tonal" size="small" class="font-weight-medium">
                        <span v-tooltip="item.tenantID" class="text-truncate" style="max-width: 150px;">Tenant ID: {{ item.tenantID }}</span>
                    </v-chip>
                    <v-chip v-else color="success" variant="tonal" size="small" class="font-weight-medium">
                        <span class="text-truncate" style="max-width: 150px;">No Tenant</span>
                    </v-chip>
                </template>
            </v-data-table>
            <AccountsTableComponent v-else />
        </v-card>
    </v-container>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { VBtn, VCard, VChip, VContainer, VDataTable, VIcon, VTextField } from 'vuetify/components';
import { Search, User } from 'lucide-vue-next';
import { useDate } from 'vuetify';

import { AccountMin } from '@/api/client.gen';
import { ROUTES } from '@/router';
import { useAppStore } from '@/store/app';
import { useUsersStore } from '@/store/users';
import { useProjectsStore } from '@/store/projects';
import { useNotificationsStore } from '@/store/notifications';
import { useLoading } from '@/composables/useLoading';
import { DataTableHeader } from '@/types/common';
import { userKindColor, userStatusColor } from '@/types/user';

import AccountsTableComponent from '@/components/AccountsTableComponent.vue';

const appStore = useAppStore();
const usersStore = useUsersStore();
const notify = useNotificationsStore();
const router = useRouter();
const date = useDate();
const { isLoading: isSearching } = useLoading();

let searchTimer: ReturnType<typeof setTimeout> | null = null;

const searchHeaders = computed<DataTableHeader[]>(() => {
    const h: DataTableHeader[] = [
        { title: 'Email', key: 'email' },
        { title: 'Name', key: 'fullName' },
        { title: 'Kind', key: 'kind' },
        { title: 'Status', key: 'status' },
        { title: 'Tenant', key: 'tenantID' },
        { title: 'Date Created', key: 'createdAt' },
    ];
    // a tenant-scoped admin only finds the accounts of its tenant.
    if (appStore.state.settings?.console?.tenantScope) {
        return h.filter(header => header.key !== 'tenantID');
    }
    return h;
});

// the search term and results are kept in the store, so they survive navigating to an account and back.
const searchTerm = computed({
    get: () => usersStore.state.searchTerm,
    set: usersStore.setSearchTerm,
});

const searchResults = computed(() => usersStore.state.searchResults);

function onSearchChange(search: string) {
    if (searchTimer) clearTimeout(searchTimer);
    if (!search || search.length < 3) {
        return;
    }
    searchTimer = setTimeout(async () => {
        isSearching.value = true;
        try {
            await usersStore.findUsers(search);
        } catch (error) {
            notify.notifyError(`Error searching users. ${error.message}`);
        } finally {
            isSearching.value = false;
        }
    }, 500);
}

function formatDate(dateString: string) {
    return `${date.format(dateString, 'shortDate')}, ${date.format(dateString, 'year')}`;
}

function goToUser(user: AccountMin): void {
    router.push({ name: ROUTES.Account.name, params: { userID: user.id } });
}

onMounted(() => {
    usersStore.clearCurrentUser();
    useProjectsStore().clearCurrentProject();
});

onBeforeUnmount(() => {
    if (searchTimer) clearTimeout(searchTimer);
});
</script>

<style scoped lang="scss">
.search-results :deep(.v-data-table-footer) {
    background: rgb(var(--v-theme-surface)) !important;
    box-shadow: none !important;
}
</style>
