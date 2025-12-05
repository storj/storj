// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-container fluid>
        <v-row>
            <v-col v-if="featureFlags.account.create" cols="6" class="d-flex justify-end align-center">
                <v-btn variant="outlined" color="default">
                    <template #prepend>
                        <v-icon :icon="PlusCircle" />
                    </template>
                    New Account
                    <NewAccountDialog />
                </v-btn>
            </v-col>
        </v-row>
        <v-row align="center" justify="center" class="search-area">
            <v-col cols="12" md="10" lg="8" xl="6">
                <v-card
                    :loading="isLoading"
                    title="Find Account"
                    subtitle="Search by ID, email, name or Stripe customer ID"
                    variant="flat" rounded="xlg" border
                >
                    <v-data-table
                        :loading="isLoading"
                        :headers="headers"
                        :items="results ?? []"
                        :height="results.length > 10 ? 520 : 'auto'"
                        class="border-0"
                        hide-default-header
                    >
                        <template #no-data>
                            {{ searchTerm?.length >= 3 ? 'No results found' : 'Enter a search term of at least 3 characters' }}
                        </template>
                        <template #loading>
                            Searching...
                        </template>
                        <template v-if="results.length <= 10" #bottom>
                            <v-container class="v-data-table-footer" />
                        </template>
                        <template #top>
                            <v-text-field
                                v-model="searchTerm" label="Search" :prepend-inner-icon="Search" single-line variant="solo-filled" flat
                                hide-details clearable density="compact" rounded="lg" class="mx-4 mb-2"
                                @update:model-value="onSearchChange"
                            />
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
                            <v-chip
                                :color="userIsPaid(item) ? 'success' : userIsNFR(item) ? 'warning' : 'info'"
                                variant="tonal" size="small" class="font-weight-medium"
                            >
                                {{ item.kind.name }}
                            </v-chip>
                        </template>

                        <template #item.status="{ item }">
                            <v-chip
                                :color="statusColor(item.status)"
                                variant="tonal" size="small" class="font-weight-medium"
                            >
                                {{ item.status.name }}
                            </v-chip>
                        </template>

                        <template #item.createdAt="{ item }">
                            <span class="text-no-wrap">
                                Created on {{ formatDate(item.createdAt) }}
                            </span>
                        </template>
                    </v-data-table>
                </v-card>
            </v-col>
        </v-row>
    </v-container>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { VContainer, VDataTable, VChip, VRow, VCol, VIcon, VBtn, VCard, VTextField } from 'vuetify/components';
import { PlusCircle, Search, User } from 'lucide-vue-next';
import { useDate } from 'vuetify';

import { AccountMin, FeatureFlags, UserStatusInfo } from '@/api/client.gen';
import { useAppStore } from '@/store/app';
import { useNotificationsStore } from '@/store/notifications';
import { ROUTES } from '@/router';
import { useUsersStore } from '@/store/users';
import { userIsNFR, userIsPaid } from '@/types/user';
import { useLoading } from '@/composables/useLoading';
import { useProjectsStore } from '@/store/projects';

import NewAccountDialog from '@/components/NewAccountDialog.vue';

const appStore = useAppStore();
const usersStore = useUsersStore();
const notify = useNotificationsStore();
const router = useRouter();
const { isLoading } = useLoading();
const date = useDate();

let timer: ReturnType<typeof setTimeout> | null = null;
const headers = [
    { title: 'Email', key: 'email' },
    { title: 'Name', key: 'fullName' },
    { title: 'Kind', key: 'kind' },
    { title: 'Status', key: 'status' },
    { title: 'Date Created', key: 'createdAt' },
];

const searchTerm = computed({
    get: () => usersStore.state.searchTerm,
    set: usersStore.setSearchTerm,
});

const results = computed(() => usersStore.state.searchResults);

const featureFlags = computed(() => appStore.state.settings.admin.features as FeatureFlags);

function statusColor(info: UserStatusInfo) {
    const status = info.name.toLowerCase();
    if (status.includes('deletion') || status.includes('deleted')) {
        return 'error';
    }
    if (status.includes('active')) {
        return 'success';
    }

    return 'warning';
}

function formatDate(dateString: string) {
    return `${date.format(dateString, 'shortDate')}, ${date.format(dateString, 'year')}`;
}

function onSearchChange(search: string) {
    if (timer) clearTimeout(timer);
    if (!search || search.length < 3) {
        return;
    }
    timer = setTimeout(async () => {
        isLoading.value = true;
        try {
            await usersStore.findUsers(search);
        } catch (error) {
            notify.notifyError(`Error searching users. ${error.message}`);
        } finally {
            isLoading.value = false;
        }
    }, 500);
}

function goToUser(user: AccountMin):void {
    router.push({ name: ROUTES.Account.name, params: { userID: user.id } });
}

onMounted(() => {
    usersStore.clearCurrentUser();
    useProjectsStore().clearCurrentProject();
});
</script>
<style scoped lang="scss">
.search-area {
    height: calc(100vh - 150px); // attempt to vertically center the search area
}

:deep(.v-data-table-footer) {
    background: rgb(var(--v-theme-surface)) !important;
    box-shadow: none !important;
}
</style>
