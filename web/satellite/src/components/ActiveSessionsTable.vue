// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card variant="outlined" border rounded="xlg">
        <v-data-table-server
            :headers="headers"
            :items="page.sessions"
            :loading="isFetching || isLoading"
            :items-length="page.totalCount"
            :items-per-page-options="tableSizeOptions(page.totalCount)"
            :item-value="(item: Session) => item"
            no-data-text="No results found"
            @update:itemsPerPage="onUpdateLimit"
            @update:page="onUpdatePage"
            @update:sortBy="onUpdateSortBy"
        >
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
                        @click="() => onLogout(item.id)"
                    >
                        Logout
                    </v-btn>
                </div>
            </template>
        </v-data-table-server>
    </v-card>
</template>

<script setup lang="ts">
import { VBtn, VCard, VDataTableServer } from 'vuetify/components';
import { computed, onMounted, ref } from 'vue';

import { Session, SessionsCursor, SessionsOrderBy, SessionsPage } from '@/types/users';
import { useUsersStore } from '@/store/modules/usersStore';
import { useNotify } from '@/utils/hooks';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { SortDirection, tableSizeOptions } from '@/types/common';
import { useLoading } from '@/composables/useLoading';
import { Time } from '@/utils/time';

const usersStore = useUsersStore();

const notify = useNotify();
const { isLoading, withLoading } = useLoading();

const FIRST_PAGE = 1;
const headers = [
    {
        title: 'User Agent',
        align: 'start',
        key: 'userAgent',
    },
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
        notify.error(`Unable to fetch Active Sessions. ${error.message}`, AnalyticsErrorEventSource.ACCOUNT_SETTINGS_AREA);
    }

    isFetching.value = false;
}

function onUpdateLimit(limit: number): void {
    fetch(page.value.currentPage, limit);
}

function onUpdatePage(page: number): void {
    fetch(page, cursor.value.limit);
}

function onUpdateSortBy(sortBy: {key: keyof SessionsOrderBy, order: keyof SortDirection}[]): void {
    if (!sortBy.length) return;

    const sorting = sortBy[0];

    usersStore.setSessionsSortingBy(SessionsOrderBy[sorting.key]);
    usersStore.setSessionsSortingDirection(SortDirection[sorting.order]);

    fetch(FIRST_PAGE, cursor.value.limit);
}

async function onLogout(sessionID: string): Promise<void> {
    await withLoading(async () => {
        try {
            // Invalidate session
        } catch (error) {
            notify.error(`Unable to invalidate session. ${error.message}`, AnalyticsErrorEventSource.ACCOUNT_SETTINGS_AREA);
        }
    });
}

onMounted(() => {
    fetch();
});
</script>
