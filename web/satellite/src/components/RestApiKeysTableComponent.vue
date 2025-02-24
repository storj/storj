// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card class="pa-4">
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
            :maxlength="MAX_SEARCH_VALUE_LENGTH"
            class="mb-5"
        />
        <v-data-table
            v-model="selected"
            :headers="headers"
            :search="search"
            :items="keys"
            :loading="isLoading"
            :item-value="(item: RestApiKey) => item"
            no-data-text="No keys found"
            hover
            show-select
            hide-default-footer
        >
            <template #item.createdAt="{ item }">
                <span class="text-no-wrap">
                    {{ Time.formattedDate(item.createdAt) }}
                </span>
            </template>
            <template #item.expiresAt="{ item }">
                <span class="text-no-wrap">
                    {{ !item.expiresAt ? 'No Expiration' : Time.formattedDate(item.expiresAt) }}
                </span>
            </template>
            <template #item.actions="{ item }">
                <v-btn
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
                            <v-list-item class="text-error" density="comfortable" link @click="() => onDeleteClick(item)">
                                <template #prepend>
                                    <component :is="Trash2" :size="18" />
                                </template>
                                <v-list-item-title class="ml-3 text-body-2 font-weight-medium">
                                    Delete API Key
                                </v-list-item-title>
                            </v-list-item>
                        </v-list>
                    </v-menu>
                </v-btn>
            </template>
        </v-data-table>
    </v-card>

    <delete-rest-api-dialog
        v-model="isDeleteDialogShown"
        :keys="keysToDelete"
        @deleted="fetch"
    />

    <v-snackbar
        rounded="lg"
        variant="elevated"
        color="surface"
        :model-value="!!selected.length"
        :timeout="-1"
        class="snackbar-multiple"
    >
        <v-row align="center" justify="space-between">
            <v-col>
                {{ selected.length }} key{{ selected.length > 1 ? 's' : '' }} selected
            </v-col>
            <v-col>
                <div class="d-flex justify-end">
                    <v-btn
                        color="error"
                        density="comfortable"
                        variant="outlined"
                        @click="isDeleteDialogShown = true"
                    >
                        <template #prepend>
                            <component :is="Trash2" :size="18" />
                        </template>
                        Delete
                    </v-btn>
                </div>
            </v-col>
        </v-row>
    </v-snackbar>
</template>

<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue';
import {
    VBtn,
    VCol,
    VDataTable,
    VIcon,
    VMenu,
    VList,
    VListItem,
    VListItemTitle,
    VCard,
    VRow,
    VSnackbar,
    VTextField,
} from 'vuetify/components';
import { Ellipsis, Search, Trash2 } from 'lucide-vue-next';

import { Time } from '@/utils/time';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/composables/useNotify';
import { DataTableHeader, MAX_SEARCH_VALUE_LENGTH } from '@/types/common';
import { useRestApiKeysStore } from '@/store/modules/apiKeysStore';
import { RestApiKey } from '@/types/restApiKeys';
import { useLoading } from '@/composables/useLoading';

import DeleteRestApiDialog from '@/components/dialogs/DeleteRestApiKeyDialog.vue';

const apiKeyStore = useRestApiKeysStore();
const notify = useNotify();
const { withLoading, isLoading } = useLoading();

const search = ref<string>('');
const isDeleteDialogShown = ref<boolean>(false);
const keyToDelete = ref<RestApiKey | undefined>();
const selected = ref<RestApiKey[]>([]);

const headers: DataTableHeader[] = [
    { title: 'Name', key: 'name', sortable: false },
    { title: 'Created', key: 'createdAt', sortable: false },
    { title: 'Expires', key: 'expiresAt', sortable: false },
    { title: '', key: 'actions', sortable: false, width: 0 },
];

/**
 * Returns REST API keys from store.
 */
const keys = computed((): RestApiKey[] => {
    return apiKeyStore.state.keys;
});

/**
 * Returns the selected keys to the delete dialog.
 */
const keysToDelete = computed<RestApiKey[]>(() => {
    if (keyToDelete.value) return [keyToDelete.value];
    return selected.value;
});

/**
 * Fetches keys.
 */
function fetch(): void {
    selected.value = [];
    withLoading(async () => {
        try {
            await apiKeyStore.getKeys();
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.API_KEYS_PAGE);
        }
    });
}

/**
 * Displays the Delete key dialog.
 */
function onDeleteClick(apiKey: RestApiKey): void {
    keyToDelete.value = apiKey;
    isDeleteDialogShown.value = true;
}

watch(isDeleteDialogShown, (shown) => {
    if (!shown) keyToDelete.value = undefined;
});

onMounted(() => {
    fetch();
});
</script>
