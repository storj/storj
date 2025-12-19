// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <v-card variant="flat" border rounded="xlg" elevation="0">
        <v-data-table-server
            :headers="headers"
            :items="bucketPage?.items ?? []"
            :search="search"
            :loading="isLoading"
            :items-length="bucketPage?.totalCount ?? 0"
            :items-per-page="pageSize"
            items-per-page-text="Buckets per page"
            no-data-text="No buckets found"
            class="border-0"
            hover
            @update:items-per-page="onUpdateLimit"
            @update:page="onUpdatePage"
        >
            <template #top>
                <v-text-field
                    v-model="search" label="Search" :prepend-inner-icon="Search" single-line variant="solo-filled" flat
                    hide-details clearable density="compact" rounded="lg" class="mx-2 mt-2 mb-2"
                />
            </template>

            <template #item.name="{ item }">
                <div class="text-no-wrap">
                    <v-btn
                        variant="outlined" color="default" size="small" class="mr-1 text-caption" density="comfortable" icon
                        width="24" height="24"
                    >
                        <BucketActionsMenu
                            :bucket="item"
                            @update="bucket => {
                                bucketToUpdate = bucket;
                                updateBucketDialog = true;
                            }"
                        />
                        <v-icon :icon="MoreHorizontal" />
                    </v-btn>
                    <v-chip variant="text" size="small" class="font-weight-bold pl-1 ml-1">
                        <template #prepend>
                            <svg class="mr-2" width="24" height="24" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
                                <rect width="32" height="32" rx="10" />
                                <rect x="0.5" y="0.5" width="31" height="31" rx="7.5" stroke="currentColor" stroke-opacity="0.2" />
                                <path
                                    d="M23.3549 14.5467C24.762 15.9538 24.7748 18.227 23.3778 19.624C22.8117 20.1901 22.1018 20.5247 21.3639 20.6289L21.1106 22.0638C21.092 23.1897 18.7878 24.1 15.9481 24.1C13.1254 24.1 10.8319 23.2006 10.7863 22.0841L10.7858 22.0638L8.84466 11.066C8.82281 10.9882 8.80883 10.9095 8.80304 10.8299L8.8 10.8122L8.8019 10.8123C8.80063 10.7903 8.8 10.7682 8.8 10.746C8.8 9.17422 12.0003 7.90002 15.9481 7.90002C19.8959 7.90002 23.0962 9.17422 23.0962 10.746C23.0962 10.7682 23.0955 10.7903 23.0943 10.8123L23.0962 10.8122L23.093 10.8311C23.0872 10.9098 23.0734 10.9876 23.0519 11.0645L22.5749 13.7666L23.3549 14.5467ZM21.2962 12.6344C19.9867 13.2218 18.076 13.592 15.9481 13.592C13.8203 13.592 11.9096 13.2219 10.6001 12.6344L12.0072 20.6077L12.2373 21.8286L12.2586 21.8452C12.3789 21.9354 12.5652 22.0371 12.807 22.1351L12.8561 22.1546C13.6355 22.4594 14.7462 22.6439 15.9481 22.6439C17.1569 22.6439 18.2733 22.4573 19.0528 22.1497C19.3337 22.0388 19.5431 21.9223 19.6661 21.8231L19.6761 21.8148L19.9019 20.5348C19.3338 20.3787 18.7955 20.0812 18.3429 19.6429L18.3004 19.6011L15.3749 16.6756C15.0906 16.3913 15.0906 15.9303 15.3749 15.646C15.6523 15.3686 16.0978 15.3618 16.3834 15.6257L16.4045 15.646L19.33 18.5715C19.5717 18.8132 19.8555 18.9861 20.1569 19.0901L21.2962 12.6344ZM22.2661 15.517L21.6408 19.0597C21.8989 18.9575 22.1402 18.8024 22.3482 18.5944C23.1641 17.7784 23.1664 16.4494 22.355 15.6065L22.3253 15.5763L22.2661 15.517ZM15.9481 9.35612C14.2013 9.35612 12.5813 9.62893 11.4322 10.0864C10.9385 10.283 10.5712 10.4995 10.3598 10.6985C10.3463 10.7112 10.334 10.7232 10.3228 10.7347L10.3122 10.7459L10.3314 10.7661L10.3598 10.7936C10.5712 10.9926 10.9385 11.2091 11.4322 11.4056C12.5813 11.8631 14.2013 12.1359 15.9481 12.1359C17.6949 12.1359 19.3149 11.8631 20.4639 11.4056C20.9577 11.2091 21.325 10.9926 21.5364 10.7936C21.5499 10.7809 21.5622 10.7688 21.5733 10.7574L21.5841 10.7459L21.5647 10.726L21.5364 10.6985C21.325 10.4995 20.9577 10.283 20.4639 10.0864C19.3149 9.62893 17.6949 9.35612 15.9481 9.35612Z"
                                    fill="currentColor"
                                />
                            </svg>
                        </template>
                        {{ item.name }}
                    </v-chip>
                </div>
            </template>
            <template #item.placement="{ item }">
                <v-chip variant="tonal" color="default" size="small" rounded="lg" class="text-capitalize">
                    {{ item.placement }}
                </v-chip>
            </template>
        </v-data-table-server>
    </v-card>

    <BucketUpdateDialog
        v-if="bucketToUpdate"
        v-model="updateBucketDialog"
        :bucket="bucketToUpdate"
        :project="props.project"
        @updated="onUpdated"
    />
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from 'vue';
import { VBtn, VCard, VChip, VDataTableServer, VIcon, VTextField } from 'vuetify/components';
import { MoreHorizontal, Search } from 'lucide-vue-next';
import { useDate } from 'vuetify';

import { DataTableHeader } from '@/types/common';
import { BucketInfo, BucketInfoPage, Project } from '@/api/client.gen';
import { useLoading } from '@/composables/useLoading';
import { useNotify } from '@/composables/useNotify';
import { Memory, Size } from '@/utils/bytesSize';
import { useBucketsStore } from '@/store/buckets';

import BucketActionsMenu from '@/components/BucketActionsMenu.vue';
import BucketUpdateDialog from '@/components/BucketUpdateDialog.vue';

const bucketsStore = useBucketsStore();

const date = useDate();
const { isLoading, withLoading } = useLoading();
const notify = useNotify();

const props = defineProps<{
    project: Project;
}>();

const search = ref<string>('');
const pageSize = ref<number>(10);
const searchTimer = ref<NodeJS.Timeout>();
const bucketPage = ref<BucketInfoPage>();
const updateBucketDialog = ref<boolean>(false);
const bucketToUpdate = ref<BucketInfo>();

const headers: DataTableHeader[] = [
    { title: 'Bucket', key: 'name' },
    {
        title: 'Storage', key: 'storage',
        value: item => Size.toBase10String((item as { storage: number }).storage * Memory.GB),
    },
    {
        title: 'Download', key: 'egress',
        value: item => Size.toBase10String((item as { egress: number }).egress * Memory.GB),
    },
    { title: 'Segments', key: 'segmentCount' },
    { title: 'Placement', key: 'placement' },
    { title: 'User Agent', key: 'userAgent', maxWidth: '300' },
    {
        title: 'Created', key: 'createdAt',
        value: item => date.format(date.date((item as { createdAt: string }).createdAt), 'fullDate'),
    },
];

function onUpdated(bucket: BucketInfo) {
    if (!bucketPage.value?.items) return;
    const index = bucketPage.value?.items?.findIndex(b => b.name === bucket.name) ?? -1;
    if (index === -1) return;
    const items = bucketPage.value.items;
    items[index] = bucket;
    bucketPage.value = { ...bucketPage.value, items };
}

/**
 * Handles update table rows limit event.
 */
function onUpdateLimit(limit: number): void {
    pageSize.value = limit;
    fetchBuckets(1, limit);
}

/**
 * Handles update table page event.
 */
function onUpdatePage(page: number): void {
    fetchBuckets(page, pageSize.value);
}

/**
 * Fetches bucket using api.
 */
function fetchBuckets(page = 1, limit = 10): void {
    withLoading(async () => {
        try {
            bucketPage.value = await bucketsStore.getBuckets(props.project.id, { search: search.value, page, limit });
        } catch (error) {
            notify.error(`Failed to fetch buckets: ${error.message}`);
        }
    });
}

/**
 * Handles update table search.
 */
watch(search, () => {
    clearTimeout(searchTimer.value);

    searchTimer.value = setTimeout(() => {
        fetchBuckets();
    }, 500); // 500ms delay for every new call.
});

watch(updateBucketDialog, async (newVal) => {
    if (newVal) return;

    // wait for dialog to close
    await new Promise(resolve => setTimeout(resolve, 300));
    bucketToUpdate.value = undefined;
});

onMounted(() => {
    fetchBuckets();
});
</script>
