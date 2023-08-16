// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card variant="flat" :border="true" class="rounded-xlg">
        <v-text-field
            v-model="search"
            label="Search"
            prepend-inner-icon="mdi-magnify"
            single-line
            hide-details
            clearable
        />

        <v-data-table-server
            v-model="selected"
            :headers="headers"
            :items="page.buckets"
            :search="search"
            :loading="areBucketsFetching"
            :items-length="page.totalCount"
            :items-per-page-options="tableSizeOptions(page.totalCount)"
            item-value="name"
            class="elevation-1"
            @update:itemsPerPage="onUpdateLimit"
            @update:page="onUpdatePage"
        >
            <template #item.name="{ item }">
                <div>
                    <v-btn
                        class="rounded-lg w-100 pl-1 pr-4 justify-start font-weight-bold text-lowercase"
                        variant="text"
                        height="40"
                        color="default"
                        @click="openBucket(item.raw.name)"
                    >
                        <img src="../assets/icon-bucket-tonal.svg" alt="Bucket" class="mr-3">
                        {{ item.raw.name }}
                    </v-btn>
                </div>
            </template>
            <template #item.storage="{ item }">
                <span>
                    {{ item.raw.storage.toFixed(2) + 'GB' }}
                </span>
            </template>
            <template #item.egress="{ item }">
                <span>
                    {{ item.raw.egress.toFixed(2) + 'GB' }}
                </span>
            </template>
            <template #item.objectCount="{ item }">
                <span>
                    {{ item.raw.objectCount.toLocaleString() }}
                </span>
            </template>
            <template #item.segmentCount="{ item }">
                <span>
                    {{ item.raw.segmentCount.toLocaleString() }}
                </span>
            </template>
            <template #item.createdAt="{ item }">
                <span>
                    {{ item.raw.since.toLocaleString() }}
                </span>
            </template>
            <template #item.actions="{ item }">
                <v-menu location="bottom end" transition="scale-transition">
                    <template #activator="{ props: activatorProps }">
                        <v-btn
                            icon="mdi-dots-vertical"
                            color="default"
                            variant="text"
                            size="small"
                            density="comfortable"
                            v-bind="activatorProps"
                        />
                    </template>
                    <v-list class="pa-0">
                        <v-list-item link @click="() => showDeleteBucketDialog(item.raw.name)">
                            <template #prepend>
                                <icon-trash />
                            </template>
                            <v-list-item-title class="text-body-2 ml-3">
                                Delete bucket
                            </v-list-item-title>
                        </v-list-item>
                    </v-list>
                </v-menu>
            </template>
        </v-data-table-server>
    </v-card>
    <delete-bucket-dialog v-model="isDeleteBucketDialogShown" :bucket-name="bucketToDelete" />
</template>

<script setup lang="ts">
import { ref, watch, onMounted, computed } from 'vue';
import { useRouter } from 'vue-router';
import { VCard, VTextField, VBtn, VMenu, VList, VListItem, VListItemTitle } from 'vuetify/components';
import { VDataTableServer } from 'vuetify/labs/components';

import { BucketPage, BucketCursor } from '@/types/buckets';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';
import { tableSizeOptions } from '@/types/common';

import IconTrash from '@poc/components/icons/IconTrash.vue';
import DeleteBucketDialog from '@poc/components/dialogs/DeleteBucketDialog.vue';

const bucketsStore = useBucketsStore();
const projectsStore = useProjectsStore();
const notify = useNotify();
const router = useRouter();

const FIRST_PAGE = 1;
const areBucketsFetching = ref<boolean>(true);
const search = ref<string>('');
const searchTimer = ref<NodeJS.Timeout>();
const selected = ref([]);
const isDeleteBucketDialogShown = ref<boolean>(false);
const bucketToDelete = ref<string>('');

const headers = [
    {
        title: 'Name',
        align: 'start',
        key: 'name',
        sortable: false,
    },
    { title: 'Storage', key: 'storage', sortable: false },
    { title: 'Egress', key: 'egress', sortable: false },
    { title: 'Objects', key: 'objectCount', sortable: false },
    { title: 'Segments', key: 'segmentCount', sortable: false },
    { title: 'Date Created', key: 'createdAt', sortable: false },
    { key: 'actions', width: '0', sortable: false },
];

/**
 * Returns buckets cursor from store.
 */
const cursor = computed((): BucketCursor => {
    return bucketsStore.state.cursor;
});

/**
 * Returns buckets page from store.
 */
const page = computed((): BucketPage => {
    return bucketsStore.state.page;
});

/**
 * Fetches bucket using api.
 */
async function fetchBuckets(page = FIRST_PAGE, limit = DEFAULT_PAGE_LIMIT): Promise<void> {
    try {
        await bucketsStore.getBuckets(page, projectsStore.state.selectedProject.id, limit);
        if (areBucketsFetching.value) areBucketsFetching.value = false;
    } catch (error) {
        notify.error(`Unable to fetch buckets. ${error.message}`, AnalyticsErrorEventSource.BUCKET_TABLE);
    }
}

/**
 * Handles update table rows limit event.
 */
function onUpdateLimit(limit: number): void {
    fetchBuckets(page.value.currentPage, limit);
}

/**
 * Handles update table page event.
 */
function onUpdatePage(page: number): void {
    fetchBuckets(page, cursor.value.limit);
}

/**
 * Handles update table search.
 */
watch(() => search.value, () => {
    clearTimeout(searchTimer.value);

    searchTimer.value = setTimeout(() => {
        bucketsStore.setBucketsSearch(search.value);
        fetchBuckets();
    }, 500); // 500ms delay for every new call.
});

/**
 * Navigates to bucket page.
 */
function openBucket(bucketName: string): void {
    router.push(`/projects/${projectsStore.state.selectedProject.id}/buckets/${bucketName}`);
}

/**
 * Displays the Delete Bucket dialog.
 */
function showDeleteBucketDialog(bucketName: string): void {
    bucketToDelete.value = bucketName;
    isDeleteBucketDialogShown.value = true;
}

onMounted(() => {
    fetchBuckets();
});
</script>
