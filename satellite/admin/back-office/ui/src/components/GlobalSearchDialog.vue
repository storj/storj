// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog v-model="model" :width="dialogWidth" transition="fade-transition">
        <v-card
            title="Find accounts or projects"
            subtitle="Search by ID, email, name or Stripe customer ID"
            rounded="xlg"
        >
            <template #append>
                <v-btn
                    :icon="X" :disabled="isLoading"
                    variant="text" size="small" color="default" @click="model = false"
                />
            </template>

            <div class="mx-2">
                <v-data-table
                    v-if="!projects.length"
                    :loading="isLoading"
                    :headers="headers"
                    :items="accounts"
                    :height="accounts.length > 10 ? 520 : 'auto'"
                    class="border-0"
                    hide-default-header
                >
                    <template #no-data>
                        {{ searchTerm?.length >= 3 ? 'No results found' : 'Enter a search term of at least 3 characters' }}
                    </template>
                    <template #loading>
                        Searching...
                    </template>
                    <template v-if="accounts.length <= 10" #bottom>
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
                            <span class="text-truncate" style="max-width: 150px;">{{ item.fullName }}</span>
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
                <v-data-table
                    v-else
                    :loading="isLoading"
                    :headers="headers"
                    :items="projects"
                    :height="projects.length > 10 ? 520 : 'auto'"
                    class="border-0"
                    hide-default-header
                >
                    <template #no-data>
                        {{ searchTerm?.length >= 3 ? 'No results found' : 'Enter a search term of at least 3 characters' }}
                    </template>
                    <template #loading>
                        Searching...
                    </template>
                    <template v-if="projects.length <= 10" #bottom>
                        <v-container class="v-data-table-footer" />
                    </template>
                    <template #top>
                        <v-text-field
                            v-model="searchTerm" label="Search" :prepend-inner-icon="Search" single-line variant="solo-filled" flat
                            hide-details clearable density="compact" rounded="lg" class="mx-4 mb-2"
                            @update:model-value="onSearchChange"
                        />
                    </template>

                    <template #item.name="{ item }">
                        <v-btn
                            v-tooltip="item.name"
                            variant="outlined"
                            color="default"
                            density="compact"
                            @click="goToProject(item)"
                        >
                            <template #prepend>
                                <v-icon :icon="Box" />
                            </template>
                            <template #default>
                                <span class="text-truncate" style="max-width: 200px;">{{ item.name }}</span>
                            </template>
                        </v-btn>
                    </template>

                    <template #item.ownerEmail="{ item }">
                        <v-chip variant="tonal" color="default" size="small" class="font-weight-medium">
                            <span class="text-truncate" style="max-width: 200px;">{{ item.owner.email }}</span>
                        </v-chip>
                    </template>

                    <template #item.createdAt="{ item }">
                        <span class="text-no-wrap">
                            Created on {{ formatDate(item.createdAt) }}
                        </span>
                    </template>
                </v-data-table>
            </div>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue';
import { useRouter } from 'vue-router';
import { VBtn, VCard, VChip, VContainer, VDataTable, VDialog, VIcon, VTextField } from 'vuetify/components';
import { Box, Search, User, X } from 'lucide-vue-next';
import { useDate, useDisplay } from 'vuetify';

import { AccountMin, Project, SearchResult, UserStatusInfo } from '@/api/client.gen';
import { useAppStore } from '@/store/app';
import { useNotificationsStore } from '@/store/notifications';
import { ROUTES } from '@/router';
import { userIsNFR, userIsPaid } from '@/types/user';
import { useLoading } from '@/composables/useLoading';

const appStore = useAppStore();
const notify = useNotificationsStore();
const router = useRouter();
const { isLoading } = useLoading();
const { mdAndUp, lgAndUp, xlAndUp } = useDisplay();
const date = useDate();

const model = defineModel<boolean>({ required: true });

let timer: ReturnType<typeof setTimeout> | null = null;

const searchTerm = ref<string>('');
const searchResult = ref<SearchResult | null>(null);

const accounts = computed<AccountMin[]>(() => searchResult.value?.accounts ?? []);
const projects = computed<Project[]>(() => searchResult.value?.project ? [searchResult.value?.project] : []);

const headers = computed(() => {
    if (!searchResult.value?.project) {
        return [
            { title: 'Email', key: 'email' },
            { title: 'Name', key: 'fullName' },
            { title: 'Kind', key: 'kind' },
            { title: 'Status', key: 'status' },
            { title: 'Date Created', key: 'createdAt' },
        ];
    }
    return [
        { title: 'Project Name', key: 'name' },
        { title: 'Owner Email', key: 'ownerEmail' },
        { title: 'Date Created', key: 'createdAt' },
    ];
});

const dialogWidth = computed(() => {
    if (xlAndUp.value) return 1100;
    if (lgAndUp.value) return 900;
    if (mdAndUp.value) return 750;
    return '';
});

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
    searchResult.value = null;
    if (timer) clearTimeout(timer);
    if (!search || search.length < 3) {
        return;
    }
    timer = setTimeout(async () => {
        isLoading.value = true;
        try {
            searchResult.value = await appStore.search(search);
        } catch (error) {
            notify.notifyError(`Error searching users. ${error.message}`);
        } finally {
            isLoading.value = false;
        }
    }, 500);
}

function goToUser(user: AccountMin):void {
    router.push({ name: ROUTES.Account.name, params: { userID: user.id } });
    model.value = false;
}

function goToProject(project: Project):void {
    router.push({ name: ROUTES.AccountProject.name, params: { projectID: project.id, userID: project.owner.id } });
    model.value = false;
}
</script>
<style scoped lang="scss">
:deep(.v-data-table-footer) {
    background: rgb(var(--v-theme-surface)) !important;
    box-shadow: none !important;
}
</style>
