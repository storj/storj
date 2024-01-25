// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card variant="outlined" :border="true" rounded="xlg">
        <v-text-field
            v-model="search"
            label="Search"
            :prepend-inner-icon="mdiMagnify"
            single-line
            variant="solo-filled"
            flat
            hide-details
            clearable
            density="comfortable"
            rounded="lg"
            :maxlength="MAX_SEARCH_VALUE_LENGTH"
            class="mx-2 mt-2"
        />

        <v-data-table-server
            v-model="selected"
            :headers="headers"
            :items="page.accessGrants"
            :loading="areGrantsFetching"
            :items-length="page.totalCount"
            :items-per-page-options="tableSizeOptions(page.totalCount)"
            item-value="name"
            no-data-text="No results found"
            select-strategy="page"
            show-select
            @update:itemsPerPage="onUpdateLimit"
            @update:page="onUpdatePage"
            @update:sortBy="onUpdateSortBy"
        >
            <template #item.name="{ item }">
                <span class="font-weight-bold">
                    {{ item.name }}
                </span>
            </template>
            <template #item.createdAt="{ item }">
                <span>
                    {{ item.createdAt.toLocaleString() }}
                </span>
            </template>
            <template #item.actions="{ item }">
                <v-btn
                    variant="outlined"
                    color="default"
                    size="small"
                    class="mr-1 text-caption"
                    density="comfortable"
                    icon
                >
                    <v-icon :icon="mdiDotsHorizontal" />
                    <v-menu activator="parent">
                        <v-list class="pa-1">
                            <v-list-item class="text-error" density="comfortable" link rounded="lg" @click="() => onDeleteClick(item.name)">
                                <template #prepend>
                                    <icon-trash bold />
                                </template>
                                <v-list-item-title class="pl-2 text-body-2 font-weight-medium">
                                    Delete Access
                                </v-list-item-title>
                            </v-list-item>
                        </v-list>
                    </v-menu>
                </v-btn>
            </template>
        </v-data-table-server>
    </v-card>

    <delete-access-dialog
        v-model="isDeleteAccessDialogShown"
        :access-names="accessesToDelete"
        @deleted="() => onUpdatePage(FIRST_PAGE)"
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
                {{ selected.length }} access{{ selected.length > 1 ? 'es' : '' }} selected
            </v-col>
            <v-col>
                <div class="d-flex justify-end">
                    <v-btn
                        color="default"
                        density="comfortable"
                        variant="outlined"
                        @click="isDeleteAccessDialogShown = true"
                    >
                        <template #prepend>
                            <icon-trash bold />
                        </template>
                        Delete
                    </v-btn>
                </div>
            </v-col>
        </v-row>
    </v-snackbar>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, computed, onBeforeUnmount } from 'vue';
import {
    VBtn,
    VCol,
    VIcon,
    VMenu,
    VList,
    VListItem,
    VListItemTitle,
    VCard,
    VRow,
    VSnackbar,
    VTextField,
    VDataTableServer,
} from 'vuetify/components';
import { mdiDotsHorizontal, mdiMagnify } from '@mdi/js';

import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { AccessGrantCursor, AccessGrantsOrderBy, AccessGrantsPage } from '@/types/accessGrants';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useNotify } from '@/utils/hooks';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';
import { SortDirection, tableSizeOptions } from '@/types/common';
import { MAX_SEARCH_VALUE_LENGTH } from '@poc/types/common';

import DeleteAccessDialog from '@poc/components/dialogs/DeleteAccessDialog.vue';
import IconTrash from '@poc/components/icons/IconTrash.vue';

const agStore = useAccessGrantsStore();
const projectsStore = useProjectsStore();
const notify = useNotify();

const FIRST_PAGE = 1;
const areGrantsFetching = ref<boolean>(true);
const search = ref<string>('');
const searchTimer = ref<NodeJS.Timeout>();
const selected = ref([]);
const isDeleteAccessDialogShown = ref<boolean>(false);
const accessNameToDelete = ref<string>('');

const headers = [
    {
        title: 'Access Name',
        align: 'start',
        key: 'name',
    },
    { title: 'Date Created', key: 'createdAt' },
    { title: '', key: 'actions', sortable: false, width: 0 },
];

/**
 * Returns access grants cursor from store.
 */
const cursor = computed((): AccessGrantCursor => {
    return agStore.state.cursor;
});

/**
 * Returns access grants page from store.
 */
const page = computed((): AccessGrantsPage => {
    return agStore.state.page;
});

/**
 * Returns the selected accesses to the delete dialog.
 */
const accessesToDelete = computed<string[]>(() => {
    if (accessNameToDelete.value) return [accessNameToDelete.value];
    return selected.value;
});

/**
 * Fetches Access records depending on page and limit.
 */
async function fetch(page = FIRST_PAGE, limit = DEFAULT_PAGE_LIMIT): Promise<void> {
    try {
        await agStore.getAccessGrants(page, projectsStore.state.selectedProject.id, limit);
        if (areGrantsFetching.value) areGrantsFetching.value = false;
    } catch (error) {
        notify.error(`Unable to fetch Access Grants. ${error.message}`, AnalyticsErrorEventSource.ACCESS_GRANTS_PAGE);
    }
}

/**
 * Handles update table rows limit event.
 */
function onUpdateLimit(limit: number): void {
    fetch(page.value.currentPage, limit);
}

/**
 * Handles update table page event.
 */
function onUpdatePage(page: number): void {
    selected.value = [];
    accessNameToDelete.value = '';
    fetch(page, cursor.value.limit);
}

/**
 * Handles update table sorting event.
 */
function onUpdateSortBy(sortBy: {key: keyof AccessGrantsOrderBy, order: keyof SortDirection}[]): void {
    if (!sortBy.length) return;

    const sorting = sortBy[0];

    agStore.setSortingBy(AccessGrantsOrderBy[sorting.key]);
    agStore.setSortingDirection(SortDirection[sorting.order]);

    fetch(FIRST_PAGE, cursor.value.limit);
}

/**
 * Displays the Delete Access dialog.
 */
function onDeleteClick(accessName: string): void {
    accessNameToDelete.value = accessName;
    isDeleteAccessDialogShown.value = true;
}

/**
 * Handles update table search.
 */
watch(() => search.value, () => {
    clearTimeout(searchTimer.value);

    searchTimer.value = setTimeout(() => {
        agStore.setSearchQuery(search.value || '');
        fetch();
    }, 500); // 500ms delay for every new call.
});

onMounted(() => {
    fetch();
});

onBeforeUnmount(() => {
    agStore.setSearchQuery('');
});
</script>
