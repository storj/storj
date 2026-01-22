// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-dialog v-model="model" :width="dialogWidth" transition="fade-transition">
        <v-card
            title="Find accounts or projects"
            subtitle="Search by ID, email, name, Stripe customer ID, or node operator email"
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
                    :loading="isLoading"
                    :headers="combinedHeaders"
                    :items="combinedResults"
                    :height="combinedResults.length > 10 ? 520 : 'auto'"
                    class="border-0"
                    hide-default-header
                >
                    <template #no-data>
                        {{ searchTerm?.length >= 3 ? 'No results found' : 'Enter a search term of at least 3 characters' }}
                    </template>
                    <template #loading>
                        Searching...
                    </template>
                    <template v-if="combinedResults.length <= 10" #bottom>
                        <v-container class="v-data-table-footer" />
                    </template>
                    <template #top>
                        <v-text-field
                            v-model="searchTerm" label="Search" :prepend-inner-icon="Search" single-line variant="solo-filled" flat
                            hide-details clearable density="compact" rounded="lg" class="mx-4 mb-2"
                            @update:model-value="onSearchChange"
                        />
                    </template>

                    <template #item.identifier="{ item }">
                        <v-btn
                            v-tooltip="item.identifier"
                            variant="outlined"
                            color="default"
                            density="compact"
                            @click="goToResult(item)"
                        >
                            <template #prepend>
                                <v-icon :icon="item.type === 'account' ? User : item.type === 'project' ? Box : Server" />
                            </template>
                            <template #default>
                                <span class="text-truncate" style="max-width: 200px;">{{ item.identifier }}</span>
                            </template>
                        </v-btn>
                    </template>

                    <template #item.name="{ item }">
                        <v-chip v-if="item.name" variant="tonal" color="default" size="small">
                            <span class="text-truncate" style="max-width: 150px;">{{ item.name }}</span>
                        </v-chip>
                    </template>

                    <template #item.type="{ item }">
                        <v-chip
                            :color="item.type === 'account' ? 'info' : item.type === 'project' ? 'success' : 'purple'"
                            variant="tonal" size="small" class="font-weight-medium"
                        >
                            {{ item.type === 'account' ? 'Account' : item.type === 'project' ? 'Project' : 'Node' }}
                        </v-chip>
                    </template>

                    <template #item.status="{ item }">
                        <v-chip
                            :color="getStatusColor(item)"
                            variant="tonal" size="small" class="font-weight-medium"
                        >
                            {{ getStatusText(item) }}
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
import { Box, Search, Server, User, X } from 'lucide-vue-next';
import { useDate, useDisplay } from 'vuetify';

import { AccountMin, NodeMinInfo, Project, SearchResult } from '@/api/client.gen';
import { useAppStore } from '@/store/app';
import { useNotificationsStore } from '@/store/notifications';
import { ROUTES } from '@/router';
import { useLoading } from '@/composables/useLoading';

interface CombinedResult {
    type: 'account' | 'node' | 'project';
    identifier: string;
    name: string;
    createdAt: string;
    original: AccountMin | NodeMinInfo | Project;
    status?: string;
    // Account-specific fields
    kind?: { name: string };
    // Node-specific fields
    online?: boolean;
    disqualified?: boolean;
    // Project-specific fields
    ownerEmail?: string;
    ownerId?: string;
}

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

const combinedResults = computed<CombinedResult[]>(() => {
    const results: CombinedResult[] = [];

    const project = searchResult.value?.project;
    if (project) {
        results.push({
            type: 'project',
            identifier: project.id,
            name: project.name,
            createdAt: project.createdAt,
            original: project,
            status: project.status?.name,
            ownerId: project.owner.id,
        });
    }

    for (const account of searchResult.value?.accounts ?? []) {
        results.push({
            type: 'account',
            identifier: account.email,
            name: account.fullName,
            createdAt: account.createdAt,
            original: account,
            kind: account.kind,
            status: account.status.name,
        });
    }

    for (const node of searchResult.value?.nodes ?? []) {
        results.push({
            type: 'node',
            identifier: node.id,
            name: '',
            createdAt: node.createdAt,
            original: node,
            online: node.online,
            disqualified: node.disqualified,
        });
    }

    return results;
});

const combinedHeaders = [
    { title: 'Identifier', key: 'identifier' },
    { title: 'Name', key: 'name' },
    { title: 'Type', key: 'type' },
    { title: 'Status', key: 'status' },
    { title: 'Date Created', key: 'createdAt' },
];

const dialogWidth = computed(() => {
    if (xlAndUp.value) return 1100;
    if (lgAndUp.value) return 900;
    if (mdAndUp.value) return 750;
    return '';
});

function getStatusColor(item: CombinedResult): string {
    if (item.type === 'node') {
        if (item.disqualified) return 'error';
        return item.online ? 'success' : 'warning';
    }
    if (item.status) {
        const status = item.status.toLowerCase();
        if (status.includes('deletion') || status.includes('deleted')) {
            return 'error';
        }
        if (status.includes('active')) {
            return 'success';
        }
        return 'warning';
    }

    return 'default';
}

function getStatusText(item: CombinedResult): string {
    if (item.type === 'node') {
        if (item.disqualified) return 'Disqualified';
        return item.online ? 'Online' : 'Offline';
    }

    return item.status ?? '_';
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
            notify.notifyError(`Error searching. ${error.message}`);
        } finally {
            isLoading.value = false;
        }
    }, 500);
}

function goToResult(item: CombinedResult): void {
    if (item.type === 'account') {
        const user = item.original as AccountMin;
        router.push({ name: ROUTES.Account.name, params: { userID: user.id } });
    } else if (item.type === 'project') {
        const project = item.original as Project;
        router.push({ name: ROUTES.AccountProject.name, params: { projectID: project.id, userID: project.owner.id } });
    } else {
        const node = item.original as NodeMinInfo;
        router.push({ name: ROUTES.NodeDetail.name, params: { nodeID: node.id } });
    }
    model.value = false;
}
</script>
<style scoped lang="scss">
:deep(.v-data-table-footer) {
    background: rgb(var(--v-theme-surface)) !important;
    box-shadow: none !important;
}
</style>
