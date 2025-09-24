// Copyright (C) 2023 Storj Labs, Inc.
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
        v-model="selected"
        :headers="headers"
        :items="page.accessGrants"
        :loading="areGrantsFetching"
        :items-length="page.totalCount"
        :items-per-page-options="tableSizeOptions(page.totalCount)"
        :item-value="(item: AccessGrant) => item"
        no-data-text="No results found"
        select-strategy="page"
        hover
        show-select
        @update:items-per-page="onUpdateLimit"
        @update:page="onUpdatePage"
        @update:sort-by="onUpdateSortBy"
    >
        <template #item.name="{ item }">
            <span class="font-weight-medium">
                {{ item.name }}
            </span>
        </template>
        <template #item.creatorEmail="{ item }">
            <span class="font-weight-medium">
                {{ item.creatorEmail }}
            </span>
        </template>
        <template #item.createdAt="{ item }">
            <span class="text-no-wrap">
                {{ Time.formattedDate(item.createdAt) }}
            </span>
        </template>
        <template #item.actions="{ item }">
            <v-btn
                title="Access Actions"
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
                        <v-list-item class="text-error" density="comfortable" link @click="() => onDeleteSingleClick(item)">
                            <template #prepend>
                                <component :is="Trash2" :size="18" />
                            </template>
                            <v-list-item-title class="ml-3 text-body-2 font-weight-medium">
                                Delete Access
                            </v-list-item-title>
                        </v-list-item>
                    </v-list>
                </v-menu>
            </v-btn>
        </template>
    </v-data-table-server>

    <cannot-delete-dialog
        v-model="isCannotDeleteDialogShown"
        :access="accessToDelete"
    />

    <delete-access-dialog
        v-model="isDeleteAccessDialogShown"
        :accesses="accessesToDelete"
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
                {{ selected.length }} key{{ selected.length > 1 ? 's' : '' }} selected
            </v-col>
            <v-col>
                <div class="d-flex justify-end">
                    <v-btn
                        color="error"
                        density="comfortable"
                        variant="outlined"
                        @click="onDeleteMultipleClick"
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
import { ref, watch, onMounted, computed, onBeforeUnmount } from 'vue';
import {
    VBtn,
    VCol,
    VIcon,
    VMenu,
    VList,
    VListItem,
    VListItemTitle,
    VRow,
    VSnackbar,
    VTextField,
    VDataTableServer,
} from 'vuetify/components';
import { Ellipsis, Search, Trash2 } from 'lucide-vue-next';

import { Time } from '@/utils/time';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { AccessGrant, AccessGrantCursor, AccessGrantsOrderBy, AccessGrantsPage } from '@/types/accessGrants';
import { useAccessGrantsStore } from '@/store/modules/accessGrantsStore';
import { useNotify } from '@/composables/useNotify';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';
import { SortDirection, tableSizeOptions, MAX_SEARCH_VALUE_LENGTH, DataTableHeader } from '@/types/common';
import { ProjectRole } from '@/types/projectMembers';
import { useUsersStore } from '@/store/modules/usersStore';

import DeleteAccessDialog from '@/components/dialogs/DeleteAccessDialog.vue';
import CannotDeleteDialog from '@/components/dialogs/CannotDeleteDialog.vue';

const userStore = useUsersStore();
const agStore = useAccessGrantsStore();
const projectsStore = useProjectsStore();
const notify = useNotify();

const FIRST_PAGE = 1;
const areGrantsFetching = ref<boolean>(true);
const search = ref<string>('');
const searchTimer = ref<NodeJS.Timeout>();
const isDeleteAccessDialogShown = ref<boolean>(false);
const isCannotDeleteDialogShown = ref<boolean>(false);
const accessToDelete = ref<AccessGrant | undefined>();
const selected = ref<AccessGrant[]>([]);

const headers = computed<DataTableHeader[]>(() => {
    const hdrs: DataTableHeader[] = [{
        title: 'Access Name',
        align: 'start',
        key: 'name',
    }];

    if (hasOtherMembers.value) {
        hdrs.push({ title: 'Creator', key: 'creatorEmail' });
    }

    hdrs.push(
        { title: 'Date Created', key: 'createdAt' },
        { title: '', key: 'actions', sortable: false, width: 0 },
    );

    return hdrs;
});

const hasOtherMembers = computed<boolean>(() => projectsStore.state.selectedProjectConfig.membersCount > 1);

const userEmail = computed<string>(() => userStore.state.user.email);

const projectRole = computed<ProjectRole>(() => projectsStore.state.selectedProjectConfig.role);

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
const accessesToDelete = computed<AccessGrant[]>(() => {
    if (accessToDelete.value) return [accessToDelete.value];
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
        notify.notifyError(error, AnalyticsErrorEventSource.ACCESS_GRANTS_PAGE);
    }
}

/**
 * Handles update table rows limit event.
 */
function onUpdateLimit(limit: number): void {
    fetch(FIRST_PAGE, limit);
}

/**
 * Handles update table page event.
 */
function onUpdatePage(page: number): void {
    selected.value = [];
    accessToDelete.value = undefined;
    fetch(page, cursor.value.limit);
}

/**
 * Handles update table sorting event.
 */
function onUpdateSortBy(sortBy: { key: keyof AccessGrantsOrderBy, order: keyof SortDirection }[]): void {
    if (!sortBy.length) return;

    const sorting = sortBy[0];

    agStore.setSortingBy(AccessGrantsOrderBy[sorting.key]);
    agStore.setSortingDirection(SortDirection[sorting.order]);

    fetch(FIRST_PAGE, cursor.value.limit);
}

/**
 * Displays the Delete Access dialog.
 */
function onDeleteSingleClick(access: AccessGrant): void {
    accessToDelete.value = access;

    if (projectRole.value === ProjectRole.Member && access.creatorEmail !== userEmail.value) {
        isCannotDeleteDialogShown.value = true;
        return;
    }
    isDeleteAccessDialogShown.value = true;
}

function onDeleteMultipleClick(): void {
    if (projectRole.value === ProjectRole.Member) {
        const restricted = accessesToDelete.value.find(a => a.creatorEmail !== userEmail.value);
        if (restricted) {
            accessToDelete.value = restricted;
            isCannotDeleteDialogShown.value = true;
            return;
        }
    }

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

watch([isDeleteAccessDialogShown, isCannotDeleteDialogShown], ([value0, value1]) => {
    if (!value0 && !value1) accessToDelete.value = undefined;
});

onMounted(() => {
    fetch();
});

onBeforeUnmount(() => {
    agStore.setSearchQuery('');
});
</script>
