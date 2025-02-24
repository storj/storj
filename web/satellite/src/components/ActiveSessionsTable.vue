// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card variant="outlined">
        <v-data-table-server
            :headers="headers"
            :items="page.sessions"
            :loading="isFetching || isLoading"
            :items-length="page.totalCount"
            :items-per-page-options="tableSizeOptions(page.totalCount)"
            :item-value="(item: Session) => item"
            no-data-text="No results found"
            class="elevation-0 border-0"
            @update:items-per-page="onUpdateLimit"
            @update:page="onUpdatePage"
            @update:sort-by="onUpdateSortBy"
        >
            <template #item.isCurrent="{ item }">
                <v-chip :color="item.isCurrent ? 'success' : 'primary'" class="font-weight-bold" size="small" label>
                    {{ item.isCurrent ? 'Yes' : 'No' }}
                </v-chip>
            </template>
            <template #item.expiresAt="{ item }">
                <span class="text-no-wrap">
                    {{ Time.formattedDate(item.expiresAt) }}
                </span>
            </template>
            <template #item.actions="{ item }">
                <div class="text-no-wrap">
                    <v-btn
                        variant="outlined"
                        color="default"
                        size="small"
                        class="mr-1 text-caption"
                        :loading="isLoading"
                        :prepend-icon="LogOut"
                        @click="() => onInvalidate(item)"
                    >
                        {{ item.isCurrent ? 'Logout' : 'Invalidate' }}
                    </v-btn>
                </div>
            </template>
        </v-data-table-server>
    </v-card>
</template>

<script setup lang="ts">
import { VBtn, VCard, VDataTableServer, VChip } from 'vuetify/components';
import { computed, onMounted, ref } from 'vue';
import { LogOut  } from 'lucide-vue-next';

import { Session, SessionsCursor, SessionsOrderBy, SessionsPage } from '@/types/users';
import { useNotify } from '@/composables/useNotify';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { DataTableHeader, SortDirection, tableSizeOptions } from '@/types/common';
import { useLoading } from '@/composables/useLoading';
import { Time } from '@/utils/time';
import { useUsersStore } from '@/store/modules/usersStore';
import { useLogout } from '@/composables/useLogout';

const usersStore = useUsersStore();

const { logout } = useLogout();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const FIRST_PAGE = 1;
const headers: DataTableHeader[] = [
    {
        title: 'User Agent',
        align: 'start',
        key: 'userAgent',
    },
    { title: 'Current', key: 'isCurrent', sortable: false },
    { title: 'Expires', key: 'expiresAt' },
    { title: '', key: 'actions', align: 'end' },
];

const isFetching = ref<boolean>(true);

const page = computed<SessionsPage>(() => usersStore.state.sessionsPage);
const cursor = computed<SessionsCursor>(() => usersStore.state.sessionsCursor);

async function fetch(page = FIRST_PAGE, limit = DEFAULT_PAGE_LIMIT): Promise<void> {
    isFetching.value = false;

    try {
        await usersStore.getSessions(page, limit);
    } catch (error) {
        notify.notifyError(error, AnalyticsErrorEventSource.ACCOUNT_SETTINGS_AREA);
    }

    isFetching.value = false;
}

function onUpdateLimit(limit: number): void {
    fetch(FIRST_PAGE, limit);
}

function onUpdatePage(page: number): void {
    fetch(page, cursor.value.limit);
}

function onUpdateSortBy(sortBy: { key: keyof SessionsOrderBy, order: keyof SortDirection }[]): void {
    if (!sortBy.length) return;

    const sorting = sortBy[0];

    usersStore.setSessionsSortingBy(SessionsOrderBy[sorting.key]);
    usersStore.setSessionsSortingDirection(SortDirection[sorting.order]);

    fetch(FIRST_PAGE, cursor.value.limit);
}

async function onInvalidate(session: Session): Promise<void> {
    await withLoading(async () => {
        try {
            if (session.isCurrent) {
                await logout();
            } else {
                await usersStore.invalidateSession(session.id);
                await fetch(cursor.value.page, cursor.value.limit);
            }
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.ACCOUNT_SETTINGS_AREA);
        }
    });
}

onMounted(() => {
    fetch();
});
</script>
