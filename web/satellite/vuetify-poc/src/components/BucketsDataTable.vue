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
            :sort-by="sortBy"
            :headers="headers"
            :items="displayedItems"
            :search="search"
            :loading="areBucketsFetching"
            :items-length="page.totalCount"
            items-per-page-text="Buckets per page"
            :items-per-page-options="tableSizeOptions(page.totalCount)"
            no-data-text="No buckets found"
            hover
            @update:itemsPerPage="onUpdateLimit"
            @update:page="onUpdatePage"
            @update:sortBy="onUpdateSort"
        >
            <template #item.name="{ item }">
                <v-btn
                    class="rounded-lg w-100 px-1 ml-n1 justify-start"
                    variant="text"
                    height="40"
                    color="default"
                    @click="openBucket(item.name)"
                >
                    <template #default>
                        <img class="mr-3" src="../assets/icon-bucket-tonal.svg" alt="Bucket">
                        <div class="max-width">
                            <p class="font-weight-bold text-lowercase white-space">{{ item.name }}</p>
                        </div>
                    </template>
                </v-btn>
            </template>
            <template #item.storage="{ item }">
                <span>
                    {{ item.storage.toFixed(2) + 'GB' }}
                </span>
            </template>
            <template #item.egress="{ item }">
                <span>
                    {{ item.egress.toFixed(2) + 'GB' }}
                </span>
            </template>
            <template #item.objectCount="{ item }">
                <span>
                    {{ item.objectCount.toLocaleString() }}
                </span>
            </template>
            <template #item.segmentCount="{ item }">
                <span>
                    {{ item.segmentCount.toLocaleString() }}
                </span>
            </template>
            <template #item.since="{ item }">
                <span>
                    {{ item.since.toLocaleString() }}
                </span>
            </template>
            <template #item.actions="{ item }">
                <v-menu location="bottom end" transition="scale-transition">
                    <template #activator="{ props: activatorProps }">
                        <v-btn
                            title="Bucket Actions"
                            :icon="mdiDotsHorizontal"
                            color="default"
                            variant="outlined"
                            size="small"
                            density="comfortable"
                            v-bind="activatorProps"
                        />
                    </template>
                    <v-list class="pa-1">
                        <v-list-item rounded-lg link @click="() => showBucketDetailsModal(item.name)">
                            <template #prepend>
                                <icon-bucket size="18" />
                            </template>
                            <v-list-item-title class="ml-3">
                                Bucket Details
                            </v-list-item-title>
                        </v-list-item>
                        <v-list-item rounded-lg link @click="() => showShareBucketDialog(item.name)">
                            <template #prepend>
                                <icon-share size="18" />
                            </template>
                            <v-list-item-title class="ml-3">
                                Share Bucket
                            </v-list-item-title>
                        </v-list-item>
                        <v-divider class="my-1" />
                        <v-list-item rounded-lg class="text-error text-body-2" link @click="() => showDeleteBucketDialog(item.name)">
                            <template #prepend>
                                <icon-trash />
                            </template>
                            <v-list-item-title class="ml-3">
                                Delete Bucket
                            </v-list-item-title>
                        </v-list-item>
                    </v-list>
                </v-menu>
            </template>
        </v-data-table-server>
    </v-card>
    <delete-bucket-dialog v-model="isDeleteBucketDialogShown" :bucket-name="bucketToDelete" />
    <enter-bucket-passphrase-dialog v-model="isBucketPassphraseDialogOpen" @passphraseEntered="passphraseDialogCallback" />
    <share-dialog v-model="isShareBucketDialogShown" :bucket-name="shareBucketName" />
    <bucket-details-dialog v-model="isBucketDetailsDialogShown" :bucket-name="bucketDetailsName" />
</template>

<script setup lang="ts">
import { ref, watch, onMounted, computed, onBeforeUnmount } from 'vue';
import { useRouter } from 'vue-router';
import {
    VCard,
    VTextField,
    VBtn,
    VMenu,
    VList,
    VListItem,
    VListItemTitle,
    VDivider,
    VDataTableServer,
} from 'vuetify/components';
import { mdiDotsHorizontal, mdiMagnify } from '@mdi/js';

import { BucketPage, BucketCursor, Bucket } from '@/types/buckets';
import { useBucketsStore } from '@/store/modules/bucketsStore';
import { useNotify } from '@/utils/hooks';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { useProjectsStore } from '@/store/modules/projectsStore';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';
import { tableSizeOptions } from '@/types/common';
import { RouteConfig } from '@/types/router';
import { EdgeCredentials } from '@/types/accessGrants';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { MAX_SEARCH_VALUE_LENGTH } from '@poc/types/common';

import IconTrash from '@poc/components/icons/IconTrash.vue';
import IconShare from '@poc/components/icons/IconShare.vue';
import IconBucket from '@poc/components/icons/IconBucket.vue';
import DeleteBucketDialog from '@poc/components/dialogs/DeleteBucketDialog.vue';
import EnterBucketPassphraseDialog from '@poc/components/dialogs/EnterBucketPassphraseDialog.vue';
import ShareDialog from '@poc/components/dialogs/ShareDialog.vue';
import BucketDetailsDialog from '@poc/components/dialogs/BucketDetailsDialog.vue';

const analyticsStore = useAnalyticsStore();
const bucketsStore = useBucketsStore();
const projectsStore = useProjectsStore();
const notify = useNotify();
const router = useRouter();

const FIRST_PAGE = 1;
const areBucketsFetching = ref<boolean>(true);
const search = ref<string>('');
const searchTimer = ref<NodeJS.Timeout>();
const bucketDetailsName = ref<string>('');
const shareBucketName = ref<string>('');
const isDeleteBucketDialogShown = ref<boolean>(false);
const bucketToDelete = ref<string>('');
const isBucketPassphraseDialogOpen = ref(false);
const isShareBucketDialogShown = ref<boolean>(false);
const isBucketDetailsDialogShown = ref<boolean>(false);
const pageWidth = ref<number>(document.body.clientWidth);
const sortBy = ref<SortItem[] | undefined>([{ key: 'name', order: 'asc' }]);

let passphraseDialogCallback: () => void = () => {};

type SortItem = {
    key: keyof Bucket;
    order: boolean | 'asc' | 'desc';
}

type DataTableHeader = {
    key: string;
    title: string;
    align?: 'start' | 'end' | 'center';
    sortable?: boolean;
    width?: number | string;
}

const displayedItems = computed<Bucket[]>(() => {
    const items = page.value.buckets;

    sort(items, sortBy.value);

    return items;
});

const isTableSortable = computed<boolean>(() => {
    return page.value.totalCount <= cursor.value.limit;
});

const headers = computed<DataTableHeader[]>(() => {
    const hdrs: DataTableHeader[] = [
        {
            title: 'Bucket',
            align: 'start',
            key: 'name',
            sortable: isTableSortable.value,
        },
        { title: 'Files', key: 'objectCount', sortable: isTableSortable.value },
        { title: 'Segments', key: 'segmentCount', sortable: isTableSortable.value },
        { title: 'Storage', key: 'storage', sortable: isTableSortable.value },
        { title: 'Download', key: 'egress', sortable: isTableSortable.value },
        { title: 'Date Created', key: 'since', sortable: isTableSortable.value },
        { title: '', key: 'actions', width: '0', sortable: false },
    ];

    if (pageWidth.value <= 1400) {
        ['segmentCount', 'objectCount'].forEach((key) => {
            const index = hdrs.findIndex((el) => el.key === key);
            if (index !== -1) hdrs.splice(index, 1);
        });
    }

    if (pageWidth.value <= 1280) {
        ['storage', 'egress'].forEach((key) => {
            const index = hdrs.findIndex((el) => el.key === key);
            if (index !== -1) hdrs.splice(index, 1);
        });
    }

    if (pageWidth.value <= 780) {
        const index = hdrs.findIndex((el) => el.key === 'createdAt');
        if (index !== -1) hdrs.splice(index, 1);
    }

    return hdrs;
});

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
 * Returns condition if user has to be prompt for passphrase from store.
 */
const promptForPassphrase = computed((): boolean => {
    return bucketsStore.state.promptForPassphrase;
});

/**
 * Returns selected bucket's name from store.
 */
const selectedBucketName = computed((): string => {
    return bucketsStore.state.fileComponentBucketName;
});

/**
 * Returns edge credentials from store.
 */
const edgeCredentials = computed((): EdgeCredentials => {
    return bucketsStore.state.edgeCredentials;
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
 * Sorts items by provided sort options.
 * We use this to correctly sort columns by value of correct type.
 * @param items
 * @param sortOptions
 */
function sort(items: Bucket[], sortOptions: SortItem[] | undefined): void {
    if (!(sortOptions && sortOptions.length)) {
        items.sort((a, b) => a.name.localeCompare(b.name));
        return;
    }

    const option = sortOptions[0];

    switch (option.key) {
    case 'egress':
        items.sort((a, b) => option.order === 'asc' ? a.egress - b.egress : b.egress - a.egress);
        break;
    case 'storage':
        items.sort((a, b) => option.order === 'asc' ? a.storage - b.storage : b.storage - a.storage);
        break;
    case 'objectCount':
        items.sort((a, b) => option.order === 'asc' ? a.objectCount - b.objectCount : b.objectCount - a.objectCount);
        break;
    case 'segmentCount':
        items.sort((a, b) => option.order === 'asc' ? a.segmentCount - b.segmentCount : b.segmentCount - a.segmentCount);
        break;
    case 'since':
        items.sort((a, b) => option.order === 'asc' ? a.since.getTime() - b.since.getTime() : b.since.getTime() - a.since.getTime());
        break;
    default:
        items.sort((a, b) => option.order === 'asc' ? a.name.localeCompare(b.name) : b.name.localeCompare(a.name));
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
 * Handles update table sorting event.
 */
function onUpdateSort(value: SortItem[]): void {
    sortBy.value = value;
}

/**
 * Navigates to bucket page.
 */
async function openBucket(bucketName: string): Promise<void> {
    if (!bucketName) {
        return;
    }
    bucketsStore.setFileComponentBucketName(bucketName);
    if (!promptForPassphrase.value) {
        if (!edgeCredentials.value.accessKeyId) {
            try {
                await bucketsStore.setS3Client(projectsStore.state.selectedProject.id);
            } catch (error) {
                notify.notifyError(error, AnalyticsErrorEventSource.BUCKET_TABLE);
                return;
            }
        }

        analyticsStore.pageVisit(RouteConfig.Buckets.with(RouteConfig.UploadFile).path);
        router.push(`/projects/${projectsStore.state.selectedProject.urlId}/buckets/${bucketsStore.state.fileComponentBucketName}`);
        return;
    }
    passphraseDialogCallback = () => openBucket(selectedBucketName.value);
    isBucketPassphraseDialogOpen.value = true;
}

/**
 * Displays the Bucket Details dialog.
 */
function showBucketDetailsModal(bucketName: string): void {
    bucketDetailsName.value = bucketName;
    isBucketDetailsDialogShown.value = true;
}

/**
 * Displays the Delete Bucket dialog.
 */
function showDeleteBucketDialog(bucketName: string): void {
    bucketToDelete.value = bucketName;
    isDeleteBucketDialogShown.value = true;
}

/**
 * Displays the Share Bucket dialog.
 */
function showShareBucketDialog(bucketName: string): void {
    shareBucketName.value = bucketName;
    if (promptForPassphrase.value) {
        bucketsStore.setFileComponentBucketName(bucketName);
        isBucketPassphraseDialogOpen.value = true;
        passphraseDialogCallback = () => isShareBucketDialogShown.value = true;
        return;
    }
    isShareBucketDialogShown.value = true;
}

/**
 * Handles page width change.
 */
function resizeHandler(): void {
    pageWidth.value = document.body.clientWidth;
}

/**
 * Handles update table search.
 */
watch(() => search.value, () => {
    clearTimeout(searchTimer.value);

    searchTimer.value = setTimeout(() => {
        bucketsStore.setBucketsSearch(search.value || '');
        fetchBuckets();
    }, 500); // 500ms delay for every new call.
});

watch(() => page.value.totalCount, () => {
    sortBy.value = [{ key: 'name', order: 'asc' }];
});

onMounted(() => {
    window.addEventListener('resize', resizeHandler);

    fetchBuckets();
});

onBeforeUnmount(() => {
    window.removeEventListener('resize', resizeHandler);
});
</script>

<style scoped lang="scss">
.max-width {
    max-width: 500px;

    @media screen and (width <= 780px) {
        max-width: 450px;
    }

    @media screen and (width <= 620px) {
        max-width: 350px;
    }

    @media screen and (width <= 490px) {
        max-width: 250px;
    }

    @media screen and (width <= 385px) {
        max-width: 185px;
    }
}

.white-space {
    white-space: break-spaces;
    text-align: left;

    @media screen and (width <= 780px) {
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }
}
</style>
