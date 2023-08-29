// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card variant="flat" :border="true" rounded="xlg">
        <v-text-field
            v-model="search"
            label="Search"
            prepend-inner-icon="mdi-magnify"
            single-line
            variant="solo-filled"
            flat
            hide-details
            clearable
            density="comfortable"
            rounded="lg"
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
            select-strategy="all"
            class="elevation-1"
            show-select
            @update:itemsPerPage="onUpdateLimit"
            @update:page="onUpdatePage"
            @update:sortBy="onUpdateSortBy"
        >
            <template #item.name="{ item }">
                <span class="font-weight-bold">
                    {{ item.raw.name }}
                </span>
            </template>
            <template #item.createdAt="{ item }">
                <span>
                    {{ item.raw.createdAt.toLocaleString() }}
                </span>
            </template>
        </v-data-table-server>
    </v-card>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, computed } from 'vue';
import { VCard, VTextField } from 'vuetify/components';
import { VDataTableServer } from 'vuetify/labs/components';

import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { AccessGrantCursor, AccessGrantsOrderBy, AccessGrantsPage } from '@/types/accessGrants';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useNotify } from '@/utils/hooks';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';
import { SortDirection, tableSizeOptions } from '@/types/common';

const agStore = useAccessGrantsStore();
const projectsStore = useProjectsStore();
const notify = useNotify();

const FIRST_PAGE = 1;
const areGrantsFetching = ref<boolean>(true);
const search = ref<string>('');
const searchTimer = ref<NodeJS.Timeout>();
const selected = ref([]);

const headers = [
    {
        title: 'Name',
        align: 'start',
        key: 'name',
    },
    { title: 'Date Created', key: 'createdAt' },
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
 * Handles update table search.
 */
watch(() => search.value, () => {
    clearTimeout(searchTimer.value);

    searchTimer.value = setTimeout(() => {
        agStore.setSearchQuery(search.value);
        fetch();
    }, 500); // 500ms delay for every new call.
});

onMounted(() => {
    fetch();
});
</script>
