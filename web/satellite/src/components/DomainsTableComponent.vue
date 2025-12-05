// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
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

    <v-data-table-server
        :headers="headers"
        :items="page.domains"
        :loading="isLoading"
        :items-length="page.totalCount"
        :items-per-page-options="tableSizeOptions(page.totalCount)"
        :item-value="(item: Domain) => item"
        no-data-text="No results found"
        @update:items-per-page="onUpdateLimit"
        @update:page="onUpdatePage"
        @update:sort-by="onUpdateSortBy"
    >
        <template #item.name="{ item }">
            <v-list-item class="font-weight-bold pl-0">
                {{ item.name }}
            </v-list-item>
        </template>
        <template #item.createdAt="{ item }">
            <span class="text-no-wrap">
                {{ Time.formattedDate(item.createdAt) }}
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
                                Delete Domain
                            </v-list-item-title>
                        </v-list-item>
                    </v-list>
                </v-menu>
            </v-btn>
        </template>
    </v-data-table-server>

    <delete-domain-dialog
        v-model="isDeleteDomainDialogShown"
        :domain-name="domainToDelete"
        @deleted="() => onUpdatePage(FIRST_PAGE)"
    />
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import {
    VDataTableServer,
    VTextField,
    VListItem,
    VMenu,
    VBtn,
    VListItemTitle,
    VList,
    VIcon,
} from 'vuetify/components';
import { Ellipsis, Search, Trash2 } from 'lucide-vue-next';

import { Domain, DomainsCursor, DomainsOrderBy, DomainsPage } from '@/types/domains';
import { useDomainsStore } from '@/store/modules/domainsStore';
import { Time } from '@/utils/time';
import { DataTableHeader, MAX_SEARCH_VALUE_LENGTH, SortDirection, tableSizeOptions } from '@/types/common';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';

import DeleteDomainDialog from '@/components/dialogs/DeleteDomainDialog.vue';

const FIRST_PAGE = 1;
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const domainsStore = useDomainsStore();

const search = ref<string>('');
const searchTimer = ref<NodeJS.Timeout>();
const isDeleteDomainDialogShown = ref<boolean>(false);
const domainToDelete = ref<string | undefined>();

const headers: DataTableHeader[] = [
    {
        title: 'Name',
        align: 'start',
        key: 'name',
    },
    { title: 'Date Created', key: 'createdAt' },
    { title: '', key: 'actions', sortable: false, width: 0 },
];

const page = computed<DomainsPage>(() => domainsStore.state.page);
const cursor = computed<DomainsCursor>(() => domainsStore.state.cursor);

function fetch(page = FIRST_PAGE, limit = DEFAULT_PAGE_LIMIT): void {
    withLoading(async () => {
        try {
            await domainsStore.fetchDomains(page, limit);
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.ACCESS_GRANTS_PAGE);
        }
    });
}

function onUpdateLimit(limit: number): void {
    fetch(FIRST_PAGE, limit);
}

function onUpdatePage(page: number): void {
    domainToDelete.value = undefined;
    fetch(page, cursor.value.limit);
}

function onUpdateSortBy(sortBy: { key: keyof DomainsOrderBy, order: keyof SortDirection }[]): void {
    if (!sortBy.length) return;

    const sorting = sortBy[0];

    domainsStore.setSortingBy(DomainsOrderBy[sorting.key]);
    domainsStore.setSortingDirection(SortDirection[sorting.order]);

    fetch(FIRST_PAGE, cursor.value.limit);
}

function onDeleteClick(domain: Domain): void {
    domainToDelete.value = domain.name;
    isDeleteDomainDialogShown.value = true;
}

watch(() => search.value, () => {
    clearTimeout(searchTimer.value);

    searchTimer.value = setTimeout(() => {
        domainsStore.setSearchQuery(search.value || '');
        fetch();
    }, 500); // 500ms delay for every new call.
});

onMounted(() => {
    fetch();
});

onBeforeUnmount(() => {
    domainsStore.setSearchQuery('');
});
</script>
